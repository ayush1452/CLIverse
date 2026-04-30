package model

// Category represents a classification bucket for disk usage.
type Category uint8

const (
	CatUnknown Category = iota
	CatApplications
	CatDocuments
	CatPhotos
	CatMusic
	CatVideos
	CatDeveloper
	CatSystemData
	CatOther
)

func (c Category) String() string {
	switch c {
	case CatApplications:
		return "Applications"
	case CatDocuments:
		return "Documents"
	case CatPhotos:
		return "Photos"
	case CatMusic:
		return "Music"
	case CatVideos:
		return "Videos"
	case CatDeveloper:
		return "Developer"
	case CatSystemData:
		return "System Data"
	case CatOther:
		return "Other"
	default:
		return "Unknown"
	}
}

// AllCategories returns all defined categories in display order.
func AllCategories() []Category {
	return []Category{
		CatApplications,
		CatDeveloper,
		CatDocuments,
		CatPhotos,
		CatMusic,
		CatVideos,
		CatSystemData,
		CatOther,
	}
}

// Confidence represents how confident we are in a classification (0-100).
type Confidence uint8

const (
	ConfidenceLow    Confidence = 25
	ConfidenceMedium Confidence = 50
	ConfidenceHigh   Confidence = 75
	ConfidenceExact  Confidence = 100
)

// CategoryGuess represents a category classification with confidence.
type CategoryGuess struct {
	Cat     Category   `json:"category"`
	Conf    Confidence `json:"confidence"`
	RuleIDs []string   `json:"rule_ids,omitempty"` // e.g., "dev:node_modules"
}

// CategoryStat holds aggregate statistics for a category.
type CategoryStat struct {
	Category   Category   `json:"category"`
	BytesAlloc int64      `json:"bytes_alloc"`
	BytesApp   int64      `json:"bytes_app"`
	FileCount  int64      `json:"file_count"`
	DirCount   int64      `json:"dir_count"`
	TopNodeIDs []NodeID   `json:"top_node_ids,omitempty"` // Top contributors
	ConfAvg    Confidence `json:"confidence_avg"`
}

// Overview provides a macOS Storage-like breakdown by category.
type Overview struct {
	ScanID   string   `json:"scan_id"`
	SizeMode SizeMode `json:"size_mode"`

	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`

	Categories []CategoryStat `json:"categories"` // Sorted by size desc

	// Top offenders for quick insights
	BiggestDir      string `json:"biggest_dir,omitempty"`
	BiggestFile     string `json:"biggest_file,omitempty"`
	BiggestDirSize  int64  `json:"biggest_dir_size,omitempty"`
	BiggestFileSize int64  `json:"biggest_file_size,omitempty"`
}

// Bytes returns the size for a category based on the mode.
func (cs *CategoryStat) Bytes(mode SizeMode) int64 {
	if mode == SizeAllocated {
		return cs.BytesAlloc
	}
	return cs.BytesApp
}
