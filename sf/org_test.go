package sf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// The output formats in this file were captured from sf 2.138.6.

func TestOrgLoginAccessTokenWritesSfState(t *testing.T) {
	home := useTempHome(t)
	org := newFakeOrg(t)

	t.Setenv("SF_ACCESS_TOKEN", fakeToken)
	out, errOut, status := run("org", "login", "access-token", "--instance-url", org.URL, "--no-prompt", "--alias", "dev", "--set-default")
	if status != 0 {
		t.Fatalf("status %d, stderr %s", status, errOut)
	}
	if want := "Successfully authorized me@example.com with org ID 00D000000000000\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if want := "Warning: Using insecure protocol: http: on url: " + org.URL + "\n"; errOut != want {
		t.Errorf("stderr = %q, want %q", errOut, want)
	}

	data, err := os.ReadFile(filepath.Join(home, ".sfdx", "me@example.com.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), fakeToken) {
		t.Error("the access token is stored without encryption")
	}
	fields, order, err := lib.ReadSFDXAuthFieldsOrdered("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := "username orgId accessToken instanceUrl loginUrl isDevHub instanceApiVersion instanceApiVersionLastRetrieved name instanceName namespacePrefix isSandbox isScratch trailExpirationDate orgEdition"
	if got := strings.Join(order, " "); got != wantOrder {
		t.Errorf("field order = %s\nwant %s", got, wantOrder)
	}
	if fields.String("accessToken") != fakeToken || fields.String("instanceUrl") != org.URL+"/" ||
		fields.String("instanceApiVersion") != "67.0" || fields.String("name") != "Fake Organization" ||
		fields.Bool("isDevHub") || fields.String("orgEdition") != "Developer Edition" {
		t.Errorf("fields = %v", fields)
	}

	alias, _ := os.ReadFile(filepath.Join(home, ".sfdx", "alias.json"))
	if string(alias) != `{"orgs":{"dev":"me@example.com"}}` {
		t.Errorf("alias.json = %s", alias)
	}
	value, found, _ := lib.GetSFConfig(lib.SFConfigTargetOrg)
	if !found || value.Value != "dev" || value.Location != lib.SFConfigLocationGlobal {
		t.Errorf("target-org = %+v", value)
	}
}

func TestOrgLoginAccessTokenJSON(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	t.Setenv("SF_ACCESS_TOKEN", fakeToken)
	out, _, status := run("org", "login", "access-token", "--instance-url", org.URL, "--no-prompt", "--json")
	if status != 0 {
		t.Fatalf("status %d, output %s", status, out)
	}
	var env struct {
		Status   int
		Result   map[string]interface{}
		Warnings []string
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result["accessToken"] != redactedSecret() || env.Result["username"] != "me@example.com" {
		t.Errorf("result = %v", env.Result)
	}
	if len(env.Warnings) != 2 || !strings.HasPrefix(env.Warnings[0], "Secrets are now hidden from 'sf org login access-token'") ||
		!strings.HasPrefix(env.Warnings[1], "Using insecure protocol: http:") {
		t.Errorf("warnings = %v", env.Warnings)
	}
	if !strings.HasPrefix(out, "{\n  \"status\": 0,\n  \"result\": {\n    \"username\": \"me@example.com\",\n    \"orgId\": \"00D000000000000\",\n    \"accessToken\": ") {
		t.Errorf("result fields are not in sf's order:\n%s", out)
	}
}

func TestOrgLoginAccessTokenRejectsMalformedToken(t *testing.T) {
	useTempHome(t)
	t.Setenv("SF_ACCESS_TOKEN", "not-a-token")
	_, errOut, status := run("org", "login", "access-token", "--instance-url", "https://example.my.salesforce.com", "--no-prompt")
	if status != 1 || !strings.HasPrefix(errOut, "Error (AccessTokenInvalidError): The access token isn't in the correct format.") {
		t.Errorf("status %d, stderr %s", status, errOut)
	}
}

func TestOrgListEmpty(t *testing.T) {
	useTempHome(t)
	expectRun(t, 0, "No Orgs found.\n\nLegend:  🌳=Default DevHub, 🍁=Default Org      Use --all to see expired and deleted scratch orgs\n", "org", "list")
	expectRun(t, 0, "No results found.\n", "org", "list", "auth")
}

func TestOrgListShowsDefaultOrgAndStatus(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")

	expectRun(t, 0, `┌────┬───────┬────────────────┬─────────────────┬───────────┐
│    │ Alias │ Username       │ Org Id          │ Status    │
├────┼───────┼────────────────┼─────────────────┼───────────┤
│ 🍁 │ dev   │ me@example.com │ 00D000000000000 │ Connected │
└────┴───────┴────────────────┴─────────────────┴───────────┘


Legend:  🌳=Default DevHub, 🍁=Default Org      Use --all to see expired and deleted scratch orgs
`, "org", "list")

	out, _, status := run("org", "list", "--json", "--skip-connection-status")
	if status != 0 {
		t.Fatalf("status %d", status)
	}
	var env struct {
		Result struct {
			Other          []map[string]interface{}
			NonScratchOrgs []map[string]interface{}
			DevHubs        []map[string]interface{}
			ScratchOrgs    []map[string]interface{}
			Sandboxes      []map[string]interface{}
		}
		Warnings []string
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Other) != 1 || len(env.Result.NonScratchOrgs) != 1 || len(env.Result.DevHubs) != 0 {
		t.Fatalf("result = %+v", env.Result)
	}
	entry := env.Result.Other[0]
	if entry["accessToken"] != redactedSecret() || entry["alias"] != "dev" || entry["isDefaultUsername"] != true || entry["isDefaultDevHubUsername"] != false {
		t.Errorf("entry = %v", entry)
	}
	if _, present := entry["connectedStatus"]; present {
		t.Error("connectedStatus is present with --skip-connection-status")
	}
	if len(env.Warnings) != 1 || !strings.HasPrefix(env.Warnings[0], "Secrets are now hidden from 'sf org list'") {
		t.Errorf("warnings = %v", env.Warnings)
	}
}

func TestOrgListReportsUnreachableOrg(t *testing.T) {
	useTempHome(t)
	writeTestAuth(t, "down@example.com", lib.SFDXAuthFields{"instanceUrl": "http://127.0.0.1:1/"})
	out, _, status := run("org", "list")
	if status != 0 || !strings.Contains(out, "│ down@example.com │ 00Dxx0000000001EAA │ ECONNREFUSED │") {
		t.Errorf("status %d, output\n%s", status, out)
	}
}

func TestOrgListAuth(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")
	pad := strings.Repeat(" ", len(org.URL)+1-len("Instance Url"))
	line := strings.Repeat("─", len(org.URL)+3)
	expectRun(t, 0, `authenticated orgs
┌───────┬────────────────┬─────────────────┬`+line+`┬─────────────┐
│ Alias │ Username       │ Org Id          │ Instance Url`+pad+` │ Auth Method │
├───────┼────────────────┼─────────────────┼`+line+`┼─────────────┤
│ dev   │ me@example.com │ 00D000000000000 │ `+org.URL+`/ │ token       │
└───────┴────────────────┴─────────────────┴`+line+`┴─────────────┘

`, "org", "list", "auth")

	out, _, _ := run("org", "list", "auth", "--json")
	want := `{
  "status": 0,
  "result": [
    {
      "configs": [
        "target-org"
      ],
      "username": "me@example.com",
      "instanceUrl": "` + org.URL + `/",
      "isScratchOrg": false,
      "isDevHub": false,
      "isSandbox": false,
      "orgId": "00D000000000000",
      "accessToken": "[REDACTED] Use 'sf org auth show-access-token' to view",
      "oauthMethod": "token",
      "isExpired": "unknown",
      "alias": "dev"
    }
  ],
  "warnings": [
    "Secrets are now hidden from 'sf org list auth' command output. Use the 'sf org auth' commands instead. As a temporary workaround, you can set SF_TEMP_SHOW_SECRETS=true to render these secrets. This workaround will be removed in an upcoming release."
  ]
}
`
	if out != want {
		t.Errorf("org list auth --json =\n%s\nwant\n%s", out, want)
	}
}

func TestOrgDisplay(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev")

	_, errOut, status := run("org", "display")
	if status != 1 || errOut != "Error (NoDefaultEnvError): No default environment found. Use -o or --target-org to specify an environment.\n\n\n" {
		t.Errorf("no default org: status %d, stderr %q", status, errOut)
	}

	out, errOut, status := run("org", "display", "--json", "-o", "nosuch")
	if status != 2 || !strings.Contains(out, `"name": "NamedOrgNotFoundError"`) ||
		!strings.Contains(out, `"message": "Parsing --target-org \n\tNo authorization information found for nosuch.\nSee more help with --help"`) ||
		!strings.Contains(out, `"context": "OrgDisplayCommand"`) || !strings.Contains(out, `"exitCode": 2`) || errOut != "" {
		t.Errorf("unknown org: status %d, stdout %s, stderr %s", status, out, errOut)
	}

	want := `{
  "status": 0,
  "result": {
    "id": "00D000000000000",
    "apiVersion": "67.0",
    "accessToken": "[REDACTED] Use 'sf org auth show-access-token' to view",
    "instanceUrl": "` + org.URL + `/",
    "username": "me@example.com",
    "connectedStatus": "Connected",
    "alias": "dev"
  },
  "warnings": [
    "Secrets are now hidden from 'sf org display' command output. Use the 'sf org auth' commands instead. As a temporary workaround, you can set SF_TEMP_SHOW_SECRETS=true to render these secrets. This workaround will be removed in an upcoming release."
  ]
}
`
	for _, args := range [][]string{
		{"org", "display", "-o", "dev", "--json"},
		{"org", "display", "--target-org", "me@example.com", "--json"},
		{"force:org:display", "-u", "dev", "--json"},
		{"org:display", "--targetusername", "dev", "--json"},
	} {
		// A legacy command id and a legacy flag name each add a warning
		// before the warning of the command.
		var deprecations string
		if args[0] == "force:org:display" {
			deprecations += `
    "The \"force org display\" command has been deprecated. Use \"org display\" instead.",`
		}
		switch args[1] {
		case "-u":
			deprecations += `
    "The \"-u\" flag has been deprecated. Use \"--target-org | -o\" instead.",`
		case "--targetusername":
			deprecations += `
    "The \"--targetusername\" flag has been deprecated. Use \"--target-org | -o\" instead.",`
		}
		expected := strings.Replace(want, `"warnings": [`, `"warnings": [`+deprecations, 1)
		expectRun(t, 0, expected, args...)
	}

	out, errOut, status = run("org", "display", "-o", "dev")
	if status != 0 || !strings.HasPrefix(out, "\nOrg Description\n┌") || !strings.Contains(out, "│ Access Token     │ [REDACTED] Use 'sf org auth show-access-token' to view │") ||
		!strings.Contains(out, "│ Connected Status │ Connected ") || !strings.HasPrefix(errOut, "Warning: Secrets are now hidden from 'sf org display' command output.") {
		t.Errorf("human output: status %d\n%s\nstderr %s", status, out, errOut)
	}
}

func TestOrgDisplayUsesTargetOrgConfigAndEnvironment(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")
	if out, _, status := run("org", "display", "--json"); status != 0 || !strings.Contains(out, `"username": "me@example.com"`) {
		t.Errorf("config default: status %d, %s", status, out)
	}
	t.Setenv("SF_TARGET_ORG", "other")
	if out, _, status := run("org", "display", "--json"); status != 1 || !strings.Contains(out, "No authorization information found for other.") {
		t.Errorf("SF_TARGET_ORG is not used before the config value: status %d, %s", status, out)
	}
}

func TestOrgDisplayUser(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")
	out, _, status := run("org", "display", "user", "--json")
	if status != 0 {
		t.Fatalf("status %d, %s", status, out)
	}
	for _, want := range []string{
		`"id": "005000000000001AAA"`, `"profileName": "System Administrator"`, `"username": "me@example.com"`, `"alias": "dev"`,
		"Secrets are now hidden from 'sf org display user' command output.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s:\n%s", want, out)
		}
	}
	out, _, _ = run("org", "display", "user")
	if !strings.HasPrefix(out, "\nUser Description\n┌") || !strings.Contains(out, "│ Profile Name │ System Administrator ") {
		t.Errorf("human output:\n%s", out)
	}
}

