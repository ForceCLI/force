package sf

import "testing"

func TestWhichNamesThePluginOfACommand(t *testing.T) {
	t.Setenv("FORCE_COLOR", "0")
	expectRun(t, 0, "org list\n\nplugin: @salesforce/plugin-org\n", "which", "org", "list")
	expectRun(t, 0, "org:list\n\nplugin: @salesforce/plugin-org\n", "which", "org:list")
	expectRun(t, 0, "{\n  \"plugin\": \"@salesforce/plugin-org\"\n}\n", "which", "org", "list", "--json")
	if _, errOut, status := run("which", "nosuch"); status != 1 || errOut != " ›   Error: command nosuch not found\n" {
		t.Errorf("unknown command: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status := run("which"); status != 1 || errOut != " ›   Error: \"which\" expects a command name\n ›   Try this: Provide a command id like this, \"sf which your:command:here\"\n" {
		t.Errorf("no command: status %d, stderr %q", status, errOut)
	}
}
