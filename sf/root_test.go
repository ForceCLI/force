package sf

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInvokedAsSf(t *testing.T) {
	cases := map[string]bool{
		"sf":                  true,
		"/usr/local/bin/sf":   true,
		`C:\tools\sf.exe`:     true,
		"./sf.EXE":            true,
		"force":               false,
		"/usr/local/bin/sfdx": false,
		"sf-old":              false,
		"":                    false,
	}
	for argv0, want := range cases {
		if got := InvokedAsSf(argv0); got != want {
			t.Errorf("InvokedAsSf(%q) = %v, want %v", argv0, got, want)
		}
	}
}

func TestStripModeFlag(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
		ok   bool
	}{
		{[]string{"--sf", "org", "list", "--json"}, []string{"org", "list", "--json"}, true},
		{[]string{"org", "list", "--sf"}, []string{"org", "list"}, true},
		{[]string{"--sf"}, []string{}, true},
		{[]string{"query", "SELECT Id FROM Account"}, []string{"query", "SELECT Id FROM Account"}, false},
		{[]string{"logins", "--sfdx"}, []string{"logins", "--sfdx"}, false},
		{[]string{"apex", "--", "--sf"}, []string{"apex", "--", "--sf"}, false},
		{[]string{}, []string{}, false},
	}
	for _, c := range cases {
		got, ok := StripModeFlag(c.in)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("StripModeFlag(%v) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"words are unchanged", []string{"org", "list", "--json"}, []string{"org", "list", "--json"}},
		{"colon form is split", []string{"org:login:web", "-a", "x"}, []string{"org", "login", "web", "-a", "x"}},
		{"legacy colon id", []string{"force:org:list", "--all"}, []string{"org", "list", "--all"}},
		{"legacy id as words", []string{"force", "org", "list", "--all"}, []string{"org", "list", "--all"}},
		{"legacy id keeps positional args", []string{"force:alias:set", "a=b"}, []string{"alias", "set", "a=b"}},
		{"colons in later args are kept", []string{"config", "set", "org-instance-url=https://x.example.com"}, []string{"config", "set", "org-instance-url=https://x.example.com"}},
		{"leading flag", []string{"--version"}, []string{"--version"}},
		{"empty", []string{}, []string{}},
	}
	for _, c := range cases {
		if got := normalizeArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: normalizeArgs(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestLegacyFlagNamesAreAccepted(t *testing.T) {
	root := NewRootCmd()
	cmd := newGroup("probe", "")
	cmd.RunE = nil
	var got string
	cmd.Run = func(c *cobra.Command, args []string) { got = targetOrgFlag(c) }
	addTargetOrgFlag(cmd)
	root.AddCommand(cmd)
	for _, args := range [][]string{
		{"probe", "--target-org", "me"},
		{"probe", "-o", "me"},
		{"probe", "--targetusername", "me"},
		{"probe", "-u", "me"},
	} {
		got = ""
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got != "me" {
			t.Errorf("%v: target org = %q, want me", args, got)
		}
	}
}

func TestUnknownCommandExits127(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Run([]string{"nosuch"}, &out, &errOut)
	if status != 127 {
		t.Errorf("status = %d, want 127", status)
	}
	want := " ›   Warning: nosuch is not a sf command.\n ›   Error: Run sf help for a list of available commands.\n"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
}

func TestVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if status := Run([]string{"version"}, &out, &errOut); status != 0 {
		t.Fatalf("status = %d, stderr %s", status, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "@salesforce/cli/"+CompatVersion+" ") {
		t.Errorf("version output = %q", out.String())
	}
	out.Reset()
	if status := Run([]string{"version", "--json"}, &out, &errOut); status != 0 {
		t.Fatalf("status = %d", status)
	}
	var result versionResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.CliVersion != "@salesforce/cli/"+CompatVersion {
		t.Errorf("cliVersion = %q", result.CliVersion)
	}
}

func TestWriteErrorFormats(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	err := &Error{Name: "NamedOrgNotFoundError", Message: "No authorization information found for x.", ExitCode: 2, Actions: []string{"Check the alias."}}
	var out, errOut bytes.Buffer
	if status := writeError(&out, &errOut, nil, err, false); status != 2 {
		t.Errorf("status = %d, want 2", status)
	}
	want := "\x1b[1m\x1b[31mError (NamedOrgNotFoundError):\x1b[39m\x1b[22m No authorization information found for x.\n\n\n" +
		"\x1b[2mTry this:\x1b[22m\n\n\x1b[2mCheck the alias.\x1b[22m\n\n"
	if errOut.String() != want {
		t.Errorf("stderr = %q, want %q", errOut.String(), want)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}

	t.Setenv("FORCE_COLOR", "0")
	errOut.Reset()
	writeError(&out, &errOut, nil, err, false)
	want = "Error (NamedOrgNotFoundError): No authorization information found for x.\n\n\nTry this:\n\nCheck the alias.\n\n"
	if errOut.String() != want {
		t.Errorf("stderr without color = %q, want %q", errOut.String(), want)
	}

	out.Reset()
	errOut.Reset()
	writeError(&out, &errOut, nil, err, true)
	var env errorEnvelope
	if jsonErr := json.Unmarshal(out.Bytes(), &env); jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if env.Name != "NamedOrgNotFoundError" || env.Status != 2 || env.ExitCode != 2 || env.Code != env.Name || len(env.Actions) != 1 {
		t.Errorf("envelope = %+v", env)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

func TestPrintTable(t *testing.T) {
	t.Setenv("FORCE_COLOR", "0")
	var out bytes.Buffer
	printTable(&out, "Alias List", []string{"Alias", "Value"}, [][]string{{"dev", "me@example.com"}, {"hub", "h@example.com"}})
	want := `Alias List
┌───────┬────────────────┐
│ Alias │ Value          │
├───────┼────────────────┤
│ dev   │ me@example.com │
│ hub   │ h@example.com  │
└───────┴────────────────┘

`
	if out.String() != want {
		t.Errorf("table =\n%s\nwant\n%s", out.String(), want)
	}
}

// Compared with a screenshot of sf in a terminal: the column headers are
// bold and blue.
func TestPrintTableStylesTheHeadersInATerminal(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	var out strings.Builder
	printTable(&out, "", []string{"Name"}, [][]string{{"dev"}})
	if !strings.Contains(out.String(), "│ \x1b[1m\x1b[34mName\x1b[39m\x1b[22m │") || !strings.Contains(out.String(), "│ dev  │") {
		t.Errorf("table = %q", out.String())
	}
}
