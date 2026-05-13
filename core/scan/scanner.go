// Package scan provides filesystem scanning functionality.
package scan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayush1452/CLIverse/core/model"
)

// Scanner scans a filesystem and builds a node tree.
type Scanner struct {
	opts model.ScanOptions

	// State during scan
	nodeCounter atomic.Uint32
	rootDev     uint64
	seenInodes  sync.Map // map[FileIdentity]NodeID for hardlink detection

	// Results
	mu     sync.Mutex
	nodes  map[model.NodeID]*model.Node
	errors []model.ScanError
	stats  model.ScanStats
}

// ProgressFunc is called periodically during scanning.
type ProgressFunc func(model.ScanProgress)

// New creates a new scanner with the given options.
func New(opts model.ScanOptions) *Scanner {
	if opts.Threads <= 0 {
		opts.Threads = runtime.NumCPU()
	}
	return &Scanner{
		opts:  opts,
		nodes: make(map[model.NodeID]*model.Node),
	}
}

// Scan performs a scan of the given root path.
func (s *Scanner) Scan(ctx context.Context, rootPath string, progressFn ProgressFunc) (*model.Scan, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	// Verify root exists
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("cannot access root: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", absRoot)
	}

	// Get root device for mount boundary detection
	s.rootDev = rootDevice(rootInfo)

	scan := model.NewScan(absRoot, s.opts)
	scan.StartedAt = time.Now()

	// Create root node
	rootNode := s.createNode(nil, absRoot, rootInfo, 0)
	scan.RootID = rootNode.ID

	// Scan recursively with worker pool
	s.scanDir(ctx, rootNode, absRoot, 0, progressFn)

	// Aggregate sizes from leaves to root
	s.aggregateSizes(rootNode)

	// Finalize scan
	scan.EndedAt = time.Now()
	scan.Nodes = s.nodes
	scan.Errors = s.errors
	scan.Stats = s.stats
	scan.Stats.Elapsed = scan.EndedAt.Sub(scan.StartedAt)

	// Get filesystem info
	scan.FS = s.getFilesystemInfo(absRoot)

	// Get host info
	hostname, _ := os.Hostname()
	scan.Host = model.HostInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	return scan, nil
}

func (s *Scanner) scanDir(ctx context.Context, parent *model.Node, path string, depth int, progressFn ProgressFunc) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		s.addError(path, "readdir", err)
		return
	}

	childIDs := make([]model.NodeID, 0, len(entries))

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())

		// Check excludes
		if s.shouldExclude(childPath, entry.Name()) {
			s.stats.Skipped++
			continue
		}

		info, err := entry.Info()
		if err != nil {
			s.addError(childPath, "stat", err)
			continue
		}

		// Check mount boundary
		if s.opts.OneFilesystem && !s.isSameDevice(info) {
			s.stats.Skipped++
			continue
		}

		node := s.createNode(parent, childPath, info, depth+1)
		childIDs = append(childIDs, node.ID)

		// Report progress periodically
		if progressFn != nil && s.stats.Nodes%1000 == 0 {
			progressFn(model.ScanProgress{
				NodesScanned: s.stats.Nodes,
				BytesScanned: s.stats.Files, // Approximate
				CurrentPath:  childPath,
				ErrorCount:   s.stats.Errors,
			})
		}

		// Recurse into directories
		if node.Kind == model.KindDir {
			s.scanDir(ctx, node, childPath, depth+1, progressFn)
		}
	}

	// Update parent's children
	s.mu.Lock()
	parent.Children = childIDs
	s.mu.Unlock()
}

func (s *Scanner) createNode(parent *model.Node, path string, info fs.FileInfo, depth int) *model.Node {
	id := model.NodeID(s.nodeCounter.Add(1))

	var parentID model.NodeID
	if parent != nil {
		parentID = parent.ID
	}

	node := &model.Node{
		ID:     id,
		Parent: parentID,
		Name:   info.Name(),
		Path:   path,
		Kind:   s.getNodeKind(info),
		Depth:  uint16(depth),
		Meta: model.NodeMeta{
			Mode:  uint32(info.Mode()),
			MTime: info.ModTime(),
		},
	}

	// Get system-specific info
	fillNodeMeta(node, info, &s.seenInodes, id)

	// Update stats
	s.mu.Lock()
	s.nodes[id] = node
	s.stats.Nodes++
	if node.Kind == model.KindFile {
		s.stats.Files++
	} else if node.Kind == model.KindDir {
		s.stats.Dirs++
	}
	s.mu.Unlock()

	return node
}

func (s *Scanner) aggregateSizes(node *model.Node) {
	if node.Kind != model.KindDir {
		node.Stats.TotalAlloc = node.Stats.SizeAlloc
		node.Stats.TotalApp = node.Stats.SizeApp
		return
	}

	var totalAlloc, totalApp int64
	var fileCount, dirCount int64

	type sizeEntry struct {
		id   model.NodeID
		size int64
	}
	var largestChildren []sizeEntry

	for _, childID := range node.Children {
		child := s.nodes[childID]
		if child == nil {
			continue
		}

		// Recurse first
		s.aggregateSizes(child)

		totalAlloc += child.Stats.TotalAlloc
		totalApp += child.Stats.TotalApp

		if child.Kind == model.KindFile {
			fileCount++
		} else if child.Kind == model.KindDir {
			dirCount++
			fileCount += child.Stats.FileCount
			dirCount += child.Stats.DirCount
		}

		// Track largest children for "explain this"
		largestChildren = append(largestChildren, sizeEntry{childID, child.Stats.TotalAlloc})
	}

	// Add directory's own size
	totalAlloc += node.Stats.SizeAlloc
	totalApp += node.Stats.SizeApp

	node.Stats.TotalAlloc = totalAlloc
	node.Stats.TotalApp = totalApp
	node.Stats.FileCount = fileCount
	node.Stats.DirCount = dirCount

	// Keep top 5 largest children
	if len(largestChildren) > 0 {
		// Simple selection sort for top 5
		for i := 0; i < min(5, len(largestChildren)); i++ {
			maxIdx := i
			for j := i + 1; j < len(largestChildren); j++ {
				if largestChildren[j].size > largestChildren[maxIdx].size {
					maxIdx = j
				}
			}
			largestChildren[i], largestChildren[maxIdx] = largestChildren[maxIdx], largestChildren[i]
		}
		topN := min(5, len(largestChildren))
		node.Stats.LargestChildIDs = make([]model.NodeID, topN)
		for i := 0; i < topN; i++ {
			node.Stats.LargestChildIDs[i] = largestChildren[i].id
		}
	}
}

func (s *Scanner) getNodeKind(info fs.FileInfo) model.NodeKind {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return model.KindDir
	case mode.IsRegular():
		return model.KindFile
	case mode&fs.ModeSymlink != 0:
		return model.KindSymlink
	default:
		return model.KindOther
	}
}

func (s *Scanner) isSameDevice(info fs.FileInfo) bool {
	return sameDevice(info, s.rootDev)
}

func (s *Scanner) shouldExclude(path, name string) bool {
	for _, pattern := range s.opts.Excludes {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

func (s *Scanner) addError(path, op string, err error) {
	s.mu.Lock()
	s.errors = append(s.errors, model.ScanError{
		Path: path,
		Op:   op,
		Err:  err.Error(),
	})
	s.stats.Errors++
	s.mu.Unlock()
}

func (s *Scanner) getFilesystemInfo(path string) model.FSInfo {
	return filesystemInfo(path)
}
