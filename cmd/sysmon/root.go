// Package sysmon wires the system-monitor TUI into the cobra command tree.
package sysmon

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	tuisysmon "github.com/ayush1452/CLIverse/tui/sysmon"
)

// NewSysmonCmd returns the cobra command for the system monitor.
func NewSysmonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sysmon",
		Short: "Real-time system monitor (CPU, memory, disk I/O, network)",
		Long: `sysmon opens an interactive full-screen system monitor (TUI by default).

Keyboard shortcuts (TUI):
  q / Ctrl+C   quit
  p            toggle process sort (CPU ↔ MEM)
  ↑ / k        move cursor up in process list
  ↓ / j        move cursor down in process list

Subcommands:
  gui          open the browser-based live dashboard`,
		RunE: runSysmon,
	}
	cmd.AddCommand(newSysmonGUICmd())
	return cmd
}

func runSysmon(_ *cobra.Command, _ []string) error {
	p := tea.NewProgram(
		tuisysmon.New(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sysmon error: %v\n", err)
		return err
	}
	return nil
}
