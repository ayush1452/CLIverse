package sysmon

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	sysmonGUI "github.com/ayush1452/CLIverse/gui/sysmon"
)

var guiNoOpen bool

func newSysmonGUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gui",
		Short: "Launch the browser system monitor",
		Long: `Open a live browser dashboard streaming real-time system metrics.

The dashboard includes CPU gauge and history, memory breakdown, per-core
bars, network and disk I/O charts, and a sortable process table.

Examples:
  cliverse sysmon gui              # Open the live dashboard
  cliverse sysmon gui --no-open   # Start server without opening browser`,
		RunE: runSysmonGUI,
	}
	cmd.Flags().BoolVar(&guiNoOpen, "no-open", false, "Start server without opening a browser")
	return cmd
}

func runSysmonGUI(_ *cobra.Command, _ []string) error {
	srv, err := sysmonGUI.NewServer()
	if err != nil {
		return err
	}

	url := srv.URL()
	fmt.Fprintf(os.Stderr, "System monitor available at %s\n", url)
	if !guiNoOpen {
		sysmonGUI.OpenBrowser(url)
	}

	return srv.Start()
}
