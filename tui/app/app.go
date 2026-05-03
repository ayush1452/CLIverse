// Package app provides the main TUI application model.
package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ayush1452/CLIverse/core/classify"
	"github.com/ayush1452/CLIverse/core/model"
	"github.com/ayush1452/CLIverse/tui/theme"
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

// SortField controls how tree children are ordered.
type SortField uint8

const (
	SortBySize  SortField = iota // default: largest first
	SortByName                   // alphabetical
	SortByMtime                  // newest first
	SortByCount                  // most files first
)

func (s SortField) String() string {
	switch s {
	case SortByName:
		return "name↑"
	case SortByMtime:
		return "mtime↓"
	case SortByCount:
		return "count↓"
	default:
		return "size↓"
	}
}

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

	// File operation dialog
	showDialog   bool
	dialogMsg    string
	dialogAction func() tea.Cmd
	dialogTarget *model.Node

	// Search / filter
	searchMode  bool
	searchQuery string

	// Sort
	sortField SortField
	sortAsc   bool

	// Messages
	statusMsg string
	opResult  string
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

	case opResultMsg:
		m.statusMsg = msg.message
		if msg.success && msg.deletedID != 0 {
			deleted := m.scan.GetNode(msg.deletedID)
			if deleted != nil {
				parent := m.scan.GetNode(deleted.Parent)
				if parent != nil {
					newChildren := make([]model.NodeID, 0, len(parent.Children))
					for _, cid := range parent.Children {
						if cid != msg.deletedID {
							newChildren = append(newChildren, cid)
						}
					}
					parent.Children = newChildren
				}
				delete(m.scan.Nodes, msg.deletedID)
			}
			m.rebuildFlatTree()
			if m.treeCursor >= len(m.treeFlat) {
				m.treeCursor = maxInt(0, len(m.treeFlat)-1)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dialog intercepts all keys
	if m.showDialog {
		return m.handleDialogKey(msg)
	}

	// Search mode intercepts all keys
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	// Global keys
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "tab":
		if m.currentView == ViewTree {
			m.currentView = ViewOverview
			m.statusMsg = "Overview view"
		} else {
			m.currentView = ViewTree
			m.statusMsg = "Explorer view"
		}
		return m, nil
	case "t":
		m.currentView = ViewTree
		m.statusMsg = "Explorer view"
		return m, nil
	case "o", "O":
		m.currentView = ViewOverview
		m.statusMsg = "Overview view"
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

func (m *Model) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		action := m.dialogAction
		m.showDialog = false
		m.dialogTarget = nil
		m.dialogMsg = ""
		m.dialogAction = nil
		if action != nil {
			return m, action()
		}
	case "n", "N", "esc":
		m.showDialog = false
		m.dialogTarget = nil
		m.dialogMsg = ""
		m.dialogAction = nil
		m.statusMsg = "Cancelled"
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.searchMode = false
		m.searchQuery = ""
		m.rebuildFlatTree()
		m.statusMsg = "Search cleared"
	case "enter":
		m.searchMode = false
		m.statusMsg = fmt.Sprintf("Showing %d matches for %q", len(m.treeFlat), m.searchQuery)
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.rebuildFlatTree()
		}
	default:
		// Accept printable single characters
		if len(key) == 1 {
			m.searchQuery += key
			m.rebuildFlatTree()
		}
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

	case "y":
		if node := m.selectedNode(); node != nil {
			m.statusMsg = "Copying path..."
			return m, cmdCopyPath(node.Path)
		}

	case "D":
		if node := m.selectedNode(); node != nil {
			m.showDialog = true
			m.dialogTarget = node
			m.dialogMsg = fmt.Sprintf("Delete %s?\n%s\n\nThis cannot be undone.", node.Kind.String(), truncatePath(node.Path, 54))
			nodeID := node.ID
			isDir := node.IsDir()
			path := node.Path
			m.dialogAction = func() tea.Cmd {
				return cmdDeleteNode(path, nodeID, isDir)
			}
		}
		return m, nil

	case "e":
		if node := m.selectedNode(); node != nil {
			return m, cmdRevealInFinder(node.Path)
		}

	case "ctrl+o":
		return m, cmdOpenGUI(m.scan.RootPath)

	case "s":
		m.sortField = (m.sortField + 1) % 4
		m.rebuildFlatTree()
		m.statusMsg = "Sort: " + m.sortField.String()

	case "/":
		m.searchMode = true
		m.searchQuery = ""
		m.statusMsg = "Search mode — type to filter"
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

	var body string
	switch m.currentView {
	case ViewOverview:
		body = m.renderOverviewView()
	default:
		body = m.renderTreeView()
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeaderBar(),
		m.renderContextBar(),
		body,
		m.renderStatusBar(),
		m.renderCommandBar(),
	)

	if m.showDialog {
		return m.renderDialog(content)
	}
	if m.showHelp {
		return m.renderHelpOverlay(content)
	}
	return content
}

