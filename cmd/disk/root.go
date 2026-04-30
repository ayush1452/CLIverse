// Package disk provides the disk command and subcommands.
package disk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/ayush/zenbox/core/classify"
	"github.com/ayush/zenbox/core/model"
	"github.com/ayush/zenbox/core/scan"
	"github.com/ayush/zenbox/core/store"
)

// Shared flags
var (
	topN          int
	depth         int
	jsonOutput    bool
	format        string
	sizeMode      string
	sortBy        string
	minSize       string
	oneFilesystem bool
	crossMounts   bool
	excludes      []string
	threads       int
	showProgress  bool

	// Compatibility flags
	compatTUI        bool
	compatDuplicates bool
	compatTrash      bool

	// Export/Import flags
	exportFile string
	importFile string
)

// NewDiskCmd creates the root disk command.
func NewDiskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disk [path]",
		Short: "Terminal-native disk usage analyzer",
		Long: `zenbox disk - A modern disk usage analyzer with CLI and TUI modes.

Provides fast scanning, category breakdown, duplicate detection, and junk cleanup
with a focus on safety and actionable insights.

Examples:
  zenbox disk                    # Scan current directory
  zenbox disk /var --top 20      # Top 20 directories in /var
  zenbox disk . --json           # JSON output for scripting
  zenbox disk tui .              # Interactive TUI mode
  zenbox disk duplicates .       # Find duplicate files
  zenbox disk junk .             # Find junk/cache directories`,
		Args: cobra.MaximumNArgs(1),
		RunE: runDiskDefault,
	}

	// Persistent flags (available on all subcommands)
	pf := cmd.PersistentFlags()
	pf.StringVar(&sizeMode, "size", "alloc", "Size mode: alloc or apparent")
	pf.StringVar(&sortBy, "sort", "size", "Sort by: size, name, count, mtime")
	pf.StringVar(&minSize, "min-size", "", "Minimum size filter (e.g., 10M, 1G)")
	pf.BoolVar(&oneFilesystem, "one-filesystem", true, "Stay on one filesystem")
	pf.BoolVar(&crossMounts, "cross-mounts", false, "Cross filesystem boundaries")
	pf.StringArrayVar(&excludes, "exclude", nil, "Exclude patterns (glob)")
	pf.IntVar(&threads, "threads", 0, "Worker threads (0=auto)")
	pf.BoolVar(&showProgress, "progress", false, "Show progress during scan")

	// Output flags
	f := cmd.Flags()
	f.IntVarP(&topN, "top", "t", 10, "Show top N entries")
	f.IntVarP(&depth, "depth", "d", 2, "Depth for aggregation")
	f.BoolVar(&jsonOutput, "json", false, "Output JSON")
	f.StringVar(&format, "format", "table", "Output format: table, json, ndjson")

	// Compatibility flags
	f.BoolVar(&compatTUI, "tui", false, "Launch interactive TUI")
	f.BoolVar(&compatDuplicates, "duplicates", false, "Show duplicates (alias for duplicates subcommand)")
	f.BoolVar(&compatTrash, "trash", false, "Show junk/trash (alias for junk subcommand)")

	// Export/Import flags
	f.StringVar(&exportFile, "export", "", "Export scan to file (e.g. scan.json.gz)")
	f.StringVar(&importFile, "import", "", "Import scan from file")

	// Add subcommands
	cmd.AddCommand(newTopCmd())
	cmd.AddCommand(newTUICmd())
	cmd.AddCommand(newOverviewCmd())
	cmd.AddCommand(newDuplicatesCmd())
	cmd.AddCommand(newJunkCmd())
	cmd.AddCommand(newCacheCmd())

	return cmd
}

func runDiskDefault(cmd *cobra.Command, args []string) error {
	// Handle compatibility flags
	if compatTUI {
		return runTUI(cmd, args)
	}
	if compatDuplicates {
		return runDuplicates(cmd, args)
	}
	if compatTrash {
		return runJunk(cmd, args)
	}

	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	var result *model.Scan
	var err error

	if importFile != "" {
		// Load from file
		result, err = store.LoadScan(importFile)
		if err != nil {
			return fmt.Errorf("load failed: %w", err)
		}
		if showProgress {
			fmt.Fprintf(os.Stderr, "Loaded scan of %s (%d nodes)\n", result.RootPath, result.Stats.Nodes)
		}
	} else {
		// Perform scan
		opts := buildScanOptions()
		scanner := scan.New(opts)

		var progressFn scan.ProgressFunc
		if showProgress {
			progressFn = func(p model.ScanProgress) {
				fmt.Fprintf(os.Stderr, "\rScanning: %d nodes, %s...", p.NodesScanned, p.CurrentPath)
			}
		}

		result, err = scanner.Scan(context.Background(), path, progressFn)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if showProgress {
			fmt.Fprintln(os.Stderr) // Clear progress line
		}
	}

	// Export if requested
	if exportFile != "" {
		if err := store.SaveScan(result, exportFile); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		if showProgress {
			fmt.Fprintf(os.Stderr, "Exported scan to %s\n", exportFile)
		}
	}

	// Generate overview with categories
	classifier := classify.New()
	overview := classifier.ClassifyScan(result)

	// Output based on format
	if jsonOutput || format == "json" {
		return outputJSON(result, overview)
	}

	return outputSummary(result, overview)
}

