package tui

import (
	"fmt"
	"os"
	"strings"

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

	// Wait for window size before rendering
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.viewHelp()
	}

	if m.renameMode {
		selectedName := ""
		if m.cursor < len(m.sessions) {
			selectedName = m.sessions[m.cursor].Name
		}
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			titleStyle.Render("Rename Session"),
			fmt.Sprintf("Renaming: %s", selectedName),
			m.renameInput.View(),
		)
	}

	// Calculate pane widths
	listWidth := m.width/2 - 2
	previewWidth := m.width - listWidth - 4
	contentHeight := m.height - 6 // account for borders, title, and help bar padding

	// Build panes
	listPane := m.viewSessionList(listWidth, contentHeight)
	previewPane := m.viewPreview(previewWidth, contentHeight)

	// Join panes horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)

	// Add title and help bar
	title := titleStyle.Render("kmux")
	helpBar := m.viewHelpBar()

	// Confirmation overlays
	if m.confirmKill {
		content = m.viewConfirmKill(m.width, m.height)
	} else if m.confirmIgnore {
		content = m.viewConfirmIgnore(m.width, m.height)
	} else if m.launchMode {
		content = m.viewLaunchModal(m.width, m.height)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content, helpBar)
}

func (m Model) viewSessionList(width, height int) string {
	var b strings.Builder

	filterActive := m.filterInput.Value() != ""

	if filterActive {
		// Filtered view - show all matching items in ranked order
		if len(m.items) == 0 {
			b.WriteString(dimStyle.Render("  No matches") + "\n")
		} else {
			for i, item := range m.items {
				line := m.renderItem(item, width)
				if i == m.cursor {
					b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
				} else {
					b.WriteString(itemStyle.Render(line) + "\n")
				}
			}
		}
	} else {
		// Normal view - show sections
		itemIdx := 0

		// Sessions section
		b.WriteString(sectionHeaderStyle.Render("Sessions") + "\n")

		if len(m.sessions) == 0 {
			b.WriteString(dimStyle.Render("  No sessions") + "\n")
		} else {
			for _, s := range m.sessions {
				line := m.renderItem(s, width)
				if itemIdx == m.cursor {
					b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
				} else {
					b.WriteString(itemStyle.Render(line) + "\n")
				}
				itemIdx++
			}
		}

		// Projects section
		if len(m.projects) > 0 {
			b.WriteString("\n")
			b.WriteString(sectionHeaderStyle.Render("Projects") + "\n")

			for _, p := range m.projects {
				line := m.renderItem(p, width)
				if itemIdx == m.cursor {
					b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
				} else {
					b.WriteString(itemStyle.Render(line) + "\n")
				}
				itemIdx++
			}
		}
	}

	style := borderStyle.Width(width).Height(height)
	return style.Render(b.String())
}

func (m Model) renderItem(item Item, width int) string {
	if item.Type == ItemSession {
		indicator := savedIndicator.String()
		if item.Status == "active" || item.Status == "detached" {
			indicator = runningIndicator.String()
		}
		name := fmt.Sprintf("%s %s", indicator, item.Name)
		panes := fmt.Sprintf("(%d)", item.PaneCount)
		return fmt.Sprintf("%-*s %s", width-8, name, panes)
	}
	// Project
	indicator := projectIndicator.String()
	name := fmt.Sprintf("%s %s", indicator, item.Name)
	return fmt.Sprintf("%-*s", width-6, name)
}

func (m Model) viewPreview(width, height int) string {
	var b strings.Builder

	item := m.SelectedItem()
	if item == nil {
		b.WriteString(dimStyle.Render("No item selected"))
	} else if item.Type == ItemSession {
		b.WriteString(previewTitleStyle.Render(item.Name) + "\n\n")

		b.WriteString(previewInfoStyle.Render(fmt.Sprintf("status: %s", item.Status)) + "\n")
		b.WriteString(previewInfoStyle.Render(fmt.Sprintf("panes:  %d", item.PaneCount)) + "\n")

		if item.CWD != "" {
			// Shorten home directory
			cwd := item.CWD
			if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(cwd, home) {
				cwd = "~" + cwd[len(home):]
			}
			b.WriteString(previewInfoStyle.Render(fmt.Sprintf("cwd:    %s", cwd)) + "\n")
		}

	} else {
		// Project
		b.WriteString(previewTitleStyle.Render(item.Name) + "\n\n")

		// Shorten home directory in path
		path := item.Path
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
			path = "~" + path[len(home):]
		}
		b.WriteString(previewInfoStyle.Render(fmt.Sprintf("path: %s", path)) + "\n\n")
		b.WriteString(dimStyle.Render("No session - press enter to create") + "\n")
	}

	style := borderStyle.Width(width).Height(height)
	return style.Render(b.String())
}

func (m Model) viewHelpBar() string {
	if m.filterMode {
		return helpStyle.Render("/ " + m.filterInput.View() + "  [enter] keep  [esc] clear")
	}
	if filter := m.filterInput.Value(); filter != "" {
		return helpStyle.Render(fmt.Sprintf("/%s  [/] edit  [esc] clear  [enter] attach  [?] help  [q] quit", filter))
	}
	// Show 'l' option when a project is selected
	if m.SelectedProject() != nil {
		return helpStyle.Render("[enter] create  [l] options  [z] browse  [d] hide  [?] help  [q] quit")
	}
	return helpStyle.Render("[enter] attach  [z] browse  [d] delete  [r] rename  [?] help  [q] quit")
}

func (m Model) viewHelp() string {
	help := `
  kmux - Session Manager

  Navigation:
    ↑/k       Move up
    ↓/j       Move down
    enter     Attach/create session
    l         Launch with options (projects)
    z         Browse filesystem
    d         Delete session / hide project
    r         Rename session
    R         Refresh list
    /         Filter (fuzzy search)
    ?         Toggle help
    q/esc     Quit (esc clears filter first)

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

func (m Model) viewConfirmIgnore(width, height int) string {
	project := m.SelectedProject()
	name := ""
	if project != nil {
		name = project.Name
	}
	msg := fmt.Sprintf("Hide project '%s'?\n\nThis adds it to your ignore list.\n\n[y] yes  [n] no", name)
	style := borderStyle.Width(45).Padding(1, 2)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, style.Render(msg))
}

func (m Model) viewLaunchModal(width, height int) string {
	var b strings.Builder

	b.WriteString(previewTitleStyle.Render("Launch Options") + "\n\n")

	// Layout section
	b.WriteString(previewInfoStyle.Render("Layout:") + "\n")
	for i, layout := range m.launchLayouts {
		indicator := "○"
		if i == m.launchCursor {
			indicator = "●"
		}
		style := itemStyle
		if i == m.launchCursor && !m.launchNameFocus {
			style = selectedItemStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("  %s %s", indicator, layout)) + "\n")
	}

	// Name section
	b.WriteString("\n")
	nameLabel := previewInfoStyle.Render("Name:")
	if m.launchNameFocus {
		nameLabel = selectedItemStyle.Render("Name:")
	}
	b.WriteString(nameLabel + "\n")
	b.WriteString("  " + m.launchNameInput.View() + "\n")

	// Help
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[↑/↓] select  [tab] switch  [enter] launch  [esc] cancel"))

	style := borderStyle.Width(45).Padding(1, 2)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, style.Render(b.String()))
}
