package main

import (
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/ayush1452/CLIverse/cmd/disk"
	"github.com/ayush1452/CLIverse/cmd/sysmon"
	home "github.com/ayush1452/CLIverse/tui/home"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "cliverse",
		Short:        "A collection of terminal-native productivity tools",
		Long:         "CLIverse - Modern terminal-native tools for developers and power users.",
		SilenceUsage: true,
		RunE:         runRoot,
	}

	rootCmd.AddCommand(disk.NewDiskCmd())
	rootCmd.AddCommand(sysmon.NewSysmonCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, _ []string) error {
	if !isInteractiveTerminal() {
		return cmd.Help()
	}

	action, err := home.Run()
	if err != nil {
		return err
	}
	if len(action.Args) == 0 {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	child := exec.Command(exe, action.Args...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	return child.Run()
}

func isInteractiveTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd())
}