func (m *Model) rebuildFlatTree() {
	m.treeFlat = m.treeFlat[:0]
	m.flattenNode(m.scan.RootID)

	// Apply search filter when query is long enough
	if m.searchMode && len(m.searchQuery) >= 2 {
		query := strings.ToLower(m.searchQuery)
		filtered := m.treeFlat[:0]
		for _, node := range m.treeFlat {
			if strings.Contains(strings.ToLower(node.Name), query) ||
				strings.Contains(strings.ToLower(node.Path), query) {
				filtered = append(filtered, node)
			}
		}
		m.treeFlat = filtered
		if m.treeCursor >= len(m.treeFlat) {
			m.treeCursor = maxInt(0, len(m.treeFlat)-1)
		}
	}
}

func (m *Model) flattenNode(id model.NodeID) {
	node := m.scan.GetNode(id)
	if node == nil {
		return
	}

	m.treeFlat = append(m.treeFlat, node)
	if !node.IsDir() || !m.treeExpanded[id] {
		return
	}

	// Sort children according to current sort field
	children := make([]model.NodeID, len(node.Children))
	copy(children, node.Children)
	sort.Slice(children, func(i, j int) bool {
		a, b := m.scan.GetNode(children[i]), m.scan.GetNode(children[j])
		if a == nil || b == nil {
			return false
		}
		switch m.sortField {
		case SortByName:
			if m.sortAsc {
				return a.Name < b.Name
			}
			return a.Name > b.Name
		case SortByMtime:
			if m.sortAsc {
				return a.Meta.MTime.Before(b.Meta.MTime)
			}
			return a.Meta.MTime.After(b.Meta.MTime)
		case SortByCount:
			if m.sortAsc {
				return a.Stats.FileCount < b.Stats.FileCount
			}
			return a.Stats.FileCount > b.Stats.FileCount
		default: // SortBySize descending
			return a.Size(m.sizeMode) > b.Size(m.sizeMode)
		}
	})

	for _, childID := range children {
		if m.scan.GetNode(childID) != nil {
			m.flattenNode(childID)
		}
	}
}

func (m Model) renderHeaderBar() string {
	logo := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Highlight).Render("CLIverse Disk")
	sep := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render("  │  ")
	path := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(truncatePath(m.scan.RootPath, maxInt(28, m.width/2-30)))

	tabs := lipgloss.JoinHorizontal(lipgloss.Left,
		m.renderTab("Explorer", m.currentView == ViewTree),
		" ",
		m.renderTab("Overview", m.currentView == ViewOverview),
	)

	sortChip := lipgloss.NewStyle().
		Foreground(m.theme.Success).
		Render(" sort:" + m.sortField.String())

	left := logo + sep + path
	right := tabs + sortChip

	pad := maxInt(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	line := left + strings.Repeat(" ", pad) + right
	return m.theme.HeaderBar.Width(m.width).Render(line)
}

