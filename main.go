package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ayush/zenbox/cmd/disk"
	"github.com/ayush/zenbox/cmd/sysmon"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "zenbox",
		Short: "A collection of terminal-native productivity tools",
		Long:  "zenbox - Modern CLI tools for developers and power users.",
	}

	rootCmd.AddCommand(disk.NewDiskCmd())
	rootCmd.AddCommand(sysmon.NewSysmonCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
