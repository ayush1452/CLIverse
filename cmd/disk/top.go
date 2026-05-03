package disk

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/ayush1452/CLIverse/core/model"
	"github.com/ayush1452/CLIverse/core/scan"
)

var (
	topPage    int
	topPerPage int
)

var (
	topShowFiles bool
	topShowAll   bool
)

func newTopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top [path]",
		Short: "Show top directories or files by size",
		Long: `Show the largest directories or files in the given path.

Examples:
  cliverse disk top .              # Top 10 directories
  cliverse disk top . --files      # Top 10 files
  cliverse disk top . -t 20        # Top 20 directories
  cliverse disk top /var --all     # Top directories and files combined`,
		Args: cobra.MaximumNArgs(1),
		RunE: runTop,
	}

	cmd.Flags().BoolVar(&topShowFiles, "files", false, "Show top files instead of directories")
	cmd.Flags().BoolVar(&topShowAll, "all", false, "Show both files and directories")
	cmd.Flags().IntVar(&topPage, "page", 1, "Page number")
	cmd.Flags().IntVar(&topPerPage, "per-page", 0, "Rows per page (0 = all)")

	return cmd
}

func runTop(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	opts := buildScanOptions()
	scanner := scan.New(opts)

	var progressFn scan.ProgressFunc
	if showProgress {
		progressFn = func(p model.ScanProgress) {
			fmt.Fprintf(os.Stderr, "\rScanning: %d nodes...", p.NodesScanned)
		}
	}

	result, err := scanner.Scan(context.Background(), path, progressFn)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if showProgress {
		fmt.Fprintln(os.Stderr)
	}

	if jsonOutput || format == "json" {
		return outputTopJSON(result, topShowFiles, topShowAll)
	}

	return outputTopTable(result, topShowFiles, topShowAll)
}

func outputTopTable(s *model.Scan, files, all bool) error {
	root := s.Root()
	if root == nil {
		return fmt.Errorf("empty scan")
	}

	var items []*model.Node

	if all {
		// Collect both files and directories
		for _, node := range s.Nodes {
			if node.ID != s.RootID {
				items = append(items, node)
			}
		}
	} else if files {
		// Only files
		for _, node := range s.Nodes {
			if node.Kind == model.KindFile {
				items = append(items, node)
			}
		}
	} else {
		// Only directories
		for _, node := range s.Nodes {
			if node.Kind == model.KindDir && node.ID != s.RootID {
				items = append(items, node)
			}
		}
	}

	// Sort by size
	sort.Slice(items, func(i, j int) bool {
		return items[i].Size(s.Opts.SizeMode) > items[j].Size(s.Opts.SizeMode)
	})

	if len(items) > topN {
		items = items[:topN]
	}

	kind := "directories"
	if files {
		kind = "files"
	} else if all {
		kind = "items"
	}

	totalSize := root.Stats.TotalAlloc

	rows := make([][]string, len(items))
	for i, node := range items {
		size := node.Size(s.Opts.SizeMode)
		pct := float64(0)
		if totalSize > 0 {
			pct = float64(size) / float64(totalSize) * 100
		}
		icon := "dir"
		if node.Kind == model.KindFile {
			icon = "file"
		}
		path := truncatePath(relativePath(s.RootPath, node.Path), 52)
		rows[i] = []string{
			fmt.Sprintf("%d", i+1),
			icon,
			path,
			colorizeSize(size),
			fmt.Sprintf("%.1f%%", pct),
		}
	}

	cfg := tableConfig{
		Title:    fmt.Sprintf("Top %d %s in %s", len(items), kind, s.RootPath),
		Headers:  []string{"#", "type", "path", "size", "share"},
		Widths:   []int{3, 4, 54, 10, 7},
		Aligns:   []lipgloss.Position{lipgloss.Right, lipgloss.Left, lipgloss.Left, lipgloss.Right, lipgloss.Right},
		Page:     topPage,
		PageSize: topPerPage,
	}
	fmt.Println()
	fmt.Print(renderTable(cfg, rows))
	fmt.Println()
	return nil
}

func outputTopJSON(s *model.Scan, files, all bool) error {
	type topItem struct {
		Rank  int    `json:"rank"`
		Path  string `json:"path"`
		Kind  string `json:"kind"`
		Size  int64  `json:"size"`
		Human string `json:"size_human"`
	}

	var items []topItem
	var nodes []*model.Node

	if all {
		for _, node := range s.Nodes {
			if node.ID != s.RootID {
				nodes = append(nodes, node)
			}
		}
	} else if files {
		for _, node := range s.Nodes {
			if node.Kind == model.KindFile {
				nodes = append(nodes, node)
			}
		}
	} else {
		for _, node := range s.Nodes {
			if node.Kind == model.KindDir && node.ID != s.RootID {
				nodes = append(nodes, node)
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Size(s.Opts.SizeMode) > nodes[j].Size(s.Opts.SizeMode)
	})

	if len(nodes) > topN {
		nodes = nodes[:topN]
	}

	for i, node := range nodes {
		size := node.Size(s.Opts.SizeMode)
		items = append(items, topItem{
			Rank:  i + 1,
			Path:  node.Path,
			Kind:  node.Kind.String(),
			Size:  size,
			Human: humanize.IBytes(uint64(size)),
		})
	}

	return outputJSONGeneric(items)
}
