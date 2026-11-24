package sfplugin

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func validManifest() Manifest {
	return Manifest{
		Name:     "example",
		Version:  "1.2.3",
		Protocol: ProtocolVersion,
		Commands: []Command{{ID: "example run", Summary: "Run it.", Org: OrgRequired}},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		change  func(m *Manifest)
		wantErr string
	}{
		{"valid", func(m *Manifest) {}, ""},
		{"no name", func(m *Manifest) { m.Name = "" }, "has no name"},
		{"name with separator", func(m *Manifest) { m.Name = "a/b" }, "path separator"},
		{"wrong protocol", func(m *Manifest) { m.Protocol = 2 }, "uses protocol 2"},
		{"no commands", func(m *Manifest) { m.Commands = nil }, "provides no commands"},
		{"empty id", func(m *Manifest) { m.Commands[0].ID = "" }, "empty id"},
		{"double space", func(m *Manifest) { m.Commands[0].ID = "example  run" }, "single spaces"},
		{"colon form", func(m *Manifest) { m.Commands[0].ID = "example:run" }, "contains a colon"},
		{"flag word", func(m *Manifest) { m.Commands[0].ID = "example --run" }, "starts with -"},
		{"duplicate", func(m *Manifest) { m.Commands = append(m.Commands, m.Commands[0]) }, "more than once"},
		{"bad org", func(m *Manifest) { m.Commands[0].Org = "sometimes" }, "org requirement"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validManifest()
			test.change(&manifest)
			err := manifest.Validate()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Validate() = %v, want an error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestOrgRequirementDefaultsToNone(t *testing.T) {
	if got := (Command{ID: "a"}).OrgRequirement(); got != OrgNone {
		t.Errorf("OrgRequirement() = %q, want %q", got, OrgNone)
	}
}

func TestRunAnswersTheManifestRequest(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	status := Run(validManifest(), []string{DescribeArg}, &out, &errOut, func([]string) int {
		called = true
		return 3
	})
	if status != 0 || called {
		t.Fatalf("status = %d, run called = %v; want 0 and false", status, called)
	}
	parsed, err := ParseManifest(out.Bytes())
	if err != nil {
		t.Fatalf("the printed manifest does not parse: %v", err)
	}
	if parsed.Name != "example" || parsed.Commands[0].Org != OrgRequired {
		t.Errorf("parsed manifest = %+v", parsed)
	}
}

func TestRunPassesOtherArgumentsToTheCommand(t *testing.T) {
	var got []string
	status := Run(validManifest(), []string{"example", "run", "--flag"}, &bytes.Buffer{}, &bytes.Buffer{}, func(args []string) int {
		got = args
		return 7
	})
	if status != 7 || strings.Join(got, " ") != "example run --flag" {
		t.Errorf("status = %d, args = %v", status, got)
	}
}

func TestRunRejectsAnInvalidManifest(t *testing.T) {
	manifest := validManifest()
	manifest.Commands = nil
	var out, errOut bytes.Buffer
	if status := Run(manifest, []string{DescribeArg}, &out, &errOut, nil); status != 1 || out.Len() != 0 {
		t.Errorf("status = %d, stdout = %q; want 1 and nothing", status, out.String())
	}
}

func TestOrgFromEnv(t *testing.T) {
	for _, name := range []string{EnvInstanceURL, EnvAccessToken, EnvAPIVersion, EnvUsername, EnvOrgID} {
		t.Setenv(name, "")
	}
	if _, ok := OrgFromEnv(); ok {
		t.Error("OrgFromEnv() reports an org when no variables are set")
	}
	t.Setenv(EnvInstanceURL, "https://example.my.salesforce.com")
	t.Setenv(EnvAccessToken, "token")
	t.Setenv(EnvAPIVersion, "66.0")
	org, ok := OrgFromEnv()
	if !ok || org.InstanceURL != "https://example.my.salesforce.com" || org.AccessToken != "token" || org.APIVersion != "66.0" {
		t.Errorf("OrgFromEnv() = %+v, %v", org, ok)
	}
}

func TestBinNameAndHostProtocol(t *testing.T) {
	t.Setenv(EnvBinName, "")
	t.Setenv(EnvProtocol, "")
	if BinName() != "sf" || HostProtocol() != 0 {
		t.Errorf("without a host: BinName() = %q, HostProtocol() = %d", BinName(), HostProtocol())
	}
	t.Setenv(EnvBinName, "force --sf")
	t.Setenv(EnvProtocol, "1")
	if BinName() != "force --sf" || HostProtocol() != 1 {
		t.Errorf("with a host: BinName() = %q, HostProtocol() = %d", BinName(), HostProtocol())
	}
}

func TestEnvelopes(t *testing.T) {
	var out bytes.Buffer
	if err := WriteResult(&out, 0, map[string]string{"output-dir": "out"}, nil); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"status\": 0,\n  \"result\": {\n    \"output-dir\": \"out\"\n  },\n  \"warnings\": []\n}\n"
	if out.String() != want {
		t.Errorf("WriteResult wrote\n%s\nwant\n%s", out.String(), want)
	}
	out.Reset()
	if err := WriteError(&out, "", "it failed", 1, "ExampleRun", []string{"careful"}); err != nil {
		t.Fatal(err)
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Name != "Error" || envelope.Code != "Error" || envelope.Status != 1 || envelope.ExitCode != 1 || envelope.CommandName != "ExampleRun" || len(envelope.Warnings) != 1 {
		t.Errorf("WriteError wrote %+v", envelope)
	}
}
