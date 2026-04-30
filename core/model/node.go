// Package model contains the core data structures for zenbox disk.
// These types are used across CLI and TUI and have no UI dependencies.
package model

import "time"

// NodeID is a unique identifier for a node in the scan tree.
type NodeID uint32

// NodeKind represents the type of filesystem entry.
type NodeKind uint8

const (
	KindUnknown NodeKind = iota
	KindDir
	KindFile
	KindSymlink
	KindOther
)

func (k NodeKind) String() string {
	switch k {
	case KindDir:
		return "dir"
	case KindFile:
		return "file"
	case KindSymlink:
		return "symlink"
	default:
		return "unknown"
	}
}

// SizeMode determines which size metric to display.
type SizeMode uint8

const (
	SizeAllocated SizeMode = iota // Actual disk blocks consumed
	SizeApparent                  // Logical file size
)

func (s SizeMode) String() string {
	if s == SizeAllocated {
		return "allocated"
	}
	return "apparent"
}

// FileIdentity uniquely identifies a file for hard-link detection.
type FileIdentity struct {
	Dev   uint64
	Inode uint64
}

// NodeFlags contains boolean markers for special node states.
type NodeFlags struct {
	IsHardlink      bool // This is a subsequent occurrence of a hard-linked file
	IsDuplicate     bool // Annotated post-scan: file has duplicates
	IsJunkCandidate bool // Annotated post-scan: matches junk patterns
	HasErrors       bool // Errors occurred reading this node
	IsCrossMount    bool // This node is on a different filesystem
}

// NodeMeta contains file metadata from stat.
type NodeMeta struct {
	Mode  uint32 // Portable subset of os.FileMode
	UID   uint32
	GID   uint32
	MTime time.Time
	Ident FileIdentity
}

// NodeStats contains size and count statistics.
type NodeStats struct {
	SizeAlloc int64 // Allocated size (blocks * 512)
	SizeApp   int64 // Apparent size (logical)

	// For directories: aggregated totals including all children
	TotalAlloc int64
	TotalApp   int64

	FileCount int64 // Number of files (recursive for dirs)
	DirCount  int64 // Number of subdirs (recursive for dirs)

	// Top K children by size - for "explain this" without recomputing
	LargestChildIDs []NodeID
}

// Node represents a single filesystem entry in the scan tree.
type Node struct {
	ID     NodeID
	Parent NodeID

	Name string // Basename
	Path string // Absolute path

	Kind  NodeKind
	Depth uint16

	Meta     NodeMeta
	Stats    NodeStats
	Flags    NodeFlags
	Category CategoryGuess

	// Children contains IDs of child nodes (directories only).
	// May be nil until expanded in lazy loading mode.
	Children []NodeID
}

// Size returns the size based on the given mode.
func (n *Node) Size(mode SizeMode) int64 {
	if n.Kind == KindDir {
		if mode == SizeAllocated {
			return n.Stats.TotalAlloc
		}
		return n.Stats.TotalApp
	}
	if mode == SizeAllocated {
		return n.Stats.SizeAlloc
	}
	return n.Stats.SizeApp
}

// IsDir returns true if this node is a directory.
func (n *Node) IsDir() bool {
	return n.Kind == KindDir
}
