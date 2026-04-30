// Package classify provides category classification for nodes.
package classify

import (
	"path/filepath"
	"strings"

	"github.com/ayush1452/CLIverse/core/model"
)

// Rule defines a classification rule.
type Rule struct {
	ID         string
	Category   model.Category
	Confidence model.Confidence
	Matcher    func(path string, node *model.Node) bool
}

// Classifier classifies nodes into categories.
type Classifier struct {
	rules []Rule
}

// New creates a new classifier with default rules.
func New() *Classifier {
	return &Classifier{
		rules: defaultRules(),
	}
}

// Classify determines the category for a node.
func (c *Classifier) Classify(node *model.Node) model.CategoryGuess {
	var bestMatch *Rule
	var bestConf model.Confidence

	for i := range c.rules {
		rule := &c.rules[i]
		if rule.Matcher(node.Path, node) {
			if rule.Confidence > bestConf {
				bestMatch = rule
				bestConf = rule.Confidence
			}
		}
	}

	if bestMatch != nil {
		return model.CategoryGuess{
			Cat:     bestMatch.Category,
			Conf:    bestMatch.Confidence,
			RuleIDs: []string{bestMatch.ID},
		}
	}

	return model.CategoryGuess{
		Cat:  model.CatOther,
		Conf: model.ConfidenceLow,
	}
}

// ClassifyScan classifies all nodes in a scan and generates an overview.
func (c *Classifier) ClassifyScan(scan *model.Scan) *model.Overview {
	// Classify each node
	for _, node := range scan.Nodes {
		node.Category = c.Classify(node)
	}

	// Aggregate by category
	catStats := make(map[model.Category]*model.CategoryStat)
	for _, cat := range model.AllCategories() {
		catStats[cat] = &model.CategoryStat{Category: cat}
	}

	var biggestDir, biggestFile string
	var biggestDirSize, biggestFileSize int64

	for _, node := range scan.Nodes {
		cat := node.Category.Cat
		stat := catStats[cat]

		// Only count leaf sizes to avoid double-counting
		if node.Kind == model.KindFile {
			stat.BytesAlloc += node.Stats.SizeAlloc
			stat.BytesApp += node.Stats.SizeApp
			stat.FileCount++

			if node.Stats.SizeAlloc > biggestFileSize {
				biggestFileSize = node.Stats.SizeAlloc
				biggestFile = node.Path
			}
		} else if node.Kind == model.KindDir {
			stat.DirCount++

			if node.Stats.TotalAlloc > biggestDirSize {
				biggestDirSize = node.Stats.TotalAlloc
				biggestDir = node.Path
			}
		}
	}

	// Build overview
	categories := make([]model.CategoryStat, 0, len(model.AllCategories()))
	for _, cat := range model.AllCategories() {
		stat := catStats[cat]
		if stat.BytesAlloc > 0 || stat.FileCount > 0 {
			categories = append(categories, *stat)
		}
	}

	// Sort by size descending
	for i := 0; i < len(categories)-1; i++ {
		for j := i + 1; j < len(categories); j++ {
			if categories[j].BytesAlloc > categories[i].BytesAlloc {
				categories[i], categories[j] = categories[j], categories[i]
			}
		}
	}

	return &model.Overview{
		ScanID:          scan.ID,
		SizeMode:        scan.Opts.SizeMode,
		TotalBytes:      scan.FS.TotalBytes,
		UsedBytes:       scan.FS.TotalBytes - scan.FS.FreeBytes,
		FreeBytes:       scan.FS.FreeBytes,
		Categories:      categories,
		BiggestDir:      biggestDir,
		BiggestDirSize:  biggestDirSize,
		BiggestFile:     biggestFile,
		BiggestFileSize: biggestFileSize,
	}
}

