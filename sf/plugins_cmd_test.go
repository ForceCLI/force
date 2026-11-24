package sf

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"

	"github.com/ForceCLI/force/sf/plugins"
)

// writeFakePlugin writes a shell script that follows the plugin protocol.
// Asked for its manifest, it prints one with the given name, version and
// commands, each written as "id" or "id=org requirement".  Run as a
// command, it prints its arguments and the variables it received, and
// exits with the status in FAKE_PLUGIN_EXIT.
func writeFakePlugin(t *testing.T, name, version string, commands ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake plugin is a shell script")
	}
	var entries []string
	for _, command := range commands {
		id, org, _ := strings.Cut(command, "=")
		if org == "" {
			org = "none"
		}
		entries = append(entries, `{"id":"`+id+`","summary":"Summary of `+id+`.","org":"`+org+`"}`)
	}
	manifest := `{"name":"` + name + `","version":"` + version + `","protocol":1,"commands":[` + strings.Join(entries, ",") + `]}`
	script := `#!/bin/sh
if [ "$#" = 1 ] && [ "$1" = "sf-plugin-manifest" ]; then
	cat <<'EOF'
` + manifest + `
EOF
	exit 0
fi
echo "args: $*"
echo "bin: $SF_PLUGIN_BIN_NAME"
echo "protocol: $SF_PLUGIN_PROTOCOL"
echo "plugin: $SF_PLUGIN_NAME $SF_PLUGIN_VERSION"
echo "instance: $SF_PLUGIN_INSTANCE_URL"
echo "token: $SF_PLUGIN_ACCESS_TOKEN"
echo "api: $SF_PLUGIN_API_VERSION"
echo "username: $SF_PLUGIN_USERNAME"
echo "to stderr" >&2
exit "${FAKE_PLUGIN_EXIT:-0}"
`
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func linkFakePlugin(t *testing.T, name, version string, commands ...string) string {
	t.Helper()
	path := writeFakePlugin(t, name, version, commands...)
	if out, errOut, status := run("plugins", "link", path); status != 0 {
		t.Fatalf("plugins link failed: status %d, stdout %s, stderr %s", status, out, errOut)
	}
	return path
}

