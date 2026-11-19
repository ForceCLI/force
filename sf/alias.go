package sf

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newAliasCmd() *cobra.Command {
	return newGroup("alias", "Commands to manage aliases.",
		newAliasListCmd(),
		newAliasSetCmd(),
		newAliasUnsetCmd(),
	)
}

type aliasListResult struct {
	Alias string `json:"alias"`
	Value string `json:"value"`
}

type aliasSetResult struct {
	Alias   string `json:"alias"`
	Success bool   `json:"success"`
	Value   string `json:"value"`
}

type aliasUnsetResult struct {
	Alias   string `json:"alias"`
	Value   string `json:"value,omitempty"`
	Success bool   `json:"success"`
}

func newAliasListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all aliases currently set on your local computer.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			aliases, err := lib.ListSFDXAliasesOrdered()
			if err != nil {
				return err
			}
			result := make([]aliasListResult, 0, len(aliases))
			rows := make([][]string, 0, len(aliases))
			for _, alias := range aliases {
				result = append(result, aliasListResult{Alias: alias.Alias, Value: alias.Value})
				rows = append(rows, []string{alias.Alias, alias.Value})
			}
			return emit(cmd, result, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(w, "No results")
					return
				}
				printTable(w, "Alias List", []string{"Alias", "Value"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

// parseKeyValueArgs splits arguments of the form key=value.
func parseKeyValueArgs(args []string) ([][2]string, error) {
	pairs := make([][2]string, 0, len(args))
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok || key == "" {
			return nil, &Error{
				Name:     "InvalidArgumentFormatError",
				Message:  fmt.Sprintf("Error in the following argument\n%s\nSet varargs with this format: key=value or key=\"value with spaces\"", arg),
				ExitCode: 1,
			}
		}
		pairs = append(pairs, [2]string{key, value})
	}
	return pairs, nil
}

func newAliasSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set name=value...",
		Short: "Set one or more aliases on your local computer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newError("ArgumentsRequiredError", "You must provide one or more aliases to set. Use the --help flag to see examples.")
			}
			pairs, err := parseKeyValueArgs(args)
			if err != nil {
				return err
			}
			result := make([]aliasSetResult, 0, len(pairs))
			rows := make([][]string, 0, len(pairs))
			for _, pair := range pairs {
				if err := lib.SetSFDXAlias(pair[0], pair[1]); err != nil {
					return err
				}
				result = append(result, aliasSetResult{Alias: pair[0], Success: true, Value: pair[1]})
				rows = append(rows, []string{pair[0], pair[1], "true"})
			}
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Alias Set", []string{"Alias", "Value", "Success"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func newAliasUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset [alias...]",
		Short: "Unset one or more aliases that are currently set on your local computer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			names := args
			if all {
				aliases, err := lib.ListSFDXAliasesOrdered()
				if err != nil {
					return err
				}
				names = nil
				for _, alias := range aliases {
					names = append(names, alias.Alias)
				}
			} else if len(args) == 0 {
				return newError("NameRequiredError", "You must provide an alias name when unsetting an alias.")
			}
			result := make([]aliasUnsetResult, 0, len(names))
			rows := make([][]string, 0, len(names))
			for _, name := range names {
				value, _, err := lib.UnsetSFDXAlias(name)
				if err != nil {
					return err
				}
				result = append(result, aliasUnsetResult{Alias: name, Value: value, Success: true})
				rows = append(rows, []string{name, value, "true"})
			}
			return emit(cmd, result, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(w, "No results")
					return
				}
				printTable(w, "Alias Unset", []string{"Alias", "Value", "Success"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("all", "a", false, "Unset all currently set aliases.")
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt the user for confirmation when unsetting all aliases.")
	return cmd
}
