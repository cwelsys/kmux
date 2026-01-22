package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"time"

	"github.com/cwel/kmux/internal/state"
	"github.com/spf13/cobra"
)

// completeSessionNames returns session names for shell completion.
func completeSessionNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	s := state.New()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sessions, _ := s.AllSessions(ctx, true)

	seen := make(map[string]bool)
	var names []string
	for _, sess := range sessions {
		if strings.HasPrefix(sess.Name, toComplete) && !seen[sess.Name] {
			seen[sess.Name] = true
			names = append(names, sess.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for kmux.

For zsh, add this to your .zshrc:
  eval "$(kmux completion zsh)"

Or generate a file for zinit/fpath:
  kmux completion zsh > ~/.local/share/zinit/completions/_kmux
`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "zsh":
			// Generate to buffer so we can remove the compdef line
			var buf bytes.Buffer
			if err := rootCmd.GenZshCompletion(&buf); err != nil {
				return err
			}
			// Remove "compdef _kmux kmux" - the #compdef magic comment
			// is sufficient for file-based completions in fpath
			lines := strings.Split(buf.String(), "\n")
			for _, line := range lines {
				if line == "compdef _kmux kmux" {
					continue
				}
				os.Stdout.WriteString(line + "\n")
			}
			return nil
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