func TestPluginCommandReceivesItsArgumentsUnchanged(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	// --to and -o are not flags of any built-in command, and -o is the
	// short form of --target-org in built-in commands.
	out, errOut, status := run("sgd", "source", "delta", "--to", "HEAD", "-o", "out", "--json", "--", "-x")
	if status != 0 {
		t.Fatalf("status = %d, stderr %s", status, errOut)
	}
	want := "args: sgd source delta --to HEAD -o out --json -- -x\n" +
		"bin: sf\n" +
		"protocol: 1\n" +
		"plugin: fake 1.2.3\n" +
		"instance: \n" +
		"token: \n" +
		"api: \n" +
		"username: \n"
	if out != want {
		t.Errorf("stdout =\n%s\nwant\n%s", out, want)
	}
	if errOut != "to stderr\n" {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestPluginCommandInColonForm(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	out, _, status := run("sgd:source:delta", "--from", "main")
	if status != 0 || !strings.HasPrefix(out, "args: sgd source delta --from main\n") {
		t.Errorf("status = %d, stdout = %s", status, out)
	}
}

func TestPluginExitStatusIsReturned(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	t.Setenv("FAKE_PLUGIN_EXIT", "42")
	if _, _, status := run("sgd", "source", "delta"); status != 42 {
		t.Errorf("status = %d, want 42", status)
	}
}

func TestPluginDoesNotInheritOrgVariables(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	t.Setenv("SF_PLUGIN_ACCESS_TOKEN", "inherited-token")
	t.Setenv("SF_PLUGIN_INSTANCE_URL", "https://inherited.example.com")
	out, _, _ := run("sgd", "source", "delta")
	if strings.Contains(out, "inherited") {
		t.Errorf("a plugin command that uses no org received inherited org variables:\n%s", out)
	}
}

func TestBuiltinCommandIsNotReplacedByAPlugin(t *testing.T) {
	useTempHome(t)
	path := writeFakePlugin(t, "fake", "1.2.3", "org list")
	_, errOut, status := run("plugins", "link", path)
	if status != 1 || !strings.Contains(errOut, "which is a built-in command") {
		t.Errorf("linking a plugin with a built-in command id: status %d, stderr %s", status, errOut)
	}
	if out, _, _ := run("plugins"); out != "No plugins installed.\n" {
		t.Errorf("plugins = %q after a refused link", out)
	}
}

func TestTwoPluginsCannotProvideTheSameCommand(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "first", "1.0.0", "shared run")
	path := writeFakePlugin(t, "second", "1.0.0", "shared run")
	_, errOut, status := run("plugins", "link", path)
	if status != 1 || !strings.Contains(errOut, "the installed plugin first also provides") {
		t.Errorf("status %d, stderr %s", status, errOut)
	}
}

func TestUnknownCommandIsUnchangedWithoutAMatchingPlugin(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	_, errOut, status := run("nosuch", "command")
	if status != 127 || !strings.Contains(errOut, "nosuch is not a sf command.") {
		t.Errorf("status = %d, stderr = %s", status, errOut)
	}
}

func TestPluginTopicListsItsCommands(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta", "sgd source other")
	out, _, status := run("sgd")
	want := "COMMANDS\n  sgd source delta  Summary of sgd source delta.\n  sgd source other  Summary of sgd source other.\n"
	if status != 0 || out != want {
		t.Errorf("status = %d, stdout =\n%s\nwant\n%s", status, out, want)
	}
}

func TestHelpOfAPluginCommandRunsThePluginWithHelp(t *testing.T) {
	useTempHome(t)
	linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	out, _, status := run("help", "sgd", "source", "delta")
	if status != 0 || !strings.HasPrefix(out, "args: sgd source delta --help\n") {
		t.Errorf("status = %d, stdout = %s", status, out)
	}
}

func TestPluginCommandsAreListed(t *testing.T) {
	useTempHome(t)
	path := linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")

	expectRun(t, 0, "fake 1.2.3 (link) "+path+"\n", "plugins")
	expectRun(t, 0, "sgd source delta\n\nplugin: fake\n", "which", "sgd", "source", "delta")
	expectRun(t, 0, "sgd:source:delta\n\nplugin: fake\n", "which", "sgd:source:delta")

	out, _, status := run("commands", "--json")
	if status != 0 {
		t.Fatalf("commands --json: status %d", status)
	}
	var commands []commandInfo
	if err := json.Unmarshal([]byte(out), &commands); err != nil {
		t.Fatal(err)
	}
	found := false
	for i, command := range commands {
		if command.Id == "sgd:source:delta" && command.Summary == "Summary of sgd source delta." {
			found = true
		}
		if i > 0 && commands[i-1].Id > command.Id {
			t.Errorf("commands are not sorted: %s before %s", commands[i-1].Id, command.Id)
		}
	}
	if !found {
		t.Error("commands --json does not list sgd:source:delta")
	}

	out, _, _ = run("plugins", "--json")
	var infos []pluginInfo
	if err := json.Unmarshal([]byte(out), &infos); err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Name != "fake" || infos[0].Type != "link" || infos[0].Root != path || len(infos[0].Commands) != 1 || infos[0].Commands[0] != "sgd:source:delta" {
		t.Errorf("plugins --json = %+v", infos)
	}

	out, _, status = run("plugins", "inspect", "fake")
	if status != 0 || !strings.Contains(out, "type:            link") || !strings.Contains(out, "sgd source delta (org: none)") {
		t.Errorf("plugins inspect: status %d, stdout %s", status, out)
	}
}

func TestUninstallRemovesThePluginCommands(t *testing.T) {
	useTempHome(t)
	path := linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	for _, alias := range []string{"uninstall", "remove", "unlink"} {
		if alias != "uninstall" {
			if _, errOut, status := run("plugins", "link", path); status != 0 {
				t.Fatalf("link: %s", errOut)
			}
		}
		expectRun(t, 0, "Uninstalled fake 1.2.3\n", "plugins", alias, "fake")
		if _, _, status := run("sgd", "source", "delta"); status != 127 {
			t.Errorf("after plugins %s: status = %d, want 127", alias, status)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the linked executable was removed: %v", err)
	}
	_, errOut, status := run("plugins", "uninstall", "fake")
	if status != 1 || !strings.Contains(errOut, "fake is not installed.") {
		t.Errorf("uninstalling again: status %d, stderr %s", status, errOut)
	}
}

func TestMissingExecutableIsReportedWithTheCommandToFixIt(t *testing.T) {
	useTempHome(t)
	path := linkFakePlugin(t, "fake", "1.2.3", "sgd source delta")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_, errOut, status := run("sgd", "source", "delta")
	if status != 1 || !strings.Contains(errOut, "PluginExecutableError") || !strings.Contains(errOut, "the file does not exist") || !strings.Contains(errOut, "sf plugins uninstall fake") {
		t.Errorf("status = %d, stderr = %s", status, errOut)
	}
}

func TestPluginCommandThatRequiresAnOrg(t *testing.T) {
	useTempHome(t)
	org := newFakeOrg(t)
	org.login(t, "--alias", "myorg")
	linkFakePlugin(t, "fake", "1.2.3", "needs org=required", "may use=optional", "uses none")
	// The fake org answers the resources of version 67.0 only.
	resources := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"sobjects": "/services/data/v66.0/sobjects"})
	}
	org.handlers["/services/data/v66.0"] = resources
	org.handlers["/services/data/v66.0/"] = resources

	// No org is named and no default is set.
	_, errOut, status := run("needs", "org")
	if status != 1 || !strings.Contains(errOut, "NoDefaultEnvError") {
		t.Errorf("required without an org: status %d, stderr %s", status, errOut)
	}
	out, _, status := run("may", "use")
	if status != 0 || !strings.Contains(out, "instance: \n") || !strings.Contains(out, "token: \n") {
		t.Errorf("optional without an org: status %d, stdout %s", status, out)
	}

	for _, args := range [][]string{
		{"needs", "org", "--target-org", "myorg", "--api-version", "66.0"},
		{"needs", "org", "--target-org=myorg", "--api-version=66.0"},
		{"needs", "org", "-o", "myorg", "--api-version", "66.0"},
		{"may", "use", "-u", "myorg", "--apiversion", "66.0"},
	} {
		out, errOut, status := run(args...)
		if status != 0 {
			t.Fatalf("%v: status %d, stderr %s", args, status, errOut)
		}
		for _, line := range []string{
			"args: " + strings.Join(args, " ") + "\n",
			"instance: " + org.URL + "\n",
			"token: " + fakeToken + "\n",
			"api: 66.0\n",
			"username: " + org.username + "\n",
		} {
			if !strings.Contains(out, line) {
				t.Errorf("%v: stdout lacks %q:\n%s", args, line, out)
			}
		}
	}

	// A command that uses no org gets no org variables, whatever flags
	// it is given.
	out, _, _ = run("uses", "none", "--target-org", "myorg")
	if !strings.Contains(out, "instance: \n") || !strings.Contains(out, "token: \n") {
		t.Errorf("a command with org none received org variables:\n%s", out)
	}

	// An org named on the command line must exist, also for a command
	// that can run without one.
	_, errOut, status = run("may", "use", "--target-org", "nosuch")
	if status != 2 || !strings.Contains(errOut, "NamedOrgNotFoundError") {
		t.Errorf("optional with an unknown org: status %d, stderr %s", status, errOut)
	}

	// The default org is used when none is named.
	if _, errOut, status := run("config", "set", "target-org=myorg", "--global"); status != 0 {
		t.Fatalf("config set: %s", errOut)
	}
	out, _, status = run("needs", "org")
	if status != 0 || !strings.Contains(out, "instance: "+org.URL+"\n") {
		t.Errorf("required with a default org: status %d, stdout %s", status, out)
	}
}

