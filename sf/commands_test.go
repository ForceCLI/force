package sf

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCommandsListsImplementedCommands(t *testing.T) {
	out, _, status := run("commands", "--json")
	if status != 0 {
		t.Fatalf("status %d", status)
	}
	var commands []commandInfo
	if err := json.Unmarshal([]byte(out), &commands); err != nil {
		t.Fatal(err)
	}
	ids := map[string]commandInfo{}
	for _, command := range commands {
		ids[command.Id] = command
	}
	for _, id := range []string{"org:list", "org:list:auth", "org:login:web", "data:query", "project:deploy:start", "force:data:bulk:status", "version"} {
		if _, ok := ids[id]; !ok {
			t.Errorf("%s is not listed", id)
		}
	}
	// A topic that only groups commands is not a command.
	for _, id := range []string{"org", "org:login", "data:create", "project"} {
		if _, ok := ids[id]; ok {
			t.Errorf("the topic %s is listed as a command", id)
		}
	}
	if flags := strings.Join(ids["data:query"].Flags, " "); !strings.Contains(flags, "query") || !strings.Contains(flags, "target-org") || strings.Contains(flags, "targetusername-short") {
		t.Errorf("data:query flags = %s", flags)
	}
	human, _, _ := run("commands")
	if !strings.Contains(human, "│ org login web ") {
		t.Errorf("human output lacks org login web:\n%s", human)
	}
}

// The compatibility document lists every implemented command.  This test
// fails when a command is added without a row in the document.
func TestCompatibilityDocumentListsEveryCommand(t *testing.T) {
	data, err := os.ReadFile("../docs/sf-compatibility.md")
	if err != nil {
		t.Fatal(err)
	}
	document := string(data)
	for _, command := range listCommands(NewRootCmd()) {
		row := "| `sf " + strings.ReplaceAll(command.Id, ":", " ") + "` |"
		if !strings.Contains(document, row) {
			t.Errorf("docs/sf-compatibility.md has no row for %s", command.Id)
		}
	}
}

// Every legacy command id that points at an implemented command resolves
// to a command in the tree.
func TestLegacyIdsOfImplementedCommandsResolve(t *testing.T) {
	implemented := map[string]bool{}
	for _, command := range listCommands(NewRootCmd()) {
		implemented[command.Id] = true
	}
	checked := 0
	for legacy, current := range legacyCommandIds {
		if !implemented[current] {
			continue
		}
		checked++
		args, _ := normalizeArgsFor(NewRootCmd(), []string{legacy})
		found, _, err := NewRootCmd().Find(args)
		if err != nil || strings.Join(strings.Fields(found.CommandPath())[1:], ":") != current {
			t.Errorf("legacy id %s resolves to %v, want %s (err %v)", legacy, args, current, err)
		}
	}
	if checked < 40 {
		t.Errorf("only %d legacy ids point at implemented commands", checked)
	}
}

func TestACommandIsNotRewrittenAsALegacyId(t *testing.T) {
	// "project:generate" is a legacy id of "template generate project".
	root := NewRootCmd()
	got, _ := normalizeArgsFor(root, []string{"project", "generate", "manifest", "-p", "force-app"})
	if strings.Join(got, " ") != "project generate manifest -p force-app" {
		t.Errorf("args = %v", got)
	}
	got, _ = normalizeArgsFor(root, []string{"project:generate", "-n", "x"})
	if strings.Join(got, " ") != "template generate project -n x" {
		t.Errorf("legacy id args = %v", got)
	}
}
