// Package disk serves an interactive browser dashboard for disk scans.
package disk

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ayush1452/CLIverse/core/model"
)

//go:embed frontend
var frontendFiles embed.FS

// Server exposes a browser dashboard for a single disk scan.
type Server struct {
	scan     *model.Scan
	overview *model.Overview
	port     int
}

// NewServer allocates a new dashboard server on a random local port.
func NewServer(scan *model.Scan, overview *model.Overview) (*Server, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	return &Server{scan: scan, overview: overview, port: port}, nil
}

// URL returns the dashboard base URL.
func (s *Server) URL() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

// Start runs the HTTP server until it is interrupted.
func (s *Server) Start() error {
	sub, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/report", s.handleReport)
	mux.HandleFunc("/api/reveal", s.handleReveal)

	return http.ListenAndServe(fmt.Sprintf("localhost:%d", s.port), mux)
}

type dashboardData struct {
	RootPath      string           `json:"root_path"`
	SizeMode      string           `json:"size_mode"`
	StartedAt     time.Time        `json:"started_at"`
	EndedAt       time.Time        `json:"ended_at"`
	ElapsedMillis int64            `json:"elapsed_ms"`
	Host          model.HostInfo   `json:"host"`
	FS            model.FSInfo     `json:"fs"`
	Stats         model.ScanStats  `json:"stats"`
	RootSize      int64            `json:"root_size"`
	UsedBytes     int64            `json:"used_bytes"`
	FreeBytes     int64            `json:"free_bytes"`
	CapacityPct   float64          `json:"capacity_pct"`
	ScanSharePct  float64          `json:"scan_share_pct"`
	Categories    []categoryDatum  `json:"categories"`
	RootChildren  []entryDatum     `json:"root_children"`
	LargestDirs   []entryDatum     `json:"largest_dirs"`
	LargestFiles  []entryDatum     `json:"largest_files"`
	RecentFiles   []entryDatum     `json:"recent_files"`
	Extensions    []extensionDatum `json:"extensions"`
}

type categoryDatum struct {
	Name    string  `json:"name"`
	Bytes   int64   `json:"bytes"`
	Percent float64 `json:"percent"`
	Files   int64   `json:"files"`
	Dirs    int64   `json:"dirs"`
}

type entryDatum struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	Category   string    `json:"category"`
	Size       int64     `json:"size"`
	Percent    float64   `json:"percent"`
	FileCount  int64     `json:"file_count,omitempty"`
	DirCount   int64     `json:"dir_count,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
}

type extensionDatum struct {
	Ext     string  `json:"ext"`
	Bytes   int64   `json:"bytes"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

func (s *Server) handleReport(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(s.buildDashboardData()) //nolint:errcheck
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	// Restrict to scanned root to prevent path traversal
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.scan.RootPath)) {
		http.Error(w, "path outside scan root", http.StatusForbidden)
		return
	}
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", "-R", path).Start() //nolint:errcheck
	case "linux":
		exec.Command("xdg-open", filepath.Dir(path)).Start() //nolint:errcheck
	case "windows":
		exec.Command("explorer", "/select,"+path).Start() //nolint:errcheck
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) buildDashboardData() dashboardData {
	root := s.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(s.scan.Opts.SizeMode)
	}

	capacityPct := 0.0
	if s.overview.TotalBytes > 0 {
		capacityPct = float64(s.overview.UsedBytes) / float64(s.overview.TotalBytes) * 100
	}

	scanSharePct := 0.0
	if s.overview.TotalBytes > 0 {
		scanSharePct = float64(rootSize) / float64(s.overview.TotalBytes) * 100
	}

	return dashboardData{
		RootPath:      s.scan.RootPath,
		SizeMode:      s.scan.Opts.SizeMode.String(),
		StartedAt:     s.scan.StartedAt,
		EndedAt:       s.scan.EndedAt,
		ElapsedMillis: int64(s.scan.Stats.Elapsed / time.Millisecond),
		Host:          s.scan.Host,
		FS:            s.scan.FS,
		Stats:         s.scan.Stats,
		RootSize:      rootSize,
		UsedBytes:     s.overview.UsedBytes,
		FreeBytes:     s.overview.FreeBytes,
		CapacityPct:   capacityPct,
		ScanSharePct:  scanSharePct,
		Categories:    buildCategories(s.overview),
		RootChildren:  buildRootChildren(s.scan, root, 10),
		LargestDirs:   buildLargestNodes(s.scan, model.KindDir, 10),
		LargestFiles:  buildLargestNodes(s.scan, model.KindFile, 12),
		RecentFiles:   buildRecentFiles(s.scan, 10),
		Extensions:    buildExtensions(s.scan, 8),
	}
}

