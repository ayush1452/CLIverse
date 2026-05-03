package disk

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ayush1452/CLIverse/core/classify"
	"github.com/ayush1452/CLIverse/core/model"
	"github.com/ayush1452/CLIverse/core/scan"
	"github.com/ayush1452/CLIverse/core/store"
	diskgui "github.com/ayush1452/CLIverse/gui/disk"
)

var guiNoOpen bool

func newGUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gui [path]",
		Short: "Launch the browser dashboard",
		Long: `Open an interactive browser dashboard for disk usage exploration.

The dashboard includes:
  - Capacity and category breakdowns
  - Root-level composition tiles
  - Largest directories and files
  - Searchable ranked lists and scan metadata

Examples:
  cliverse disk gui .                   # Scan current directory and open dashboard
  cliverse disk gui /var                # Inspect /var in the browser
  cliverse disk gui --import scan.json.gz # Open a previously exported scan`,
		Args: cobra.MaximumNArgs(1),
		RunE: runGUI,
	}

	cmd.Flags().BoolVar(&guiNoOpen, "no-open", false, "Start the dashboard server without opening a browser")
	cmd.Flags().StringVar(&importFile, "import", "", "Import scan from file")

	return cmd
}

func runGUI(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	result, err := loadDiskScan(path)
	if err != nil {
		return err
	}

	overview := classify.New().ClassifyScan(result)

	srv, err := diskgui.NewServer(result, overview)
	if err != nil {
		return err
	}

	url := srv.URL()
	fmt.Fprintf(os.Stderr, "Disk dashboard available at %s\n", url)
	if !guiNoOpen {
		diskgui.OpenBrowser(url)
	}

	return srv.Start()
}

func loadDiskScan(path string) (*model.Scan, error) {
	if importFile != "" {
		result, err := store.LoadScan(importFile)
		if err != nil {
			return nil, fmt.Errorf("load failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Loaded scan of %s (%d nodes)\n", result.RootPath, result.Stats.Nodes)
		return result, nil
	}

	opts := buildScanOptions()
	scanner := scan.New(opts)

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", path)
	result, err := scanner.Scan(context.Background(), path, func(p model.ScanProgress) {
		fmt.Fprintf(os.Stderr, "\rScanning: %d nodes, %d errors...", p.NodesScanned, p.ErrorCount)
	})
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\rScan complete: %d nodes in %s\n",
		result.Stats.Nodes, result.Stats.Elapsed.Round(1e6))
	return result, nil
}
