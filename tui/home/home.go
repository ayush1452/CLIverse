// Package home provides the CLIverse launcher surface shown on bare `cliverse`.
package home

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	hostinfo "github.com/shirou/gopsutil/v3/host"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Action describes the command chosen from the launcher.
type Action struct {
	Args  []string
	Label string
}

type launcherItem struct {
	Key            string
	Title          string
	Enabled        bool
	LaunchArgs     []string
	CommandPreview string
	ActiveFlag     string
	OtherFlags     []string
	PlaceholderDir string
}

type launcherGroup struct {
	Icon   string
	Title  string
	Accent lipgloss.Color
	Items  []launcherItem
}

type launcherInfo struct {
	Host   string
	User   string
	CWD    string
	Uptime string
}

type shortcutLoc struct {
	Col int
	Row int
}

type launcherPalette struct {
	Background lipgloss.Color
	Surface    lipgloss.Color
	Foreground lipgloss.Color
	Muted      lipgloss.Color
	Divider    lipgloss.Color
	Border     lipgloss.Color
	Green      lipgloss.Color
	Yellow     lipgloss.Color
	Blue       lipgloss.Color
	Cyan       lipgloss.Color
	Red        lipgloss.Color
	BadgeBG    lipgloss.Color
}

// Model holds all launcher state.
type Model struct {
	palette     launcherPalette
	width       int
	height      int
	selectedCol int
	selectedRow int
	focused     bool
	showHelp    bool
	status      string
	info        launcherInfo
	groups      []launcherGroup
	shortcuts   map[string]shortcutLoc
	selectedRun Action
}

// Run opens the launcher TUI and returns the chosen action.
func Run() (Action, error) {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	fm, err := p.Run()
	if err != nil {
		return Action{}, err
	}
	if m, ok := fm.(Model); ok {
		return m.selectedRun, nil
	}
	return Action{}, nil
}

func newModel() Model {
	groups := launcherGroups()
	return Model{
		palette:   neonPalette(),
		info:      loadLauncherInfo(),
		groups:    groups,
		shortcuts: buildShortcutMap(groups),
	}
}

func neonPalette() launcherPalette {
	return launcherPalette{
		Background: lipgloss.Color("#000000"),
		Surface:    lipgloss.Color("#0b0f0d"),
		Foreground: lipgloss.Color("#aeb8b0"),
		Muted:      lipgloss.Color("#6e6e6e"),
		Divider:    lipgloss.Color("#1b5a34"),
		Border:     lipgloss.Color("#173324"),
		Green:      lipgloss.Color("#2f9f5f"),
		Yellow:     lipgloss.Color("#ffaf5f"),
		Blue:       lipgloss.Color("#5fafff"),
		Cyan:       lipgloss.Color("#5fd7d7"),
		Red:        lipgloss.Color("#ff5f5f"),
		BadgeBG:    lipgloss.Color("#071f12"),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		key := strings.ToLower(msg.String())
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "tab":
			m.advanceSelection(1)
			return m, nil
		case "shift+tab":
			m.advanceSelection(-1)
			return m, nil
		case "right", "l":
			m.moveSelection(1, 0)
			return m, nil
		case "left", "h":
			m.moveSelection(-1, 0)
			return m, nil
		case "down", "j":
			m.moveSelection(0, 1)
			return m, nil
		case "up", "k":
			m.moveSelection(0, -1)
			return m, nil
		case "enter":
			return m.activateSelection()
		}
		if loc, ok := m.shortcuts[key]; ok {
			m.focused = true
			m.selectedCol = loc.Col
			m.selectedRow = loc.Row
			m.status = m.selectionStatus()
			return m.activateSelection()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading CLIverse launcher..."
	}

	bg := m.palette.Background
	padX := 4
	padY := 2
	if m.width < 72 {
		padX = 2
	}
	if m.height < 24 {
		padY = 1
	}

	contentWidth := maxInt(20, m.width-(padX*2))
	blank := fl("", contentWidth, bg)

	top := []string{}
	top = append(top, strings.Split(m.renderHeader(contentWidth), "\n")...)
	top = append(top, blank)
	for _, line := range strings.Split(m.renderInfoBar(contentWidth), "\n") {
		top = append(top, fl(line, contentWidth, bg))
	}
	top = append(top, blank)
	top = append(top, strings.Split(m.renderGrid(contentWidth), "\n")...)

	bottom := []string{}
	bottom = append(bottom, fl(m.renderHelpBar(), contentWidth, bg))
	bottom = append(bottom, m.divider(contentWidth))
	commandWidth := contentWidth
	for _, line := range strings.Split(m.renderCommandBox(commandWidth), "\n") {
		bottom = append(bottom, fl(line, contentWidth, bg))
	}
	bottom = append(bottom, m.renderFlagsLine(contentWidth))
	bottom = append(bottom, blank)
	bottom = append(bottom, m.dottedDivider(contentWidth))
	bottom = append(bottom, fl(m.renderStatusLine(contentWidth), contentWidth, bg))

	spacerHeight := maxInt(1, m.height-len(top)-len(bottom)-(padY*2))
	lines := make([]string, 0, len(top)+len(bottom)+spacerHeight)
	lines = append(lines, top...)
	for i := 0; i < spacerHeight; i++ {
		lines = append(lines, blank)
	}
	lines = append(lines, bottom...)

	if m.showHelp {
		return m.renderHelpOverlay()
	}

	fullBlank := sp(m.width, bg)
	frameLines := make([]string, 0, len(lines)+(padY*2))
	for i := 0; i < padY; i++ {
		frameLines = append(frameLines, fullBlank)
	}
	for _, line := range lines {
		frameLines = append(frameLines, frameLine(line, m.width, padX, bg))
	}
	for i := 0; i < padY; i++ {
		frameLines = append(frameLines, fullBlank)
	}

	return strings.Join(frameLines, "\n")
}

