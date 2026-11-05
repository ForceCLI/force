package command

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func captureActiveOutput(t *testing.T, flag string) string {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().BoolP("json", "j", false, "")
	cmd.Flags().BoolP("session", "s", false, "")
	if flag != "" {
		if err := cmd.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("failed to set flag: %v", err)
		}
	}
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	printCurrentAccount(cmd)
	w.Close()
	os.Stdout = origStdout
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return string(out)
}

// useExpiredSFDXSession points the global force at a server that rejects the
// token "expired" and a session with no user id, like one built from an SFDX
// auth file without a userId.
func useExpiredSFDXSession(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `[{"errorCode":"INVALID_SESSION_ID","message":"Session expired"}]`)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/userinfo"):
			fmt.Fprint(w, `{"preferred_username":"scratch@example.com","organization_id":"00D123456789012AAA","user_id":"005123456789012AAA"}`)
		case strings.HasSuffix(r.URL.Path, "/query"):
			fmt.Fprint(w, `{"totalSize":1,"done":true,"records":[{"NamespacePrefix":"myns"}]}`)
		default:
			fmt.Fprint(w, `{"ProfileId":"00e123456789012AAA"}`)
		}
	}))
	t.Cleanup(server.Close)

	origForce := force
	t.Cleanup(func() { force = origForce })
	force = NewForce(&ForceSession{
		AccessToken: "expired",
		InstanceUrl: server.URL,
		Id:          "00D123456789012AAA",
		UserInfo:    &UserInfo{UserName: "scratch@example.com", OrgId: "00D123456789012AAA"},
		SessionOptions: &SessionOptions{
			RefreshFunc: func(f *Force) error {
				f.Credentials.AccessToken = "fresh"
				return nil
			},
		},
	})
}

func TestActiveSessionPrintsARefreshedToken(t *testing.T) {
	useExpiredSFDXSession(t)

	out := captureActiveOutput(t, "session")

	if strings.TrimSpace(out) != "fresh" {
		t.Fatalf("expected the refreshed token, got %q", out)
	}
}

func TestActiveLoadsOrgNamespace(t *testing.T) {
	useExpiredSFDXSession(t)

	out := captureActiveOutput(t, "")

	if !strings.Contains(out, "ns:myns") {
		t.Fatalf("expected the org namespace in the output, got %q", out)
	}
}
