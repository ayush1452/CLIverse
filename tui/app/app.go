// Package app provides the main TUI application model.
package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ayush1452/CLIverse/core/classify"
	"github.com/ayush1452/CLIverse/core/model"
	"github.com/ayush1452/CLIverse/tui/theme"
	"github.com/ayush1452/CLIverse/tui/widgets"
)

// ViewType represents the current view/screen.
type ViewType uint8

const (
	ViewTree ViewType = iota
	ViewOverview
	ViewDuplicates
	ViewJunk
	ViewHelp
)

// Options configures the TUI application.
type Options struct {
	Theme     string
	Icons     string
	StartView ViewType
}

// Model is the main TUI application model.
type Model struct {
	// Data
	scan     *model.Scan
	overview *model.Overview

	// Navigation
	currentView ViewType
	viewStack   []ViewType

	// Tree state
	treeRoot     model.NodeID
	treeCursor   int
	treeExpanded map[model.NodeID]bool
	treeMarked   map[model.NodeID]bool
	treeFlat     []*model.Node // Flattened visible tree

	// Overview state
	overviewCursor int

	// Display
	width     int
	height    int
	theme     theme.Theme
	showIcons bool
	sizeMode  model.SizeMode

	// Modals
	showHelp bool

	// Messages
	statusMsg string
}

// New creates a new TUI model.
func New(scan *model.Scan, opts Options) Model {
	t := theme.Get(opts.Theme)

	// Generate overview
	classifier := classify.New()
	overview := classifier.ClassifyScan(scan)

	m := Model{
		scan:         scan,
		overview:     overview,
		currentView:  opts.StartView,
		treeRoot:     scan.RootID,
		treeCursor:   0,
		treeExpanded: make(map[model.NodeID]bool),
		treeMarked:   make(map[model.NodeID]bool),
		treeFlat:     make([]*model.Node, 0, 100), // Pre-allocate
		theme:        t,
		showIcons:    opts.Icons != "off",
		sizeMode:     scan.Opts.SizeMode,
	}

	// Expand root by default
	m.treeExpanded[scan.RootID] = true

	// Build initial flat tree (using pointer to modify the slice)
	ptr := &m
	ptr.rebuildFlatTree()

	return m
}

// Init initializes the model (Bubble Tea interface).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages (Bubble Tea interface).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "esc":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m.handleEscape()
	}

	if m.showHelp {
		return m, nil // Ignore other keys when help is shown
	}

	// View-specific keys
	switch m.currentView {
	case ViewTree:
		return m.handleTreeKey(msg)
	case ViewOverview:
		return m.handleOverviewKey(msg)
	}

	return m, nil
}

func (m *Model) handleTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.treeCursor < len(m.treeFlat)-1 {
			m.treeCursor++
		}
	case "k", "up":
		if m.treeCursor > 0 {
			m.treeCursor--
		}
	case "enter", "l", "right":
		if m.treeCursor < len(m.treeFlat) {
			node := m.treeFlat[m.treeCursor]
			if node.Kind == model.KindDir {
				m.treeExpanded[node.ID] = !m.treeExpanded[node.ID]
				m.rebuildFlatTree()
			}
		}
	case "h", "left", "backspace":
		if m.treeCursor < len(m.treeFlat) {
			node := m.treeFlat[m.treeCursor]
			if node.Kind == model.KindDir && m.treeExpanded[node.ID] {
				m.treeExpanded[node.ID] = false
				m.rebuildFlatTree()
			} else if node.Parent != 0 {
				// Move to parent
				for i, n := range m.treeFlat {
					if n.ID == node.Parent {
						m.treeCursor = i
						break
					}
				}
			}
		}
	case "x":
		if m.treeCursor < len(m.treeFlat) {
			node := m.treeFlat[m.treeCursor]
			if m.treeMarked[node.ID] {
				delete(m.treeMarked, node.ID)
			} else {
				m.treeMarked[node.ID] = true
			}
		}
	case "a":
		// Toggle size mode
		if m.sizeMode == model.SizeAllocated {
			m.sizeMode = model.SizeApparent
			m.statusMsg = "Showing apparent size"
		} else {
			m.sizeMode = model.SizeAllocated
			m.statusMsg = "Showing allocated size"
		}
	case "O", "o":
		m.currentView = ViewOverview
	case "g":
		m.treeCursor = 0
	case "G":
		m.treeCursor = len(m.treeFlat) - 1
	}

	return m, nil
}