func (m *Model) moveSelection(dx, dy int) {
	m.focused = true
	m.selectedCol = clamp(m.selectedCol+dx, 0, len(m.groups)-1)
	m.selectedRow = clamp(m.selectedRow+dy, 0, len(m.groups[m.selectedCol].Items)-1)
	m.status = m.selectionStatus()
}

func (m *Model) advanceSelection(delta int) {
	m.focused = true
	total := 0
	for _, g := range m.groups {
		total += len(g.Items)
	}
	if total == 0 {
		return
	}

	linear := 0
	for col := 0; col < m.selectedCol; col++ {
		linear += len(m.groups[col].Items)
	}
	linear += m.selectedRow
	linear = (linear + delta + total) % total

	for col := range m.groups {
		if linear < len(m.groups[col].Items) {
			m.selectedCol = col
			m.selectedRow = linear
			break
		}
		linear -= len(m.groups[col].Items)
	}
	m.status = m.selectionStatus()
}

func (m Model) activateSelection() (tea.Model, tea.Cmd) {
	item := m.currentItem()
	if item.Enabled {
		m.selectedRun = Action{
			Args:  append([]string(nil), item.LaunchArgs...),
			Label: item.Title,
		}
		return m, tea.Quit
	}

	m.status = fmt.Sprintf("Planned module: %s", item.PlaceholderDir)
	return m, nil
}

func (m Model) currentItem() launcherItem {
	return m.groups[m.selectedCol].Items[m.selectedRow]
}

func (m Model) selectionStatus() string {
	item := m.currentItem()
	if item.Enabled {
		return "Selection: " + item.Title
	}
	return "Planned module: " + item.PlaceholderDir
}

