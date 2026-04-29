package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blai/clean-slate/internal/config"
	"github.com/blai/clean-slate/internal/manifest"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Long: `Generate a shell completion script for cs.

Examples:
  # bash
  source <(cs completion bash)
  # Permanent: write to /etc/bash_completion.d/ or add the line above to ~/.bashrc

  # zsh
  cs completion zsh > "${fpath[1]}/_cs"
  # Or temporarily: source <(cs completion zsh)

  # fish
  cs completion fish | source
  # Permanent: cs completion fish > ~/.config/fish/completions/cs.fish
`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(out, true)
		case "zsh":
			return rootCmd.GenZshCompletion(out)
		case "fish":
			return rootCmd.GenFishCompletion(out, true)
		default:
			return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// Register task-name completion for commands that take a task name.
	// Scans the tasks dir and returns names that exist (have a task.json).
	taskNameCompletion := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cfg := config.Load()
		entries, err := os.ReadDir(cfg.TasksDir)
		if err != nil {
			// Missing tasks dir → no completions, but don't report an error
			// to the shell (that surfaces as an ugly "completion error").
			if os.IsNotExist(err) {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveError
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			// Skip _archive/ and similar
			if e.Name() == "_archive" {
				continue
			}
			if _, err := manifest.Read(filepath.Join(cfg.TasksDir, e.Name())); err == nil {
				names = append(names, e.Name())
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}

	for _, c := range []*cobra.Command{infoCmd, statusCmd, openCmd, cleanCmd, addContextCmd} {
		c.ValidArgsFunction = taskNameCompletion
	}
}
