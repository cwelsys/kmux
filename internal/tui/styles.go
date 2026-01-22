package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette
var (
	// Core colors
	blue     = lipgloss.Color("#89b4fa") // primary
	lavender = lipgloss.Color("#b4befe") // accent
	green    = lipgloss.Color("#a6e3a1") // success
	peach    = lipgloss.Color("#fab387") // warning

	// Neutral tones
	subtext1 = lipgloss.Color("#bac2de")
	subtext0 = lipgloss.Color("#a6adc8")
	overlay1 = lipgloss.Color("#7f849c")
	overlay0 = lipgloss.Color("#6c7086")
	surface1 = lipgloss.Color("#45475a")
)

var (
	// Theme aliases
	primaryColor = blue
	successColor = green

	// Borders
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface1)

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	// List items
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(subtext0)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(primaryColor).
				Bold(true)

	// Status indicators
	runningIndicator = lipgloss.NewStyle().
				Foreground(successColor).
				SetString("●")

	savedIndicator = lipgloss.NewStyle().
			Foreground(overlay0).
			SetString("○")

	projectIndicator = lipgloss.NewStyle().
				Foreground(peach).
				SetString("◆")

	// Section header style
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(overlay1).
				Bold(true)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(overlay1).
			Padding(1, 2)

	// Preview pane
	previewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lavender)

	previewInfoStyle = lipgloss.NewStyle().
				Foreground(subtext1)

	// Dimmed text
	dimStyle = lipgloss.NewStyle().
			Foreground(overlay0)
)
