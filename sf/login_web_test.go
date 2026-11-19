package sf

import (
	"strings"
	"testing"
)

func TestOrgLoginWebRejectsABrowserItCannotStart(t *testing.T) {
	useTempHome(t)
	_, errOut, status := run("org", "login", "web", "-b", "lynx")
	if status != 2 || !strings.Contains(errOut, "Expected --browser=lynx to be one of: chrome, edge, firefox") {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("org", "login", "web", "--client-app", "app", "--username", "me@example.com")
	if status != 1 || !strings.Contains(errOut, "The --client-app flag is not supported") {
		t.Errorf("client app: status %d, stderr %q", status, errOut)
	}
}
