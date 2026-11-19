package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

type envelope struct {
	Status   int         `json:"status"`
	Result   interface{} `json:"result"`
	Warnings []string    `json:"warnings"`
}

func writeJSON(w io.Writer, v interface{}) error {
	if jsonColorEnabled(w) {
		var buffer strings.Builder
		enc := json.NewEncoder(&buffer)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return err
		}
		_, err := io.WriteString(w, colorizeJSON(buffer.String()))
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func jsonRequested(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("json")
	return flag != nil && flag.Value.String() == "true"
}

// emit prints the result of a command.  With --json it prints sf's envelope
// around result; otherwise it calls human to print the human-readable form.
func emit(cmd *cobra.Command, result interface{}, human func(w io.Writer)) error {
	return emitStatus(cmd, 0, result, human)
}

func emitStatus(cmd *cobra.Command, status int, result interface{}, human func(w io.Writer)) error {
	if jsonRequested(cmd) {
		return writeJSON(cmd.OutOrStdout(), envelope{Status: status, Result: result, Warnings: warningsOf(cmd)})
	}
	if human != nil {
		human(cmd.OutOrStdout())
	}
	return nil
}

type runState struct {
	warnings []string
	// deprecations are the warnings for a legacy command id and legacy
	// flag names on the command line.  With --json they come before the
	// warnings of the command.
	deprecations []string
	// flagDeprecations are the warnings for legacy flag names that have
	// not been reported yet.  sf reports them after it has parsed the
	// command line, so they are dropped when the command line is invalid.
	flagDeprecations []string
}

// deprecationWriter is the output of a command that was given legacy flag
// names.  It reports the deprecation warnings before the first output of
// the command.
type deprecationWriter struct {
	io.Writer
	report func()
}

func (w *deprecationWriter) Write(p []byte) (int, error) {
	w.report()
	return w.Writer.Write(p)
}

type runStateKey struct{}

func stateOf(cmd *cobra.Command) *runState {
	if cmd == nil || cmd.Context() == nil {
		return nil
	}
	state, _ := cmd.Context().Value(runStateKey{}).(*runState)
	return state
}

func warningsOf(cmd *cobra.Command) []string {
	warnings := []string{}
	if state := stateOf(cmd); state != nil {
		warnings = append(warnings, state.deprecations...)
		warnings = append(warnings, state.flagDeprecations...)
		warnings = append(warnings, state.warnings...)
	}
	return warnings
}

// warn reports a warning.  With --json it is added to the "warnings" array
// of the envelope; otherwise it is printed on stderr.
func warn(cmd *cobra.Command, message string) {
	if jsonRequested(cmd) {
		warnJSON(cmd, message)
		return
	}
	errOut := cmd.ErrOrStderr()
	fmt.Fprintf(errOut, "%s %s\n", paint(errOut, styleBoldYellow, "Warning:"), message)
}

// warnJSON adds a warning to the envelope and prints nothing in human
// output.
func warnJSON(cmd *cobra.Command, message string) {
	if state := stateOf(cmd); state != nil && jsonRequested(cmd) {
		state.warnings = append(state.warnings, message)
	}
}

// printTable prints a table in the format sf uses: an optional title line,
// a box-drawing border, and a blank line.
func printTable(w io.Writer, title string, headers []string, rows [][]string) {
	printStyledTable(w, title, headers, rows, nil)
}

// printStyledTable is printTable with a style for each row.  The style
// applies to the text of each cell and not to its padding.
func printStyledTable(w io.Writer, title string, headers []string, rows [][]string, rowStyle func(row int) *style) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			for _, part := range strings.Split(cell, "\n") {
				if i < len(widths) && runewidth.StringWidth(part) > widths[i] {
					widths[i] = runewidth.StringWidth(part)
				}
			}
		}
	}
	border := func(left, middle, right string) string {
		parts := make([]string, len(widths))
		for i, width := range widths {
			parts[i] = strings.Repeat("─", width+2)
		}
		return left + strings.Join(parts, middle) + right
	}
	line := func(cells []string, cellStyle *style) string {
		parts := make([]string, len(widths))
		for i, width := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			text := cell
			if cellStyle != nil && cell != "" {
				text = paint(w, *cellStyle, cell)
			}
			parts[i] = " " + text + strings.Repeat(" ", width-runewidth.StringWidth(cell)) + " "
		}
		return "│" + strings.Join(parts, "│") + "│"
	}
	if title != "" {
		fmt.Fprintln(w, title)
	}
	fmt.Fprintln(w, border("┌", "┬", "┐"))
	// In a terminal sf prints the column headers bold and blue.
	fmt.Fprintln(w, line(headers, &styleTableHeader))
	fmt.Fprintln(w, border("├", "┼", "┤"))
	for i, row := range rows {
		var cellStyle *style
		if rowStyle != nil {
			cellStyle = rowStyle(i)
		}
		// A cell with several lines makes the row that many lines high.
		height := 1
		for _, cell := range row {
			if lines := strings.Count(cell, "\n") + 1; lines > height {
				height = lines
			}
		}
		if height == 1 {
			fmt.Fprintln(w, line(row, cellStyle))
			continue
		}
		for n := 0; n < height; n++ {
			cells := make([]string, len(row))
			for c, cell := range row {
				if parts := strings.Split(cell, "\n"); n < len(parts) {
					cells[c] = parts[n]
				}
			}
			fmt.Fprintln(w, line(cells, cellStyle))
		}
	}
	fmt.Fprintln(w, border("└", "┴", "┘"))
	fmt.Fprintln(w)
}