func (m Model) renderContextBar() string {
	root := m.scan.Root()
	scanned := "0 B"
	if root != nil {
		scanned = m.formatSize(root.Size(m.sizeMode))
	}

	stats := []string{
		m.renderStatChip("Scanned", scanned),
		m.renderStatChip("Files", m.formatCount(m.scan.Stats.Files)),
		m.renderStatChip("Dirs", m.formatCount(m.scan.Stats.Dirs)),
		m.renderStatChip("Mode", m.sizeMode.String()),
	}
	if m.scan.Stats.Errors > 0 {
		stats = append(stats, m.renderStatChip("Errors", m.formatCount(m.scan.Stats.Errors)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, stats...)
}

func (m Model) renderTab(label string, active bool) string {
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Subtle).
		Foreground(m.theme.Subtle)
	if active {
		style = style.
			BorderForeground(m.theme.Highlight).
			Foreground(m.theme.Foreground).
			Bold(true)
	}
	return style.Render(label)
}

func (m Model) renderStatChip(label, value string) string {
	return lipgloss.NewStyle().
		Padding(0, 1).
		MarginRight(1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Subtle).
		Render(
			lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(strings.ToUpper(label)) +
				" " +
				lipgloss.NewStyle().Bold(true).Foreground(m.theme.Foreground).Render(value),
		)
}

func (m Model) renderStatusBar() string {
	nodeCount := lipgloss.NewStyle().Foreground(m.theme.Foreground).Render(fmt.Sprintf("%d nodes", len(m.treeFlat)))
	marked := lipgloss.NewStyle().Foreground(m.theme.Warning).Render(fmt.Sprintf("%d marked", len(m.treeMarked)))
	sep := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render("  │  ")

	pos := ""
	scrollPct := ""
	if len(m.treeFlat) > 0 {
		pos = fmt.Sprintf("%d/%d", m.treeCursor+1, len(m.treeFlat))
		pct := int(float64(m.treeCursor) / float64(maxInt(1, len(m.treeFlat)-1)) * 100)
		scrollPct = fmt.Sprintf("%d%%", pct)
	}
	posStr := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(pos + "  " + scrollPct)

	msg := m.statusMsg
	if msg == "" {
		if node := m.selectedNode(); node != nil {
			msg = truncatePath(node.Path, maxInt(24, m.width/2))
		} else {
			msg = "Ready"
		}
	}
	msgStr := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(msg)

	left := nodeCount + sep + marked + sep + msgStr
	right := posStr
	pad := maxInt(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", pad) + right
}

func (m Model) renderCommandBar() string {
	if m.searchMode {
		return m.renderSearchBox()
	}

	type hint struct{ key, action string }
	var hints []hint

	if m.showDialog {
		hints = []hint{{"y", "confirm"}, {"N", "cancel"}}
	} else if m.currentView == ViewOverview {
		hints = []hint{{"j/k", "navigate"}, {"enter", "drill"}, {"tab", "tree"}, {"?", "help"}, {"q", "quit"}}
	} else {
		hints = []hint{
			{"enter", "expand"}, {"x", "mark"}, {"y", "copy path"},
			{"D", "delete"}, {"e", "reveal"}, {"/", "search"},
			{"s", "sort"}, {"a", "size"}, {"?", "help"}, {"q", "quit"},
		}
	}

	keyStyle := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	actStyle := lipgloss.NewStyle().Foreground(m.theme.Foreground)
	sepStyle := lipgloss.NewStyle().Foreground(m.theme.Subtle)

	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, keyStyle.Render("<"+h.key+">")+actStyle.Render(" "+h.action))
	}
	line := strings.Join(parts, sepStyle.Render("  │  "))

	pad := maxInt(0, m.width-lipgloss.Width(line))
	return m.theme.CommandBar.Width(m.width).Render(line + strings.Repeat(" ", pad))
}