func TestOrgLogoutRemovesAuthAliasAndDefault(t *testing.T) {
	home := useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "dev", "--set-default")

	_, errOut, status := run("org", "logout", "-o", "nosuch", "--no-prompt")
	if status != 1 || errOut != "Error (NoAuthFoundForTargetOrgError): No authenticated org found with the nosuch username or alias.\n\n\n" {
		t.Errorf("unknown org: status %d, stderr %q", status, errOut)
	}

	expectRun(t, 0, "Successfully logged out of orgs: me@example.com\n", "org", "logout", "-o", "dev", "--no-prompt")
	if _, err := os.Stat(filepath.Join(home, ".sfdx", "me@example.com.json")); !os.IsNotExist(err) {
		t.Errorf("auth file still exists: %v", err)
	}
	alias, _ := os.ReadFile(filepath.Join(home, ".sfdx", "alias.json"))
	if string(alias) != `{"orgs":{}}` {
		t.Errorf("alias.json = %s", alias)
	}
	if _, found, _ := lib.GetSFConfig(lib.SFConfigTargetOrg); found {
		t.Error("target-org is still set after logout")
	}
	expectRun(t, 0, "No orgs found to log out of.\n", "org", "logout", "--all", "--no-prompt")
}

func TestOrgLogoutJSON(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t)
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    "me@example.com"
  ],
  "warnings": []
}
`, "org", "logout", "--all", "--no-prompt", "--json")
}

func TestExpiredTokenIsRefreshedAndSavedToTheAuthFile(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	writeTestAuth(t, "me@example.com", lib.SFDXAuthFields{
		"accessToken":        "00D000000000000!expired",
		"refreshToken":       "refresh-token",
		"clientId":           "client-id",
		"instanceUrl":        org.URL,
		"loginUrl":           org.URL,
		"instanceApiVersion": "67.0",
	})

	out, errOut, status := run("org", "display", "user", "-o", "me@example.com", "--json")
	if status != 0 {
		t.Fatalf("status %d, stdout %s, stderr %s", status, out, errOut)
	}
	if len(org.refreshRequests) != 1 {
		t.Fatalf("token endpoint requests = %d, want 1", len(org.refreshRequests))
	}
	form := org.refreshRequests[0]
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "refresh-token" || form.Get("client_id") != "client-id" {
		t.Errorf("refresh request = %v", form)
	}
	fields, err := lib.ReadSFDXAuthFields("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if fields.String("accessToken") != fakeToken {
		t.Errorf("stored access token = %q, want the refreshed token", fields.String("accessToken"))
	}
	if fields.String("refreshToken") != "refresh-token" {
		t.Errorf("refresh token changed to %q", fields.String("refreshToken"))
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".force")); !os.IsNotExist(err) {
		t.Errorf("sf mode wrote to ~/.force: %v", err)
	}
}