func (m *Model) handleOverviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.overviewCursor < len(m.overview.Categories)-1 {
			m.overviewCursor++
		}
	case "k", "up":
		if m.overviewCursor > 0 {
			m.overviewCursor--
		}
	case "enter":
		// Could drill down to category (future)
		return m, nil
	}

	return m, nil
}

func (m *Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewOverview, ViewDuplicates, ViewJunk:
		m.currentView = ViewTree
	}
	return m, nil
}

// View renders the TUI (Bubble Tea interface).
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string
	switch m.currentView {
	case ViewTree:
		content = m.renderTreeView()
	case ViewOverview:
		content = m.renderOverviewView()
	default:
		content = m.renderTreeView()
	}

	// Add help overlay
	if m.showHelp {
		content = m.renderHelpOverlay(content)
	}

	return content
}

func (m *Model) rebuildFlatTree() {
	m.treeFlat = m.treeFlat[:0]
	m.flattenNode(m.scan.RootID, 0)
}

func (m *Model) flattenNode(id model.NodeID, depth int) {
	node := m.scan.GetNode(id)
	if node == nil {
		return
	}

	m.treeFlat = append(m.treeFlat, node)

	if node.Kind == model.KindDir && m.treeExpanded[id] {
		for _, childID := range node.Children {
			childNode := m.scan.GetNode(childID)
			if childNode != nil {
				m.flattenNode(childID, depth+1)
			}
		}
	}
}

func (m Model) renderTreeView() string {
	// Header
	header := lipgloss.JoinVertical(lipgloss.Left,
		m.theme.Title.Render("📁 "+m.scan.RootPath),
		m.theme.Subtitle.Render(m.getTreeStats()),
	)

	// Tree content
	treeContent := m.renderTree()

	// Details panel
	details := m.renderDetails()

	// Help bar
	help := m.theme.Help.Render("↑↓ navigate • Enter expand • x mark • a size mode • O overview • ? help • q quit")

	// Layout
	mainWidth := m.width - 40
	if mainWidth < 40 {
		mainWidth = m.width
	}

	leftPanel := lipgloss.NewStyle().
		Width(mainWidth).
		Height(m.height - 5).
		Render(treeContent)

	rightPanel := lipgloss.NewStyle().
		Width(38).
		Height(m.height - 5).
		Render(details)

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		main,
		"",
		help,
	)
}

func (m Model) getTreeStats() string {
	root := m.scan.Root()
	if root == nil {
		return ""
	}
	return lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(
		m.formatSize(root.Stats.TotalAlloc) + " • " +
			m.formatCount(m.scan.Stats.Files) + " files • " +
			m.formatCount(m.scan.Stats.Dirs) + " dirs",
	)
}

