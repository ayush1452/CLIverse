package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderDialog overlays a centered confirmation modal on top of the base content.
func (m Model) renderDialog(base string) string {
	btnYes := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Success).
		Render("[y] Confirm")
	btnNo := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Error).
		Render("[N] Cancel")
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, btnYes, "   ", btnNo)

	body := lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(m.theme.Warning).Render("Confirm Action"),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Foreground).Render(m.dialogMsg),
		"",
		buttons,
	)

	box := m.theme.DialogBox.Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(m.theme.Subtle),
	)
}

// renderSearchBox renders an inline search widget shown in the command bar area.
func (m Model) renderSearchBox() string {
	prompt := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Highlight).Render("/")
	cursor := lipgloss.NewStyle().Foreground(m.theme.Foreground).Render(m.searchQuery + "█")

	matchInfo := ""
	if len(m.searchQuery) >= 2 {
		matchInfo = lipgloss.NewStyle().Foreground(m.theme.Subtle).
			Render(fmt.Sprintf("  (%d matches)", len(m.treeFlat)))
	}

	hint := lipgloss.NewStyle().Foreground(m.theme.Subtle).Render("  <esc> clear  <enter> jump")
	content := prompt + " " + cursor + matchInfo + hint

	pad := maxInt(0, m.width-lipgloss.Width(content))
	return m.theme.CommandBar.Width(m.width).Render(content + strings.Repeat(" ", pad))
}