func defaultRules() []Rule {
	return []Rule{
		// Developer - high confidence patterns
		{ID: "dev:node_modules", Category: model.CatDeveloper, Confidence: model.ConfidenceExact,
			Matcher: containsSegment("node_modules")},
		{ID: "dev:vendor", Category: model.CatDeveloper, Confidence: model.ConfidenceExact,
			Matcher: containsSegment("vendor")},
		{ID: "dev:target", Category: model.CatDeveloper, Confidence: model.ConfidenceHigh,
			Matcher: containsSegment("target")},
		{ID: "dev:build", Category: model.CatDeveloper, Confidence: model.ConfidenceHigh,
			Matcher: containsSegment("build")},
		{ID: "dev:dist", Category: model.CatDeveloper, Confidence: model.ConfidenceHigh,
			Matcher: containsSegment("dist")},
		{ID: "dev:git", Category: model.CatDeveloper, Confidence: model.ConfidenceExact,
			Matcher: containsSegment(".git")},
		{ID: "dev:venv", Category: model.CatDeveloper, Confidence: model.ConfidenceExact,
			Matcher: func(path string, _ *model.Node) bool {
				return containsSegment("venv")(path, nil) ||
					containsSegment(".venv")(path, nil) ||
					containsSegment("__pycache__")(path, nil)
			}},

		// Photos
		{ID: "photos:ext", Category: model.CatPhotos, Confidence: model.ConfidenceHigh,
			Matcher: hasExtensions(".jpg", ".jpeg", ".png", ".heic", ".raw", ".cr2", ".nef")},
		{ID: "photos:dir", Category: model.CatPhotos, Confidence: model.ConfidenceMedium,
			Matcher: containsPath("Pictures")},

		// Music
		{ID: "music:ext", Category: model.CatMusic, Confidence: model.ConfidenceHigh,
			Matcher: hasExtensions(".mp3", ".flac", ".m4a", ".aac", ".wav", ".ogg")},
		{ID: "music:dir", Category: model.CatMusic, Confidence: model.ConfidenceMedium,
			Matcher: containsPath("Music")},

		// Videos
		{ID: "videos:ext", Category: model.CatVideos, Confidence: model.ConfidenceHigh,
			Matcher: hasExtensions(".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm")},
		{ID: "videos:dir", Category: model.CatVideos, Confidence: model.ConfidenceMedium,
			Matcher: containsPath("Movies")},

		// Documents
		{ID: "docs:ext", Category: model.CatDocuments, Confidence: model.ConfidenceHigh,
			Matcher: hasExtensions(".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md")},
		{ID: "docs:dir", Category: model.CatDocuments, Confidence: model.ConfidenceMedium,
			Matcher: containsPath("Documents")},

		// Applications (macOS)
		{ID: "apps:bundle", Category: model.CatApplications, Confidence: model.ConfidenceExact,
			Matcher: hasExtensions(".app")},
		{ID: "apps:dir", Category: model.CatApplications, Confidence: model.ConfidenceExact,
			Matcher: isExactPath("/Applications")},

		// System Data
		{ID: "sys:library", Category: model.CatSystemData, Confidence: model.ConfidenceHigh,
			Matcher: containsPath("Library")},
		{ID: "sys:cache", Category: model.CatSystemData, Confidence: model.ConfidenceHigh,
			Matcher: containsPath("Caches")},
		{ID: "sys:logs", Category: model.CatSystemData, Confidence: model.ConfidenceHigh,
			Matcher: containsPath("Logs")},
		{ID: "sys:system", Category: model.CatSystemData, Confidence: model.ConfidenceExact,
			Matcher: func(path string, _ *model.Node) bool {
				return strings.HasPrefix(path, "/System") ||
					strings.HasPrefix(path, "/private/var")
			}},
	}
}

// Helper matchers
func containsSegment(segment string) func(string, *model.Node) bool {
	return func(path string, _ *model.Node) bool {
		parts := strings.Split(path, string(filepath.Separator))
		for _, p := range parts {
			if p == segment {
				return true
			}
		}
		return false
	}
}

func containsPath(substr string) func(string, *model.Node) bool {
	return func(path string, _ *model.Node) bool {
		return strings.Contains(path, substr)
	}
}

func isExactPath(target string) func(string, *model.Node) bool {
	return func(path string, _ *model.Node) bool {
		return path == target
	}
}

func hasExtensions(exts ...string) func(string, *model.Node) bool {
	return func(path string, _ *model.Node) bool {
		ext := strings.ToLower(filepath.Ext(path))
		for _, e := range exts {
			if ext == e {
				return true
			}
		}
		return false
	}
}
