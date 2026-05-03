package disk

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ayush1452/CLIverse/tui/app"
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
  - Overview with category breakdown and hotspot triage
  - Duplicate file detection
  - Junk/cache cleanup recommendations
  - Safe multi-select deletion with staging
  - A companion browser dashboard via 'cliverse disk gui'

Examples:
  cliverse disk tui .              # Browse current directory
  cliverse disk tui /var           # Browse /var
  cliverse disk tui . --theme dim  # Use dim theme`,
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
		Long:  "Shorthand for 'cliverse disk tui --start overview'",
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

	result, err := loadDiskScan(path)
	if err != nil {
		return err
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
