package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ForceCLI/force/lib"
)

// The layout and messages in this file follow the sf 2.138.6 org plugin
// source.  They were not compared with sf against a Dev Hub.

func TestScratchSettingsFiles(t *testing.T) {
	parsed, err := parseOrderedJSON([]byte(`{"lightningExperienceSettings":{"enableS1DesktopEnabled":true},"securitySettings":{"passwordPolicies":{"minimumPasswordLength":"10"},"sessionSettings":{"trustedIpRanges":[{"start":"1.1.1.1"},{"start":"2.2.2.2"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	files := scratchSettingsFiles(parsed.(*orderedMap))
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<LightningExperienceSettings xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <enableS1DesktopEnabled>true</enableS1DesktopEnabled>\n</LightningExperienceSettings>\n"
	if got := string(files["settings/LightningExperience.settings"]); got != want {
		t.Errorf("LightningExperience.settings =\n%s\nwant\n%s", got, want)
	}
	security := string(files["settings/Security.settings"])
	if !strings.Contains(security, "    <passwordPolicies>\n        <minimumPasswordLength>10</minimumPasswordLength>\n    </passwordPolicies>\n") ||
		strings.Count(security, "<trustedIpRanges>") != 2 {
		t.Errorf("Security.settings =\n%s", security)
	}
	manifest := string(files["package.xml"])
	if !strings.Contains(manifest, "<members>LightningExperience</members>\n        <members>Security</members>\n        <name>Settings</name>") {
		t.Errorf("package.xml =\n%s", manifest)
	}
}

func TestOrgCreateScratchFromDefinitionFile(t *testing.T) {
	org := devHubSetup(t)
	var request map[string]interface{}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&request)
		// The scratch org answers as a different user from here on.
		org.username = "test-scratch@example.com"
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"2SR000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo/2SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"attributes":{"type":"ScratchOrgInfo"},"Id":"2SR000000000001AAA","Status":"Active","SignupUsername":"test-scratch@example.com","LoginUrl":"`+org.URL+`","AuthCode":"auth-code","ScratchOrg":"00D000000000000","ExpirationDate":"2026-12-31","SignupInstance":"CS99","Edition":"Developer"}`)
	}
	var deployed bool
	org.handlers["/services/Soap/m/67.0"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(body), "checkDeployStatus") {
			io.WriteString(w, soapBody("<checkDeployStatusResponse><result><done>true</done><id>0Af000000000001AAA</id><status>Succeeded</status><success>true</success></result></checkDeployStatusResponse>"))
			return
		}
		deployed = true
		io.WriteString(w, soapBody("<deployResponse><result><done>false</done><id>0Af000000000001AAA</id></result></deployResponse>"))
	}
	definition := filepath.Join(os.Getenv("HOME"), "project-scratch-def.json")
	os.WriteFile(definition, []byte(`{
  "orgName": "Acme Company",
  "edition": "enterprise",
  "features": ["EnableSetPasswordInApi", "Communities"],
  "settings": {"lightningExperienceSettings": {"enableS1DesktopEnabled": true}}
}`), 0o644)

	out, errOut, status := run("org", "create", "scratch", "-f", definition, "-a", "scratch", "-d", "-y", "3")
	if status != 0 {
		t.Fatalf("status %d, stderr %s\n%s", status, errOut, out)
	}
	for _, want := range []string{
		" ────────────── Creating Scratch Org ──────────────\n", " ✔ Wait For Org ", " ✔ Deploy Settings ", " ✔ Done ",
		" Request Id: 2SR000000000001AAA (" + org.URL + "/2SR000000000001AAA)\n", " OrgId: 00D000000000000\n", " Username: test-scratch@example.com\n", " Alias: scratch\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "Your scratch org is ready.\n") {
		t.Errorf("output does not end with the success message:\n%s", out)
	}
	if request["OrgName"] != "Acme Company" || request["Edition"] != "Enterprise" || request["Features"] != "EnableSetPasswordInApi;Communities" ||
		request["DurationDays"] != float64(3) || request["ConnectedAppConsumerKey"] != "PlatformCLI" || request["Namespace"] != "acme" {
		t.Errorf("ScratchOrgInfo request = %v", request)
	}
	if !deployed {
		t.Error("the settings were not deployed")
	}

	fields, err := lib.ReadSFDXAuthFields("test-scratch@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if fields.String("devHubUsername") != "me@example.com" || !fields.Bool("isScratch") || fields.String("expirationDate") != "2026-12-31" ||
		fields.String("refreshToken") != "new-refresh-token" || fields.String("clientId") != "PlatformCLI" {
		t.Errorf("scratch auth fields = %v", fields)
	}
	if lib.ResolveSFDXUsername("scratch") != "test-scratch@example.com" {
		t.Error("the alias was not set")
	}
	if value, _, _ := lib.GetSFConfig(lib.SFConfigTargetOrg); value.Value != "scratch" {
		t.Errorf("target-org = %+v", value)
	}
	// The Dev Hub authorization is unchanged.
	if hub, err := lib.ReadSFDXAuthFields("me@example.com"); err != nil || hub.Bool("isScratch") {
		t.Errorf("Dev Hub auth = %v, err = %v", hub, err)
	}

	// The scratch org is listed as a scratch org, with the status of its
	// record in the Dev Hub.
	org.queries["FROM ScratchOrgInfo WHERE ScratchOrg IN ('00D000000000000')"] = []map[string]interface{}{{
		"attributes": map[string]interface{}{"type": "ScratchOrgInfo"}, "Status": "Active", "SignupUsername": "test-scratch@example.com", "ScratchOrg": "00D000000000000",
		"ExpirationDate": "2026-12-31", "Edition": "Developer", "OrgName": "Company", "CreatedDate": "2026-11-01T00:00:00.000+0000", "Namespace": nil,
		"CreatedBy": map[string]interface{}{"Username": "me@example.com"}, "LoginUrl": org.URL,
	}}
	out, _, _ = run("org", "list", "--json", "--skip-connection-status")
	if !strings.Contains(out, `"scratchOrgs": [
      {`) || !strings.Contains(out, `"devHubUsername": "me@example.com"`) || !strings.Contains(out, `"status": "Active",
        "isExpired": false,
        "namespace": null`) {
		t.Errorf("org list:\n%s", out)
	}
	out, _, _ = run("org", "list")
	if !strings.Contains(out, "│ Scratch │") || !strings.Contains(out, "test-scratch@example.com │ 00D000000000000 │ Active") {
		t.Errorf("org list:\n%s", out)
	}
	// A scratch org has an expiration date, which gives the table a column.
	if !strings.Contains(out, "│ Status    │ Expires    │") || !strings.Contains(out, "│ Active    │ 2026-12-31 │") {
		t.Errorf("org list lacks the expiration date:\n%s", out)
	}
	out, _, _ = run("org", "list", "--verbose", "--skip-connection-status")
	if !strings.Contains(out, "│ Instance Url ") || !strings.Contains(out, "│ Namespace │ Dev Hub Id      │ Created                      │ Expires    │") ||
		!strings.Contains(out, "│ 00D000000000000 │ 2026-11-01T00:00:00.000+0000 │ 2026-12-31 │") {
		t.Errorf("org list --verbose:\n%s", out)
	}

	// sf describes the org from that record, without a connected status.
	out, _, _ = run("org", "display", "-o", "test-scratch@example.com", "--json")
	for _, want := range []string{`"devHubId": "me@example.com"`, `"status": "Active"`, `"expirationDate": "2026-12-31"`, `"createdBy": "me@example.com"`, `"edition": "Developer"`, `"orgName": "Company"`, `"signupUsername": "test-scratch@example.com"`} {
		if !strings.Contains(out, want) {
			t.Errorf("org display lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "connectedStatus") {
		t.Errorf("org display of a scratch org has a connected status:\n%s", out)
	}
	out, _, _ = run("org", "display", "-o", "test-scratch@example.com")
	if !strings.Contains(out, "│ Dev Hub Id      │ me@example.com") || !strings.Contains(out, "│ Status          │ Active") {
		t.Errorf("org display:\n%s", out)
	}

	// A deleted scratch org is listed only with --all, and a scratch org
	// without a record in the Dev Hub is not listed and cannot be
	// displayed.
	org.queries["FROM ScratchOrgInfo WHERE ScratchOrg IN ('00D000000000000')"] = []map[string]interface{}{{
		"attributes": map[string]interface{}{"type": "ScratchOrgInfo"}, "Status": "Deleted", "SignupUsername": "test-scratch@example.com", "ScratchOrg": "00D000000000000",
		"ExpirationDate": "2026-12-31", "Edition": "Developer", "OrgName": "Company", "CreatedDate": "2026-11-01T00:00:00.000+0000", "Namespace": nil,
		"CreatedBy": map[string]interface{}{"Username": "me@example.com"}, "LoginUrl": org.URL,
	}}
	if out, _, _ := run("org", "list"); strings.Contains(out, "test-scratch@example.com") {
		t.Errorf("a deleted scratch org is listed without --all:\n%s", out)
	}
	if out, _, _ := run("org", "list", "--all"); !strings.Contains(out, "test-scratch@example.com │ 00D000000000000 │ Deleted") {
		t.Errorf("org list --all:\n%s", out)
	}
	org.queries["FROM ScratchOrgInfo WHERE ScratchOrg IN ('00D000000000000')"] = nil
	if out, _, _ := run("org", "list", "--all"); strings.Contains(out, "test-scratch@example.com") {
		t.Errorf("a scratch org without a Dev Hub record is listed:\n%s", out)
	}
	_, errOut, status = run("org", "display", "-o", "test-scratch@example.com")
	if status != 1 || !strings.Contains(errOut, "Error (NoScratchInfo): No information for scratch org with ID 00D000000000000 found in Dev Hub me@example.com.") {
		t.Errorf("org display without a record: status %d, stderr %q", status, errOut)
	}

	// --clean removes the authorizations of scratch orgs that are not
	// active.  Without --no-prompt it asks, and an empty answer is "no".
	org.queries["FROM ScratchOrgInfo WHERE ScratchOrg IN ('00D000000000000')"] = []map[string]interface{}{{
		"attributes": map[string]interface{}{"type": "ScratchOrgInfo"}, "Status": "Deleted", "SignupUsername": "test-scratch@example.com", "ScratchOrg": "00D000000000000",
		"ExpirationDate": "2026-12-31", "Edition": "Developer", "OrgName": "Company", "CreatedDate": "2026-11-01T00:00:00.000+0000", "Namespace": nil,
		"CreatedBy": map[string]interface{}{"Username": "me@example.com"}, "LoginUrl": org.URL,
	}}
	_, cleanErr, _ := run("org", "list", "--skip-connection-status")
	if !strings.Contains(cleanErr, "You have 1 expired or deleted local scratch org authorizations.") {
		t.Errorf("warning for an inactive scratch org: %q", cleanErr)
	}
	if _, cleanErr, _ = run("org", "list", "--clean", "--skip-connection-status"); !strings.Contains(cleanErr, "Found (1) org configurations to delete. Are you sure (yes/no)?") {
		t.Errorf("clean prompt: %q", cleanErr)
	}
	if _, err := lib.ReadSFDXAuthFields("test-scratch@example.com"); err != nil {
		t.Errorf("a declined clean removed the authorization: %v", err)
	}
	if _, errOut, status := run("org", "list", "--clean", "--no-prompt", "--skip-connection-status"); status != 0 || strings.Contains(errOut, "expired or deleted") {
		t.Errorf("clean: status %d, stderr %q", status, errOut)
	}
	if _, err := lib.ReadSFDXAuthFields("test-scratch@example.com"); err == nil {
		t.Errorf("--clean kept the authorization of the deleted scratch org")
	}
	if _, err := lib.ReadSFDXAuthFields("me@example.com"); err != nil {
		t.Errorf("--clean removed the Dev Hub authorization: %v", err)
	}
}

// An access token that the org refuses is not reported as connected. The
// list of API versions is answered without a token, so the status comes
// from a request that needs one.
func TestConnectedStatusNeedsAValidAccessToken(t *testing.T) {
	org := loggedInFakeOrg(t)
	refuse := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `[{"errorCode":"INVALID_SESSION_ID","message":"Session expired or invalid"}]`)
	}
	org.handlers["/services/data/v67.0"] = refuse
	org.handlers["/services/data/v67.0/"] = refuse
	out, errOut, status := run("org", "display", "-o", "dev")
	if status != 0 || strings.Contains(out, "│ Connected Status │ Connected") || !strings.HasPrefix(errOut, "Warning: unable to refresh auth for org\n") {
		t.Errorf("status %d, stdout\n%s\nstderr %q", status, out, errOut)
	}
	if out, _, _ := run("org", "list"); strings.Contains(out, "│ Connected") {
		t.Errorf("org list reports the org as connected:\n%s", out)
	}
}

