package sf

import (
	"fmt"
	"github.com/ForceCLI/force/lib"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sandboxOrg is a fake production org.  The sandbox process it reports is
// taken from processes, one for each query, and the last one is repeated.
type sandboxOrg struct {
	*fakeOrg
	processes []string
	requests  []string
	queries   []string
}

func sandboxProcessJSON(status string, progress int, finished bool) string {
	end := "null"
	if finished {
		end = `"2026-01-02T04:04:05.000+0000"`
	}
	return fmt.Sprintf(`{"attributes":{"type":"SandboxProcess"},"Id":"0GR000000000001AAA","Status":"%s","SandboxName":"dev1","SandboxInfoId":"0GQ000000000001AAA","LicenseType":"DEVELOPER","CreatedDate":"2026-01-02T03:04:05.000+0000","CopyProgress":%d,"SandboxOrganization":null,"SourceId":null,"Description":null,"EndDate":%s,"Features":null}`, status, progress, end)
}

func newSandboxOrg(t *testing.T) *sandboxOrg {
	t.Helper()
	useTempHome(t)
	sleep := sandboxSleep
	sandboxSleep = func(time.Duration) {}
	t.Cleanup(func() { sandboxSleep = sleep })
	org := &sandboxOrg{fakeOrg: newFakeOrg(t)}
	org.login(t, "--alias", "prod", "--set-default")
	// The sandbox is served by the same fake org, under another username.
	org.username = "me@example.com.dev1"
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		org.queries = append(org.queries, query)
		var records []string
		switch {
		case strings.Contains(query, "FROM SandboxProcess"):
			if len(org.processes) > 0 {
				records = []string{org.processes[0]}
				if len(org.processes) > 1 {
					org.processes = org.processes[1:]
				}
			}
		case strings.Contains(query, "FROM SourceTrackingSettings"):
			records = []string{`{"IsSourceTrackingSandboxesEnabled":true}`}
		case strings.Contains(query, "FROM SandboxInfo WHERE SandboxName='dev1'"):
			records = []string{`{"attributes":{"type":"SandboxInfo"},"Id":"0GQ000000000001AAA","SandboxName":"dev1","LicenseType":"DEVELOPER","HistoryDays":0,"CopyChatter":false,"AutoActivate":false,"Description":null,"SourceId":null,"IsDeleted":false,"CreatedDate":"2026-01-01T00:00:00.000+0000","CreatedById":"005000000000001AAA","LastModifiedDate":"2026-01-01T00:00:00.000+0000","LastModifiedById":"005000000000001AAA"}`}
		}
		io.WriteString(w, fmt.Sprintf(`{"done":true,"totalSize":%d,"records":[%s]}`, len(records), strings.Join(records, ",")))
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/SandboxInfo"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		org.requests = append(org.requests, r.Method+" SandboxInfo "+string(body))
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"0GQ000000000001AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/SandboxInfo/0GQ000000000001AAA"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		org.requests = append(org.requests, r.Method+" SandboxInfo/0GQ000000000001AAA "+string(body))
		w.WriteHeader(http.StatusNoContent)
	}
	org.handlers["/services/data/v67.0/tooling/sandboxAuth"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		org.requests = append(org.requests, r.Method+" sandboxAuth "+string(body))
		io.WriteString(w, fmt.Sprintf(`{"authUserName":"me@example.com.dev1","authCode":"code1","instanceUrl":"%s","loginUrl":"%s"}`, org.URL, org.URL))
	}
	return org
}

func sandboxCacheText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".sf", "sandbox-create-cache.json"))
	if err != nil {
		return ""
	}
	return string(data)
}

