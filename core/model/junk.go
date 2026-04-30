package model

import "time"

// SafetyLevel indicates how safe it is to delete a junk candidate.
type SafetyLevel uint8

const (
	SafetyUnknown SafetyLevel = iota
	SafetySafe                // node_modules, build dirs - regenerable
	SafetyCaution             // caches - usually safe but verify
	SafetyDanger              // logs, containers - may contain important data
)

func (s SafetyLevel) String() string {
	switch s {
	case SafetySafe:
		return "safe"
	case SafetyCaution:
		return "caution"
	case SafetyDanger:
		return "danger"
	default:
		return "unknown"
	}
}

// Symbol returns an icon/symbol for the safety level.
func (s SafetyLevel) Symbol() string {
	switch s {
	case SafetySafe:
		return "✓"
	case SafetyCaution:
		return "⚠"
	case SafetyDanger:
		return "⛔"
	default:
		return "?"
	}
}

// JunkProfile defines a set of junk detection rules.
type JunkProfile uint8

const (
	JunkProfileAuto   JunkProfile = iota // Detect based on context
	JunkProfileDev                       // Developer-focused (node_modules, build, etc.)
	JunkProfileSystem                    // System caches and logs
	JunkProfileTrash                     // Trash/recycle bin contents
	JunkProfileAll                       // Everything
)

func (p JunkProfile) String() string {
	switch p {
	case JunkProfileDev:
		return "dev"
	case JunkProfileSystem:
		return "system"
	case JunkProfileTrash:
		return "trash"
	case JunkProfileAll:
		return "all"
	default:
		return "auto"
	}
}

// JunkCandidate represents a detected junk item.
type JunkCandidate struct {
	NodeID NodeID `json:"node_id"`
	Path   string `json:"path"`

	BytesAlloc int64 `json:"bytes_alloc"`
	BytesApp   int64 `json:"bytes_app"`

	ReasonID   string      `json:"reason_id"`   // e.g., "dev:node_modules"
	ReasonText string      `json:"reason_text"` // User-facing description
	Safety     SafetyLevel `json:"safety"`

	// Recommended non-destructive action (better UX than raw delete)
	SuggestedCommand string `json:"suggested_command,omitempty"`
}

// JunkOptions configures junk detection.
type JunkOptions struct {
	Profile    JunkProfile `json:"profile"`
	Aggressive bool        `json:"aggressive"` // Include cautionary items
}

// DefaultJunkOptions returns sensible defaults.
func DefaultJunkOptions() JunkOptions {
	return JunkOptions{
		Profile:    JunkProfileAuto,
		Aggressive: false,
	}
}

// JunkReport contains the results of junk detection.
type JunkReport struct {
	ScanID   string `json:"scan_id"`
	RootPath string `json:"root_path"`

	Candidates []JunkCandidate `json:"candidates"`

	TotalBytesAlloc int64 `json:"total_bytes_alloc"`
	TotalBytesApp   int64 `json:"total_bytes_app"`

	// Breakdown by safety level
	SafeBytes    int64 `json:"safe_bytes"`
	CautionBytes int64 `json:"caution_bytes"`
	DangerBytes  int64 `json:"danger_bytes"`

	GeneratedAt time.Time `json:"generated_at"`
}

// Bytes returns size based on mode.
func (j *JunkCandidate) Bytes(mode SizeMode) int64 {
	if mode == SizeAllocated {
		return j.BytesAlloc
	}
	return j.BytesApp
}

// FilterBySafety returns candidates at or below the given safety level.
func (r *JunkReport) FilterBySafety(maxLevel SafetyLevel) []JunkCandidate {
	result := make([]JunkCandidate, 0)
	for _, c := range r.Candidates {
		if c.Safety <= maxLevel {
			result = append(result, c)
		}
	}
	return result
}
