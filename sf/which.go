package sf

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newWhichCmd shows the sf plugin that provides a command.  This mode has
// no plugins; it reports the plugin that provides the command in sf.
func newWhichCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "which",
		Short: "Show which plugin a command is in.",
		RunE: func(cmd *cobra.Command, args []string) error {
			errOut := cmd.ErrOrStderr()
			if len(args) == 0 {
				fmt.Fprintln(errOut, " ›   Error: \"which\" expects a command name")
				fmt.Fprintf(errOut, " ›   Try this: Provide a command id like this, \"%s which your:command:here\"\n", binName)
				return exitStatus(1)
			}
			typed := strings.Join(args, " ")
			plugin, ok := commandPlugins[strings.ReplaceAll(typed, " ", ":")]
			if !ok {
				fmt.Fprintf(errOut, " ›   Error: command %s not found\n", strings.ReplaceAll(typed, " ", ":"))
				return exitStatus(1)
			}
			if jsonRequested(cmd) {
				// sf prints this object without the status envelope.
				return writeJSON(cmd.OutOrStdout(), orderedFields("plugin", plugin))
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, paint(out, styleBold, typed))
			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s: %s\n", paint(out, style{"\x1b[36m", "\x1b[39m"}, "plugin"), plugin)
			return nil
		},
	}
	addJSONFlag(cmd)
	return cmd
}
