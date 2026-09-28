package lib

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSFDXAuthReadsNamespacePrefix(t *testing.T) {
	dir := t.TempDir()
	authFile := `{
	"accessToken": "token",
	"instanceUrl": "https://example.my.salesforce.com",
	"orgId": "00D000000000001AAA",
	"userId": "005000000000001AAA",
	"username": "user@example.com",
	"namespacePrefix": "myns"
}`
	if err := os.WriteFile(filepath.Join(dir, "user@example.com.json"), []byte(authFile), 0600); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}
	t.Setenv("FORCE_SFDX_STATE_DIRS", dir)

	auth, err := GetSFDXAuth("user@example.com")
	if err != nil {
		t.Fatalf("GetSFDXAuth returned error: %v", err)
	}
	if auth.NamespacePrefix != "myns" {
		t.Fatalf("expected namespace prefix myns, got %q", auth.NamespacePrefix)
	}
}

func TestSFDXAuthToForceSessionSetsOrgNamespace(t *testing.T) {
	session := SFDXAuthToForceSession(SFDXAuth{
		AccessToken:     "token",
		Id:              "00D000000000001AAA",
		InstanceUrl:     "https://example.my.salesforce.com",
		Username:        "user@example.com",
		NamespacePrefix: "myns",
	})
	if session.UserInfo == nil {
		t.Fatal("expected UserInfo to be set")
	}
	if session.UserInfo.OrgNamespace != "myns" {
		t.Fatalf("expected org namespace myns, got %q", session.UserInfo.OrgNamespace)
	}
}
