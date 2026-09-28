package command

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	forceConfig "github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/lib"
)

func TestNamespaceFilterMatchesOrgNamespaceCaseInsensitively(t *testing.T) {
	filter := namespaceFilter("MyNs")
	cases := []struct {
		name    string
		session ForceSession
		want    bool
	}{
		{"matching namespace", ForceSession{UserInfo: &UserInfo{OrgNamespace: "myns"}}, true},
		{"different namespace", ForceSession{UserInfo: &UserInfo{OrgNamespace: "other"}}, false},
		{"no namespace", ForceSession{UserInfo: &UserInfo{}}, false},
		{"no user info", ForceSession{}, false},
	}
	for _, c := range cases {
		if got := filter(c.session); got != c.want {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, got)
		}
	}
}

func TestEmptyNamespaceFilterMatchesLoginsWithoutNamespace(t *testing.T) {
	filter := namespaceFilter("")
	cases := []struct {
		name    string
		session ForceSession
		want    bool
	}{
		{"no namespace", ForceSession{UserInfo: &UserInfo{}}, true},
		{"no user info", ForceSession{}, true},
		{"namespace", ForceSession{UserInfo: &UserInfo{OrgNamespace: "myns"}}, false},
	}
	for _, c := range cases {
		if got := filter(c.session); got != c.want {
			t.Errorf("%s: expected %v, got %v", c.name, c.want, got)
		}
	}
}

func TestNamespaceFlagSetToEmptyStringFiltersToLoginsWithoutNamespace(t *testing.T) {
	flag := loginsCmd.Flags().Lookup("namespace")
	t.Cleanup(func() {
		flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
	if err := loginsCmd.Flags().Set("namespace", ""); err != nil {
		t.Fatalf("failed to set namespace flag: %v", err)
	}

	fs := filters(loginsCmd)
	if len(fs) != 1 {
		t.Fatalf("expected one filter, got %d", len(fs))
	}
	if !fs[0](ForceSession{UserInfo: &UserInfo{}}) {
		t.Error("expected login without namespace to match")
	}
	if fs[0](ForceSession{UserInfo: &UserInfo{OrgNamespace: "myns"}}) {
		t.Error("expected login with namespace to be excluded")
	}
}

func TestNamespaceFlagNotSetAddsNoFilter(t *testing.T) {
	if fs := filters(loginsCmd); len(fs) != 0 {
		t.Fatalf("expected no filters, got %d", len(fs))
	}
}

func saveTestLogin(t *testing.T, name string, namespace string) {
	t.Helper()
	creds := ForceSession{
		InstanceUrl:    "https://" + name + ".my.salesforce.com",
		UserInfo:       &UserInfo{UserName: name, OrgNamespace: namespace},
		SessionOptions: &SessionOptions{},
	}
	body, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("failed to marshal creds: %v", err)
	}
	if err := forceConfig.Config.Save("accounts", name, string(body)); err != nil {
		t.Fatalf("failed to save login: %v", err)
	}
}

func captureLoginsOutput(t *testing.T, filters []accountFilter) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	runLogins(filters, false)
	w.Close()
	os.Stdout = origStdout
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return string(out)
}

func TestLoginsShowsAndFiltersByNamespace(t *testing.T) {
	origConfig := forceConfig.Config
	t.Cleanup(func() { forceConfig.Config = origConfig })
	if err := forceConfig.UseConfigDirectory(t.TempDir()); err != nil {
		t.Fatalf("failed to set config directory: %v", err)
	}
	saveTestLogin(t, "packaging", "myns")
	saveTestLogin(t, "subscriber", "")
	if err := SetActiveLogin("subscriber"); err != nil {
		t.Fatalf("failed to set active login: %v", err)
	}

	out := captureLoginsOutput(t, nil)
	var packagingLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "packaging") {
			packagingLine = line
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(packagingLine), "myns") {
		t.Fatalf("expected packaging login to show namespace myns, got %q", packagingLine)
	}
	if !strings.Contains(out, "subscriber") {
		t.Fatalf("expected unfiltered output to include subscriber login, got %q", out)
	}

	out = captureLoginsOutput(t, []accountFilter{namespaceFilter("myns")})
	if !strings.Contains(out, "packaging") {
		t.Fatalf("expected filtered output to include packaging login, got %q", out)
	}
	if strings.Contains(out, "subscriber") {
		t.Fatalf("expected filtered output to exclude subscriber login, got %q", out)
	}

	out = captureLoginsOutput(t, []accountFilter{namespaceFilter("")})
	if strings.Contains(out, "packaging") {
		t.Fatalf("expected empty-namespace output to exclude packaging login, got %q", out)
	}
	if !strings.Contains(out, "subscriber") {
		t.Fatalf("expected empty-namespace output to include subscriber login, got %q", out)
	}
}