func TestOrgCreateSandboxAsyncAndResume(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Pending", 0, false)}
	out, errOut, status := run("org", "create", "sandbox", "-n", "dev1", "-l", "Developer_Pro", "-a", "mydev", "--async", "--no-prompt")
	if status != 68 || errOut != "" {
		t.Fatalf("async create: status %d, stderr %q\n%s", status, errOut, out)
	}
	for _, want := range []string{
		"─ Sandbox Create (async) ─", " ▶ Creating new sandbox ", "   ▸ Name: dev1\n   ▸ ID: 0GR000000000001AAA\n", " ◼ Refreshing org\n ◼ Authenticating\n",
		" Status: Pending\n Copy progress: 0%\n Elapsed Time: ",
		"Run \"sf org resume sandbox --job-id 0GR000000000001AAA -o me@example.com\" to check for status.\nIf the org is ready, checking the status also authorizes the org for use with Salesforce CLI.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("async create lacks %q:\n%s", want, out)
		}
	}
	if len(org.requests) != 1 || org.requests[0] != `POST SandboxInfo {"SandboxName":"dev1","LicenseType":"Developer_Pro"}` {
		t.Errorf("requests = %q", org.requests)
	}
	cache := sandboxCacheText(t)
	for _, want := range []string{`"dev1": {`, `"alias": "mydev"`, `"setDefault": false`, `"prodOrgUsername": "me@example.com"`, `"action": "Create"`, `"Id": "0GR000000000001AAA"`, `"timestamp": "`} {
		if !strings.Contains(cache, want) {
			t.Errorf("cache lacks %s:\n%s", want, cache)
		}
	}

	// The sandbox is not ready.
	out, _, status = run("org", "resume", "sandbox", "--use-most-recent", "--json")
	if status != 68 || !strings.Contains(out, `"status": 68`) || !strings.Contains(out, `"Message": "The sandbox creation has not completed."`) || !strings.Contains(out, `"SandboxUsername": "me@example.com.dev1"`) {
		t.Errorf("resume of a pending sandbox: status %d\n%s", status, out)
	}
	_, errOut, status = run("org", "resume", "sandbox")
	if status != 1 || errOut != "Error (NoSandboxNameOrJobIdError): No sandbox name or job ID were provided.\n\n\n" {
		t.Errorf("resume without a name: status %d, stderr %q", status, errOut)
	}

	// The sandbox is ready: it is authorized, and the alias of the
	// request is set.
	org.processes = []string{sandboxProcessJSON("Completed", 100, true)}
	org.requests = nil
	out, errOut, status = run("org", "resume", "sandbox", "-n", "dev1")
	if status != 0 {
		t.Fatalf("resume: status %d, stderr %q\n%s", status, errOut, out)
	}
	for _, want := range []string{"─ Resume Sandbox ─", " ✔ Creating new sandbox ", " ✔ Authenticating ", " Status: Completed\n Copy progress: 100%\n Elapsed Time: ",
		"Your sandbox is ready.\nYou can open it by running \"sf org open -o me@example.com.dev1\"\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("resume lacks %q:\n%s", want, out)
		}
	}
	if len(org.requests) != 1 || org.requests[0] != `POST sandboxAuth {"clientId":"PlatformCLI","sandboxName":"dev1","callbackUrl":"http://localhost:1717/OauthRedirect"}` {
		t.Errorf("requests = %q", org.requests)
	}
	sandbox, err := loadOrg("mydev")
	if err != nil {
		t.Fatalf("the alias does not resolve: %v", err)
	}
	if sandbox.Username != "me@example.com.dev1" || !sandbox.Fields.Bool("isSandbox") || !sandbox.Fields.Bool("tracksSource") || sandbox.Fields.String("refreshToken") == "" {
		t.Errorf("sandbox authorization: %s, fields %v", sandbox.Username, sandbox.Fields)
	}
	config, err := readSandboxConfig(sandbox.Fields.String("orgId"))
	if err != nil || config.ProdOrgUsername != "me@example.com" || config.SandboxName != "dev1" || config.SandboxInfoID != "0GQ000000000001AAA" || config.SandboxProcessID != "0GR000000000001AAA" {
		t.Errorf("sandbox file: %+v, %v", config, err)
	}
	if cache := sandboxCacheText(t); strings.Contains(cache, "dev1") {
		t.Errorf("the cache entry was not removed:\n%s", cache)
	}
	if _, errOut, status = run("org", "resume", "sandbox", "--use-most-recent"); status != 1 || !strings.Contains(errOut, "Error (LatestSandboxRequestNotFoundError): Retry the command using either the --name or --job-id flags.") {
		t.Errorf("resume with an empty cache: status %d, stderr %q", status, errOut)
	}
}

