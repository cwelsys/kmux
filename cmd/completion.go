package cmd

import (
	"bytes"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

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
