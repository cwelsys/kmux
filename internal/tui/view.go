package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.showHelp {
		return m.viewHelp()
	}

	// Calculate pane widths
	listWidth := m.width/2 - 2
	previewWidth := m.width - listWidth - 4
	contentHeight := m.height - 4 // account for borders and help bar

	// Build panes
	listPane := m.viewSessionList(listWidth, contentHeight)
	previewPane := m.viewPreview(previewWidth, contentHeight)

	// Join panes horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)

	// Add title and help bar
	title := titleStyle.Render("kmux")
	helpBar := m.viewHelpBar()

	// Confirmation overlay
	if m.confirmKill {
		content = m.viewConfirmKill(m.width, m.height)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content, helpBar)
}

func (m Model) viewSessionList(width, height int) string {
	var b strings.Builder

	b.WriteString(dimStyle.Render("Sessions") + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", width-4)) + "\n")

	if len(m.sessions) == 0 {
		b.WriteString(dimStyle.Render("  No sessions"))
	}

	for i, s := range m.sessions {
		indicator := savedIndicator.String()
		if s.HasRunning {
			indicator = runningIndicator.String()
		}

		name := fmt.Sprintf("%s %s", indicator, s.Name)
		panes := fmt.Sprintf("(%d)", s.PaneCount)

		line := fmt.Sprintf("%-*s %s", width-8, name, panes)

		if i == m.cursor {
			b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
		} else {
			b.WriteString(itemStyle.Render(line) + "\n")
		}
	}

	style := borderStyle.Width(width).Height(height)
	return style.Render(b.String())
}

func (m Model) viewPreview(width, height int) string {
	var b strings.Builder

	if len(m.sessions) == 0 || m.cursor >= len(m.sessions) {
		b.WriteString(dimStyle.Render("No session selected"))
	} else {
		s := m.sessions[m.cursor]

		b.WriteString(previewTitleStyle.Render(s.Name) + "\n")
		b.WriteString(previewInfoStyle.Render(fmt.Sprintf("saved: %s", timeAgo(s.SavedAt))) + "\n\n")

		for i, tab := range s.Tabs {
			tabLine := fmt.Sprintf("[tab%d] %s", i, tab.Title)
			if len(tab.Windows) > 1 {
				var cmds []string
				for _, w := range tab.Windows {
					if w.Command != "" {
						cmds = append(cmds, w.Command)
					} else {
						cmds = append(cmds, "shell")
					}
				}
				tabLine += " │ " + strings.Join(cmds, " │ ")
			} else if len(tab.Windows) == 1 && tab.Windows[0].Command != "" {
				tabLine += ": " + tab.Windows[0].Command
			}
			b.WriteString(tabLine + "\n")
		}
	}

	style := borderStyle.Width(width).Height(height)
	return style.Render(b.String())
}

func (m Model) viewHelpBar() string {
	if m.filterMode {
		return helpStyle.Render("Filter: " + m.filterInput.View())
	}
	return helpStyle.Render("[enter] attach  [d] delete  [r] rename  [/] filter  [?] help  [q] quit")
}

func (m Model) viewHelp() string {
	help := `
  kmux - Session Manager

  Navigation:
    ↑/k       Move up
    ↓/j       Move down
    enter     Attach to session
    d         Delete session
    r         Rename session
    /         Filter sessions
    ?         Toggle help
    q/esc     Quit

  Press any key to close this help.
`
	style := borderStyle.Width(50).Padding(1, 2)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, style.Render(help))
}

func (m Model) viewConfirmKill(width, height int) string {
	name := m.SelectedSession()
	msg := fmt.Sprintf("Kill session '%s'?\n\n[y] yes  [n] no", name)
	style := borderStyle.Width(40).Padding(1, 2)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, style.Render(msg))
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
