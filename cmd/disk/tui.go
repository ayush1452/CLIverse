package disk

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ayush/zenbox/core/model"
	"github.com/ayush/zenbox/core/scan"
	"github.com/ayush/zenbox/core/store"
	"github.com/ayush/zenbox/tui/app"
)

var (
	tuiIcons string
	tuiTheme string
	tuiStart string
)

func newTUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui [path]",
		Short: "Launch interactive TUI mode",
		Long: `Launch the interactive terminal user interface for browsing disk usage.

The TUI provides:
  - Tree view for navigating directories
  - Overview with category breakdown and charts
  - Duplicate file detection
  - Junk/cache cleanup recommendations
  - Safe multi-select deletion with staging

Examples:
  zenbox disk tui .              # Browse current directory
  zenbox disk tui /var           # Browse /var
  zenbox disk tui . --theme dim  # Use dim theme`,
		Args: cobra.MaximumNArgs(1),
		RunE: runTUI,
	}

	cmd.Flags().StringVar(&tuiIcons, "icons", "auto", "Icon mode: auto, on, off")
	cmd.Flags().StringVar(&tuiTheme, "theme", "dark", "Theme: dark, dim, hc")
	cmd.Flags().StringVar(&tuiStart, "start", "tree", "Start view: tree, overview")
	cmd.Flags().StringVar(&importFile, "import", "", "Import scan from file")

	return cmd
}

func newOverviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overview [path]",
		Short: "Launch TUI in Overview mode",
		Long:  "Shorthand for 'zenbox disk tui --start overview'",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tuiStart = "overview"
			return runTUI(cmd, args)
		},
	}

	cmd.Flags().StringVar(&tuiIcons, "icons", "auto", "Icon mode: auto, on, off")
	cmd.Flags().StringVar(&tuiTheme, "theme", "dark", "Theme: dark, dim, hc")
	cmd.Flags().StringVar(&importFile, "import", "", "Import scan from file")

	return cmd
}

func runTUI(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Load or Scan
	var result *model.Scan
	var err error

	if importFile != "" {
		result, err = store.LoadScan(importFile)
		if err != nil {
			return fmt.Errorf("load failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Loaded scan of %s (%d nodes)\n", result.RootPath, result.Stats.Nodes)
	} else {
		// Perform initial scan
		opts := buildScanOptions()
		scanner := scan.New(opts)

		fmt.Fprintf(os.Stderr, "Scanning %s...\n", path)

		result, err = scanner.Scan(context.Background(), path, func(p model.ScanProgress) {
			fmt.Fprintf(os.Stderr, "\rScanning: %d nodes, %d errors...", p.NodesScanned, p.ErrorCount)
		})
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		fmt.Fprintf(os.Stderr, "\rScan complete: %d nodes in %s\n",
			result.Stats.Nodes, result.Stats.Elapsed.Round(1e6))
	}

	// Create and run TUI
	startView := app.ViewTree
	if tuiStart == "overview" {
		startView = app.ViewOverview
	}

	appModel := app.New(result, app.Options{
		Theme:     tuiTheme,
		Icons:     tuiIcons,
		StartView: startView,
	})

	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
