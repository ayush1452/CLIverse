package disk

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage CLIverse scan cache",
		Long:  "List and purge cached scan results.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List cached scans",
		RunE:  runCacheList,
	})

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Purge cached scans",
		RunE:  runCachePurge,
	}
	purgeCmd.Flags().Bool("all", false, "Purge all cached scans")
	purgeCmd.Flags().String("older-than", "", "Purge scans older than duration (e.g., 30d)")
	cmd.AddCommand(purgeCmd)

	return cmd
}

func getCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "cliverse", "disk", "scans")
}

func runCacheList(cmd *cobra.Command, args []string) error {
	cacheDir := getCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No cached scans found.")
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No cached scans found.")
		return nil
	}

	fmt.Printf("\n📦 Cached scans in %s:\n\n", cacheDir)

	var totalSize int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalSize += info.Size()
		fmt.Printf("   %s  %10s  %s\n",
			entry.Name(),
			humanize.IBytes(uint64(info.Size())),
			humanize.Time(info.ModTime()))
	}

	fmt.Printf("\n   Total: %d scans, %s\n\n", len(entries), humanize.IBytes(uint64(totalSize)))

	return nil
}

func runCachePurge(cmd *cobra.Command, args []string) error {
	cacheDir := getCacheDir()

	purgeAll, _ := cmd.Flags().GetBool("all")
	// olderThan, _ := cmd.Flags().GetString("older-than")

	if !purgeAll {
		fmt.Println("Use --all to purge all cached scans")
		return nil
	}

	err := os.RemoveAll(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to purge cache: %w", err)
	}

	fmt.Println("✅ Cache purged successfully")
	return nil
}