func TestOrgCreateSandboxWaitsForTheSandbox(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Pending", 0, false), sandboxProcessJSON("Processing", 40, false), sandboxProcessJSON("Completed", 100, true)}
	out, errOut, status := run("org", "create", "sandbox", "-n", "dev1", "--no-prompt", "--set-default", "--no-track-source", "--json")
	if status != 0 || !strings.Contains(out, `"Status": "Completed"`) || !strings.Contains(out, `"SandboxUsername": "me@example.com.dev1"`) {
		t.Fatalf("create: status %d, stderr %q\n%s", status, errOut, out)
	}
	if org.requests[0] != `POST SandboxInfo {"SandboxName":"dev1","LicenseType":"Developer"}` {
		t.Errorf("request = %s", org.requests[0])
	}
	sandbox, err := loadOrg("me@example.com.dev1")
	if err != nil || sandbox.Fields.Bool("tracksSource") {
		t.Errorf("sandbox authorization: %v, %v", sandbox, err)
	}
	if out, _, _ := run("config", "get", "target-org", "--json"); !strings.Contains(out, `"value": "me@example.com.dev1"`) {
		t.Errorf("default org:\n%s", out)
	}

	// The wait ends before the sandbox is ready.
	org.processes = []string{sandboxProcessJSON("Processing", 40, false)}
	// The first run made the sandbox the default org, so the production
	// org is named.
	out, errOut, status = run("org", "create", "sandbox", "-n", "dev1", "--no-prompt", "-w", "1", "-i", "30", "-o", "prod")
	if status != 68 || errOut != "Warning: The wait time for the sandbox create has been exhausted. See the results below for more information.\n" ||
		!strings.Contains(out, " ▶ Creating new sandbox ") || !strings.Contains(out, " Status: Processing\n Copy progress: 40%\n") || !strings.Contains(out, "Run \"sf org resume sandbox --job-id 0GR000000000001AAA -o me@example.com\"") {
		t.Errorf("timeout: status %d, stderr %q\n%s", status, errOut, out)
	}
}

