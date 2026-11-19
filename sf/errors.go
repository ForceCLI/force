package sf

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Error is an error reported in sf's format.  Name is the error name sf
// prints, such as NoDefaultEnvError.
type Error struct {
	Name     string
	Message  string
	ExitCode int
	Actions  []string
	// Label replaces Name in the human "Error (<label>):" line and in the
	// "code" field of the JSON envelope.  sf prints the exit code there
	// for some errors that have a name.
	Label string
	// Data is written as the "data" field of the JSON envelope.
	Data interface{}
	// Compact prints one blank line after the message instead of two, as
	// the sf plugins that are installed on first use do.
	Compact bool
}

func (e *Error) Error() string {
	return e.Message
}

func newError(name string, format string, args ...interface{}) *Error {
	return &Error{Name: name, Message: fmt.Sprintf(format, args...), ExitCode: 1}
}

// unknownCommandError reports an id that matches no command.  sf exits 127.
type unknownCommandError struct {
	id string
}

func (e *unknownCommandError) Error() string {
	return fmt.Sprintf("%s is not a sf command.", e.id)
}

// exitStatus is returned by a command that has already printed its result
// and only needs the process to exit with a non-zero status.
type exitStatus int

func (e exitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(e))
}

func asError(err error) *Error {
	if e, ok := err.(*Error); ok {
		if e.ExitCode == 0 {
			e.ExitCode = 1
		}
		return e
	}
	message := err.Error()
	if flag, ok := strings.CutPrefix(message, "unknown flag: "); ok {
		return &Error{Message: fmt.Sprintf("Nonexistent flag: %s\nSee more help with --help", flag), ExitCode: 2}
	}
	if flag, ok := strings.CutPrefix(message, "unknown shorthand flag: "); ok {
		return &Error{Message: fmt.Sprintf("Nonexistent flag: %s\nSee more help with --help", flag), ExitCode: 2}
	}
	return &Error{Message: message, ExitCode: 1}
}

type errorEnvelope struct {
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

// writeError prints err the way sf does and returns the exit status.  With
// --json the envelope goes to stdout; otherwise the message goes to stderr.
func writeError(out, errOut io.Writer, cmd *cobra.Command, err error, jsonOutput bool) int {
	abandonStatusLine()
	if status, ok := err.(exitStatus); ok {
		return int(status)
	}
	if unknown, ok := err.(*unknownCommandError); ok {
		// For a command inside a topic, sf names the topic in the hint.
		help := binName + " help"
		if words := strings.Fields(unknown.id); len(words) > 1 {
			help += " " + words[0]
		}
		fmt.Fprintf(errOut, " ›   Warning: %s is not a sf command.\n", unknown.id)
		fmt.Fprintf(errOut, " ›   Error: Run %s for a list of available commands.\n", help)
		return 127
	}
	e := asError(err)
	if jsonOutput {
		name := e.Name
		if name == "" {
			name = "Error"
		}
		commandName := commandClassName(cmd)
		// The code of an error without a name of its own is its exit
		// status.
		code := name
		if e.Name == "" {
			code = fmt.Sprintf("%d", e.ExitCode)
		}
		if e.Label != "" {
			code = e.Label
		}
		writeJSON(out, errorEnvelope{
			Name:        name,
			Message:     e.Message,
			ExitCode:    e.ExitCode,
			Actions:     e.Actions,
			Context:     commandName,
			Data:        e.Data,
			Warnings:    warningsOf(cmd),
			Code:        code,
			Status:      e.ExitCode,
			CommandName: commandName,
		})
		return e.ExitCode
	}
	label := e.Name
	if e.Label != "" {
		label = e.Label
	}
	if label == "" {
		label = fmt.Sprintf("%d", e.ExitCode)
	}
	ending := "\n\n\n"
	if e.Compact {
		ending = "\n\n"
	}
	fmt.Fprintf(errOut, "%s %s%s", paint(errOut, styleBoldRed, fmt.Sprintf("Error (%s):", label)), e.Message, ending)
	if len(e.Actions) > 0 {
		fmt.Fprintf(errOut, "%s\n\n", paint(errOut, styleDim, "Try this:"))
		for _, action := range e.Actions {
			// sf dims each line of an action on its own, and prints an
			// empty action as an empty line.
			if action == "" {
				fmt.Fprintln(errOut)
				continue
			}
			for _, line := range strings.Split(action, "\n") {
				fmt.Fprintf(errOut, "%s\n", paint(errOut, styleDim, line))
			}
		}
		fmt.Fprintln(errOut)
	}
	return e.ExitCode
}

const commandNameAnnotation = "sfCommandName"

// commandClassName returns the value sf reports as "context" and
// "commandName" in error envelopes.  sf uses the class name of the command;
// commands whose class name differs from the id set the annotation.
func commandClassName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if name, ok := cmd.Annotations[commandNameAnnotation]; ok {
		return name
	}
	var b strings.Builder
	for _, part := range strings.Fields(cmd.CommandPath())[1:] {
		for _, word := range strings.Split(part, "-") {
			if word == "" {
				continue
			}
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	b.WriteString("Command")
	return b.String()
}