func (m Model) renderTreeView() string {
	leftWidth := int(float64(m.width) * 0.62)
	if leftWidth < 56 {
		leftWidth = m.width
	}
	rightWidth := m.width - leftWidth - 2
	if rightWidth < 34 {
		rightWidth = 34
	}

	explorer := m.renderPanel(
		"Explorer",
		fmt.Sprintf("%d visible nodes / cursor %d", len(m.treeFlat), m.treeCursor+1),
		m.renderTreeRows(leftWidth-4),
		leftWidth,
	)

	inspector := m.renderPanel(
		"Inspector",
		"Selected path, contribution, and child pressure",
		m.renderDetails(rightWidth-4),
		rightWidth,
	)

	if m.width < 116 {
		return lipgloss.JoinVertical(lipgloss.Left, explorer, "", inspector)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, explorer, "  ", inspector)
}

func (m Model) renderPanel(title, subtitle, content string, width int) string {
	header := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(m.theme.Foreground).Render(title),
		lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(subtitle),
	)

	body := lipgloss.JoinVertical(lipgloss.Left, header, "", content)
	return lipgloss.NewStyle().
		Width(width).
		Padding(1, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Subtle).
		Render(body)
}

func (m Model) renderTreeRows(panelWidth int) string {
	if len(m.treeFlat) == 0 {
		return m.theme.Subtitle.Render("No nodes available.")
	}

	visibleHeight := maxInt(8, m.height-10)
	startIdx := 0
	if m.treeCursor >= visibleHeight {
		startIdx = m.treeCursor - visibleHeight + 1
	}
	endIdx := minInt(len(m.treeFlat), startIdx+visibleHeight)

	lines := make([]string, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, m.renderTreeLine(m.treeFlat[i], i == m.treeCursor, panelWidth))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderTreeLine(node *model.Node, selected bool, panelWidth int) string {
	root := m.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(m.sizeMode)
	}

	pct := 0.0
	if rootSize > 0 {
		pct = float64(node.Size(m.sizeMode)) / float64(rootSize) * 100
	}

	indent := strings.Repeat("  ", int(node.Depth))
	glyph := "•"
	if node.IsDir() {
		if m.treeExpanded[node.ID] {
			glyph = "▾"
		} else {
			glyph = "▸"
		}
	}

	mark := " "
	if m.treeMarked[node.ID] {
		mark = "●"
	}

	bar := renderMeter(pct, 8, m.nodeColor(node), m.theme.Subtle)
	size := lipgloss.NewStyle().Foreground(m.theme.Foreground).Render(m.formatSize(node.Size(m.sizeMode)))
	pctStr := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(fmt.Sprintf("%5.1f%%", pct))

	nameWidth := maxInt(12, panelWidth-34-int(node.Depth)*2)
	name := truncate(node.Name, nameWidth)
	line := fmt.Sprintf("%s%s %s %-*s  %s  %s  %s", indent, mark, glyph, nameWidth, name, bar, size, pctStr)

	style := lipgloss.NewStyle().Width(panelWidth).Foreground(m.nodeColor(node))
	if node.Kind == model.KindFile {
		style = style.Foreground(m.theme.Foreground)
	}
	if m.treeMarked[node.ID] {
		style = style.Foreground(m.theme.Warning).Bold(true)
	}
	if selected {
		style = style.Background(m.theme.Highlight).Foreground(m.theme.Background).Bold(true)
	}
	return style.Render(line)
}

