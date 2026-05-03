package disk

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/ayush1452/CLIverse/core/model"
)

var isTTY = isatty.IsTerminal(os.Stdout.Fd())

type tableConfig struct {
	Headers  []string
	Widths   []int
	Aligns   []lipgloss.Position
	Page     int
	PageSize int
	Title    string
}

// renderTable renders rows as a styled table. Falls back to plain text when stdout is not a TTY.
func renderTable(cfg tableConfig, rows [][]string) string {
	total := len(rows)
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = total
	}
	page := cfg.Page
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= total && total > 0 {
		start = total - 1
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageRows := rows[start:end]
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	if !isTTY {
		return renderPlainTable(cfg, pageRows)
	}

	// Compute column widths
	widths := make([]int, len(cfg.Headers))
	for i, h := range cfg.Headers {
		widths[i] = len(h)
		if i < len(cfg.Widths) && cfg.Widths[i] > 0 {
			widths[i] = cfg.Widths[i]
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	evenRowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))
	oddRowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a9b1d6"))
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")).Italic(true)

	var sb strings.Builder

	if cfg.Title != "" {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5")).Render(cfg.Title))
		sb.WriteString("\n")
	}

	// Header row
	headers := make([]string, len(cfg.Headers))
	for i, h := range cfg.Headers {
		align := lipgloss.Left
		if i < len(cfg.Aligns) {
			align = cfg.Aligns[i]
		}
		headers[i] = lipgloss.NewStyle().Width(widths[i]).Align(align).Render(
			headerStyle.Render(strings.ToUpper(h)),
		)
	}
	sb.WriteString(strings.Join(headers, "  "))
	sb.WriteString("\n")

	// Separator
	seps := make([]string, len(cfg.Headers))
	for i := range cfg.Headers {
		seps[i] = sepStyle.Render(strings.Repeat("─", widths[i]))
	}
	sb.WriteString(strings.Join(seps, "  "))
	sb.WriteString("\n")

	// Rows
	for ri, row := range pageRows {
		cols := make([]string, len(cfg.Headers))
		for i := range cfg.Headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			align := lipgloss.Left
			if i < len(cfg.Aligns) {
				align = cfg.Aligns[i]
			}
			// Strip ANSI from val to measure plain width, then pad
			plain := lipgloss.NewStyle().Render(val) // ensures we measure rendered width
			_ = plain
			cols[i] = lipgloss.NewStyle().Width(widths[i]).Align(align).Render(val)
		}
		rowStr := strings.Join(cols, "  ")
		if ri%2 == 0 {
			sb.WriteString(evenRowStyle.Render(rowStr))
		} else {
			sb.WriteString(oddRowStyle.Render(rowStr))
		}
		sb.WriteString("\n")
	}

	// Footer with pagination info
	if total > 0 {
		footer := fmt.Sprintf("Page %d/%d  ·  Showing %d–%d of %d", page, totalPages, start+1, end, total)
		if pageSize >= total {
			footer = fmt.Sprintf("%d entries", total)
		}
		sb.WriteString(footerStyle.Render(footer))
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderPlainTable(cfg tableConfig, rows [][]string) string {
	var sb strings.Builder
	if cfg.Title != "" {
		sb.WriteString(cfg.Title + "\n")
	}
	headers := make([]string, len(cfg.Headers))
	widths := make([]int, len(cfg.Headers))
	for i, h := range cfg.Headers {
		widths[i] = len(h)
		if i < len(cfg.Widths) && cfg.Widths[i] > 0 {
			widths[i] = cfg.Widths[i]
		}
		headers[i] = fmt.Sprintf("%-*s", widths[i], strings.ToUpper(h))
	}
	sb.WriteString(strings.Join(headers, "  ") + "\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, row := range rows {
		cols := make([]string, len(cfg.Headers))
		for i := range cfg.Headers {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			cols[i] = fmt.Sprintf("%-*s", widths[i], val)
		}
		sb.WriteString(strings.Join(cols, "  ") + "\n")
	}
	return sb.String()
}

// colorizeSize returns a lipgloss-styled size string colored by magnitude.
func colorizeSize(bytes int64) string {
	const (
		GB = 1 << 30
		MB = 1 << 20
	)
	s := humanBytes(bytes)
	switch {
	case bytes >= GB:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true).Render(s)
	case bytes >= 100*MB:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Render(s)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Render(s)
	}
}

// colorizeSafety returns a colored safety symbol + label.
func colorizeSafety(level model.SafetyLevel) string {
	switch level {
	case model.SafetySafe:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Render("✓ safe")
	case model.SafetyCaution:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Render("⚠ caution")
	case model.SafetyDanger:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true).Render("⛔ danger")
	default:
		return level.Symbol()
	}
}

// humanBytes formats bytes into a human-readable string.
func humanBytes(b int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/TB)
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
