// Package sysmon implements the system-monitor TUI panel.
package sysmon

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	core "github.com/ayush1452/CLIverse/core/sysmon"
	gui "github.com/ayush1452/CLIverse/gui/sysmon"
	"github.com/ayush1452/CLIverse/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

// ─── constants ────────────────────────────────────────────────────────────────

const (
	histLen      = 60
	maxProcs     = 18
	tickInterval = time.Second
)

type sortBy int

const (
	byCPU sortBy = iota
	byMem
)

// ─── messages ─────────────────────────────────────────────────────────────────

type tickMsg time.Time
type snapMsg *core.Snapshot
type guiReadyMsg string // carries the URL once the GUI server is up

// ─── model ────────────────────────────────────────────────────────────────────

// Model is the Bubble Tea model for the system monitor view.
type Model struct {
	collector *core.Collector
	snap      *core.Snapshot
	hist      []float64 // CPU% history for sparkline
	sortMode  sortBy
	cursor    int
	w, h      int
	guiURL    string // non-empty once GUI server is running
}

// New returns an initialised Model.
func New() Model {
	return Model{
		collector: core.New(),
		hist:      make([]float64, 0, histLen),
	}
}

// ─── tea interface ────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(doCollect(m.collector), nextTick())
}

func nextTick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func doCollect(c *core.Collector) tea.Cmd {
	return func() tea.Msg { return snapMsg(c.Collect()) }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height

	case tickMsg:
		return m, doCollect(m.collector)

	case snapMsg:
		s := (*core.Snapshot)(msg)
		m.snap = s
		m.hist = append(m.hist, s.CPUTotal)
		if len(m.hist) > histLen {
			m.hist = m.hist[len(m.hist)-histLen:]
		}
		return m, nextTick()

	case guiReadyMsg:
		m.guiURL = string(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			if m.sortMode == byCPU {
				m.sortMode = byMem
			} else {
				m.sortMode = byCPU
			}
		case "v":
			if m.guiURL == "" {
				return m, launchGUICmd()
			}
			// Already running — re-open the browser.
			return m, func() tea.Msg {
				gui.OpenBrowser(m.guiURL)
				return nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.snap != nil {
				limit := clamp(len(m.snap.Procs), 0, maxProcs) - 1
				if m.cursor < limit {
					m.cursor++
				}
			}
		}
	}
	return m, nil
}

// launchGUICmd starts the embedded HTTP/SSE server and opens the browser.
func launchGUICmd() tea.Cmd {
	return func() tea.Msg {
		srv, err := gui.NewServer()
		if err != nil {
			return nil
		}
		go srv.Start()                     //nolint:errcheck
		time.Sleep(150 * time.Millisecond) // wait for the listener to bind
		url := srv.URL()
		gui.OpenBrowser(url)
		return guiReadyMsg(url)
	}
}

