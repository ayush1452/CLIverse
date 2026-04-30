package store

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ayush1452/CLIverse/core/model"
)

// ScanMetadata stores metadata about a saved scan.
type ScanMetadata struct {
	ID        string    `json:"id"`
	RootPath  string    `json:"root_path"`
	Created   time.Time `json:"created"`
	Nodes     int64     `json:"nodes"`
	TotalSize int64     `json:"total_size"`
}

// ScanDTO is a data transfer object for serialization
type ScanDTO struct {
	*model.Scan
	NodeList []*model.Node `json:"nodes"`
}

// SaveScan saves a scan to a file (gzipped JSON).
func SaveScan(scan *model.Scan, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	// Convert map to list for serialization
	dto := ScanDTO{
		Scan:     scan,
		NodeList: make([]*model.Node, 0, len(scan.Nodes)),
	}
	for _, node := range scan.Nodes {
		dto.NodeList = append(dto.NodeList, node)
	}

	enc := json.NewEncoder(gw)
	if err := enc.Encode(dto); err != nil {
		return fmt.Errorf("encode scan: %w", err)
	}

	return nil
}

// LoadScan loads a scan from a file (gzipped JSON).
func LoadScan(path string) (*model.Scan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	var dto ScanDTO
	dec := json.NewDecoder(gr)
	if err := dec.Decode(&dto); err != nil {
		return nil, fmt.Errorf("decode scan: %w", err)
	}

	scan := dto.Scan
	scan.Nodes = make(map[model.NodeID]*model.Node, len(dto.NodeList))

	// Reconstruct map
	for _, node := range dto.NodeList {
		scan.Nodes[node.ID] = node
	}

	return scan, nil
}
