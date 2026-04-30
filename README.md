# zenbox disk

<div align="center">

**Terminal-native disk usage analyzer with actionable insights**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

A modern disk usage analyzer that combines the power of `ncdu`/`gdu` with macOS Storage-like insights, duplicate detection, junk cleanup — all in a beautiful TUI.

## Features

- 🚀 **Fast parallel scanning** — optimized for SSDs
- 📊 **Category breakdown** — Applications, Developer, Documents, Photos, etc.
- 🔁 **Duplicate detection** — size → hash pipeline for accuracy
- ♻️ **Junk detection** — node_modules, build dirs, caches with safety tiers
- 🎨 **Beautiful TUI** — Tree view, Overview with charts
- 📝 **Scriptable CLI** — JSON output for automation
- 🔒 **Safety-first** — Staged deletions, trash support

## Installation

```bash
# Build from source
git clone https://github.com/ayush/zenbox
cd zenbox
make build

# Or install directly
go install github.com/ayush/zenbox@latest
```

## Usage

### Quick Start

```bash
# Scan current directory
zenbox disk

# Show top 20 directories
zenbox disk -t 20

# Launch interactive TUI
zenbox disk tui

# Find duplicates
zenbox disk duplicates

# Find cleanable junk
zenbox disk junk
```

### CLI Mode

```bash
# Default summary output
zenbox disk /path/to/scan

# Show top files instead of directories
zenbox disk top . --files

# JSON output for scripting
zenbox disk . --json

# Find duplicates over 10MB
zenbox disk duplicates . --min-size 10M

# Find developer junk (node_modules, build, etc.)
zenbox disk junk . --profile dev
```

### TUI Mode

```bash
# Launch tree view
zenbox disk tui .

# Start in overview mode
zenbox disk overview .
```

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate up/down |
| `Enter` | Expand/collapse directory |
| `h` or `←` | Collapse or go to parent |
| `x` | Mark/unmark item |
| `a` | Toggle allocated/apparent size |
| `O` | Switch to Overview |
| `?` | Show help |
| `q` | Quit |

## Architecture

```
zenbox/
├── cmd/disk/          # CLI commands (Cobra)
├── core/
│   ├── model/         # Data structures
│   ├── scan/          # Filesystem scanner
│   └── classify/      # Category classification
└── tui/
    ├── app/           # Bubble Tea application
    └── theme/         # Lipgloss themes
```

## Roadmap

### V1 (Current) ✅
- Fast parallel scanner
- CLI with top/duplicates/junk
- TUI Tree & Overview views
- Category classification

### V1.5 (Planned)
- Scan diff mode ("what changed?")
- Command palette (`:`)
- Export/import scans
- Duplicates TUI screen
- Junk TUI screen

### V2 (Future)
- Dedupe via hardlinks/reflinks
- Cleanup recommendations
- Huge-tree scaling mode

## Credits

Inspired by:
- [ncdu](https://dev.yorhel.nl/ncdu) — Classic interactive disk usage
- [gdu](https://github.com/dundee/gdu) — Fast Go disk analyzer
- [dua-cli](https://github.com/Byron/dua-cli) — Deletion safety
- [dust](https://github.com/bootandy/dust) — Quick clarity

Built with:
- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Styling

## License

MIT License - see [LICENSE](LICENSE) for details.