func (m Model) View() string {
	if m.w == 0 || m.snap == nil {
		return "\n  Collecting system metrics…  Press q to quit.\n"
	}

	th := theme.Get("dark")
	s := m.snap
	w := m.w

	var buf strings.Builder
	writeln := func(line string) { buf.WriteString(line); buf.WriteByte('\n') }

	// ── Header ────────────────────────────────────────────────────────────────
	title := "  CLIverse · System Monitor"
	ts := s.Time.Format("15:04:05  ")
	pad := w - lipgloss.Width(title) - lipgloss.Width(ts)
	if pad < 0 {
		pad = 0
	}
	writeln(
		th.Title.Render(title) +
			strings.Repeat(" ", pad) +
			th.Help.Render(ts),
	)
	writeln(th.Help.Render(strings.Repeat("─", w)))
	writeln("")

	// ── CPU ───────────────────────────────────────────────────────────────────
	cpuBarW := w - 20 // "  CPU  " 7 + "  100.0%" 8 + safety 5
	if cpuBarW < 10 {
		cpuBarW = 10
	}
	cc := levelColor(s.CPUTotal)
	writeln(fmt.Sprintf("  CPU  %s  %s",
		styledBar(s.CPUTotal, cpuBarW, cc),
		lipgloss.NewStyle().Foreground(cc).Bold(true).Render(fmt.Sprintf("%5.1f%%", s.CPUTotal)),
	))

	// Sparkline — same indent as bar
	spark := lipgloss.NewStyle().Foreground(th.Subtle).Render(sparkline(m.hist, cpuBarW))
	writeln("       " + spark)
	writeln("")

	// Per-core grid
	if len(s.CPUCores) > 0 {
		writeln(coreGrid(s.CPUCores, w, th))
	}

	// ── Memory ────────────────────────────────────────────────────────────────
	writeln(th.Help.Render(strings.Repeat("─", w)))
	writeln("")

	memBarW := w - 38 // "  MEM  " 7 + value ~28 + spacing
	if memBarW < 10 {
		memBarW = 10
	}

	mc := levelColor(s.MemPct)
	writeln(fmt.Sprintf("  MEM  %s  %s / %s  %s",
		styledBar(s.MemPct, memBarW, mc),
		lipgloss.NewStyle().Foreground(mc).Render(fmt.Sprintf("%8s", humanize.IBytes(s.MemUsed))),
		fmt.Sprintf("%-8s", humanize.IBytes(s.MemTotal)),
		lipgloss.NewStyle().Foreground(mc).Render(fmt.Sprintf("%5.1f%%", s.MemPct)),
	))

	if s.SwapTotal > 0 {
		sc := levelColor(s.SwapPct)
		writeln(fmt.Sprintf("  SWP  %s  %s / %s  %s",
			styledBar(s.SwapPct, memBarW, sc),
			lipgloss.NewStyle().Foreground(sc).Render(fmt.Sprintf("%8s", humanize.IBytes(s.SwapUsed))),
			fmt.Sprintf("%-8s", humanize.IBytes(s.SwapTotal)),
			lipgloss.NewStyle().Foreground(sc).Render(fmt.Sprintf("%5.1f%%", s.SwapPct)),
		))
	} else {
		writeln("  SWP  " + th.Help.Render("disabled"))
	}

	// ── I/O ───────────────────────────────────────────────────────────────────
	writeln("")
	writeln(th.Help.Render(strings.Repeat("─", w)))
	writeln("")

	hi := lipgloss.Color("#7aa2f7") // blue  – read / receive
	lo := lipgloss.Color("#bb9af7") // purple – write / send

	writeln(fmt.Sprintf("  DISK  Read %s/s   Write %s/s",
		lipgloss.NewStyle().Foreground(hi).Render(fmt.Sprintf("%10s", humanize.IBytes(uint64(s.DiskReadBPS)))),
		lipgloss.NewStyle().Foreground(lo).Render(fmt.Sprintf("%10s", humanize.IBytes(uint64(s.DiskWriteBPS)))),
	))
	writeln(fmt.Sprintf("  NET   ↓    %s/s     ↑   %s/s",
		lipgloss.NewStyle().Foreground(hi).Render(fmt.Sprintf("%10s", humanize.IBytes(uint64(s.NetRecvBPS)))),
		lipgloss.NewStyle().Foreground(lo).Render(fmt.Sprintf("%10s", humanize.IBytes(uint64(s.NetSentBPS)))),
	))

	// ── Processes ─────────────────────────────────────────────────────────────
	writeln("")
	writeln(th.Help.Render(strings.Repeat("─", w)))

	sortLabel := "CPU"
	if m.sortMode == byMem {
		sortLabel = "MEM"
	}
	writeln("  " + th.PanelTitle.Render("PROCESSES") +
		th.Help.Render(fmt.Sprintf("  sort: [%s]", sortLabel)))

	writeln(th.Help.Render(fmt.Sprintf(
		"  %-7s  %-24s  %7s  %10s", "PID", "NAME", "CPU%", "MEMORY",
	)))

	procs := orderedProcs(s.Procs, m.sortMode)
	shown := clamp(len(procs), 0, maxProcs)

	for i := 0; i < shown; i++ {
		p := procs[i]
		prefix := "  "
		rowStyle := lipgloss.NewStyle().Foreground(th.Foreground)
		if i == m.cursor {
			prefix = "> "
			rowStyle = th.Selected
		}
		cpuStr := fmt.Sprintf("%6.1f%%", p.CPU)
		memStr := fmt.Sprintf("%10s", humanize.IBytes(p.MemRSS))
		row := fmt.Sprintf("%s%-7d  %-24s  %s  %s",
			prefix,
			p.PID,
			truncate(p.Name, 24),
			lipgloss.NewStyle().Foreground(levelColor(p.CPU)).Render(cpuStr),
			memStr,
		)
		writeln(rowStyle.Render(row))
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	writeln("")
	writeln(th.Help.Render(strings.Repeat("─", w)))
	writeln(th.Help.Render("  [q] Quit   [p] Toggle sort (CPU/MEM)   [↑↓ / jk] Navigate   [v] Open GUI"))
	if m.guiURL != "" {
		writeln(lipgloss.NewStyle().Foreground(th.Success).
			Render(fmt.Sprintf("  ◉ GUI live → %s", m.guiURL)))
	}

	return buf.String()
}

// ─── rendering helpers ────────────────────────────────────────────────────────

// styledBar returns a width-character progress bar coloured by fillColor.
func styledBar(pct float64, width int, fillColor lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	n := int(math.Round(pct / 100.0 * float64(width)))
	n = clamp(n, 0, width)
	filled := lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat("█", n))
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("#2d2d44")).Render(strings.Repeat("░", width-n))
	return filled + empty
}

