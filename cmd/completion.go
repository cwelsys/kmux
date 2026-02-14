package cmd

import (
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

bash:
  eval "$(kmux completion bash)"

zsh:
  kmux completion zsh > ~/.local/share/zsh/site-functions/_kmux

fish:
  kmux completion fish > ~/.config/fish/completions/kmux.fish
`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
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
