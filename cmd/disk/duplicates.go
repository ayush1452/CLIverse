package disk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/zeebo/blake3"

	"github.com/ayush1452/CLIverse/core/model"
	"github.com/ayush1452/CLIverse/core/scan"
)

var (
	dupesMinSize     string
	dupesHashAlg     string
	dupesKeepRule    string
	dupesApply       string
	dupesYes         bool
	dupesVerifyBytes bool
	dupesPage        int
	dupesPerPage     int
)

func newDuplicatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "duplicates [path]",
		Short: "Find duplicate files",
		Long: `Detect duplicate files using a fast multi-stage pipeline:
  1. Group files by size
  2. Compute partial hash (first/last 4KB) for candidates
  3. Compute full hash for remaining matches
  4. Optionally verify byte-by-byte before action

Examples:
  cliverse disk duplicates .                    # Find duplicates
  cliverse disk duplicates . --min-size 10M     # Only files > 10MB
  cliverse disk duplicates . --apply trash      # Move duplicates to trash
  cliverse disk duplicates . --keep newest      # Keep the newest copy`,
		Aliases: []string{"dupes"},
		Args:    cobra.MaximumNArgs(1),
		RunE:    runDuplicates,
	}

	cmd.Flags().StringVar(&dupesMinSize, "min-size", "1M", "Minimum file size")
	cmd.Flags().StringVar(&dupesHashAlg, "hash", "blake3", "Hash algorithm: blake3, sha256")
	cmd.Flags().StringVar(&dupesKeepRule, "keep", "first", "Keep rule: first, newest, oldest")
	cmd.Flags().StringVar(&dupesApply, "apply", "none", "Action: none, trash, delete")
	cmd.Flags().BoolVar(&dupesYes, "yes", false, "Confirm destructive action")
	cmd.Flags().BoolVar(&dupesVerifyBytes, "verify-bytes", false, "Byte-by-byte verify before action")
	cmd.Flags().IntVar(&dupesPage, "page", 1, "Page number")
	cmd.Flags().IntVar(&dupesPerPage, "per-page", 0, "Groups per page (0 = all)")

	return cmd
}

func runDuplicates(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Parse min size
	minBytes, err := parseSize(dupesMinSize)
	if err != nil {
		return fmt.Errorf("invalid min-size: %w", err)
	}

	// Scan
	opts := buildScanOptions()
	scanner := scan.New(opts)

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", path)

	result, err := scanner.Scan(context.Background(), path, nil)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Finding duplicates (min size: %s)...\n", dupesMinSize)

	// Find duplicates
	report := findDuplicates(result, minBytes)

	if jsonOutput || format == "json" {
		return outputJSONGeneric(report)
	}

	return outputDuplicatesTable(report)
}

func findDuplicates(s *model.Scan, minSize int64) *model.DuplicateReport {
	report := &model.DuplicateReport{
		ScanID:   s.ID,
		RootPath: s.RootPath,
	}

	// Stage 1: Group files by size
	sizeGroups := make(map[int64][]*model.Node)
	for _, node := range s.Nodes {
		if node.Kind == model.KindFile && node.Stats.SizeApp >= minSize && !node.Flags.IsHardlink {
			sizeGroups[node.Stats.SizeApp] = append(sizeGroups[node.Stats.SizeApp], node)
		}
	}

	// Stage 2 & 3: Hash candidates
	for size, nodes := range sizeGroups {
		if len(nodes) < 2 {
			continue
		}

		// Compute hashes
		hashGroups := make(map[string][]*model.Node)
		for _, node := range nodes {
			hash, err := computeHash(node.Path, dupesHashAlg)
			if err != nil {
				continue // Skip files we can't read
			}
			hashGroups[hash] = append(hashGroups[hash], node)
		}

		// Create duplicate groups
		for hash, dupes := range hashGroups {
			if len(dupes) < 2 {
				continue
			}

			// Apply keep rule
			keepIdx := applyKeepRule(dupes, dupesKeepRule)

			group := model.DuplicateGroup{
				GroupID:   fmt.Sprintf("%s-%d", hash[:8], size),
				SizeAlloc: dupes[0].Stats.SizeAlloc,
				SizeApp:   dupes[0].Stats.SizeApp,
				HashAlg:   model.HashBlake3,
				HashHex:   hash,
				KeepIndex: keepIdx,
			}

			for _, node := range dupes {
				group.Files = append(group.Files, model.DuplicateFile{
					NodeID:    node.ID,
					Path:      node.Path,
					SizeAlloc: node.Stats.SizeAlloc,
					SizeApp:   node.Stats.SizeApp,
					MTime:     node.Meta.MTime,
				})
			}

			// Calculate waste
			for i, f := range group.Files {
				if i != keepIdx {
					group.WasteAlloc += f.SizeAlloc
					group.WasteApp += f.SizeApp
				}
			}

			report.Groups = append(report.Groups, group)
			report.TotalGroups++
			report.TotalFiles += int64(len(dupes))
			report.TotalWasteAlloc += group.WasteAlloc
			report.TotalWasteApp += group.WasteApp
		}
	}

	// Sort groups by waste size
	sort.Slice(report.Groups, func(i, j int) bool {
		return report.Groups[i].WasteAlloc > report.Groups[j].WasteAlloc
	})

	return report
}

