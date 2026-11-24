package sfplugin

import (
	"encoding/json"
	"io"
)

// ResultEnvelope is what an sf command prints on stdout with --json when it
// succeeds.
type ResultEnvelope struct {
	Status   int         `json:"status"`
	Result   interface{} `json:"result"`
	Warnings []string    `json:"warnings"`
}

// ErrorEnvelope is what an sf command prints on stdout with --json when it
// fails.
type ErrorEnvelope struct {
	Name        string      `json:"name"`
	Message     string      `json:"message"`
	ExitCode    int         `json:"exitCode"`
	Actions     []string    `json:"actions,omitempty"`
	Context     string      `json:"context"`
	Data        interface{} `json:"data,omitempty"`
	Warnings    []string    `json:"warnings"`
	Code        string      `json:"code"`
	Status      int         `json:"status"`
	CommandName string      `json:"commandName"`
}

// WriteResult writes the --json envelope of a command that exits with
// status.
func WriteResult(w io.Writer, status int, result interface{}, warnings []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	return writeJSON(w, ResultEnvelope{Status: status, Result: result, Warnings: warnings})
}

// WriteError writes the --json envelope of a failed command.  name is the
// error name, such as "InvalidFlagError", and commandName is the class name
// sf reports for the command, such as "SourceDeltaGenerate".
func WriteError(w io.Writer, name, message string, exitCode int, commandName string, warnings []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	if name == "" {
		name = "Error"
	}
	return writeJSON(w, ErrorEnvelope{
		Name:        name,
		Message:     message,
		ExitCode:    exitCode,
		Context:     commandName,
		Warnings:    warnings,
		Code:        name,
		Status:      exitCode,
		CommandName: commandName,
	})
}

func writeJSON(w io.Writer, v interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