func (m Model) renderDetails(panelWidth int) string {
	node := m.selectedNode()
	if node == nil {
		return m.theme.Subtitle.Render("Nothing selected.")
	}

	root := m.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(m.sizeMode)
	}
	share := 0.0
	if rootSize > 0 {
		share = float64(node.Size(m.sizeMode)) / float64(rootSize) * 100
	}

	lines := []string{
		m.renderKeyValue("Name", node.Name),
		m.renderKeyValue("Kind", node.Kind.String()),
		m.renderKeyValue("Path", truncatePath(node.Path, maxInt(20, panelWidth-8))),
		m.renderKeyValue("Share", fmt.Sprintf("%s of scan", m.formatPercent(share))),
		m.renderKeyValue("Selected", m.formatSize(node.Size(m.sizeMode))),
		m.renderKeyValue("Allocated", m.formatSize(node.Stats.SizeAlloc)),
		m.renderKeyValue("Apparent", m.formatSize(node.Stats.SizeApp)),
	}

	if node.IsDir() {
		lines = append(lines,
			m.renderKeyValue("Files", m.formatCount(node.Stats.FileCount)),
			m.renderKeyValue("Dirs", m.formatCount(node.Stats.DirCount)),
			m.renderKeyValue("Recursive", m.formatSize(node.Stats.TotalAlloc)),
		)
	}
	if !node.Meta.MTime.IsZero() {
		lines = append(lines, m.renderKeyValue("Modified", node.Meta.MTime.Format("2006-01-02 15:04")))
	}
	if node.Category.Cat != model.CatUnknown {
		lines = append(lines, m.renderKeyValue("Category", node.Category.Cat.String()))
	}
	lines = append(lines, "", m.theme.PanelTitle.Render("Largest children"))
	lines = append(lines, m.renderLargestChildren(node, panelWidth)...)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderLargestChildren(node *model.Node, panelWidth int) []string {
	if len(node.Stats.LargestChildIDs) == 0 {
		return []string{m.theme.Subtitle.Render("No child breakdown available.")}
	}

	lines := make([]string, 0, len(node.Stats.LargestChildIDs))
	total := node.Size(m.sizeMode)
	for _, childID := range node.Stats.LargestChildIDs {
		child := m.scan.GetNode(childID)
		if child == nil {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = float64(child.Size(m.sizeMode)) / float64(total) * 100
		}
		name := truncate(child.Name, maxInt(12, panelWidth-24))
		line := fmt.Sprintf("%-18s %s %s", name, renderMeter(pct, 6, m.nodeColor(child), m.theme.Subtle), m.formatSize(child.Size(m.sizeMode)))
		lines = append(lines, lipgloss.NewStyle().Foreground(m.nodeColor(child)).Render(line))
	}
	if len(lines) == 0 {
		return []string{m.theme.Subtitle.Render("No child breakdown available.")}
	}
	return lines
}

func (m Model) renderOverviewView() string {
	summary := m.renderPanel(
		"Storage snapshot",
		"Filesystem occupancy and scan share",
		m.renderOverviewSummaryContent(),
		maxInt(40, m.width/3),
	)
	categories := m.renderPanel(
		"Category breakdown",
		"Sorted by size in the current mode",
		m.renderOverviewCategories(),
		maxInt(40, m.width/3),
	)
	hotspots := m.renderPanel(
		"Hot paths",
		"Largest immediate children in the scanned root",
		m.renderOverviewHotspots(),
		maxInt(40, m.width/3),
	)
	offenders := m.renderPanel(
		"Largest objects",
		"Fast triage for where the footprint concentrates",
		m.renderOverviewOffenders(),
		m.width,
	)

	if m.width < 130 {
		return lipgloss.JoinVertical(lipgloss.Left, summary, "", categories, "", hotspots, "", offenders)
	}

	top := lipgloss.JoinHorizontal(lipgloss.Top, summary, "  ", categories, "  ", hotspots)
	return lipgloss.JoinVertical(lipgloss.Left, top, "", offenders)
}