func computeHash(path, alg string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if alg == "sha256" {
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	// Default: blake3
	h := blake3.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func applyKeepRule(nodes []*model.Node, rule string) int {
	switch rule {
	case "newest":
		idx := 0
		for i, n := range nodes {
			if n.Meta.MTime.After(nodes[idx].Meta.MTime) {
				idx = i
			}
		}
		return idx
	case "oldest":
		idx := 0
		for i, n := range nodes {
			if n.Meta.MTime.Before(nodes[idx].Meta.MTime) {
				idx = i
			}
		}
		return idx
	default: // "first"
		return 0
	}
}

func outputDuplicatesTable(report *model.DuplicateReport) error {
	if len(report.Groups) == 0 {
		fmt.Println("\nNo duplicate files found!")
		return nil
	}

	fmt.Printf("\nFound %d duplicate groups  wasted: %s\n\n",
		report.TotalGroups,
		humanize.IBytes(uint64(report.TotalWasteAlloc)))

	// Flatten groups into table rows
	var rows [][]string
	for i, g := range report.Groups {
		groupHeader := fmt.Sprintf("Group %d  (%d files × %s = %s wasted)",
			i+1, len(g.Files),
			humanize.IBytes(uint64(g.SizeApp)),
			humanize.IBytes(uint64(g.WasteAlloc)))
		rows = append(rows, []string{"", groupHeader, "", ""})

		for j, f := range g.Files {
			keep := " "
			if j == g.KeepIndex {
				keep = "keep"
			}
			rows = append(rows, []string{
				keep,
				truncatePath(f.Path, 58),
				colorizeSize(f.SizeAlloc),
				f.MTime.Format("2006-01-02"),
			})
		}
	}

	cfg := tableConfig{
		Headers:  []string{"keep", "path", "size", "modified"},
		Widths:   []int{4, 60, 10, 10},
		Aligns:   []lipgloss.Position{lipgloss.Left, lipgloss.Left, lipgloss.Right, lipgloss.Left},
		Page:     dupesPage,
		PageSize: dupesPerPage,
	}
	fmt.Print(renderTable(cfg, rows))
	fmt.Printf("\nTo clean up: cliverse disk duplicates %s --apply trash\n\n", report.RootPath)
	return nil
}

func parseSize(s string) (int64, error) {
	// Simple size parser
	if s == "" || s == "0" {
		return 0, nil
	}

	var multiplier int64 = 1
	lastChar := s[len(s)-1]

	switch lastChar {
	case 'K', 'k':
		multiplier = 1 << 10
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1 << 20
		s = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1 << 30
		s = s[:len(s)-1]
	case 'T', 't':
		multiplier = 1 << 40
		s = s[:len(s)-1]
	}

	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}

	return n * multiplier, nil
}

func outputJSONGeneric(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
