package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Uses ANSI terminal colors (0-15) so output adapts to the user's terminal theme.
// Styling mirrors charmbracelet/fang's AnsiColorScheme.
var (
	helpDescStyle = lipgloss.NewStyle()

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("4")) // terminal blue (matches fang sections)

	helpCmdNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")) // terminal cyan

	helpCmdDescStyle = lipgloss.NewStyle()

	helpFlagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")) // terminal magenta

	helpFlagDescStyle = lipgloss.NewStyle()

	helpFlagDefaultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("13")) // bright magenta
)

func styledHelp(cmd *cobra.Command, _ []string) {
	var b strings.Builder

	// Description (fang leads with description, not the command name)
	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	if desc != "" {
		b.WriteString("\n  " + helpDescStyle.Render(desc))
		b.WriteString("\n")
	}

	// Usage
	b.WriteString("\n  " + helpSectionStyle.Render("USAGE") + "\n\n")
	usageLine := cmd.UseLine()
	if cmd.HasAvailableSubCommands() {
		usageLine = cmd.CommandPath() + " [command] [flags]"
	}
	b.WriteString("    " + usageLine)
	b.WriteString("\n")

	// Commands
	commands := cmd.Commands()
	var visible []*cobra.Command
	for _, c := range commands {
		if !c.Hidden && c.Name() != "help" && c.Name() != "completion" {
			visible = append(visible, c)
		}
	}

	if len(visible) > 0 {
		b.WriteString("\n  " + helpSectionStyle.Render("COMMANDS") + "\n\n")

		// Find max command name length for alignment
		maxLen := 0
		for _, c := range visible {
			if len(c.Name()) > maxLen {
				maxLen = len(c.Name())
			}
		}

		for _, c := range visible {
			name := helpCmdNameStyle.Render(fmt.Sprintf("    %-*s", maxLen+2, c.Name()))
			desc := helpCmdDescStyle.Render(c.Short)
			b.WriteString(name + desc + "\n")
		}
	}

	// Flags
	flags := cmd.Flags()
	if flags.HasAvailableFlags() {
		b.WriteString("\n  " + helpSectionStyle.Render("FLAGS") + "\n\n")

		// Collect flags and find max length for alignment
		type flagEntry struct {
			name string
			desc string
		}
		var entries []flagEntry
		maxLen := 0

		flags.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			var nameStr string
			if f.Shorthand != "" {
				nameStr = fmt.Sprintf("-%s --%s", f.Shorthand, f.Name)
			} else {
				nameStr = fmt.Sprintf("    --%s", f.Name)
			}

			desc := f.Usage
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "[]" {
				desc += " " + helpFlagDefaultStyle.Render(fmt.Sprintf("(%s)", f.DefValue))
			}

			if len(nameStr) > maxLen {
				maxLen = len(nameStr)
			}
			entries = append(entries, flagEntry{name: nameStr, desc: desc})
		})

		for _, e := range entries {
			name := helpFlagStyle.Render(fmt.Sprintf("    %-*s", maxLen+2, e.name))
			b.WriteString(name + helpFlagDescStyle.Render(e.desc) + "\n")
		}
	}

	fmt.Print(b.String())
}