func (m Model) renderTree() string {
	if len(m.treeFlat) == 0 {
		return "Empty"
	}

	// Calculate visible range
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := 0
	if m.treeCursor >= visibleHeight {
		startIdx = m.treeCursor - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(m.treeFlat) {
		endIdx = len(m.treeFlat)
	}

	var lines []string
	for i := startIdx; i < endIdx; i++ {
		node := m.treeFlat[i]
		lines = append(lines, m.renderTreeLine(node, i == m.treeCursor))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderTreeLine(node *model.Node, selected bool) string {
	// Calculate available width for the tree panel
	treeWidth := m.width - 42 // Leave room for details panel
	if treeWidth < 40 {
		treeWidth = 40
	}

	// Indent based on depth
	indent := ""
	for i := 0; i < int(node.Depth); i++ {
		indent += "  "
	}

	// Icon
	icon := "📄"
	if node.Kind == model.KindDir {
		if m.treeExpanded[node.ID] {
			icon = "📂"
		} else {
			icon = "📁"
		}
	}
	if !m.showIcons {
		if node.Kind == model.KindDir {
			icon = "/"
		} else {
			icon = " "
		}
	}

	// Mark indicator
	mark := " "
	if m.treeMarked[node.ID] {
		mark = "•"
	}

	// Size and percentage
	size := m.formatSize(node.Size(m.sizeMode))
	pctStr := ""
	root := m.scan.Root()
	if root != nil && root.Stats.TotalAlloc > 0 {
		p := float64(node.Size(m.sizeMode)) / float64(root.Stats.TotalAlloc) * 100
		if p >= 0.1 {
			pctStr = m.formatPercent(p)
		}
	}

	// Calculate name width (leave space for size + pct)
	sizeWidth := 16                // "  8.0 KB 12.5%"
	indentWidth := len(indent) + 4 // indent + mark + icon + space
	nameWidth := treeWidth - indentWidth - sizeWidth
	if nameWidth < 10 {
		nameWidth = 10
	}

	// Truncate name if needed
	name := node.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-3] + "..."
	}

	// Build the line with fixed-width columns
	nameCol := lipgloss.NewStyle().Width(nameWidth).Render(name)
	sizeCol := lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Render(size)
	pctCol := lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Foreground(m.theme.Subtle).Render(pctStr)

	line := indent + mark + icon + " " + nameCol + sizeCol + pctCol

	// Apply selection/mark styling
	lineStyle := lipgloss.NewStyle().Width(treeWidth)
	if selected {
		return lineStyle.Background(m.theme.Highlight).Foreground(m.theme.Background).Bold(true).Render(line)
	}
	if m.treeMarked[node.ID] {
		return lineStyle.Foreground(m.theme.Warning).Bold(true).Render(line)
	}
	if node.Kind == model.KindDir {
		return lineStyle.Foreground(m.theme.Highlight).Bold(true).Render(line)
	}
	return lineStyle.Foreground(m.theme.Foreground).Render(line)
}

func (m Model) renderDetails() string {
	if m.treeCursor >= len(m.treeFlat) {
		return ""
	}

	node := m.treeFlat[m.treeCursor]

	title := m.theme.PanelTitle.Render("Details")

	details := []string{
		title,
		"",
		"Name: " + node.Name,
		"Path: " + truncatePath(node.Path, 30),
		"Kind: " + node.Kind.String(),
		"",
		"Allocated: " + m.formatSize(node.Stats.SizeAlloc),
		"Apparent:  " + m.formatSize(node.Stats.SizeApp),
	}

	if node.Kind == model.KindDir {
		details = append(details,
			"",
			"Total:     "+m.formatSize(node.Stats.TotalAlloc),
			"Files:     "+m.formatCount(node.Stats.FileCount),
			"Dirs:      "+m.formatCount(node.Stats.DirCount),
		)
	}

	if !node.Meta.MTime.IsZero() {
		details = append(details,
			"",
			"Modified:  "+node.Meta.MTime.Format("2006-01-02"),
		)
	}

	if node.Category.Cat != model.CatUnknown {
		details = append(details,
			"",
			"Category:  "+node.Category.Cat.String(),
		)
	}

	return m.theme.Panel.Width(36).Render(
		lipgloss.JoinVertical(lipgloss.Left, details...),
	)
}

func (m Model) renderOverviewView() string {
	header := lipgloss.JoinVertical(lipgloss.Left,
		m.theme.Title.Render("📊 Disk Usage Overview"),
		m.theme.Subtitle.Render(m.scan.RootPath),
	)

	// Disk usage bar
	usageBar := m.renderUsageBar()

	// Pie chart with labels - shows top directories
	pieChart := m.renderPieChartWithLabels()

	help := m.theme.Help.Render("↑↓ navigate • t tree • Esc back • q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		usageBar,
		"",
		pieChart,
		"",
		help,
	)
}

func (m Model) renderPieChartWithLabels() string {
	// Get top 10 directories + Others
	slices := m.getTopDirectorySlices(10)
	if len(slices) == 0 {
		return "No data to display"
	}

	// Color palette for pie slices
	colors := []lipgloss.Color{
		"#4285F4", // Blue
		"#EA4335", // Red
		"#FBBC05", // Yellow
		"#34A853", // Green
		"#FF6D01", // Orange
		"#46BDC6", // Cyan
		"#7B1FA2", // Purple
		"#C2185B", // Pink
		"#00ACC1", // Teal
		"#8D6E63", // Brown
		"#757575", // Gray (Others)
	}

	// Assign colors
	for i := range slices {
		if i < len(colors) {
			slices[i].Color = colors[i]
		} else {
			slices[i].Color = colors[len(colors)-1]
		}
	}

	// Create Enhanced Pie Chart
	chartStr := widgets.EnhancedPieChartWithLegend(slices, 60, 20)

	return chartStr
}

func (m Model) getTopDirectorySlices(maxSlices int) []widgets.PieSlice {
	root := m.scan.Root()
	if root == nil {
		return nil
	}

	// Collect all immediate children of root
	type dirSize struct {
		name string
		size int64
	}

	var dirs []dirSize
	for _, childID := range root.Children {
		child := m.scan.GetNode(childID)
		if child != nil {
			size := child.Stats.TotalAlloc
			if child.Kind != model.KindDir {
				size = child.Stats.SizeAlloc
			}
			dirs = append(dirs, dirSize{name: child.Name, size: size})
		}
	}

	// Sort by size descending
	for i := 0; i < len(dirs)-1; i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[j].size > dirs[i].size {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	// Calculate total
	var total int64
	for _, d := range dirs {
		total += d.size
	}
	if total == 0 {
		return nil
	}

	// Take top N and group rest as Others
	var slices []widgets.PieSlice
	var othersSize int64
	othersCount := 0

	for i, d := range dirs {
		if i < maxSlices-1 || len(dirs) <= maxSlices {
			slices = append(slices, widgets.PieSlice{
				Label: d.name,
				Value: float64(d.size),
			})
		} else {
			othersSize += d.size
			othersCount++
		}
	}

	// Add Others if needed
	if othersCount > 0 {
		slices = append(slices, widgets.PieSlice{
			Label: "Others (" + formatInt(int64(othersCount)) + " items)",
			Value: float64(othersSize),
		})
	}

	return slices
}

// Helper math functions
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func atan2(y, x float64) float64 {
	// Approximate atan2 using coordinate-based approach
	if x == 0 {
		if y > 0 {
			return 3.14159 / 2
		} else if y < 0 {
			return -3.14159 / 2
		}
		return 0
	}

	// Simple atan approximation for |t| <= 1
	atanApprox := func(t float64) float64 {
		absT := t
		if absT < 0 {
			absT = -absT
		}

		// For |t| > 1, use atan(t) = π/2 - atan(1/t)
		if absT > 1 {
			sign := 1.0
			if t < 0 {
				sign = -1.0
			}
			t2 := 1 / absT
			t4 := t2 * t2
			return sign * (3.14159/2 - t2*(1-t4*(1.0/3-t4*(1.0/5-t4/7))))
		}

		// Taylor series for |t| <= 1
		t2 := t * t
		return t * (1 - t2*(1.0/3-t2*(1.0/5-t2*(1.0/7-t2/9))))
	}

	result := atanApprox(y / x)
	if x < 0 {
		if y >= 0 {
			result += 3.14159
		} else {
			result -= 3.14159
		}
	}
	return result
}

func angleInSegment(angle, start, end float64) bool {
	// Normalize all angles to [-π, π]
	normalize := func(a float64) float64 {
		for a > 3.14159 {
			a -= 2 * 3.14159
		}
		for a < -3.14159 {
			a += 2 * 3.14159
		}
		return a
	}

	angle = normalize(angle)
	start = normalize(start)
	end = normalize(end)

	if start <= end {
		return angle >= start && angle < end
	}
	// Segment wraps around
	return angle >= start || angle < end
}

func (m Model) renderUsageBar() string {
	if m.overview.TotalBytes == 0 {
		return ""
	}

	usedPct := float64(m.overview.UsedBytes) / float64(m.overview.TotalBytes)
	barWidth := m.width - 20
	if barWidth < 20 {
		barWidth = 20
	}

	filledWidth := int(float64(barWidth) * usedPct)
	if filledWidth > barWidth {
		filledWidth = barWidth
	}

	filled := lipgloss.NewStyle().Background(m.theme.Highlight).Render(
		repeatStr(" ", filledWidth),
	)
	empty := lipgloss.NewStyle().Background(m.theme.Subtle).Render(
		repeatStr(" ", barWidth-filledWidth),
	)

	bar := filled + empty

	label := m.formatSize(m.overview.UsedBytes) + " / " +
		m.formatSize(m.overview.TotalBytes) + " (" +
		m.formatPercent(usedPct*100) + ")"

	return lipgloss.JoinVertical(lipgloss.Left,
		m.theme.PanelTitle.Render("💾 Disk Usage"),
		bar,
		m.theme.Subtitle.Render(label),
	)
}

func (m Model) renderCategories() string {
	title := m.theme.PanelTitle.Render("📦 By Category")

	if len(m.overview.Categories) == 0 {
		return title + "\n  No data"
	}

	var lines []string
	for i, cat := range m.overview.Categories {
		if cat.BytesAlloc == 0 {
			continue
		}

		pct := float64(cat.BytesAlloc) / float64(m.overview.UsedBytes) * 100
		color := m.theme.CategoryColor(int(cat.Category))

		indicator := lipgloss.NewStyle().Foreground(color).Render("█")
		name := cat.Category.String()
		size := m.formatSize(cat.BytesAlloc)
		pctStr := m.formatPercent(pct)

		line := "  " + indicator + " " + name
		// Pad
		for len(line) < 25 {
			line += " "
		}
		line += size + "  " + pctStr

		if i == m.overviewCursor && m.currentView == ViewOverview {
			line = m.theme.Selected.Render(line)
		}

		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, lines...)...,
	)
}

func (m Model) renderTopOffenders() string {
	title := m.theme.PanelTitle.Render("🔥 Top Offenders")

	var lines []string

	if m.overview.BiggestDir != "" {
		lines = append(lines, "  📁 Biggest dir:  "+truncatePath(m.overview.BiggestDir, 40)+
			" ("+m.formatSize(m.overview.BiggestDirSize)+")")
	}

	if m.overview.BiggestFile != "" {
		lines = append(lines, "  📄 Biggest file: "+truncatePath(m.overview.BiggestFile, 40)+
			" ("+m.formatSize(m.overview.BiggestFileSize)+")")
	}

	if len(lines) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, lines...)...,
	)
}

