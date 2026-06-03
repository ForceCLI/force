package command

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func sourceDirForDirectory(t *testing.T, directory string) string {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().StringP("directory", "d", "src", "relative path to package.xml")
	if err := cmd.Flags().Set("directory", directory); err != nil {
		t.Fatalf("failed to set directory flag: %v", err)
	}
	return sourceDir(cmd)
}

func TestSourceDir_AbsoluteTrailingSlashStripped(t *testing.T) {
	root := sourceDirForDirectory(t, "/tmp/sf-education-cloud/shape_1780009540656/")
	want := "/tmp/sf-education-cloud/shape_1780009540656"
	if root != want {
		t.Errorf("Expected %s, got %s", want, root)
	}
}

func TestSourceDir_AbsoluteNoTrailingSlash(t *testing.T) {
	root := sourceDirForDirectory(t, "/tmp/sf-education-cloud/shape_1780009540656")
	want := "/tmp/sf-education-cloud/shape_1780009540656"
	if root != want {
		t.Errorf("Expected %s, got %s", want, root)
	}
}

func TestSourceDir_RelativeTrailingSlashStripped(t *testing.T) {
	root := sourceDirForDirectory(t, "unpackaged/")
	// Relative paths are resolved against the working directory; the result
	// must not retain a trailing separator.
	if filepath.Base(root) != "unpackaged" {
		t.Errorf("Expected base 'unpackaged', got %s (full: %s)", filepath.Base(root), root)
	}
}