func (m Model) renderHeader(width int) string {
	bg := m.palette.Background
	logoStyle := lipgloss.NewStyle().
		Foreground(m.palette.Green).
		Background(bg).
		Bold(true)

	logo := []string{
		`  ____ _     ___                         `,
		` / ___| |   |_ _|_   _____ _ __ ___  ___ `,
		`| |   | |    | |\ \ / / _ \ '__/ __|/ _ \`,
		`| |___| |___ | | \ V /  __/ |  \__ \  __/`,
		` \____|_____|___| \_/ \___|_|  |___/\___|`,
	}

	lines := make([]string, 0, len(logo)+7)
	lines = append(lines, fl("", width, bg))
	for _, line := range logo {
		lines = append(lines, centerLine(logoStyle.Render(line), width, bg))
	}
	lines = append(lines, fl("", width, bg))
	lines = append(lines, fl("", width, bg))

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ade80")).
		Background(bg).
		Bold(true).
		Render("CLIverse Suite v0.1.0")
	title += lipgloss.NewStyle().
		Foreground(m.palette.Muted).
		Background(bg).
		Render("  —  Three-Surface Developer Utility")

	subtitle := lipgloss.NewStyle().
		Foreground(m.palette.Foreground).
		Background(bg).
		Bold(true).
		Render("Type ")
	subtitle += lipgloss.NewStyle().
		Foreground(m.palette.Yellow).
		Background(bg).
		Bold(true).
		Render("`?`")
	subtitle += lipgloss.NewStyle().
		Foreground(m.palette.Muted).
		Background(bg).
		Bold(true).
		Render(" for help, or use shortcuts to launch tools.")

	lines = append(lines, centerLine(title, width, bg))
	lines = append(lines, centerLine(subtitle, width, bg))
	lines = append(lines, fl("", width, bg))
	lines = append(lines, m.dottedDivider(width))
	return strings.Join(lines, "\n")
}

func (m Model) renderInfoBar(width int) string {
	bg := m.palette.Background
	icons := launcherIcons()
	hostValue := clipNoEllipsis(compactHostName(m.info.Host), 14)
	userValue := clipNoEllipsis(m.info.User, 9)
	uptimeValue := clipNoEllipsis(m.info.Uptime, 12)
	cwdValue := fitInfoPath(m.info.CWD, infoPathBudget(width, hostValue, userValue, uptimeValue))

	chips := []string{
		m.renderChip(icons.Host, "host", hostValue, m.palette.Blue),
		m.renderChip(icons.User, "user", userValue, m.palette.Yellow),
		m.renderChip(icons.CWD, "cwd", cwdValue, m.palette.Green),
		m.renderChip(icons.Uptime, "up", uptimeValue, m.palette.Muted),
	}

	gap := verticalSpacer(2, lipgloss.Height(chips[0]), bg)
	parts := make([]string, 0, len(chips)*2-1)
	for idx, chip := range chips {
		if idx > 0 {
			parts = append(parts, gap)
		}
		parts = append(parts, chip)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return lipgloss.NewStyle().Background(bg).Render(row)
}

func (m Model) renderChip(icon, label, value string, accent lipgloss.Color) string {
	surface := m.palette.Surface
	iconText := lipgloss.NewStyle().
		Foreground(accent).
		Background(surface).
		Bold(true).
		Render(icon + " ")
	labelText := lipgloss.NewStyle().
		Foreground(m.palette.Muted).
		Background(surface).
		Bold(true).
		Render(label + ":")
	valueText := lipgloss.NewStyle().
		Foreground(m.palette.Foreground).
		Background(surface).
		Render(" " + value)

	chipText := iconText + labelText + valueText

	return lipgloss.NewStyle().
		Padding(0, 1).
		Background(surface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.palette.Border).
		BorderBackground(m.palette.Background).
		Render(chipText)
}

func (m Model) renderGrid(width int) string {
	bg := m.palette.Background
	cols := 4
	if m.width < 132 {
		cols = 2
	}
	if m.width < 82 {
		cols = 1
	}
	if cols > len(m.groups) {
		cols = len(m.groups)
	}

	baseGap := 6
	columnWidth := maxInt(18, (width-baseGap*(cols-1))/cols)
	gapWidths := distributeWidths(maxInt(0, width-(columnWidth*cols)), maxInt(1, cols-1), baseGap)
	gridRows := []string{}

	for start := 0; start < len(m.groups); start += cols {
		end := start + cols
		if end > len(m.groups) {
			end = len(m.groups)
		}

		columns := make([][]string, 0, end-start)
		maxHeight := 0
		for idx := start; idx < end; idx++ {
			col := m.renderGroup(idx, columnWidth)
			columns = append(columns, col)
			if len(col) > maxHeight {
				maxHeight = len(col)
			}
		}

		for row := 0; row < maxHeight; row++ {
			parts := make([]string, 0, len(columns)*2)
			for col := 0; col < cols; col++ {
				if col < len(columns) {
					if row < len(columns[col]) {
						parts = append(parts, columns[col][row])
					} else {
						parts = append(parts, fl("", columnWidth, bg))
					}
				} else {
					parts = append(parts, fl("", columnWidth, bg))
				}
				if col < cols-1 {
					parts = append(parts, sp(gapWidths[col], bg))
				}
			}
			gridRows = append(gridRows, fl(strings.Join(parts, ""), width, bg))
		}

		if end < len(m.groups) {
			gridRows = append(gridRows, fl("", width, bg))
		}
	}

	return strings.Join(gridRows, "\n")
}

func (m Model) renderGroup(col, width int) []string {
	group := m.groups[col]
	bg := m.palette.Background

	header := lipgloss.NewStyle().
		Foreground(group.Accent).
		Background(bg).
		Bold(true).
		Render(group.Icon + "  " + group.Title)

	lines := []string{
		fl(header, width, bg),
		fl(lipgloss.NewStyle().Foreground(m.palette.Divider).Background(bg).Render(strings.Repeat("─", maxInt(1, width))), width, bg),
		fl("", width, bg),
	}

	for row := range group.Items {
		lines = append(lines, m.renderItem(col, row, width))
		if row < len(group.Items)-1 {
			lines = append(lines, fl("", width, bg))
		}
	}

	return lines
}

func (m Model) renderItem(col, row, width int) string {
	bg := m.palette.Background
	item := m.groups[col].Items[row]
	selected := m.focused && col == m.selectedCol && row == m.selectedRow

	bracketFG := m.palette.Green
	keyFG := m.palette.Green
	badgeBG := m.palette.BadgeBG
	titleFG := m.palette.Muted
	rowBG := bg

	if selected {
		bracketFG = m.palette.Background
		keyFG = m.palette.Background
		badgeBG = m.palette.Green
		titleFG = lipgloss.Color("#f4fff7")
		rowBG = lipgloss.Color("#111b14")
	}

	bracketStyle := lipgloss.NewStyle().
		Foreground(bracketFG).
		Background(badgeBG)
	keyStyle := lipgloss.NewStyle().
		Foreground(keyFG).
		Background(badgeBG).
		Bold(true)

	badge := lipgloss.NewStyle().
		Padding(0, 1).
		Background(badgeBG).
		Render(bracketStyle.Render("[") + keyStyle.Render(item.Key) + bracketStyle.Render("]"))

	label := lipgloss.NewStyle().
		Foreground(titleFG).
		Background(rowBG)

	if selected {
		label = label.Bold(true)
	}

	line := lipgloss.NewStyle().Background(rowBG).Render(" ") +
		badge +
		lipgloss.NewStyle().Background(rowBG).Render("  ") +
		label.Render(truncate(item.Title, maxInt(8, width-8)))

	if selected {
		return lipgloss.NewStyle().
			Background(rowBG).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(m.palette.Border).
			BorderBackground(bg).
			Render(fl(line, maxInt(1, width-1), rowBG))
	}

	return fl(line, width, bg)
}

func (m Model) renderHelpBar() string {
	bg := m.palette.Background
	key := lipgloss.NewStyle().Foreground(m.palette.Blue).Background(bg).Bold(true)
	text := lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg)

	return key.Render("Tab") + text.Render(" complete") +
		text.Render("  ") + key.Render("Ctrl+C") + text.Render(" cancel") +
		text.Render("  ") + key.Render("?") + text.Render(" help")
}

func (m Model) renderCommandBox(width int) string {
	bg := m.palette.Background
	item := m.currentItem()

	command := item.CommandPreview
	if !item.Enabled {
		command = "planned module: " + item.PlaceholderDir
	}

	innerWidth := maxInt(8, width-2)
	top := lipgloss.NewStyle().
		Foreground(m.palette.Border).
		Background(bg).
		Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	bottom := lipgloss.NewStyle().
		Foreground(m.palette.Border).
		Background(bg).
		Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	prompt := lipgloss.NewStyle().Foreground(m.palette.Green).Background(bg).Bold(true).Render(" → ")
	home := lipgloss.NewStyle().Foreground(m.palette.Blue).Background(bg).Bold(true).Render("~ ")
	body := lipgloss.NewStyle().
		Foreground(m.palette.Foreground).
		Background(bg).
		Render(truncate(command, maxInt(8, innerWidth-10)))
	cursor := lipgloss.NewStyle().
		Foreground(m.palette.Green).
		Background(bg).
		Bold(true).
		Render("▌")
	content := fl(prompt+home+body+cursor, maxInt(4, innerWidth-2), bg)

	middle := lipgloss.NewStyle().Foreground(m.palette.Border).Background(bg).Render("│") +
		sp(1, bg) +
		content +
		sp(1, bg) +
		lipgloss.NewStyle().Foreground(m.palette.Border).Background(bg).Render("│")

	return strings.Join([]string{top, middle, bottom}, "\n")
}

func (m Model) renderFlagsLine(width int) string {
	bg := m.palette.Background
	item := m.currentItem()

	active := item.ActiveFlag
	others := item.OtherFlags
	if !item.Enabled {
		active = "--planned"
		others = []string{"--scaffolded"}
	}

	line := sp(5, bg) + lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render("Flags: ")
	if active != "" {
		line += lipgloss.NewStyle().
			Foreground(m.palette.Yellow).
			Background(lipgloss.Color("#231b07")).
			Bold(true).
			Padding(0, 1).
			Render(active)
	}
	for _, flag := range others {
		line += lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render("  " + flag)
	}
	return fl(line, width, bg)
}

func (m Model) renderStatusLine(width int) string {
	bg := m.palette.Background
	left := lipgloss.NewStyle().Foreground(m.palette.Green).Background(bg).Render("✓ ")
	left += lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render("Last run: ")
	left += lipgloss.NewStyle().Foreground(m.palette.Foreground).Background(bg).Bold(true).Render("Success")
	left += lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render(" (0s)  Output: /var/log/cliverse/last.log")

	right := lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render("Surface: ")
	right += lipgloss.NewStyle().
		Foreground(m.palette.Background).
		Background(m.palette.Green).
		Bold(true).
		Render(" CLI ")

	if width < 110 {
		left = lipgloss.NewStyle().Foreground(m.palette.Green).Background(bg).Render("✓ ")
		left += lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render("Last run: ")
		left += lipgloss.NewStyle().Foreground(m.palette.Foreground).Background(bg).Bold(true).Render("Success")
		left += lipgloss.NewStyle().Foreground(m.palette.Muted).Background(bg).Render(" (0s)")
		return left
	}

	leftBudget := maxInt(12, width-lipgloss.Width(right)-1)
	if lipgloss.Width(left) > leftBudget {
		base := lipgloss.NewStyle().
			Foreground(m.palette.Green).
			Background(bg).
			Render("✓ ")
		base += lipgloss.NewStyle().
			Foreground(m.palette.Muted).
			Background(bg).
			Render("Last run: ")
		base += lipgloss.NewStyle().
			Foreground(m.palette.Foreground).
			Background(bg).
			Bold(true).
			Render("Success")
		base += lipgloss.NewStyle().
			Foreground(m.palette.Muted).
			Background(bg).
			Render(" (0s)")

		if leftBudget < 72 {
			left = base
		} else {
			pathBudget := maxInt(8, leftBudget-lipgloss.Width(base)-10)
			left = base +
				lipgloss.NewStyle().
					Foreground(m.palette.Muted).
					Background(bg).
					Render("  Output: "+truncate("/var/log/cliverse/last.log", pathBudget))
		}
	}

	if lipgloss.Width(left) > leftBudget {
		left = lipgloss.NewStyle().
			Foreground(m.palette.Green).
			Background(bg).
			Render("✓ ") +
			lipgloss.NewStyle().
				Foreground(m.palette.Muted).
				Background(bg).
				Render("Last run: ") +
			lipgloss.NewStyle().
				Foreground(m.palette.Foreground).
				Background(bg).
				Bold(true).
				Render("Success") +
			lipgloss.NewStyle().
				Foreground(m.palette.Muted).
				Background(bg).
				Render(" (0s)")
	}

	padding := maxInt(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + sp(padding, bg) + right
}

func (m Model) renderHelpOverlay() string {
	bg := m.palette.Background
	lines := []string{
		"CLIverse Launcher Help",
		"",
		"Navigation",
		"  left / right        move across columns",
		"  up / down           move inside a column",
		"  tab / shift+tab     cycle all entries",
		"",
		"Actions",
		"  enter               launch selected tool",
		"  shortcut key        jump and launch directly",
		"  q / ctrl+c          exit launcher",
		"",
		"Live now",
		"  [1] System Monitor  -> cliverse sysmon --watch",
		"  [2] Disk Analyzer   -> cliverse disk tui .",
	}

	box := lipgloss.NewStyle().
		Width(66).
		Padding(1, 2).
		Background(bg).
		Foreground(m.palette.Foreground).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.palette.Green).
		BorderBackground(bg).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(bg),
	)
}

func (m Model) divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(m.palette.Divider).
		Background(m.palette.Background).
		Render(strings.Repeat("─", maxInt(1, width)))
}

func (m Model) dottedDivider(width int) string {
	return lipgloss.NewStyle().
		Foreground(m.palette.Divider).
		Background(m.palette.Background).
		Render(strings.Repeat(".", maxInt(1, width)))
}

func infoPathBudget(width int, host, user, uptime string) int {
	// Keep the top info pills visually compact. The path gets a little more room
	// on wide terminals, but should never dominate the strip.
	switch {
	case width >= 150:
		return 20
	case width >= 118:
		return 16
	default:
		return maxInt(9, width-lipgloss.Width(host)-lipgloss.Width(user)-lipgloss.Width(uptime)-42)
	}
}

func fitInfoPath(path string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= maxLen {
		return path
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if strings.HasPrefix(path, "~/") {
		parts = strings.Split(strings.TrimPrefix(path, "~/"), "/")
		if len(parts) >= 2 {
			tail := "~/" + strings.Join(parts[len(parts)-2:], "/")
			if lipgloss.Width(tail) <= maxLen {
				return tail
			}
		}
		if len(parts) > 0 {
			tail := "~/" + parts[len(parts)-1]
			if lipgloss.Width(tail) <= maxLen {
				return tail
			}
			return clipNoEllipsis(tail, maxLen)
		}
	}

	if len(parts) >= 2 {
		tail := strings.Join(parts[len(parts)-2:], "/")
		if lipgloss.Width(tail) <= maxLen {
			return tail
		}
	}
	if len(parts) > 0 {
		return clipNoEllipsis(parts[len(parts)-1], maxLen)
	}
	return clipNoEllipsis(path, maxLen)
}

func compactHostName(host string) string {
	host = strings.TrimSuffix(host, ".local")
	host = strings.ReplaceAll(host, "-MacBook-Pro", "-MBP")
	host = strings.ReplaceAll(host, "-MacBook-Air", "-MBA")
	return host
}

type launcherIconSet struct {
	Host   string
	User   string
	CWD    string
	Uptime string
	System string
	Files  string
	Dev    string
	Tools  string
}

func launcherIcons() launcherIconSet {
	if strings.EqualFold(os.Getenv("CLIVERSE_ICONS"), "nerd") {
		return launcherIconSet{
			Host:   "",
			User:   "",
			CWD:    "",
			Uptime: "",
			System: "",
			Files:  "",
			Dev:    "",
			Tools:  "",
		}
	}

	return launcherIconSet{
		Host:   "▣",
		User:   "●",
		CWD:    "▰",
		Uptime: "◷",
		System: "◆",
		Files:  "▣",
		Dev:    "</>",
		Tools:  "◇",
	}
}

func launcherGroups() []launcherGroup {
	icons := launcherIcons()
	blue := lipgloss.Color("#5fafff")
	yellow := lipgloss.Color("#ffaf5f")
	cyan := lipgloss.Color("#5fd7d7")
	red := lipgloss.Color("#ff5f5f")

	return []launcherGroup{
		{
			Icon:   icons.System,
			Title:  "SYSTEM",
			Accent: blue,
			Items: []launcherItem{
				{Key: "1", Title: "System Monitor", Enabled: true, LaunchArgs: []string{"sysmon"}, CommandPreview: "cliverse sysmon --watch", ActiveFlag: "--watch", OtherFlags: []string{"--json"}},
				{Key: "2", Title: "Disk Analyzer", Enabled: true, LaunchArgs: []string{"disk", "tui", "."}, CommandPreview: "cliverse disk tui .", ActiveFlag: "--size=apparent", OtherFlags: []string{"--threads=8"}},
				{Key: "3", Title: "Processes", PlaceholderDir: "features/system/processes"},
				{Key: "4", Title: "TUI Metrics", PlaceholderDir: "features/system/tui-metrics"},
			},
		},
		{
			Icon:   icons.Files,
			Title:  "FILES",
			Accent: yellow,
			Items: []launcherItem{
				{Key: "F", Title: "File Manager", PlaceholderDir: "features/files/file-manager"},
				{Key: "D", Title: "Duplicates", PlaceholderDir: "features/files/duplicates"},
				{Key: "J", Title: "Junk Cleanup", PlaceholderDir: "features/files/junk-cleanup"},
				{Key: "C", Title: "Classifier", PlaceholderDir: "features/files/classifier"},
			},
		},
		{
			Icon:   icons.Dev,
			Title:  "DEV",
			Accent: cyan,
			Items: []launcherItem{
				{Key: "E", Title: "Editor", PlaceholderDir: "features/dev/editor"},
				{Key: "G", Title: "Git Multi-Repo", PlaceholderDir: "features/dev/git-multi-repo"},
				{Key: "A", Title: "API Client", PlaceholderDir: "features/dev/api-client"},
				{Key: "S", Title: "SQL Console", PlaceholderDir: "features/dev/sql-console"},
			},
		},
		{
			Icon:   icons.Tools,
			Title:  "TOOLS",
			Accent: red,
			Items: []launcherItem{
				{Key: "O", Title: "Omnishell", PlaceholderDir: "features/tools/omnishell"},
				{Key: "V", Title: "Vault", PlaceholderDir: "features/tools/vault"},
				{Key: "B", Title: "Brew Manager", PlaceholderDir: "features/tools/brew-manager"},
				{Key: "P", Title: "Personal Hub", PlaceholderDir: "features/tools/personal-hub"},
			},
		},
	}
}

func buildShortcutMap(groups []launcherGroup) map[string]shortcutLoc {
	m := make(map[string]shortcutLoc)
	for col, group := range groups {
		for row, item := range group.Items {
			m[strings.ToLower(item.Key)] = shortcutLoc{Col: col, Row: row}
		}
	}
	return m
}

func loadLauncherInfo() launcherInfo {
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	host = strings.TrimSuffix(host, ".local")

	usr := os.Getenv("USER")
	if current, err := user.Current(); err == nil && current.Username != "" {
		usr = current.Username
	}
	if usr == "" {
		usr = "developer"
	}

	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}

	uptime := "unavailable"
	if seconds, err := hostinfo.Uptime(); err == nil {
		uptime = formatUptime(seconds)
	}

	return launcherInfo{
		Host:   host,
		User:   usr,
		CWD:    compactHomePath(cwd),
		Uptime: uptime,
	}
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

func truncatePath(path string, maxLen int) string {
	clean := filepath.Clean(path)
	if len(clean) <= maxLen {
		return clean
	}
	return "..." + clean[len(clean)-maxLen+3:]
}

func compactHomePath(path string) string {
	clean := filepath.Clean(path)
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		home = filepath.Clean(home)
		if clean == home {
			return "~"
		}
		if strings.HasPrefix(clean, home+string(os.PathSeparator)) {
			return "~/" + strings.TrimPrefix(clean, home+string(os.PathSeparator))
		}
	}
	return clean
}

func clipNoEllipsis(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 0 {
		return ""
	}
	return string(runes[:max])
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:1])
	}
	return string(runes[:max-1]) + "…"
}

func fl(line string, width int, bg lipgloss.Color) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line
	}
	return line + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-lineWidth))
}

func frameLine(line string, width, padX int, bg lipgloss.Color) string {
	contentWidth := maxInt(1, width-(padX*2))
	return sp(padX, bg) + fl(line, contentWidth, bg) + sp(padX, bg)
}

func centerLine(line string, width int, bg lipgloss.Color) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line
	}
	left := (width - lineWidth) / 2
	right := width - lineWidth - left
	return sp(left, bg) + line + sp(right, bg)
}

func sp(n int, bg lipgloss.Color) string {
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n))
}

func verticalSpacer(width, height int, bg lipgloss.Color) string {
	lines := make([]string, maxInt(1, height))
	for i := range lines {
		lines[i] = sp(width, bg)
	}
	return strings.Join(lines, "\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func distributeWidths(total, count, minWidth int) []int {
	if count <= 0 {
		return nil
	}
	if total < count*minWidth {
		total = count * minWidth
	}

	base := total / count
	remainder := total % count
	widths := make([]int, count)
	for i := range widths {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
