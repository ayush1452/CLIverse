package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/ayush/zenbox/core/model"
	"github.com/ayush/zenbox/core/scan"
)

var (
	junkProfile    string
	junkAggressive bool
	junkApply      string
	junkYes        bool
)

func newJunkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "junk [path]",
		Short: "Find junk and cache directories",
		Long: `Detect junk files and directories that can be safely cleaned.

Categories detected:
  - Developer: node_modules, build, dist, .git, vendor, __pycache__
  - System: caches, logs, temp files
  - Trash: recycle bin contents

Safety levels:
  ✓ SAFE    - Regenerable (node_modules, build artifacts)
  ⚠ CAUTION - Usually safe but verify (caches, logs)
  ⛔ DANGER  - May contain important data

Examples:
  zenbox disk junk .                     # Find junk in current dir
  zenbox disk junk . --profile dev       # Developer-focused detection
  zenbox disk junk . --aggressive        # Include cautionary items
  zenbox disk junk . --apply trash       # Move junk to trash`,
		Aliases: []string{"trash", "clean"},
		Args:    cobra.MaximumNArgs(1),
		RunE:    runJunk,
	}

	cmd.Flags().StringVar(&junkProfile, "profile", "auto", "Profile: auto, dev, system, trash, all")
	cmd.Flags().BoolVar(&junkAggressive, "aggressive", false, "Include cautionary items")
	cmd.Flags().StringVar(&junkApply, "apply", "none", "Action: none, trash, delete")
	cmd.Flags().BoolVar(&junkYes, "yes", false, "Confirm destructive action")

	return cmd
}

func runJunk(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Scan
	opts := buildScanOptions()
	scanner := scan.New(opts)

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", path)

	result, err := scanner.Scan(context.Background(), path, nil)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Detecting junk...\n")

	// Find junk
	report := detectJunk(result, junkProfile, junkAggressive)

	if jsonOutput || format == "json" {
		return outputJSONGeneric(report)
	}

	return outputJunkTable(report)
}

// JunkRule defines a junk detection pattern.
type JunkRule struct {
	ID           string
	Pattern      string // Directory or file name pattern
	Safety       model.SafetyLevel
	Profile      model.JunkProfile
	ReasonText   string
	SuggestedCmd string
	MatchFunc    func(path, name string) bool
}