// levelColor maps a 0-100 percentage to a green/yellow/red colour.
func levelColor(pct float64) lipgloss.Color {
	switch {
	case pct >= 80:
		return lipgloss.Color("#f7768e") // red
	case pct >= 50:
		return lipgloss.Color("#e0af68") // yellow
	default:
		return lipgloss.Color("#9ece6a") // green
	}
}

// sparkline converts a slice of 0-100 values into a block-character line.
func sparkline(vals []float64, width int) string {
	if width <= 0 {
		return ""
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder
	// pad left if we have fewer samples than width
	if len(vals) < width {
		sb.WriteString(strings.Repeat("▁", width-len(vals)))
	}
	start := 0
	if len(vals) > width {
		start = len(vals) - width
	}
	for _, v := range vals[start:] {
		idx := int(v / 100.0 * float64(len(blocks)-1))
		idx = clamp(idx, 0, len(blocks)-1)
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}

// coreGrid lays out per-core usage bars in a responsive grid.
func coreGrid(cores []float64, totalW int, th theme.Theme) string {
	const cellW = 18 // "  0  ████░░  42%" ≈ 18 chars
	cols := (totalW - 2) / cellW
	if cols < 1 {
		cols = 1
	}

	var sb strings.Builder
	for i, pct := range cores {
		cc := levelColor(pct)
		barW := 6
		cell := fmt.Sprintf("  %2d  %s %3.0f%%",
			i,
			styledBar(pct, barW, cc),
			pct,
		)
		sb.WriteString(lipgloss.NewStyle().Foreground(cc).Render(cell))
		if (i+1)%cols == 0 {
			sb.WriteByte('\n')
		}
	}
	if len(cores)%cols != 0 {
		sb.WriteByte('\n')
	}
	return sb.String()
}

// orderedProcs returns a sorted copy of procs according to sortMode.
func orderedProcs(procs []core.ProcInfo, mode sortBy) []core.ProcInfo {
	out := make([]core.ProcInfo, len(procs))
	copy(out, procs)
	if mode == byMem {
		sort.Slice(out, func(i, j int) bool { return out[i].MemRSS > out[j].MemRSS })
	}
	// byCPU is already sorted by the collector
	return out
}

// truncate shortens s to max rune count, appending "…" if needed.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
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