func buildScanOptions() model.ScanOptions {
	opts := model.DefaultScanOptions()

	if sizeMode == "apparent" {
		opts.SizeMode = model.SizeApparent
	}

	opts.OneFilesystem = oneFilesystem
	if crossMounts {
		opts.OneFilesystem = false
	}

	opts.Excludes = excludes
	opts.Threads = threads

	return opts
}

func outputSummary(scan *model.Scan, overview *model.Overview) error {
	root := scan.Root()
	if root == nil {
		return fmt.Errorf("empty scan")
	}

	// Header
	fmt.Printf("\n📁 %s\n", scan.RootPath)
	fmt.Printf("   Scanned %d files, %d directories in %s\n",
		scan.Stats.Files, scan.Stats.Dirs,
		scan.Stats.Elapsed.Round(1e6))

	if scan.Stats.Errors > 0 {
		fmt.Printf("   ⚠ %d errors (permission denied, etc.)\n", scan.Stats.Errors)
	}

	// Filesystem usage
	if overview.TotalBytes > 0 {
		usedPct := float64(overview.UsedBytes) / float64(overview.TotalBytes) * 100
		fmt.Printf("\n💾 Disk: %s used of %s (%.1f%%), %s free\n",
			humanize.IBytes(uint64(overview.UsedBytes)),
			humanize.IBytes(uint64(overview.TotalBytes)),
			usedPct,
			humanize.IBytes(uint64(overview.FreeBytes)))
	}

	// Top directories
	fmt.Printf("\n📊 Top %d directories:\n\n", topN)

	topDirs := getTopDirectories(scan, root, topN)
	for i, node := range topDirs {
		size := node.Size(scan.Opts.SizeMode)
		pct := float64(size) / float64(root.Stats.TotalAlloc) * 100
		fmt.Printf("   %2d. %-50s %10s  (%.1f%%)\n",
			i+1,
			truncatePath(relativePath(scan.RootPath, node.Path), 50),
			humanize.IBytes(uint64(size)),
			pct)
	}

	// Category breakdown
	if len(overview.Categories) > 0 {
		fmt.Printf("\n📦 By category:\n\n")
		for _, cat := range overview.Categories {
			if cat.BytesAlloc > 0 {
				pct := float64(cat.BytesAlloc) / float64(root.Stats.TotalAlloc) * 100
				fmt.Printf("   %-15s %10s  (%.1f%%)\n",
					cat.Category.String(),
					humanize.IBytes(uint64(cat.BytesAlloc)),
					pct)
			}
		}
	}

	// Quick insights
	fmt.Printf("\n💡 Quick actions:\n")
	fmt.Printf("   zenbox disk tui %s        # Interactive browser\n", scan.RootPath)
	fmt.Printf("   zenbox disk duplicates %s # Find duplicates\n", scan.RootPath)
	fmt.Printf("   zenbox disk junk %s       # Find cleanable junk\n\n", scan.RootPath)

	return nil
}

func outputJSON(scan *model.Scan, overview *model.Overview) error {
	output := struct {
		Scan     *model.Scan     `json:"scan"`
		Overview *model.Overview `json:"overview"`
	}{
		Scan:     scan,
		Overview: overview,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func getTopDirectories(s *model.Scan, root *model.Node, n int) []*model.Node {
	// Collect directories at depth 1-2
	var dirs []*model.Node
	collectDirs(s, root, 0, depth, &dirs)

	// Sort by size
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Stats.TotalAlloc > dirs[j].Stats.TotalAlloc
	})

	if len(dirs) > n {
		dirs = dirs[:n]
	}
	return dirs
}

func collectDirs(s *model.Scan, node *model.Node, currentDepth, maxDepth int, result *[]*model.Node) {
	if currentDepth > 0 && node.Kind == model.KindDir {
		*result = append(*result, node)
	}

	if currentDepth >= maxDepth {
		return
	}

	for _, childID := range node.Children {
		child := s.GetNode(childID)
		if child != nil && child.Kind == model.KindDir {
			collectDirs(s, child, currentDepth+1, maxDepth, result)
		}
	}
}

func relativePath(root, path string) string {
	if len(path) > len(root) && path[:len(root)] == root {
		rel := path[len(root):]
		if len(rel) > 0 && rel[0] == '/' {
			rel = rel[1:]
		}
		if rel == "" {
			return "."
		}
		return rel
	}
	return path
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
