package sf

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newConfigCmd() *cobra.Command {
	return newGroup("config", "Commands to configure Salesforce CLI.",
		newConfigGetCmd(),
		newConfigListCmd(),
		newConfigSetCmd(),
		newConfigUnsetCmd(),
	)
}

type configError struct {
	Name     string                  `json:"name"`
	ExitCode int                     `json:"exitCode"`
	Cause    *map[string]interface{} `json:"cause,omitempty"`
}

// configResult is one entry in the result of config get and config list.
type configResult struct {
	Name     string       `json:"name"`
	Key      string       `json:"key,omitempty"`
	Value    string       `json:"value,omitempty"`
	Path     string       `json:"path,omitempty"`
	Success  bool         `json:"success"`
	Location string       `json:"location,omitempty"`
	Error    *configError `json:"error,omitempty"`
	Message  string       `json:"message,omitempty"`
}

// configChange is one entry in the result of config set and config unset.
// The field order differs between successes and failures in sf, so each has
// its own type.
type configChangeSuccess struct {
	Name    string `json:"name"`
	Value   string `json:"value,omitempty"`
	Success bool   `json:"success"`
}

type configChangeFailure struct {
	Name    string       `json:"name"`
	Success bool         `json:"success"`
	Value   string       `json:"value,omitempty"`
	Error   *configError `json:"error"`
	Message string       `json:"message"`
}

type configChangeResult struct {
	Successes []configChangeSuccess `json:"successes"`
	Failures  []configChangeFailure `json:"failures"`
}

func unknownConfigKey(name string) (*configError, string) {
	return &configError{Name: "UnknownConfigKeyError", ExitCode: 1}, fmt.Sprintf("Unknown config name: %s.", name)
}

func newConfigListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the configuration variables that you've previously set.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := lib.ListSFConfig()
			if err != nil {
				return err
			}
			result := make([]configResult, 0, len(values))
			rows := make([][]string, 0, len(values))
			for _, value := range values {
				result = append(result, configResult{
					Name:     value.Key,
					Key:      value.Key,
					Value:    value.Value,
					Path:     value.Path,
					Success:  true,
					Location: value.Location,
				})
				rows = append(rows, []string{value.Key, value.Location, value.Value})
			}
			return emit(cmd, result, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(w, "No results found")
					return
				}
				printTable(w, "List Config", []string{"Name", "Location", "Value"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get name...",
		Short: "Get the value of a configuration variable.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newError("NoConfigKeysFoundError", "%s", `You must provide one or more configuration variables to get. Run "`+binName+` config list" to see the configuration variables you've previously set.`)
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			status := 0
			result := make([]configResult, 0, len(args))
			rows := make([][]string, 0, len(args))
			failed := false
			for _, name := range args {
				entry := configResult{Name: name}
				if !lib.IsSFConfigKey(name) {
					entry.Error, entry.Message = unknownConfigKey(name)
					status = 1
					failed = true
				} else {
					value, found, err := lib.GetSFConfig(name)
					if err != nil {
						return err
					}
					entry.Key = name
					entry.Success = true
					if found {
						entry.Value = value.Value
						entry.Path = value.Path
						entry.Location = value.Location
					}
				}
				result = append(result, entry)
				row := []string{entry.Name}
				if verbose {
					row = append(row, entry.Location)
				}
				row = append(row, entry.Value, fmt.Sprintf("%t", entry.Success))
				rows = append(rows, append(row, entry.Message))
			}
			headers := []string{"Name"}
			if verbose {
				headers = append(headers, "Location")
			}
			headers = append(headers, "Value", "Success")
			if failed {
				headers = append(headers, "Message")
			}
			if err := emitStatus(cmd, status, result, func(w io.Writer) {
				printTable(w, "Get Config", headers, rows)
			}); err != nil {
				return err
			}
			if status != 0 {
				return exitStatus(status)
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("verbose", false, "Display whether the configuration variables are set locally or globally.")
	return cmd
}

var apiVersionPattern = regexp.MustCompile(`^\d+\.0$`)

// validateConfigValue applies the checks sf makes before it stores a value.
func validateConfigValue(name, value string) string {
	switch name {
	case lib.SFConfigTargetOrg, lib.SFConfigTargetDevHub:
		if _, err := lib.ReadSFDXAuthFields(lib.ResolveSFDXUsername(value)); err != nil {
			return fmt.Sprintf("Invalid config value: org \"%s\" is not authenticated.", value)
		}
	case lib.SFConfigApiVersion:
		if !apiVersionPattern.MatchString(value) {
			return "Invalid config value: Specify a valid Salesforce API version, for example, 42.0.."
		}
	}
	return ""
}

// notInProjectError is the error sf reports when a local config file is
// needed and the working directory is not inside a project.
func notInProjectError() *Error {
	wd, _ := os.Getwd()
	return &Error{
		Name:     "InvalidProjectWorkspaceError",
		Message:  fmt.Sprintf("%s does not contain a valid Salesforce DX project.", wd),
		ExitCode: 1,
		Actions:  []string{"Run with --global to set for your entire workspace."},
	}
}

func emitConfigChange(cmd *cobra.Command, title string, withValue bool, result configChangeResult) error {
	status := 0
	if len(result.Failures) > 0 {
		status = 1
	}
	if err := emitStatus(cmd, status, result, func(w io.Writer) {
		headers := []string{"Name"}
		if withValue {
			headers = append(headers, "Value")
		}
		headers = append(headers, "Success")
		if status != 0 {
			headers = append(headers, "Message")
		}
		var rows [][]string
		for _, success := range result.Successes {
			row := []string{success.Name}
			if withValue {
				row = append(row, success.Value)
			}
			rows = append(rows, append(row, "true", ""))
		}
		for _, failure := range result.Failures {
			row := []string{failure.Name}
			if withValue {
				row = append(row, failure.Value)
			}
			rows = append(rows, append(row, "false", failure.Message))
		}
		printTable(w, title, headers, rows)
	}); err != nil {
		return err
	}
	if status != 0 {
		return exitStatus(status)
	}
	return nil
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set name=value...",
		Short: "Set one or more configuration variables, such as your default org.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newError("ArgumentsRequiredError", "You must provide one or more configuration variables to set. Use the --help flag to see examples.")
			}
			pairs, err := parseKeyValueArgs(args)
			if err != nil {
				return err
			}
			global, _ := cmd.Flags().GetBool("global")
			if !global {
				if _, ok := lib.SFLocalConfigPath(); !ok {
					return notInProjectError()
				}
			}
			result := configChangeResult{Successes: []configChangeSuccess{}, Failures: []configChangeFailure{}}
			for _, pair := range pairs {
				name, value := pair[0], pair[1]
				if !lib.IsSFConfigKey(name) {
					configErr, message := unknownConfigKey(name)
					result.Failures = append(result.Failures, configChangeFailure{Name: name, Value: value, Error: configErr, Message: message})
					continue
				}
				if message := validateConfigValue(name, value); message != "" {
					result.Failures = append(result.Failures, configChangeFailure{
						Name:    name,
						Value:   value,
						Error:   &configError{Name: "Error", ExitCode: 1, Cause: &map[string]interface{}{}},
						Message: message,
					})
					continue
				}
				if _, err := lib.SetSFConfig(name, value, global); err != nil {
					return err
				}
				result.Successes = append(result.Successes, configChangeSuccess{Name: name, Value: value, Success: true})
			}
			return emitConfigChange(cmd, "Set Config", true, result)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("global", "g", false, "Set the configuration variables globally, so they can be used from any Salesforce DX project.")
	return cmd
}

func newConfigUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset name...",
		Short: "Unset local or global configuration variables.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newError("NoConfigKeysFoundError", "%s", `You must provide one or more configuration variables to unset. Run "`+binName+` config list" to see the configuration variables you've previously set.`)
			}
			global, _ := cmd.Flags().GetBool("global")
			if !global {
				if _, ok := lib.SFLocalConfigPath(); !ok {
					return notInProjectError()
				}
			}
			result := configChangeResult{Successes: []configChangeSuccess{}, Failures: []configChangeFailure{}}
			for _, name := range args {
				if !lib.IsSFConfigKey(name) {
					configErr, message := unknownConfigKey(name)
					result.Failures = append(result.Failures, configChangeFailure{Name: name, Error: configErr, Message: message})
					continue
				}
				if _, err := lib.UnsetSFConfig(name, global); err != nil {
					return err
				}
				result.Successes = append(result.Successes, configChangeSuccess{Name: name, Success: true})
			}
			return emitConfigChange(cmd, "Unset Config", false, result)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("global", "g", false, "Unset the configuration variables globally.")
	return cmd
}
