package sf

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// platform returns the operating system and architecture in the form sf
// prints, such as linux-x64.
func platform() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	os := runtime.GOOS
	if os == "windows" {
		os = "win32"
	}
	return os + "-" + arch
}

func forceVersion() string {
	if lib.Version == "" {
		return "dev"
	}
	return lib.Version
}

type versionResult struct {
	Architecture string `json:"architecture"`
	CliVersion   string `json:"cliVersion"`
	NodeVersion  string `json:"nodeVersion"`
}

// newVersionCmd reports the sf release this mode is compatible with.  sf
// prints the Node.js version in the third field; this prints the force
// version there.
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonRequested(cmd) {
				// sf prints this object without the status envelope.
				return writeJSON(cmd.OutOrStdout(), versionResult{
					Architecture: platform(),
					CliVersion:   "@salesforce/cli/" + CompatVersion,
					NodeVersion:  "force-" + forceVersion(),
				})
			}
			return emit(cmd, nil, func(w io.Writer) {
				fmt.Fprintln(w, versionString())
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("verbose", false, "Show additional information about the CLI.")
	return cmd
}
