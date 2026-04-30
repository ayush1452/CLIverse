# CLIverse

<div align="center">

**Terminal-native developer toolkit with disk and system insights**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)

</div>

---

CLIverse is a collection of terminal-native tools for developers and power users.

Current tools:

- `disk` for scanning disk usage, finding duplicates, and identifying junk
- `sysmon` for a real-time system monitor in the terminal

## Features

- Fast parallel disk scanning optimized for SSDs
- Category breakdowns for developer and personal storage analysis
- Duplicate detection with a size-to-hash verification pipeline
- Junk detection for caches, build artifacts, and common cleanup targets
- Real-time system monitor for CPU, memory, disk I/O, and network activity
- Terminal-first UI built with Bubble Tea and Lipgloss
- Scriptable CLI with JSON output for automation

## Installation

```bash
# Build from source
git clone https://github.com/ayush1452/CLIverse
cd CLIverse
make build

# Or install directly
go install github.com/ayush1452/CLIverse@latest
```

The installed binary is `cliverse`.

## Usage

### Quick start

```bash
# Disk scan
cliverse disk

# Disk TUI
cliverse disk tui

# Duplicate finder
cliverse disk duplicates

# Junk detection
cliverse disk junk

# System monitor
cliverse sysmon
```

### Disk command

```bash
# Default summary output
cliverse disk /path/to/scan

# Show top files instead of directories
cliverse disk top . --files

# JSON output for scripting
cliverse disk . --json

# Find duplicates over 10MB
cliverse disk duplicates . --min-size 10M

# Find developer junk (node_modules, build, etc.)
cliverse disk junk . --profile dev
```

### TUI commands

```bash
# Launch disk tree view
cliverse disk tui .

# Start disk TUI in overview mode
cliverse disk overview .

# Launch the system monitor
cliverse sysmon
```

**Disk TUI keyboard shortcuts:**

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

```text
CLIverse/
├── cmd/disk/          # Disk CLI commands (Cobra)
├── cmd/sysmon/        # System monitor command
├── core/
│   ├── model/         # Data structures
│   ├── scan/          # Filesystem scanner
│   └── classify/      # Category classification
├── gui/sysmon/        # System monitor frontend assets
└── tui/
    ├── app/           # Disk TUI application
    ├── sysmon/        # System monitor TUI
    └── theme/         # Lipgloss themes
```

## Roadmap

### V1 (Current) ✅
- Fast parallel scanner
- CLI with top/duplicates/junk
- TUI Tree and Overview views
- Category classification
- Real-time system monitor

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
- [ncdu](https://dev.yorhel.nl/ncdu) - Classic interactive disk usage
- [gdu](https://github.com/dundee/gdu) - Fast Go disk analyzer
- [dua-cli](https://github.com/Byron/dua-cli) - Deletion safety
- [dust](https://github.com/bootandy/dust) - Quick clarity

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