func TestOrgCreateScratchValidation(t *testing.T) {
	devHubSetup(t)
	_, errOut, status := run("org", "create", "scratch")
	if status != 2 || !strings.Contains(errOut, "Exactly one of the following must be provided: --definition-file, --edition, --snapshot, --source-org") {
		t.Errorf("no definition: status %d, stderr %q", status, errOut)
	}
	definition := filepath.Join(os.Getenv("HOME"), "def.json")
	os.WriteFile(definition, []byte(`{"edition":"Developer","objectSettings":{}}`), 0o644)
	_, errOut, status = run("org", "create", "scratch", "-f", definition)
	if status != 1 || !strings.Contains(errOut, `The scratch org definition key "objectSettings" is not supported`) {
		t.Errorf("unsupported key: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "create", "scratch", "-e", "developer", "--snapshot", "snap")
	if status != 2 || !strings.Contains(errOut, "--edition=developer cannot also be provided when using --snapshot") {
		t.Errorf("--snapshot with --edition: status %d, stderr %q", status, errOut)
	}
	os.WriteFile(definition, []byte(`{"features":["Communities"],"edition":"Developer"}`), 0o644)
	_, errOut, status = run("org", "create", "scratch", "-f", definition, "--snapshot", "snap")
	if status != 1 || !strings.Contains(errOut, "Error (orgSnapshot): Org snapshots don’t support one or more options you specified: features, edition") {
		t.Errorf("--snapshot with features: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "create", "scratch", "--source-org", "00D000000000001AAA")
	if status != 2 || !strings.Contains(errOut, "The id must be 15 characters.") {
		t.Errorf("--source-org: status %d, stderr %q", status, errOut)
	}
}

func TestOrgCreateScratchSnapshotSourceOrgAndAncestors(t *testing.T) {
	org := devHubSetup(t)
	var request map[string]interface{}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo"] = func(w http.ResponseWriter, r *http.Request) {
		request = nil
		json.NewDecoder(r.Body).Decode(&request)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"2SR000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo/2SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"attributes":{"type":"ScratchOrgInfo"},"Id":"2SR000000000001AAA","Status":"New"}`)
	}
	var queries []string
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		queries = append(queries, query)
		switch {
		case strings.Contains(query, "MajorVersion = 1 AND MinorVersion = 2 AND PatchVersion = 0"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"05i000000000012AAA","IsReleased":true}]}`)
		case strings.Contains(query, "SubscriberPackageVersionId = '04t000000000001AAA'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"05i000000000001AAA"}]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		}
	}
	project, _ := os.Getwd()
	writeProject := func(directories string) {
		os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte(`{"packageDirectories":[`+directories+`],"packageAliases":{"billing":"0Ho000000000001AAA","billing@1.0.0-1":"04t000000000001AAA"}}`), 0o644)
	}

	if _, _, status := run("org", "create", "scratch", "--snapshot", "nightly", "--async"); status != 0 || request["Snapshot"] != "nightly" || request["Edition"] != nil {
		t.Errorf("snapshot: status %d, request %v", status, request)
	}
	if _, _, status := run("org", "create", "scratch", "--source-org", "00D000000000001", "--async"); status != 0 || request["SourceOrg"] != "00D000000000001" {
		t.Errorf("source org: status %d, request %v", status, request)
	}
	if _, sent := request["Package2AncestorIds"]; sent {
		t.Errorf("a project without packages sent ancestors: %v", request)
	}

	// The ancestors named by version, by 04t Id and by alias are sent as
	// Package2Version Ids.  NONE names no ancestor.
	writeProject(`{"path":"force-app","default":true,"package":"billing","ancestorVersion":"1.2.0"},` +
		`{"path":"other","package":"billing","ancestorId":"04t000000000001AAA"},` +
		`{"path":"third","package":"billing","ancestorId":"billing@1.0.0-1"},` +
		`{"path":"fourth","package":"billing","ancestorId":"NONE"}`)
	if _, errOut, status := run("org", "create", "scratch", "-e", "developer", "--async"); status != 0 || request["Package2AncestorIds"] != "05i000000000012AAA;05i000000000001AAA;04t000000000001AAA" {
		t.Errorf("ancestors: status %d, stderr %q, request %v", status, errOut, request)
	}
	if !strings.Contains(strings.Join(queries, "\n"), "WHERE Package2Id = '0Ho000000000001AAA' AND MajorVersion = 1 AND MinorVersion = 2 AND PatchVersion = 0 and IsReleased = true") {
		t.Errorf("queries = %q", queries)
	}
	for _, flag := range []string{"--no-ancestors", "--no-namespace"} {
		run("org", "create", "scratch", "-e", "developer", "--async", flag)
		if _, sent := request["Package2AncestorIds"]; sent {
			t.Errorf("%s sent ancestors: %v", flag, request)
		}
	}

	writeProject(`{"path":"force-app","default":true,"package":"billing","ancestorVersion":"9.9.9"}`)
	_, errOut, status := run("org", "create", "scratch", "-e", "developer", "--async")
	if status != 1 || !strings.Contains(errOut, "Error (NoMatchingAncestorError): The ancestor for ancestorVersion 9.9.9 can't be found. Package ID billing.") || !strings.Contains(errOut, "hasn’t been promoted and released") {
		t.Errorf("unknown ancestor: status %d, stderr %q", status, errOut)
	}
	writeProject(`{"path":"force-app","default":true,"package":"billing","ancestorVersion":"1.2"}`)
	if _, errOut, status = run("org", "create", "scratch", "-e", "developer", "--async"); status != 1 || !strings.Contains(errOut, "The ancestor versionNumber must be in the format major.minor.patch but the value found is 1.2") {
		t.Errorf("version format: status %d, stderr %q", status, errOut)
	}
}

func TestOrgCreateScratchAsyncAndResume(t *testing.T) {
	org := devHubSetup(t)
	home := os.Getenv("HOME")
	cachePath := filepath.Join(home, ".sf", "scratch-create-cache.json")
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"2SR000000000001AAA","success":true,"errors":[]}`)
	}
	requestStatus := "New"
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo/2SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		if requestStatus == "Active" {
			// The scratch org answers as a different user from here on.
			org.username = "test-scratch@example.com"
		}
		io.WriteString(w, `{"attributes":{"type":"ScratchOrgInfo"},"Id":"2SR000000000001AAA","Status":"`+requestStatus+`","SignupUsername":"test-scratch@example.com","LoginUrl":"`+org.URL+`","AuthCode":"auth-code","ScratchOrg":"00D000000000000","ExpirationDate":"2026-12-31","SignupInstance":"CS99","Edition":"Developer"}`)
	}

	// Nothing to resume yet.
	_, errOut, status := run("org", "resume", "scratch", "--use-most-recent")
	if status != 1 || errOut != "Error (NoRecentJobIdError): There are no recent job IDs (ScratchOrgInfo requests) in your cache. Maybe it completed or already resumed?\n\n\n" {
		t.Errorf("empty cache: status %d, stderr %q", status, errOut)
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "Error (2): The following error occurred:\n  Exactly one of the following must be provided: --use-most-recent, --job-id\nSee more help with --help\n\n\n"},
		{[]string{"-i", "0Af000000000001AAA"}, "Error (InvalidPrefixError): Parsing --job-id \n\tThe id must begin with 2SR.\nSee more help with --help\n\n\n"},
	} {
		if _, errOut, status := run(append([]string{"org", "resume", "scratch"}, c.args...)...); status != 2 || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}

	out, errOut, status := run("org", "create", "scratch", "-e", "developer", "-a", "later", "--async")
	if status != 0 {
		t.Fatalf("async: status %d, stderr %s\n%s", status, errOut, out)
	}
	want := `
 ────────── Creating Scratch Org (async) ──────────

 ✔ Prepare Request T
 ✔ Send Request T
 ✔ Done T

 Request Id: 2SR000000000001AAA (HUB/2SR000000000001AAA)
 OrgId: 00D000000000000
 Username: test-scratch@example.com
 Alias: later
 Elapsed Time: T

Resume scratch org creation by running "sf org resume scratch --job-id 2SR000000000001AAA"
`
	if got := strings.ReplaceAll(durationPattern.ReplaceAllString(out, "T"), org.URL, "HUB"); got != want {
		t.Errorf("async output =\n%s\nwant\n%s", got, want)
	}
	cache := regexp.MustCompile(`"timestamp": "[^"]+"`).ReplaceAllString(readFile(t, cachePath), `"timestamp": "T"`)
	wantCache := `{
  "2SR000000000001AAA": {
    "hubUsername": "me@example.com",
    "hubBaseUrl": "` + org.URL + `",
    "definitionjson": {
      "edition": "developer"
    },
    "alias": "later",
    "setDefault": false,
    "tracksSource": true,
    "timestamp": "T"
  }
}`
	if cache != wantCache {
		t.Errorf("cache =\n%s\nwant\n%s", cache, wantCache)
	}
	if _, err := lib.ReadSFDXAuthFields("test-scratch@example.com"); err == nil {
		t.Error("an async create authorized the org")
	}

	// The org is not ready, and the command does not wait by default.
	out, errOut, status = run("org", "resume", "scratch", "-i", "2SR000000000001AAA")
	if status != 1 || errOut != "Error (StillInProgressError): The scratch org isn't ready yet (Status = New).\n\n\nTry this:\n\nIncrease the --wait time because creating a scratch org can take a while.\n\n" ||
		!strings.Contains(out, " ✘ Send Request ") || !strings.Contains(out, " Request Id: ✘\n OrgId: ✘\n Username: ✘\n") {
		t.Errorf("not ready: status %d, stderr %q\n%s", status, errOut, out)
	}
	// A job id that is not in the cache.
	_, errOut, status = run("org", "resume", "scratch", "-i", "2SR000000000009AAA")
	if status != 1 || errOut != "Error (JobIdMismatchError): There are no recent job IDs (ScratchOrgInfo requests) in your cache that match 2SR000000000009AAA. Maybe it completed?\n\n\n" {
		t.Errorf("other id: status %d, stderr %q", status, errOut)
	}

	requestStatus = "Active"
	out, errOut, status = run("org", "resume", "scratch", "--use-most-recent")
	if status != 0 {
		t.Fatalf("resume: status %d, stderr %s\n%s", status, errOut, out)
	}
	want = `
 ────────────── Resuming Scratch Org ──────────────

 ◯ Prepare Request - Skipped
 ✔ Send Request T
 ◯ Wait For Org - Skipped
 ✔ Available T
 ✔ Authenticate T
 ✔ Deploy Settings T
 ✔ Done T

 Request Id: 2SR000000000001AAA (HUB/2SR000000000001AAA)
 OrgId: 00D000000000000
 Username: test-scratch@example.com
 Alias: later
 Elapsed Time: T


Your scratch org is ready.
`
	if got := strings.ReplaceAll(durationPattern.ReplaceAllString(out, "T"), org.URL, "HUB"); got != want {
		t.Errorf("resume output =\n%s\nwant\n%s", got, want)
	}
	fields, err := lib.ReadSFDXAuthFields("test-scratch@example.com")
	if err != nil || fields.String("devHubUsername") != "me@example.com" || !fields.Bool("isScratch") || fields["tracksSource"] != true {
		t.Errorf("scratch auth fields = %v, err %v", fields, err)
	}
	if lib.ResolveSFDXUsername("later") != "test-scratch@example.com" {
		t.Error("the alias was not set")
	}
	if got := readFile(t, cachePath); got != "{}" {
		t.Errorf("cache after resume = %s", got)
	}
}

