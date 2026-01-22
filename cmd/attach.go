package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var (
	attachLayout string
	attachCWD    string
	attachHost   string
)

var attachCmd = &cobra.Command{
	Use:     "attach [name | path [name]]",
	Aliases: []string{"a"},
	Short:   "Attach to a session",
	Long: `Attach to an existing session or create a new one.

Examples:
  kmux a                    # session named after current directory
  kmux a myproject          # session named "myproject"
  kmux a ~/src/foo          # session "foo" starting in ~/src/foo
  kmux a ~/src/foo bar      # session "bar" starting in ~/src/foo
  kmux a myproject --host devbox  # remote session on devbox`,
	Args:              cobra.RangeArgs(0, 2),
	ValidArgsFunction: completeSessionNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, cwd, err := resolveAttachArgs(args, attachCWD)
		if err != nil {
			return err
		}

		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		s := state.New()

		// Determine which host to use
		host := attachHost
		if host == "" {
			// Auto-detect: find which host(s) have a session with this name
			host = autoDetectSessionHost(s, name)
		}

		result, err := manager.AttachSession(s, manager.AttachOpts{
			Name:         name,
			Host:         host,
			CWD:          cwd,
			Layout:       attachLayout,
			BeforePinned: true,
		})
		if err != nil {
			return err
		}

		// Print result
		switch result.Action {
		case "focused":
			fmt.Printf("Focused existing session: %s\n", result.SessionName)
		default:
			if result.Host != "local" {
				fmt.Printf("Attached to session: %s@%s\n", result.SessionName, result.Host)
			} else {
				fmt.Printf("Attached to session: %s\n", result.SessionName)
			}
		}
		return nil
	},
}

// isPath returns true if the argument looks like a path (starts with /, ~, or .)
func isPath(arg string) bool {
	return strings.HasPrefix(arg, "/") ||
		strings.HasPrefix(arg, "~") ||
		strings.HasPrefix(arg, ".")
}

// expandPath expands ~ to home directory and resolves to absolute path.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}

// resolveAttachArgs determines session name and cwd from command arguments.
// Args patterns:
//   - 0 args: name = cwd basename, cwd = current
//   - 1 arg (path): name = path basename, cwd = path
//   - 1 arg (name): name = arg, cwd = current
//   - 2 args: name = args[1], cwd = args[0] (path)
func resolveAttachArgs(args []string, cwdOverride string) (name, cwd string, err error) {
	// Start with current directory
	cwd, err = os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get cwd: %w", err)
	}

	switch len(args) {
	case 0:
		// No args: derive name from cwd
		name = filepath.Base(cwd)

	case 1:
		if isPath(args[0]) {
			// Single path arg: derive name from path, use path as cwd
			cwd, err = expandPath(args[0])
			if err != nil {
				return "", "", fmt.Errorf("expand path: %w", err)
			}
			name = filepath.Base(cwd)
		} else {
			// Single name arg: use as session name
			name = args[0]
		}

	case 2:
		// Two args: path + name
		cwd, err = expandPath(args[0])
		if err != nil {
			return "", "", fmt.Errorf("expand path: %w", err)
		}
		name = args[1]
	}

	// Override cwd if flag provided
	if cwdOverride != "" {
		cwd, err = expandPath(cwdOverride)
		if err != nil {
			return "", "", fmt.Errorf("expand cwd override: %w", err)
		}
	}

	return name, cwd, nil
}

func init() {
	attachCmd.Flags().StringVarP(&attachLayout, "layout", "l", "", "create session from layout template")
	attachCmd.Flags().StringVarP(&attachCWD, "cwd", "C", "", "working directory for panes (overrides path)")
	attachCmd.Flags().StringVarP(&attachHost, "host", "H", "", "remote host (SSH alias from config)")
	rootCmd.AddCommand(attachCmd)
}
