package sf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// useTempHome points the sf state directories at a temporary home
// directory, keeps the encryption key in key.json there, and clears the
// environment variables that override config values.
func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("FORCE_SFDX_STATE_DIRS", "")
	t.Setenv("SF_USE_GENERIC_UNIX_KEYCHAIN", "true")
	t.Setenv("FORCE_COLOR", "0")
	secretTool := filepath.Join(home, "secret-tool")
	if err := os.WriteFile(secretTool, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SFDX_SECRET_TOOL_PATH", secretTool)
	for _, name := range []string{
		"SF_TARGET_ORG", "SFDX_DEFAULTUSERNAME", "SF_TARGET_DEV_HUB", "SFDX_DEFAULTDEVHUBUSERNAME",
		"SF_ORG_API_VERSION", "SFDX_API_VERSION", "SF_ORG_INSTANCE_URL", "SFDX_INSTANCE_URL",
		"SF_ORG_MAX_QUERY_LIMIT", "SFDX_MAX_QUERY_LIMIT", "SF_DISABLE_TELEMETRY", "SFDX_DISABLE_TELEMETRY",
		"SF_ACCESS_TOKEN", "SF_INSTANCE_URL",
	} {
		t.Setenv(name, "")
	}
	chdir(t, home)
	return home
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
}

// newProject creates a directory with sfdx-project.json under home and
// makes it the working directory.
func newProject(t *testing.T, home string) string {
	t.Helper()
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"packageDirectories":[{"path":"force-app","default":true}]}`
	if err := os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, project)
	return project
}

// run executes the sf command tree and returns stdout, stderr and the exit
// status.
func run(args ...string) (string, string, int) {
	var out, errOut bytes.Buffer
	status := Run(args, &out, &errOut)
	return out.String(), errOut.String(), status
}

// expectRun fails the test unless the command exits with status and prints
// exactly stdout.
func expectRun(t *testing.T, status int, stdout string, args ...string) {
	t.Helper()
	out, errOut, got := run(args...)
	if got != status {
		t.Errorf("sf %v: status = %d, want %d (stderr %q)", args, got, status, errOut)
	}
	if out != stdout {
		t.Errorf("sf %v: stdout =\n%s\nwant\n%s", args, out, stdout)
	}
}