func (m Model) renderHelpOverlay(content string) string {
	help := `
Keyboard Shortcuts

Navigation
  j/k, ↑/↓     Move up/down
  Enter, l     Expand/collapse directory
  h, ←         Collapse or go to parent
  g            Go to top
  G            Go to bottom

Actions
  x            Mark/unmark item
  a            Toggle allocated/apparent size
  O            Switch to Overview
  Esc          Back / close

General
  ?            Toggle this help
  q            Quit
`

	helpBox := m.theme.Panel.
		Width(50).
		Render(help)

	// Center the help box
	x := (m.width - 50) / 2
	y := (m.height - 20) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		helpBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(m.theme.Background),
	)
}

// Helper functions
func (m Model) formatSize(bytes int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
	)

	switch {
	case bytes >= TB:
		return lipgloss.NewStyle().Render(
			lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
				formatFloat(float64(bytes)/TB) + " TB",
			),
		)
	case bytes >= GB:
		return lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
			formatFloat(float64(bytes)/GB) + " GB",
		)
	case bytes >= MB:
		return lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
			formatFloat(float64(bytes)/MB) + " MB",
		)
	case bytes >= KB:
		return lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
			formatFloat(float64(bytes)/KB) + " KB",
		)
	default:
		return lipgloss.NewStyle().Width(6).Align(lipgloss.Right).Render(
			formatInt(bytes) + " B",
		)
	}
}

func (m Model) formatPercent(pct float64) string {
	if pct >= 10 {
		return formatFloat(pct) + "%"
	}
	return formatFloatPrecise(pct) + "%"
}

func (m Model) formatCount(n int64) string {
	if n >= 1000000 {
		return formatFloat(float64(n)/1000000) + "M"
	}
	if n >= 1000 {
		return formatFloat(float64(n)/1000) + "K"
	}
	return formatInt(n)
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	}
	if f >= 10 {
		return lipgloss.NewStyle().Render(
			lipgloss.NewStyle().Render(formatInt(int64(f))),
		)
	}
	return lipgloss.NewStyle().Render(
		lipgloss.NewStyle().Render(formatFloatPrecise(f)),
	)
}

func formatFloatPrecise(f float64) string {
	s := ""
	if f >= 10 {
		s = formatInt(int64(f))
	} else {
		intPart := int64(f)
		decPart := int64((f - float64(intPart)) * 10)
		s = formatInt(intPart) + "." + formatInt(decPart)
	}
	return s
}

func formatInt(n int64) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
