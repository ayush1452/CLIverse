// Package theme provides styling for the TUI.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and styles for the TUI.
type Theme struct {
	Name string

	// Base colors
	Background lipgloss.Color
	Foreground lipgloss.Color
	Subtle     lipgloss.Color
	Highlight  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color

	// Category colors (stable order for charts)
	CategoryColors []lipgloss.Color

	// Styles
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Help       lipgloss.Style
	StatusBar  lipgloss.Style
	Selected   lipgloss.Style
	Marked     lipgloss.Style
	Directory  lipgloss.Style
	File       lipgloss.Style
	Size       lipgloss.Style
	Percentage lipgloss.Style
	Border     lipgloss.Style
	Panel      lipgloss.Style
	PanelTitle lipgloss.Style

	// Layout bar styles
	HeaderBar  lipgloss.Style
	CommandBar lipgloss.Style
	DialogBox  lipgloss.Style
}

// Dark is the default dark theme.
var Dark = Theme{
	Name:       "dark",
	Background: lipgloss.Color("#1a1b26"),
	Foreground: lipgloss.Color("#c0caf5"),
	Subtle:     lipgloss.Color("#565f89"),
	Highlight:  lipgloss.Color("#7aa2f7"),
	Success:    lipgloss.Color("#9ece6a"),
	Warning:    lipgloss.Color("#e0af68"),
	Error:      lipgloss.Color("#f7768e"),

	CategoryColors: []lipgloss.Color{
		"#7aa2f7", // Applications - blue
		"#bb9af7", // Developer - purple
		"#7dcfff", // Documents - cyan
		"#9ece6a", // Photos - green
		"#e0af68", // Music - yellow
		"#f7768e", // Videos - red
		"#565f89", // System Data - gray
		"#414868", // Other - dark gray
	},
}

// Dim is a muted theme for low-light environments.
var Dim = Theme{
	Name:       "dim",
	Background: lipgloss.Color("#1e1e2e"),
	Foreground: lipgloss.Color("#cdd6f4"),
	Subtle:     lipgloss.Color("#6c7086"),
	Highlight:  lipgloss.Color("#89b4fa"),
	Success:    lipgloss.Color("#a6e3a1"),
	Warning:    lipgloss.Color("#f9e2af"),
	Error:      lipgloss.Color("#f38ba8"),

	CategoryColors: []lipgloss.Color{
		"#89b4fa", // Applications
		"#cba6f7", // Developer
		"#89dceb", // Documents
		"#a6e3a1", // Photos
		"#f9e2af", // Music
		"#f38ba8", // Videos
		"#6c7086", // System Data
		"#45475a", // Other
	},
}

// HighContrast is an accessibility-focused theme.
var HighContrast = Theme{
	Name:       "hc",
	Background: lipgloss.Color("#000000"),
	Foreground: lipgloss.Color("#ffffff"),
	Subtle:     lipgloss.Color("#808080"),
	Highlight:  lipgloss.Color("#00ffff"),
	Success:    lipgloss.Color("#00ff00"),
	Warning:    lipgloss.Color("#ffff00"),
	Error:      lipgloss.Color("#ff0000"),

	CategoryColors: []lipgloss.Color{
		"#00ffff", // Applications
		"#ff00ff", // Developer
		"#00ff00", // Documents
		"#ffff00", // Photos
		"#ff8000", // Music
		"#ff0000", // Videos
		"#808080", // System Data
		"#404040", // Other
	},
}

// Get returns a theme by name.
func Get(name string) Theme {
	switch name {
	case "dim":
		return Dim.Init()
	case "hc", "high-contrast":
		return HighContrast.Init()
	default:
		return Dark.Init()
	}
}

// Init initializes the theme's styles.
func (t Theme) Init() Theme {
	t.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Foreground)

	t.Subtitle = lipgloss.NewStyle().
		Foreground(t.Subtle)

	t.Help = lipgloss.NewStyle().
		Foreground(t.Subtle)

	t.StatusBar = lipgloss.NewStyle().
		Background(t.Highlight).
		Foreground(t.Background).
		PaddingLeft(1).
		PaddingRight(1)

	t.Selected = lipgloss.NewStyle().
		Background(t.Highlight).
		Foreground(t.Background).
		Bold(true)

	t.Marked = lipgloss.NewStyle().
		Foreground(t.Warning).
		Bold(true)

	t.Directory = lipgloss.NewStyle().
		Foreground(t.Highlight).
		Bold(true)

	t.File = lipgloss.NewStyle().
		Foreground(t.Foreground)

	t.Size = lipgloss.NewStyle().
		Foreground(t.Subtle).
		Align(lipgloss.Right)

	t.Percentage = lipgloss.NewStyle().
		Foreground(t.Subtle).
		Width(6).
		Align(lipgloss.Right)

	t.Border = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Subtle)

	t.Panel = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Subtle).
		Padding(0, 1)

	t.PanelTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Highlight)

	t.HeaderBar = lipgloss.NewStyle().
		Bold(true).
		Background(t.Background).
		Foreground(t.Foreground)

	t.CommandBar = lipgloss.NewStyle().
		Background(t.Background).
		Foreground(t.Subtle)

	t.DialogBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Warning).
		Padding(1, 2)

	return t
}

// CategoryColor returns the color for a category index.
func (t Theme) CategoryColor(index int) lipgloss.Color {
	if index < 0 || index >= len(t.CategoryColors) {
		return t.Subtle
	}
	return t.CategoryColors[index]
}