var junkRules = []JunkRule{
	// Developer - SAFE
	{ID: "dev:node_modules", Pattern: "node_modules", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "npm dependencies (regenerable)",
		SuggestedCmd: "rm -rf node_modules && npm install"},
	{ID: "dev:vendor", Pattern: "vendor", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Go/PHP vendor dependencies"},
	{ID: "dev:build", Pattern: "build", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Build output directory"},
	{ID: "dev:dist", Pattern: "dist", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Distribution build output"},
	{ID: "dev:target", Pattern: "target", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Rust/Maven build output"},
	{ID: "dev:pycache", Pattern: "__pycache__", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Python bytecode cache"},
	{ID: "dev:pytest", Pattern: ".pytest_cache", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Pytest cache"},
	{ID: "dev:coverage", Pattern: "coverage", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Test coverage reports"},
	{ID: "dev:gradle", Pattern: ".gradle", Safety: model.SafetySafe,
		Profile: model.JunkProfileDev, ReasonText: "Gradle cache"},

	// System caches - CAUTION
	{ID: "sys:cache", Pattern: "Caches", Safety: model.SafetyCaution,
		Profile: model.JunkProfileSystem, ReasonText: "Application caches"},
	{ID: "sys:dotcache", Pattern: ".cache", Safety: model.SafetyCaution,
		Profile: model.JunkProfileSystem, ReasonText: "User caches"},
	{ID: "sys:logs", Pattern: "Logs", Safety: model.SafetyCaution,
		Profile: model.JunkProfileSystem, ReasonText: "Application logs"},
	{ID: "sys:tmp", Pattern: "tmp", Safety: model.SafetyCaution,
		Profile: model.JunkProfileSystem, ReasonText: "Temporary files"},

	// Trash - SAFE (already deleted)
	{ID: "trash:macos", Pattern: ".Trash", Safety: model.SafetySafe,
		Profile: model.JunkProfileTrash, ReasonText: "Trash contents"},
	{ID: "trash:linux", Pattern: "Trash", Safety: model.SafetySafe,
		Profile: model.JunkProfileTrash, ReasonText: "Trash contents"},
}

func detectJunk(s *model.Scan, profileStr string, aggressive bool) *model.JunkReport {
	report := &model.JunkReport{
		ScanID:      s.ID,
		RootPath:    s.RootPath,
		GeneratedAt: time.Now(),
	}

	profile := parseProfile(profileStr)

	for _, node := range s.Nodes {
		if node.Kind != model.KindDir {
			continue
		}

		for _, rule := range junkRules {
			if !matchesProfile(rule.Profile, profile) {
				continue
			}

			if !aggressive && rule.Safety >= model.SafetyCaution {
				continue
			}

			if matchesRule(node, rule) {
				candidate := model.JunkCandidate{
					NodeID:           node.ID,
					Path:             node.Path,
					BytesAlloc:       node.Stats.TotalAlloc,
					BytesApp:         node.Stats.TotalApp,
					ReasonID:         rule.ID,
					ReasonText:       rule.ReasonText,
					Safety:           rule.Safety,
					SuggestedCommand: rule.SuggestedCmd,
				}

				report.Candidates = append(report.Candidates, candidate)
				report.TotalBytesAlloc += node.Stats.TotalAlloc
				report.TotalBytesApp += node.Stats.TotalApp

				switch rule.Safety {
				case model.SafetySafe:
					report.SafeBytes += node.Stats.TotalAlloc
				case model.SafetyCaution:
					report.CautionBytes += node.Stats.TotalAlloc
				case model.SafetyDanger:
					report.DangerBytes += node.Stats.TotalAlloc
				}

				break // Don't double-count same directory
			}
		}
	}

	// Sort by size
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].BytesAlloc > report.Candidates[j].BytesAlloc
	})

	return report
}

func matchesRule(node *model.Node, rule JunkRule) bool {
	if rule.MatchFunc != nil {
		return rule.MatchFunc(node.Path, node.Name)
	}

	// Simple name match
	return node.Name == rule.Pattern ||
		strings.HasSuffix(node.Path, string(filepath.Separator)+rule.Pattern)
}

func matchesProfile(ruleProfile, targetProfile model.JunkProfile) bool {
	if targetProfile == model.JunkProfileAll || targetProfile == model.JunkProfileAuto {
		return true
	}
	return ruleProfile == targetProfile
}

func parseProfile(s string) model.JunkProfile {
	switch s {
	case "dev":
		return model.JunkProfileDev
	case "system":
		return model.JunkProfileSystem
	case "trash":
		return model.JunkProfileTrash
	case "all":
		return model.JunkProfileAll
	default:
		return model.JunkProfileAuto
	}
}

func outputJunkTable(report *model.JunkReport) error {
	if len(report.Candidates) == 0 {
		fmt.Println("\n✅ No junk found!")
		return nil
	}

	fmt.Printf("\n♻ Found %d junk items (%s)\n\n",
		len(report.Candidates),
		humanize.IBytes(uint64(report.TotalBytesAlloc)))

	// Breakdown by safety
	if report.SafeBytes > 0 {
		fmt.Printf("   ✓ Safe to clean:    %s\n", humanize.IBytes(uint64(report.SafeBytes)))
	}
	if report.CautionBytes > 0 {
		fmt.Printf("   ⚠ Needs review:     %s\n", humanize.IBytes(uint64(report.CautionBytes)))
	}
	if report.DangerBytes > 0 {
		fmt.Printf("   ⛔ Use caution:      %s\n", humanize.IBytes(uint64(report.DangerBytes)))
	}

	fmt.Println("\n   Top junk items:\n")

	maxShow := 15
	if len(report.Candidates) < maxShow {
		maxShow = len(report.Candidates)
	}

	for i := 0; i < maxShow; i++ {
		c := report.Candidates[i]
		fmt.Printf("   %s %-50s %10s  %s\n",
			c.Safety.Symbol(),
			truncatePath(c.Path, 50),
			humanize.IBytes(uint64(c.BytesAlloc)),
			c.ReasonText)
	}

	if len(report.Candidates) > maxShow {
		fmt.Printf("\n   ... and %d more items\n", len(report.Candidates)-maxShow)
	}

	fmt.Printf("\n💡 To clean up: zenbox disk junk %s --apply trash\n\n", report.RootPath)

	return nil
}