func TestPublishInstallAndUpdateFromARegistry(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	ref := host + "/test/fake:stable"
	platform := plugins.HostPlatform().String()

	first := writeFakePlugin(t, "fake", "1.0.0", "sgd source delta")
	out, errOut, status := run("plugins", "publish", ref, "--plain-http", "--executable", platform+"="+first, "--source", "https://example.com/repo", "--revision", "abc")
	if status != 0 || !strings.HasPrefix(out, "Published fake 1.0.0 to "+ref+"\n") {
		t.Fatalf("publish: status %d, stdout %s, stderr %s", status, out, errOut)
	}

	out, errOut, status = run("plugins", "install", ref, "--plain-http")
	if status != 0 || !strings.HasPrefix(out, "Installed fake 1.0.0 from "+ref+"\n") {
		t.Fatalf("install: status %d, stdout %s, stderr %s", status, out, errOut)
	}
	expectRun(t, 0, "fake 1.0.0 ("+ref+")\n", "plugins")

	out, _, status = run("sgd", "source", "delta", "--from", "main")
	if status != 0 || !strings.Contains(out, "args: sgd source delta --from main\n") || !strings.Contains(out, "plugin: fake 1.0.0\n") {
		t.Errorf("installed plugin command: status %d, stdout %s", status, out)
	}

	store := &plugins.Store{Dir: filepath.Join(home, ".sf", "plugins")}
	installed, found, err := store.Get("fake")
	if err != nil || !found {
		t.Fatalf("the plugin is not recorded: %v", err)
	}
	if !strings.HasPrefix(installed.Executable, store.Dir) || installed.IndexDigest == "" || installed.ManifestDigest == "" {
		t.Errorf("record = %+v", installed)
	}

	expectRun(t, 0, "fake 1.0.0 is up to date\n", "plugins", "update")

	// Publishing to the same tag needs --overwrite; update then installs
	// the new executable and removes the old one.
	second := writeFakePlugin(t, "fake", "1.1.0", "sgd source delta")
	if _, errOut, status := run("plugins", "publish", ref, "--plain-http", "--executable", platform+"="+second); status != 1 || !strings.Contains(errOut, "already exists") {
		t.Errorf("publish to an existing tag: status %d, stderr %s", status, errOut)
	}
	if _, errOut, status := run("plugins", "publish", ref, "--plain-http", "--overwrite", "--executable", platform+"="+second); status != 0 {
		t.Fatalf("publish --overwrite: %s", errOut)
	}
	out, errOut, status = run("plugins", "update", "fake")
	if status != 0 || !strings.HasPrefix(out, "Updated fake from 1.0.0 to 1.1.0") {
		t.Fatalf("update: status %d, stdout %s, stderr %s", status, out, errOut)
	}
	if _, err := os.Stat(installed.Executable); !os.IsNotExist(err) {
		t.Errorf("the executable of the previous version still exists (err %v)", err)
	}
	out, _, _ = run("sgd", "source", "delta")
	if !strings.Contains(out, "plugin: fake 1.1.0\n") {
		t.Errorf("after update the command ran:\n%s", out)
	}

	expectRun(t, 0, "Uninstalled fake 1.1.0\n", "plugins", "uninstall", "fake")
	updated, _, _ := store.Get("fake")
	if updated.Name != "" {
		t.Errorf("the plugin is still recorded: %+v", updated)
	}
	if entries, _ := filepath.Glob(filepath.Join(store.Dir, "store", "sha256", "*", "*")); len(entries) != 0 {
		t.Errorf("executables left in the store: %v", entries)
	}
}

func TestInstallOfAMissingPlugin(t *testing.T) {
	useTempHome(t)
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	_, errOut, status := run("plugins", "install", host+"/test/missing:v1", "--plain-http")
	if status != 1 || !strings.Contains(errOut, "not found in the registry") {
		t.Errorf("status = %d, stderr = %s", status, errOut)
	}
}

func TestPublishArgumentChecks(t *testing.T) {
	useTempHome(t)
	_, errOut, status := run("plugins", "publish", "localhost:1/test/fake:v1")
	if status != 2 || !strings.Contains(errOut, "Missing required flag executable") {
		t.Errorf("without --executable: status %d, stderr %s", status, errOut)
	}
	_, errOut, status = run("plugins", "publish", "localhost:1/test/fake:v1", "--executable", "linux-amd64")
	if status != 1 || !strings.Contains(errOut, "not in the form os/arch=path") {
		t.Errorf("malformed --executable: status %d, stderr %s", status, errOut)
	}
	_, errOut, status = run("plugins", "publish", "localhost:1/test/fake:v1", "--executable", "plan9/mips=/nowhere")
	if status != 1 || !strings.Contains(errOut, "Give --manifest") {
		t.Errorf("no manifest source: status %d, stderr %s", status, errOut)
	}
}