func (m Model) renderOverviewSummaryContent() string {
	root := m.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(m.sizeMode)
	}

	capacityPct := 0.0
	if m.overview.TotalBytes > 0 {
		capacityPct = float64(m.overview.UsedBytes) / float64(m.overview.TotalBytes) * 100
	}
	sharePct := 0.0
	if m.overview.TotalBytes > 0 {
		sharePct = float64(rootSize) / float64(m.overview.TotalBytes) * 100
	}

	lines := []string{
		m.renderKeyValue("Mounted at", truncatePath(m.scan.FS.MountPoint, 28)),
		m.renderKeyValue("Filesystem", m.scan.FS.FSType),
		m.renderKeyValue("Used", fmt.Sprintf("%s / %s", m.formatSize(m.overview.UsedBytes), m.formatSize(m.overview.TotalBytes))),
		renderMeterLine("Capacity", capacityPct, 16, m.theme.Highlight, m.theme.Subtle),
		m.renderKeyValue("Scanned path", m.formatSize(rootSize)),
		renderMeterLine("Scan share", sharePct, 16, m.theme.Success, m.theme.Subtle),
		m.renderKeyValue("Free", m.formatSize(m.overview.FreeBytes)),
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderOverviewCategories() string {
	if len(m.overview.Categories) == 0 {
		return m.theme.Subtitle.Render("No category data available.")
	}

	lines := make([]string, 0, len(m.overview.Categories))
	for i, cat := range m.overview.Categories {
		bytes := cat.Bytes(m.sizeMode)
		pct := 0.0
		if m.overview.UsedBytes > 0 {
			pct = float64(bytes) / float64(m.overview.UsedBytes) * 100
		}
		color := m.theme.CategoryColor(int(cat.Category))
		prefix := " "
		if i == m.overviewCursor && m.currentView == ViewOverview {
			prefix = ">"
		}
		line := fmt.Sprintf("%s %-12s %s %s",
			prefix,
			truncate(cat.Category.String(), 12),
			renderMeter(pct, 10, color, m.theme.Subtle),
			m.formatSize(bytes),
		)
		style := lipgloss.NewStyle().Foreground(color)
		if i == m.overviewCursor && m.currentView == ViewOverview {
			style = m.theme.Selected
		}
		lines = append(lines, style.Render(line))
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(
			fmt.Sprintf("  %s / %s files / %s dirs", m.formatPercent(pct), m.formatCount(cat.FileCount), m.formatCount(cat.DirCount)),
		))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderOverviewHotspots() string {
	children := m.topRootChildren(8)
	if len(children) == 0 {
		return m.theme.Subtitle.Render("No child paths available.")
	}

	root := m.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(m.sizeMode)
	}

	lines := make([]string, 0, len(children))
	for _, child := range children {
		pct := 0.0
		if rootSize > 0 {
			pct = float64(child.Size(m.sizeMode)) / float64(rootSize) * 100
		}
		line := fmt.Sprintf("%-16s %s %s",
			truncate(child.Name, 16),
			renderMeter(pct, 8, m.nodeColor(child), m.theme.Subtle),
			m.formatSize(child.Size(m.sizeMode)),
		)
		lines = append(lines, lipgloss.NewStyle().Foreground(m.nodeColor(child)).Render(line))
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(
			fmt.Sprintf("  %s / %s", m.formatPercent(pct), truncatePath(child.Path, 24)),
		))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderOverviewOffenders() string {
	root := m.scan.Root()
	rootSize := int64(0)
	if root != nil {
		rootSize = root.Size(m.sizeMode)
	}

	var nodes []*model.Node
	for _, node := range m.scan.Nodes {
		if node == nil || node.ID == m.scan.RootID {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Size(m.sizeMode) > nodes[j].Size(m.sizeMode) })
	if len(nodes) > 8 {
		nodes = nodes[:8]
	}
	if len(nodes) == 0 {
		return m.theme.Subtitle.Render("No offenders available.")
	}

	lines := make([]string, 0, len(nodes))
	for _, node := range nodes {
		pct := 0.0
		if rootSize > 0 {
			pct = float64(node.Size(m.sizeMode)) / float64(rootSize) * 100
		}
		line := fmt.Sprintf("%-34s %-8s %s %s",
			truncate(node.Path, 34),
			node.Kind.String(),
			m.formatSize(node.Size(m.sizeMode)),
			m.formatPercent(pct),
		)
		lines = append(lines, lipgloss.NewStyle().Foreground(m.nodeColor(node)).Render(line))
	}
	if m.overview.BiggestFile != "" {
		lines = append(lines, "")
		lines = append(lines, m.theme.Subtitle.Render("Largest file: "+truncatePath(m.overview.BiggestFile, 54)))
	}
	if m.overview.BiggestDir != "" {
		lines = append(lines, m.theme.Subtitle.Render("Largest dir:  "+truncatePath(m.overview.BiggestDir, 54)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderHelpOverlay(content string) string {
	help := strings.Join([]string{
		"Keyboard Shortcuts",
		"",
		"Navigation",
		"  j/k, up/down    Move the cursor",
		"  enter, l        Expand or collapse directories",
		"  h, left         Collapse or jump to parent",
		"  g / G           Jump to top or bottom",
		"  tab             Switch between Explorer and Overview",
		"  t / o           Open Explorer or Overview directly",
		"",
		"Actions",
		"  x               Mark or unmark the selected node",
		"  y               Copy path to clipboard",
		"  D               Delete selected node (with confirmation)",
		"  e               Reveal in file manager",
		"  ctrl+o          Open browser GUI",
		"  /               Search / filter tree",
		"  s               Cycle sort order (size/name/mtime/count)",
		"  a               Toggle allocated vs apparent size",
		"  esc             Back to Explorer / close help / clear search",
		"",
		"General",
		"  ?               Toggle help",
		"  q               Quit",
	}, "\n")

	helpBox := lipgloss.NewStyle().
		Width(56).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Highlight).
		Render(help)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		helpBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(m.theme.Background),
	)
}

func (m Model) selectedNode() *model.Node {
	if m.treeCursor < 0 || m.treeCursor >= len(m.treeFlat) {
		return nil
	}
	return m.treeFlat[m.treeCursor]
}

func (m Model) topRootChildren(limit int) []*model.Node {
	root := m.scan.Root()
	if root == nil {
		return nil
	}

	children := make([]*model.Node, 0, len(root.Children))
	for _, childID := range root.Children {
		if child := m.scan.GetNode(childID); child != nil {
			children = append(children, child)
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Size(m.sizeMode) > children[j].Size(m.sizeMode)
	})
	if len(children) > limit {
		children = children[:limit]
	}
	return children
}

func (m Model) nodeColor(node *model.Node) lipgloss.Color {
	if node == nil {
		return m.theme.Foreground
	}
	if node.Category.Cat != model.CatUnknown {
		return m.theme.CategoryColor(int(node.Category.Cat))
	}
	if node.IsDir() {
		return m.theme.Highlight
	}
	return m.theme.Foreground
}

func (m Model) renderKeyValue(label, value string) string {
	return lipgloss.NewStyle().Foreground(m.theme.Subtle).Render(label+": ") +
		lipgloss.NewStyle().Foreground(m.theme.Foreground).Render(value)
}

func renderMeterLine(label string, pct float64, width int, fill, empty lipgloss.Color) string {
	left := lipgloss.NewStyle().Foreground(empty).Render(label)
	right := lipgloss.NewStyle().Foreground(empty).Render(fmt.Sprintf("%5.1f%%", pct))
	return fmt.Sprintf("%-10s %s %s", left, renderMeter(pct, width, fill, empty), right)
}

func renderMeter(pct float64, width int, fill, empty lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	filled := int((pct / 100) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return lipgloss.NewStyle().Foreground(fill).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(empty).Render(strings.Repeat("░", width-filled))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:1])
	}
	return string(r[:max-1]) + "…"
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