func TestStageFrameWrapsALongValueUnderItsStart(t *testing.T) {
	t.Setenv("FORCE_COLOR", "0")
	var b strings.Builder
	writeStageFrame(&b, "Resuming Scratch Org", "", false, []stage{{label: "Done", state: "done"}}, [][2]string{
		{"Request Id", "2SRRp000000FepxOAC (https://example-dev-hub-domain.my.salesforce.com/2SRRp000000FepxOAC)"},
		{"OrgId", "00DEc00000qjY7d"},
	})
	want := `
 ────────────── Resuming Scratch Org ──────────────

 ✔ Done 0ms

 Request Id: 2SRRp000000FepxOAC
             (https://example-dev-hub-domain.my.salesforce.com/2SRRp000000FepxOAC)
 OrgId: 00DEc00000qjY7d

`
	if b.String() != want {
		t.Errorf("frame =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestOrgCreateScratchWithAClientId(t *testing.T) {
	org := devHubSetup(t)
	var request map[string]interface{}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&request)
		org.username = "test-scratch@example.com"
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"2SR000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo/2SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"attributes":{"type":"ScratchOrgInfo"},"Id":"2SR000000000001AAA","Status":"Active","SignupUsername":"test-scratch@example.com","LoginUrl":"`+org.URL+`","AuthCode":"auth-code","ScratchOrg":"00D000000000000","ExpirationDate":"2026-12-31","SignupInstance":"CS99","Edition":"Developer"}`)
	}
	// The secret is read from standard input, which is empty here.
	_, errOut, status := run("org", "create", "scratch", "-e", "developer", "-i", lib.ClientId)
	if status != 0 || !strings.HasPrefix(errOut, "? OAuth client secret of your personal connected app ") {
		t.Fatalf("status %d, stderr %s", status, errOut)
	}
	// The connected app of force has a callback URL of its own.
	if request["ConnectedAppConsumerKey"] != lib.ClientId || request["ConnectedAppCallbackUrl"] != lib.RedirectUri {
		t.Errorf("ScratchOrgInfo request = %v", request)
	}
	fields, err := lib.ReadSFDXAuthFields("test-scratch@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if fields.String("clientId") != lib.ClientId {
		t.Errorf("clientId = %q", fields.String("clientId"))
	}
	if client := scratchOrgClient("3MVGcustom", "secret"); client.RedirectURI != "http://localhost:1717/OauthRedirect" || client.ClientSecret != "secret" {
		t.Errorf("custom client = %+v", client)
	}
}