func TestOrgCreateSandboxRequestAndFlags(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Pending", 0, false)}
	definition := filepath.Join(t.TempDir(), "sandbox.json")
	os.WriteFile(definition, []byte(`{"sandboxName":"fromfile","licenseType":"Partial","description":"For testing","features":["A","B"]}`), 0o644)

	// Without --no-prompt the request is shown and an empty answer is "no".
	out, errOut, status := run("org", "create", "sandbox", "-f", definition, "-n", "dev1")
	if status != 1 || !strings.HasPrefix(out, "Config Sandbox Request\n") || !strings.Contains(out, "│ SandboxName │ dev1 ") || !strings.Contains(out, "│ LicenseType │ Partial ") ||
		!strings.Contains(errOut, "Error (UserNotSatisfiedWithSandboxConfigError): The sandbox request configuration isn't acceptable.") || len(org.requests) != 0 {
		t.Errorf("declined: status %d, stderr %q, requests %q\n%s", status, errOut, org.requests, out)
	}
	if _, _, status = run("org", "create", "sandbox", "-f", definition, "--async", "--no-prompt"); status != 68 || len(org.requests) != 1 ||
		org.requests[0] != `POST SandboxInfo {"SandboxName":"fromfile","LicenseType":"Partial","Description":"For testing","Features":"[\"A\",\"B\"]"}` {
		t.Errorf("definition file: status %d, requests %q", status, org.requests)
	}
	_, errOut, _ = run("org", "create", "sandbox", "--async", "--no-prompt")
	if !strings.HasPrefix(errOut, "Warning: No SandboxName defined, generating new SandboxName: sbx") {
		t.Errorf("generated name: stderr %q", errOut)
	}

	for _, c := range []struct {
		args   []string
		status int
		stderr string
	}{
		{[]string{"-n", "elevenchars"}, 2, "The sandbox name \"elevenchars\" should be 10 or fewer characters."},
		{[]string{"--async", "-w", "5"}, 2, "--async cannot also be provided when using --wait"},
		{[]string{"-l", "Full", "--source-sandbox-name", "other"}, 2, "--license-type=Full cannot also be provided when using --source-sandbox-name"},
		{[]string{"-l", "Huge"}, 2, "Expected --license-type=Huge to be one of: Developer, Developer_Pro, Partial, Full"},
		{[]string{"-w", "1", "-i", "90"}, 1, "Error (PollIntervalGreaterThanWaitError): The poll interval (90 seconds) can't be larger that wait (60 in seconds)"},
		{[]string{"-i", "5"}, 2, "Parsing --poll-interval \n\tThe value must be at least 15."},
		{[]string{"--source-id", "001000000000001AAA"}, 2, "The id must begin with 0GQ."},
		{[]string{"-f", "nofile.json"}, 2, "No file found at nofile.json"},
	} {
		_, errOut, status := run(append([]string{"org", "create", "sandbox", "--no-prompt"}, c.args...)...)
		if status != c.status || !strings.Contains(errOut, c.stderr) {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
}

func TestOrgRefreshSandbox(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Pending", 0, false)}
	_, errOut, status := run("org", "refresh", "sandbox", "--no-prompt")
	if status != 1 || errOut != "Error (NoSandboxNameError): Must specify a sandbox name using the `--name` or `--definition-file` flag.\n\n\n" {
		t.Errorf("no name: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "refresh", "sandbox", "-n", "other", "--no-prompt")
	if status != 1 || !strings.Contains(errOut, "Error (SandboxNotFoundError): The sandbox name \"other\" could not be found in production org \"me@example.com\".") || !strings.Contains(errOut, "Ensure the sandbox name and casing is correct.") {
		t.Errorf("unknown sandbox: status %d, stderr %q", status, errOut)
	}

	// The request is shown with "null" for the empty values.
	out, _, status := run("org", "refresh", "sandbox", "-n", "dev1")
	if status != 1 || !strings.HasPrefix(out, "Config Sandbox Refresh\n") || !strings.Contains(out, "│ Description      │ null ") || !strings.Contains(out, "│ AutoActivate     │ true ") {
		t.Errorf("declined: status %d\n%s", status, out)
	}

	out, errOut, status = run("org", "refresh", "sandbox", "-n", "dev1", "--no-auto-activate", "--async", "--no-prompt")
	if status != 68 || errOut != "" {
		t.Fatalf("async refresh: status %d, stderr %q\n%s", status, errOut, out)
	}
	for _, want := range []string{"─ Sandbox Refresh (async) ─", " ◯ Creating new sandbox - Skipped\n ▶ Refreshing org ", " ◼ Authenticating\n", " Status: Pending\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("async refresh lacks %q:\n%s", want, out)
		}
	}
	// The fields that cannot be changed are not sent.
	if len(org.requests) != 1 || org.requests[0] != `PATCH SandboxInfo/0GQ000000000001AAA {"SandboxName":"dev1","LicenseType":"DEVELOPER","HistoryDays":0,"CopyChatter":false,"AutoActivate":false,"Description":null,"SourceId":null}` {
		t.Errorf("requests = %q", org.requests)
	}
	if cache := sandboxCacheText(t); !strings.Contains(cache, `"action": "Refresh"`) {
		t.Errorf("cache:\n%s", cache)
	}

	// Resuming a refresh shows the refresh stages.
	org.processes = []string{sandboxProcessJSON("Completed", 100, true)}
	out, errOut, status = run("org", "resume", "sandbox", "-i", "0GR000000000001AAA", "-w", "5")
	if status != 0 || !strings.Contains(out, " ◯ Creating new sandbox - Skipped\n ✔ Refreshing org ") || !strings.Contains(out, " ✔ Authenticating ") || !strings.Contains(out, "Your sandbox is ready.") {
		t.Errorf("resume of a refresh: status %d, stderr %q\n%s", status, errOut, out)
	}
}

func TestOrgResumeSandboxThatCannotBeResumed(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Stopped", 10, false)}
	_, errOut, status := run("org", "resume", "sandbox", "-n", "dev1")
	if status != 1 || !strings.Contains(errOut, "Error (SandboxNotResumableError): The sandbox dev1 cannot resume with status of Stopped.") {
		t.Errorf("stopped: status %d, stderr %q", status, errOut)
	}
	org.processes = nil
	_, errOut, status = run("org", "resume", "sandbox", "-n", "dev1")
	if status != 1 || !strings.Contains(errOut, "Error (SingleRecordQuery_NoRecords): No record found for SELECT Id,Status,SandboxName") {
		t.Errorf("no process: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status = run("org", "resume", "sandbox", "-n", "dev1", "-i", "0GR000000000001AAA"); status != 2 || !strings.Contains(errOut, "--job-id=0GR000000000001AAA cannot also be provided when using --name") {
		t.Errorf("both: status %d, stderr %q", status, errOut)
	}

	// The end date is set, and the production org does not authorize the
	// sandbox yet.
	org.processes = []string{sandboxProcessJSON("Completed", 100, true)}
	org.handlers["/services/data/v67.0/tooling/sandboxAuth"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `[{"message":"The sandbox is not ready","errorCode":"INVALID_STATUS"}]`)
	}
	out, _, status := run("org", "resume", "sandbox", "-n", "dev1", "--json")
	if status != 68 || !strings.Contains(out, `"Message": "The sandbox org completed, but the authentication endpoint is not ready yet (INVALID_STATUS: The sandbox is not ready). Run \"org resume sandbox\" to retry."`) {
		t.Errorf("not authorized yet: status %d\n%s", status, out)
	}
}

func TestOrgResumeSandboxReportsAFailedAuthorizationRequest(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Completed", 100, true)}
	org.handlers["/services/data/v67.0/tooling/sandboxAuth"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `[{"message":"Sandbox authorization ConnectionException - Sandbox: Failure during validation of the callbackUrl = http://localhost:1717/OauthRedirect","errorCode":"UNKNOWN_EXCEPTION"}]`)
	}
	// Compared with sf: the error is printed after the display, and the
	// exit status is 1.
	out, errOut, status := run("org", "resume", "sandbox", "-n", "dev1")
	if status != 1 || errOut != "Error (1): Sandbox authorization ConnectionException - Sandbox: Failure during validation of the callbackUrl = http://localhost:1717/OauthRedirect\n\n\n" ||
		!strings.Contains(out, " ✘ Creating new sandbox ") || !strings.Contains(out, " Status: Completed\n Copy progress: 100%\n") {
		t.Errorf("status %d, stderr %q\n%s", status, errOut, out)
	}

	// A failure to log in to the sandbox after the production org granted
	// the authorization is reported in the result, with the exit status 68.
	org.handlers["/services/data/v67.0/tooling/sandboxAuth"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"authUserName":"","authCode":"code1","instanceUrl":"`+org.URL+`","loginUrl":"`+org.URL+`"}`)
	}
	out, _, status = run("org", "resume", "sandbox", "-n", "dev1", "--json")
	if status != 68 || !strings.Contains(out, `"Message": "The sandbox org completed successfully, but the local authentication file could not be written: The sandbox dev1 does not have an authorized username.. Run \"org resume sandbox\" again to retry authentication."`) {
		t.Errorf("login failure: status %d\n%s", status, out)
	}
}

func TestSandboxAuthorizationNamesTheCallbackOfTheConnectedApp(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Completed", 100, true)}
	// The production org was authorized with the connected app of force.
	if err := lib.UpdateSFDXAuthFields("me@example.com", lib.SFDXAuthFields{"clientId": lib.ClientId}); err != nil {
		t.Fatal(err)
	}
	if _, errOut, status := run("org", "resume", "sandbox", "-n", "dev1", "-o", "prod"); status != 0 {
		t.Fatalf("resume: status %d, stderr %q", status, errOut)
	}
	want := `POST sandboxAuth {"clientId":"` + lib.ClientId + `","sandboxName":"dev1","callbackUrl":"` + lib.RedirectUri + `"}`
	if len(org.requests) != 1 || org.requests[0] != want {
		t.Errorf("requests = %q, want %q", org.requests, want)
	}
	var exchange map[string][]string
	for _, form := range org.refreshRequests {
		if len(form["grant_type"]) == 1 && form["grant_type"][0] == "authorization_code" {
			exchange = form
		}
	}
	if exchange == nil || exchange["client_id"][0] != lib.ClientId || exchange["redirect_uri"][0] != lib.RedirectUri {
		t.Errorf("code exchange = %v", exchange)
	}
}

func TestSandboxProgressDisplayColors(t *testing.T) {
	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Processing", 27, false)}
	t.Setenv("FORCE_COLOR", "1")
	out, _, status := run("org", "resume", "sandbox", "-n", "dev1")
	for _, want := range []string{
		" \x1b[35m▶\x1b[39m Creating new sandbox \x1b[2m", " \x1b[2m◼\x1b[22m Refreshing org\n", " Status: \x1b[1mProcessing\x1b[22m\n",
		"\x1b[2mRun \"sf org resume sandbox --job-id 0GR000000000001AAA -o me@example.com\" to check for status.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status %d, output lacks %q:\n%q", status, want, out)
		}
	}
}

func TestSuggestedCommandsNameTheBinaryThatWasStarted(t *testing.T) {
	previous := binName
	t.Cleanup(func() { binName = previous })
	for _, c := range []struct {
		argv0    string
		modeFlag bool
		want     string
	}{
		{"sf", false, "sf"},
		{"/tmp/sf", false, "/tmp/sf"},
		{"force", true, "force --sf"},
		{"./force", true, "./force --sf"},
		{"/opt/my tools/sf", false, "sf"},
		{"./my force", true, "force --sf"},
		{"/tmp/go-build3780294797/b001/exe/main", true, "force --sf"},
	} {
		if SetInvocation(c.argv0, c.modeFlag); binName != c.want {
			t.Errorf("SetInvocation(%q, %v): binName = %q, want %q", c.argv0, c.modeFlag, binName, c.want)
		}
	}
	SetInvocation("force", true)

	org := newSandboxOrg(t)
	org.processes = []string{sandboxProcessJSON("Pending", 0, false)}
	out, _, status := run("org", "resume", "sandbox", "-n", "dev1")
	if status != 68 || !strings.Contains(out, "Run \"force --sf org resume sandbox --job-id 0GR000000000001AAA -o me@example.com\" to check for status.") {
		t.Errorf("resume: status %d\n%s", status, out)
	}
	if strings.Contains(out, "\"sf org") {
		t.Errorf("the output names sf:\n%s", out)
	}
	if out, _, _ := run("org", "display", "--json"); !strings.Contains(out, `"accessToken": "[REDACTED] Use 'force --sf org auth show-access-token' to view"`) {
		t.Errorf("org display:\n%s", out)
	}
	if _, errOut, status := run("nosuchcommand"); status != 127 || !strings.Contains(errOut, "Run force --sf help for a list of available commands.") {
		t.Errorf("unknown command: status %d, stderr %q", status, errOut)
	}
}
