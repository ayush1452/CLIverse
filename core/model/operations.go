package model

import "time"

// OpType specifies the type of operation to perform.
type OpType uint8

const (
	OpTrash    OpType = iota // Move to system trash (recoverable)
	OpDelete                 // Permanent deletion
	OpHardlink               // Replace with hardlink
	OpReflink                // Replace with reflink (CoW)
)

func (o OpType) String() string {
	switch o {
	case OpTrash:
		return "trash"
	case OpDelete:
		return "delete"
	case OpHardlink:
		return "hardlink"
	case OpReflink:
		return "reflink"
	default:
		return "unknown"
	}
}

// IsDestructive returns true if the operation permanently removes data.
func (o OpType) IsDestructive() bool {
	return o == OpDelete
}

// OpTarget represents a single target for an operation.
type OpTarget struct {
	Path   string `json:"path"`
	NodeID NodeID `json:"node_id"`
	Size   int64  `json:"size"` // For progress/summary
}

// OpRequest represents a requested batch operation.
type OpRequest struct {
	Type    OpType     `json:"type"`
	Targets []OpTarget `json:"targets"`

	DryRun    bool `json:"dry_run"`
	Confirmed bool `json:"confirmed"` // User explicitly confirmed
}

// TotalSize returns the total size of all targets.
func (r *OpRequest) TotalSize() int64 {
	var total int64
	for _, t := range r.Targets {
		total += t.Size
	}
	return total
}

// OpResultStatus indicates the outcome of a single operation.
type OpResultStatus uint8

const (
	OpResultPending OpResultStatus = iota
	OpResultSuccess
	OpResultSkipped
	OpResultFailed
)

// OpResult represents the result of a single target operation.
type OpResult struct {
	Path   string         `json:"path"`
	Status OpResultStatus `json:"status"`
	Err    string         `json:"error,omitempty"`
}

// Operation represents a completed (or in-progress) batch operation.
type Operation struct {
	ID      string    `json:"id"`
	Request OpRequest `json:"request"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`

	Results []OpResult `json:"results"`

	// Summary
	TotalCount   int   `json:"total_count"`
	SuccessCount int   `json:"success_count"`
	FailedCount  int   `json:"failed_count"`
	BytesFreed   int64 `json:"bytes_freed"`
}

// Progress represents the current state of an ongoing operation.
type Progress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentPath string `json:"current_path"`
	BytesDone   int64  `json:"bytes_done"`
	BytesTotal  int64  `json:"bytes_total"`
}

// Percent returns the progress as a percentage (0-100).
func (p *Progress) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Current) / float64(p.Total) * 100
}

// ScanProgress represents scanning progress.
type ScanProgress struct {
	NodesScanned int64  `json:"nodes_scanned"`
	BytesScanned int64  `json:"bytes_scanned"`
	CurrentPath  string `json:"current_path"`
	ErrorCount   int64  `json:"error_count"`
}
