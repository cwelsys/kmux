package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("39")  // cyan
	secondaryColor = lipgloss.Color("243") // gray
	accentColor    = lipgloss.Color("211") // pink
	successColor   = lipgloss.Color("78")  // green

	// Borders
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor)

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	// List items
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(primaryColor).
				Bold(true)

	// Status indicators
	runningIndicator = lipgloss.NewStyle().
				Foreground(successColor).
				SetString("●")

	savedIndicator = lipgloss.NewStyle().
			Foreground(secondaryColor).
			SetString(" ")

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Padding(0, 1)

	// Preview pane
	previewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	previewInfoStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	// Dimmed text
	dimStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)
)
