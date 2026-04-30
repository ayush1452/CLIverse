package model

import "time"

// HashAlg specifies the hashing algorithm used for duplicate detection.
type HashAlg uint8

const (
	HashBlake3 HashAlg = iota
	HashSHA256
	HashXXH3
)

func (h HashAlg) String() string {
	switch h {
	case HashBlake3:
		return "blake3"
	case HashSHA256:
		return "sha256"
	case HashXXH3:
		return "xxh3"
	default:
		return "unknown"
	}
}

// DuplicateFile represents a single file in a duplicate group.
type DuplicateFile struct {
	NodeID    NodeID       `json:"node_id"`
	Path      string       `json:"path"`
	SizeAlloc int64        `json:"size_alloc"`
	SizeApp   int64        `json:"size_app"`
	MTime     time.Time    `json:"mtime"`
	Ident     FileIdentity `json:"-"` // For internal use
}

// DuplicateGroup represents a set of identical files.
type DuplicateGroup struct {
	GroupID string `json:"group_id"`

	SizeAlloc int64 `json:"size_alloc"` // Size of one file
	SizeApp   int64 `json:"size_app"`

	HashAlg HashAlg `json:"hash_alg"`
	HashHex string  `json:"hash_hex"`

	Files []DuplicateFile `json:"files"`

	// UX helpers
	KeepIndex  int   `json:"keep_index"`  // Which file to keep (index into Files)
	WasteAlloc int64 `json:"waste_alloc"` // Sum of all files except kept
	WasteApp   int64 `json:"waste_app"`
}

// KeepRule determines which duplicate to keep in a group.
type KeepRule uint8

const (
	KeepFirst  KeepRule = iota // First found (default)
	KeepNewest                 // Most recent mtime
	KeepOldest                 // Oldest mtime
	KeepPath                   // Matches a path prefix
)

func (k KeepRule) String() string {
	switch k {
	case KeepNewest:
		return "newest"
	case KeepOldest:
		return "oldest"
	case KeepPath:
		return "path"
	default:
		return "first"
	}
}

// DuplicateOptions configures duplicate detection.
type DuplicateOptions struct {
	MinSizeBytes int64    `json:"min_size_bytes"`
	HashAlg      HashAlg  `json:"hash_alg"`
	PartialBytes int64    `json:"partial_bytes"` // Bytes for partial hash stage
	VerifyBytes  bool     `json:"verify_bytes"`  // Byte-by-byte verify
	KeepRule     KeepRule `json:"keep_rule"`
	KeepPrefix   string   `json:"keep_prefix,omitempty"` // For KeepPath rule
}

// DefaultDuplicateOptions returns sensible defaults.
func DefaultDuplicateOptions() DuplicateOptions {
	return DuplicateOptions{
		MinSizeBytes: 1 << 20, // 1MB
		HashAlg:      HashBlake3,
		PartialBytes: 4 << 20, // 4MB
		VerifyBytes:  false,
		KeepRule:     KeepFirst,
	}
}

// DuplicateReport contains the results of duplicate detection.
type DuplicateReport struct {
	ScanID   string `json:"scan_id"`
	RootPath string `json:"root_path"`

	Groups []DuplicateGroup `json:"groups"`

	TotalGroups     int64 `json:"total_groups"`
	TotalFiles      int64 `json:"total_files"`       // Total duplicate files (including originals)
	TotalWasteAlloc int64 `json:"total_waste_alloc"` // Space that could be reclaimed
	TotalWasteApp   int64 `json:"total_waste_app"`

	GeneratedAt time.Time `json:"generated_at"`
}

// WastedBytes returns wasted bytes based on mode.
func (d *DuplicateGroup) WastedBytes(mode SizeMode) int64 {
	if mode == SizeAllocated {
		return d.WasteAlloc
	}
	return d.WasteApp
}

// Duplicates returns all files except the kept one.
func (d *DuplicateGroup) Duplicates() []DuplicateFile {
	result := make([]DuplicateFile, 0, len(d.Files)-1)
	for i, f := range d.Files {
		if i != d.KeepIndex {
			result = append(result, f)
		}
	}
	return result
}

// Keeper returns the file that should be kept.
func (d *DuplicateGroup) Keeper() DuplicateFile {
	if d.KeepIndex >= 0 && d.KeepIndex < len(d.Files) {
		return d.Files[d.KeepIndex]
	}
	return d.Files[0]
}
