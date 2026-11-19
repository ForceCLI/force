package sf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandInfo struct {
	Id      string   `json:"id"`
	Summary string   `json:"summary"`
	Flags   []string `json:"flags"`
}

// listCommands returns the commands of the tree that run something, with
// ids in sf's colon form.
func listCommands(root *cobra.Command) []commandInfo {
	var commands []commandInfo
	var walk func(cmd *cobra.Command, path []string)
	walk = func(cmd *cobra.Command, path []string) {
		for _, child := range cmd.Commands() {
			if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
				continue
			}
			childPath := append(append([]string{}, path...), child.Name())
			// A topic that only holds subcommands has flags of its own
			// only when it is also a command.
			if child.Flags().HasFlags() || !child.HasSubCommands() {
				info := commandInfo{Id: strings.Join(childPath, ":"), Summary: child.Short, Flags: []string{}}
				child.Flags().VisitAll(func(flag *pflag.Flag) {
					if !flag.Hidden {
						info.Flags = append(info.Flags, flag.Name)
					}
				})
				sort.Strings(info.Flags)
				commands = append(commands, info)
			}
			walk(child, childPath)
		}
	}
	walk(root, nil)
	sort.Slice(commands, func(i, j int) bool { return commands[i].Id < commands[j].Id })
	return commands
}

func newCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commands",
		Short: "List all sf commands.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			commands := listCommands(cmd.Root())
			if jsonRequested(cmd) {
				// sf prints this array without the status envelope.
				return writeJSON(cmd.OutOrStdout(), commands)
			}
			rows := make([][]string, 0, len(commands))
			for _, command := range commands {
				rows = append(rows, []string{strings.ReplaceAll(command.Id, ":", " "), command.Summary})
			}
			return emit(cmd, commands, func(w io.Writer) {
				printTable(w, "", []string{"Id", "Summary"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

// newAutocompleteCmd prints how to set up shell completion.  sf completes
// with a cache it builds from its plugins; this mode uses the completion
// scripts cobra generates, so the instructions differ from sf's.
func newAutocompleteCmd() *cobra.Command {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	script := &cobra.Command{
		Use:    "script shell",
		Short:  "Print the completion script for a shell.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			}
			return &Error{Message: fmt.Sprintf("%s is not a supported shell for autocomplete", args[0]), ExitCode: 1}
		},
	}
	cmd := &cobra.Command{
		Use:   "autocomplete [shell]",
		Short: "Display autocomplete installation instructions.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := filepath.Base(os.Getenv("SHELL"))
			if len(args) == 1 {
				shell = args[0]
			}
			supported := false
			for _, known := range shells {
				if shell == known {
					supported = true
				}
			}
			if !supported {
				return &Error{Message: fmt.Sprintf("%s is not a supported shell for autocomplete", shell), ExitCode: 1}
			}
			out := cmd.OutOrStdout()
			line := map[string]string{
				"bash":       "source <(sf autocomplete script bash)",
				"zsh":        "source <(sf autocomplete script zsh)",
				"fish":       "sf autocomplete script fish | source",
				"powershell": "sf autocomplete script powershell | Out-String | Invoke-Expression",
			}[shell]
			fmt.Fprintf(out, "Setup Instructions for SF CLI Autocomplete ---\n\n")
			fmt.Fprintf(out, "1) Add the autocomplete script to your %s profile and source it\n\n  %s\n\n", shell, line)
			fmt.Fprintf(out, "2) Test it out, e.g.:\n\n  sf <TAB>                 # Command completion\n  sf command --<TAB>       # Flag completion\n")
			return nil
		},
	}
	cmd.Flags().BoolP("refresh-cache", "r", false, "Refresh cache (ignores displaying instructions)")
	cmd.AddCommand(script)
	return cmd
}
