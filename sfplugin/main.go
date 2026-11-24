package sfplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Main is the entry point of a plugin executable.  It answers the manifest
// request, and otherwise calls run with the process arguments, which start
// with the words of the command id.  The process exits with the status run
// returns.
func Main(manifest Manifest, run func(args []string) int) {
	os.Exit(Run(manifest, os.Args[1:], os.Stdout, os.Stderr, run))
}

// Run is Main without the exit, for tests.
func Run(manifest Manifest, args []string, out, errOut io.Writer, run func(args []string) int) int {
	if len(args) == 1 && args[0] == DescribeArg {
		if err := manifest.Validate(); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := WriteManifest(out, manifest); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	return run(args)
}

// WriteManifest writes the manifest as indented JSON.
func WriteManifest(w io.Writer, manifest Manifest) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}
