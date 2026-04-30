package model

import "time"

// HostInfo contains information about the host where the scan was performed.
type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// FSInfo contains filesystem information for the scanned mount.
type FSInfo struct {
	MountPoint string `json:"mount_point"`
	Device     string `json:"device"`
	FSType     string `json:"fs_type"`
	TotalBytes int64  `json:"total_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
}

// ScanOptions configures how the scan is performed.
type ScanOptions struct {
	SizeMode       SizeMode `json:"size_mode"`
	OneFilesystem  bool     `json:"one_filesystem"`
	FollowSymlinks bool     `json:"follow_symlinks"`
	Excludes       []string `json:"excludes,omitempty"`
	MinSizeBytes   int64    `json:"min_size_bytes"`
	Threads        int      `json:"threads"` // 0 = auto
}

// DefaultScanOptions returns sensible defaults.
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		SizeMode:       SizeAllocated,
		OneFilesystem:  true,
		FollowSymlinks: false,
		Threads:        0, // Auto-detect
	}
}

// ScanError represents an error encountered during scanning.
type ScanError struct {
	Path string `json:"path"`
	Op   string `json:"op"`  // "readdir", "stat", "open", etc.
	Err  string `json:"err"` // Error message (string for portability)
}

// ScanStats contains aggregate statistics about the scan.
type ScanStats struct {
	Nodes   int64         `json:"nodes"`
	Files   int64         `json:"files"`
	Dirs    int64         `json:"dirs"`
	Skipped int64         `json:"skipped"`
	Errors  int64         `json:"errors"`
	Elapsed time.Duration `json:"elapsed_ms"`
}

// Scan represents a complete scan result.
type Scan struct {
	ID        string    `json:"id"`
	RootPath  string    `json:"root_path"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Host  HostInfo    `json:"host"`
	FS    FSInfo      `json:"fs"`
	Opts  ScanOptions `json:"options"`
	Stats ScanStats   `json:"stats"`

	RootID NodeID           `json:"root_id"`
	Nodes  map[NodeID]*Node `json:"-"` // Excluded from JSON for separate handling

	Errors []ScanError `json:"errors,omitempty"`
}

// NewScan creates a new scan with the given root path.
func NewScan(rootPath string, opts ScanOptions) *Scan {
	return &Scan{
		RootPath:  rootPath,
		StartedAt: time.Now(),
		Opts:      opts,
		Nodes:     make(map[NodeID]*Node),
	}
}

// GetNode returns the node with the given ID, or nil if not found.
func (s *Scan) GetNode(id NodeID) *Node {
	return s.Nodes[id]
}

// Root returns the root node of the scan.
func (s *Scan) Root() *Node {
	return s.Nodes[s.RootID]
}

// TotalSize returns the total size of the scan in the configured mode.
func (s *Scan) TotalSize() int64 {
	root := s.Root()
	if root == nil {
		return 0
	}
	return root.Size(s.Opts.SizeMode)
}