func TestOrgCreateScratchTriesTheCallbackURLOfForce(t *testing.T) {
	org := devHubSetup(t)
	var callbacks []string
	accept := false
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo"] = func(w http.ResponseWriter, r *http.Request) {
		var request map[string]interface{}
		json.NewDecoder(r.Body).Decode(&request)
		callbacks = append(callbacks, request["ConnectedAppCallbackUrl"].(string))
		if accept && request["ConnectedAppCallbackUrl"] == lib.RedirectUri {
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":"2SR000000000001AAA","success":true,"errors":[]}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `[{"message":"The callback URL provided is not valid","errorCode":"FIELD_INTEGRITY_EXCEPTION","fields":[]}]`)
	}
	_, errOut, status := run("org", "create", "scratch", "-e", "developer", "-i", "3MVGcustom")
	if status != 1 || len(callbacks) != 2 || callbacks[0] != "http://localhost:1717/OauthRedirect" || callbacks[1] != lib.RedirectUri ||
		!strings.Contains(errOut, "The callback URL provided is not valid") || !strings.Contains(errOut, "to the callback URLs of the connected app 3MVGcustom") {
		t.Errorf("status %d, callbacks %v, stderr %q", status, callbacks, errOut)
	}
	// An app with the callback URL of force is accepted on the second
	// request.
	accept, callbacks = true, nil
	org.handlers["/services/data/v67.0/sobjects/ScratchOrgInfo/2SR000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Id":"2SR000000000001AAA","Status":"New","SignupUsername":"test-scratch@example.com"}`)
	}
	if _, errOut, status := run("org", "create", "scratch", "-e", "developer", "-i", "3MVGcustom", "--async"); status != 0 || len(callbacks) != 2 {
		t.Errorf("second callback: status %d, callbacks %v, stderr %q", status, callbacks, errOut)
	}
}
