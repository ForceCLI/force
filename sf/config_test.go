package sf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// The expected output in this file was captured from sf 2.138.6.

func writeTestAuth(t *testing.T, username string, extra lib.SFDXAuthFields) {
	t.Helper()
	fields := lib.SFDXAuthFields{
		"accessToken": "00Dxx!token",
		"instanceUrl": "https://example.my.salesforce.com",
		"loginUrl":    "https://login.salesforce.com",
		"orgId":       "00Dxx0000000001EAA",
		"username":    username,
		"clientId":    "PlatformCLI",
	}
	for name, value := range extra {
		fields[name] = value
	}
	if err := lib.WriteSFDXAuthFields(fields); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSetValidatesValues(t *testing.T) {
	useTempHome(t)

	expectRun(t, 1, `Set Config
┌────────────┬───────────────┬─────────┬─────────────────────────────────────────────────────────────────┐
│ Name       │ Value         │ Success │ Message                                                         │
├────────────┼───────────────┼─────────┼─────────────────────────────────────────────────────────────────┤
│ target-org │ x@example.com │ false   │ Invalid config value: org "x@example.com" is not authenticated. │
└────────────┴───────────────┴─────────┴─────────────────────────────────────────────────────────────────┘

`, "config", "set", "target-org=x@example.com", "--global")

	expectRun(t, 1, `{
  "status": 1,
  "result": {
    "successes": [
      {
        "name": "org-api-version",
        "value": "60.0",
        "success": true
      }
    ],
    "failures": [
      {
        "name": "target-dev-hub",
        "success": false,
        "value": "h@example.com",
        "error": {
          "name": "Error",
          "exitCode": 1,
          "cause": {}
        },
        "message": "Invalid config value: org \"h@example.com\" is not authenticated."
      }
    ]
  },
  "warnings": []
}
`, "config", "set", "org-api-version=60.0", "target-dev-hub=h@example.com", "--global", "--json")

	out, _, status := run("config", "set", "org-api-version=abc", "--global")
	if status != 1 || !strings.Contains(out, "Invalid config value: Specify a valid Salesforce API version, for example, 42.0..") {
		t.Errorf("invalid api version: status %d, output %s", status, out)
	}
}

func TestConfigGetListAndUnset(t *testing.T) {
	home := useTempHome(t)
	if _, errOut, status := run("config", "set", "org-api-version=60.0", "--global"); status != 0 {
		t.Fatalf("status %d, stderr %s", status, errOut)
	}

	expectRun(t, 0, `Get Config
┌─────────────────┬───────┬─────────┐
│ Name            │ Value │ Success │
├─────────────────┼───────┼─────────┤
│ target-org      │       │ true    │
│ org-api-version │ 60.0  │ true    │
└─────────────────┴───────┴─────────┘

`, "config", "get", "target-org", "org-api-version")

	expectRun(t, 0, `Get Config
┌────────────┬──────────┬───────┬─────────┐
│ Name       │ Location │ Value │ Success │
├────────────┼──────────┼───────┼─────────┤
│ target-org │          │       │ true    │
└────────────┴──────────┴───────┴─────────┘

`, "config", "get", "target-org", "--verbose")

	expectRun(t, 1, `{
  "status": 1,
  "result": [
    {
      "name": "target-org",
      "key": "target-org",
      "success": true
    },
    {
      "name": "nosuch",
      "success": false,
      "error": {
        "name": "UnknownConfigKeyError",
        "exitCode": 1
      },
      "message": "Unknown config name: nosuch."
    }
  ],
  "warnings": []
}
`, "config", "get", "target-org", "nosuch", "--json")

	t.Setenv("SF_DISABLE_TELEMETRY", "true")
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "name": "disable-telemetry",
      "key": "disable-telemetry",
      "value": "true",
      "path": "$SF_DISABLE_TELEMETRY",
      "success": true,
      "location": "Environment"
    },
    {
      "name": "org-api-version",
      "key": "org-api-version",
      "value": "60.0",
      "path": "`+filepath.Join(home, ".sf", "config.json")+`",
      "success": true,
      "location": "Global"
    }
  ],
  "warnings": []
}
`, "config", "list", "--json")
	t.Setenv("SF_DISABLE_TELEMETRY", "")

	expectRun(t, 0, `Unset Config
┌─────────────────┬─────────┐
│ Name            │ Success │
├─────────────────┼─────────┤
│ org-api-version │ true    │
└─────────────────┴─────────┘

`, "config", "unset", "org-api-version", "--global")

	expectRun(t, 1, `{
  "status": 1,
  "result": {
    "successes": [
      {
        "name": "target-org",
        "success": true
      }
    ],
    "failures": [
      {
        "name": "nosuch",
        "success": false,
        "error": {
          "name": "UnknownConfigKeyError",
          "exitCode": 1
        },
        "message": "Unknown config name: nosuch."
      }
    ]
  },
  "warnings": []
}
`, "config", "unset", "target-org", "nosuch", "--global", "--json")

	data, err := os.ReadFile(filepath.Join(home, ".sf", "config.json"))
	if err != nil || string(data) != "{}" {
		t.Errorf("config.json = %q, err = %v", data, err)
	}
}

func TestConfigSetLocalRequiresProject(t *testing.T) {
	home := useTempHome(t)
	writeTestAuth(t, "me@example.com", nil)

	_, errOut, status := run("config", "set", "target-org=me@example.com")
	want := "Error (InvalidProjectWorkspaceError): " + home + " does not contain a valid Salesforce DX project.\n\n\n" +
		"Try this:\n\nRun with --global to set for your entire workspace.\n\n"
	if status != 1 || errOut != want {
		t.Errorf("status %d, stderr %q, want %q", status, errOut, want)
	}

	project := newProject(t, home)
	expectRun(t, 0, `Set Config
┌────────────┬────────────────┬─────────┐
│ Name       │ Value          │ Success │
├────────────┼────────────────┼─────────┤
│ target-org │ me@example.com │ true    │
└────────────┴────────────────┴─────────┘

`, "config", "set", "target-org=me@example.com")
	data, err := os.ReadFile(filepath.Join(project, ".sf", "config.json"))
	if err != nil || !strings.Contains(string(data), `"target-org": "me@example.com"`) {
		t.Errorf("local config = %q, err = %v", data, err)
	}
}

func TestConfigSetAcceptsAnAlias(t *testing.T) {
	useTempHome(t)
	writeTestAuth(t, "me@example.com", nil)
	if err := lib.SetSFDXAlias("dev", "me@example.com"); err != nil {
		t.Fatal(err)
	}
	if out, errOut, status := run("config", "set", "target-org=dev", "--global"); status != 0 {
		t.Errorf("status %d, stdout %s, stderr %s", status, out, errOut)
	}
	value, found, _ := lib.GetSFConfig(lib.SFConfigTargetOrg)
	if !found || value.Value != "dev" {
		t.Errorf("target-org = %+v", value)
	}
}