func buildCategories(overview *model.Overview) []categoryDatum {
	total := float64(0)
	for _, cat := range overview.Categories {
		total += float64(cat.Bytes(overview.SizeMode))
	}

	out := make([]categoryDatum, 0, len(overview.Categories))
	for _, cat := range overview.Categories {
		bytes := cat.Bytes(overview.SizeMode)
		pct := 0.0
		if total > 0 {
			pct = float64(bytes) / total * 100
		}
		out = append(out, categoryDatum{
			Name:    cat.Category.String(),
			Bytes:   bytes,
			Percent: pct,
			Files:   cat.FileCount,
			Dirs:    cat.DirCount,
		})
	}
	return out
}

func buildRootChildren(scan *model.Scan, root *model.Node, limit int) []entryDatum {
	if root == nil {
		return nil
	}

	nodes := make([]*model.Node, 0, len(root.Children))
	for _, childID := range root.Children {
		if child := scan.GetNode(childID); child != nil {
			nodes = append(nodes, child)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Size(scan.Opts.SizeMode) > nodes[j].Size(scan.Opts.SizeMode)
	})
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	return encodeEntries(scan, nodes, root.Size(scan.Opts.SizeMode))
}

func buildLargestNodes(scan *model.Scan, kind model.NodeKind, limit int) []entryDatum {
	nodes := make([]*model.Node, 0, len(scan.Nodes))
	for _, node := range scan.Nodes {
		if node == nil || node.ID == scan.RootID || node.Kind != kind {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Size(scan.Opts.SizeMode) > nodes[j].Size(scan.Opts.SizeMode)
	})
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	rootTotal := int64(0)
	if root := scan.Root(); root != nil {
		rootTotal = root.Size(scan.Opts.SizeMode)
	}
	return encodeEntries(scan, nodes, rootTotal)
}

func buildRecentFiles(scan *model.Scan, limit int) []entryDatum {
	nodes := make([]*model.Node, 0, len(scan.Nodes))
	for _, node := range scan.Nodes {
		if node == nil || node.Kind != model.KindFile {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.MTime.After(nodes[j].Meta.MTime)
	})
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	rootTotal := int64(0)
	if root := scan.Root(); root != nil {
		rootTotal = root.Size(scan.Opts.SizeMode)
	}
	return encodeEntries(scan, nodes, rootTotal)
}

func buildExtensions(scan *model.Scan, limit int) []extensionDatum {
	type stat struct {
		bytes int64
		count int64
	}

	stats := make(map[string]stat)
	total := int64(0)
	for _, node := range scan.Nodes {
		if node == nil || node.Kind != model.KindFile {
			continue
		}
		ext := strings.ToLower(filepath.Ext(node.Name))
		if ext == "" {
			ext = "[none]"
		}
		size := node.Size(scan.Opts.SizeMode)
		item := stats[ext]
		item.bytes += size
		item.count++
		stats[ext] = item
		total += size
	}

	entries := make([]extensionDatum, 0, len(stats))
	for ext, item := range stats {
		pct := 0.0
		if total > 0 {
			pct = float64(item.bytes) / float64(total) * 100
		}
		entries = append(entries, extensionDatum{
			Ext:     ext,
			Bytes:   item.bytes,
			Count:   item.count,
			Percent: pct,
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Bytes > entries[j].Bytes })
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func encodeEntries(scan *model.Scan, nodes []*model.Node, total int64) []entryDatum {
	out := make([]entryDatum, 0, len(nodes))
	for _, node := range nodes {
		pct := 0.0
		if total > 0 {
			pct = float64(node.Size(scan.Opts.SizeMode)) / float64(total) * 100
		}
		out = append(out, entryDatum{
			Name:       node.Name,
			Path:       node.Path,
			Kind:       node.Kind.String(),
			Category:   node.Category.Cat.String(),
			Size:       node.Size(scan.Opts.SizeMode),
			Percent:    pct,
			FileCount:  node.Stats.FileCount,
			DirCount:   node.Stats.DirCount,
			ModifiedAt: node.Meta.MTime,
		})
	}
	return out
}

// OpenBrowser opens a URL in the system default browser.
func OpenBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "linux":
		cmd, args = "xdg-open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", "", url}
	default:
		return
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
