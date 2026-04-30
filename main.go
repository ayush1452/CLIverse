package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ayush1452/CLIverse/cmd/disk"
	"github.com/ayush1452/CLIverse/cmd/sysmon"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cliverse",
		Short: "A collection of terminal-native productivity tools",
		Long:  "CLIverse - Modern terminal-native tools for developers and power users.",
	}

	rootCmd.AddCommand(disk.NewDiskCmd())
	rootCmd.AddCommand(sysmon.NewSysmonCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
