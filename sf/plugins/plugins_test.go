package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForceCLI/force/sfplugin"
)

func TestParseReference(t *testing.T) {
	t.Setenv(RegistryPrefixEnv, "")
	tests := []struct {
		input   string
		want    string
		wantErr string
	}{
		{"sfdx-git-delta", "ghcr.io/forcecli/sf-plugins/sfdx-git-delta:latest", ""},
		{"sfdx-git-delta:v7.7.2", "ghcr.io/forcecli/sf-plugins/sfdx-git-delta:v7.7.2", ""},
		{"acme/tool", "ghcr.io/forcecli/sf-plugins/acme/tool:latest", ""},
		{"ghcr.io/acme/tool", "ghcr.io/acme/tool:latest", ""},
		{"ghcr.io/acme/tool:v1", "ghcr.io/acme/tool:v1", ""},
		{"localhost:5000/test/tool:dev", "localhost:5000/test/tool:dev", ""},
		{"localhost/test/tool", "localhost/test/tool:latest", ""},
		{"127.0.0.1:5000/tool", "127.0.0.1:5000/tool:latest", ""},
		{"", "", "is empty"},
		{"https://ghcr.io/acme/tool", "", "URL scheme"},
		{"ghcr.io/acme/tool@sha256:abc", "", "digest"},
		{"ghcr.io/acme/tool:", "", "empty tag"},
		{"ghcr.io/Acme/Tool", "", "not a valid registry reference"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			ref, err := ParseReference(test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ParseReference(%q) = %v, %v; want an error containing %q", test.input, ref, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReference(%q) failed: %v", test.input, err)
			}
			if ref.String() != test.want {
				t.Errorf("ParseReference(%q) = %s, want %s", test.input, ref, test.want)
			}
		})
	}
}

func TestParseReferenceUsesTheConfiguredPrefix(t *testing.T) {
	t.Setenv(RegistryPrefixEnv, "registry.example.com/plugins/")
	ref, err := ParseReference("tool:v2")
	if err != nil {
		t.Fatal(err)
	}
	if ref.String() != "registry.example.com/plugins/tool:v2" {
		t.Errorf("reference = %s", ref)
	}
}

func TestParsePlatform(t *testing.T) {
	platform, err := ParsePlatform("linux/arm64")
	if err != nil || platform.OS != "linux" || platform.Arch != "arm64" {
		t.Errorf("ParsePlatform = %+v, %v", platform, err)
	}
	for _, bad := range []string{"linux", "/amd64", "linux/", "linux/arm/v7"} {
		if _, err := ParsePlatform(bad); err == nil {
			t.Errorf("ParsePlatform(%q) succeeded", bad)
		}
	}
	if got := ExecutableName("tool", Platform{OS: "windows", Arch: "amd64"}); got != "tool.exe" {
		t.Errorf("ExecutableName for windows = %q", got)
	}
	if got := ExecutableName("tool", Platform{OS: "linux", Arch: "amd64"}); got != "tool" {
		t.Errorf("ExecutableName for linux = %q", got)
	}
}

func record(name string, ids ...string) Record {
	manifest := sfplugin.Manifest{Name: name, Version: "1.0.0", Protocol: sfplugin.ProtocolVersion}
	for _, id := range ids {
		manifest.Commands = append(manifest.Commands, sfplugin.Command{ID: id})
	}
	return Record{Name: name, Executable: "/nowhere/" + name, Manifest: manifest}
}

func TestFindUsesTheLongestCommandId(t *testing.T) {
	records := []Record{record("a", "sgd source", "other run"), record("b", "sgd source delta")}
	tests := []struct {
		words     string
		wantName  string
		wantWords int
		wantFound bool
	}{
		{"sgd source delta", "b", 3, true},
		{"sgd source delta extra", "b", 3, true},
		{"sgd source", "a", 2, true},
		{"sgd source other", "a", 2, true},
		{"other run", "a", 2, true},
		{"sgd", "", 0, false},
		{"unknown", "", 0, false},
		{"", "", 0, false},
	}
	for _, test := range tests {
		match, found := Find(records, strings.Fields(test.words))
		if found != test.wantFound || match.Record.Name != test.wantName || match.Words != test.wantWords {
			t.Errorf("Find(%q) = %s, %d words, found %v; want %s, %d, %v", test.words, match.Record.Name, match.Words, found, test.wantName, test.wantWords, test.wantFound)
		}
	}
}

func TestCheckConflicts(t *testing.T) {
	records := []Record{record("a", "sgd source delta")}
	builtin := func(words []string) bool { return strings.Join(words, " ") == "org list" }

	if err := CheckConflicts(records, record("b", "other run").Manifest, builtin); err != nil {
		t.Errorf("no conflict expected, got %v", err)
	}
	if err := CheckConflicts(records, record("a", "sgd source delta").Manifest, builtin); err != nil {
		t.Errorf("a plugin does not conflict with its own record, got %v", err)
	}
	err := CheckConflicts(records, record("b", "sgd source delta").Manifest, builtin)
	if err == nil || !strings.Contains(err.Error(), "installed plugin a also provides") {
		t.Errorf("conflict with another plugin: %v", err)
	}
	err = CheckConflicts(records, record("b", "org list").Manifest, builtin)
	if err == nil || !strings.Contains(err.Error(), "built-in command") {
		t.Errorf("conflict with a built-in command: %v", err)
	}
}

func TestStoreRecordsPlugins(t *testing.T) {
	store := &Store{Dir: filepath.Join(t.TempDir(), "plugins")}
	records, err := store.Load()
	if err != nil || len(records) != 0 {
		t.Fatalf("Load() of an empty store = %v, %v", records, err)
	}
	for _, name := range []string{"zeta", "alpha"} {
		if err := store.Put(record(name, name+" run")); err != nil {
			t.Fatal(err)
		}
	}
	records, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Name != "alpha" || records[1].Name != "zeta" {
		t.Fatalf("Load() = %+v, want alpha then zeta", records)
	}
	if _, found, _ := store.Get("alpha"); !found {
		t.Error("Get(alpha) did not find the record")
	}
	if _, found, _ := store.Get("missing"); found {
		t.Error("Get(missing) found a record")
	}
	removed, found, err := store.Remove("alpha")
	if err != nil || !found || removed.Name != "alpha" {
		t.Fatalf("Remove(alpha) = %+v, %v, %v", removed, found, err)
	}
	if _, found, _ := store.Remove("alpha"); found {
		t.Error("Remove(alpha) found the record a second time")
	}
	entries, _ := os.ReadDir(store.Dir)
	for _, entry := range entries {
		if entry.Name() != "plugins.json" {
			t.Errorf("unexpected file %s left in the store directory", entry.Name())
		}
	}
}

func TestStoreRemovesFilesNoRecordUses(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	place := func(digest string) string {
		dir, err := store.digestDir(digest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "tool")
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	first := record("tool", "tool run")
	first.ManifestDigest = "sha256:aaaa"
	first.Executable = place(first.ManifestDigest)
	if err := store.Put(first); err != nil {
		t.Fatal(err)
	}
	second := record("tool", "tool run")
	second.ManifestDigest = "sha256:bbbb"
	second.Executable = place(second.ManifestDigest)
	if err := store.Put(second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.Executable); !os.IsNotExist(err) {
		t.Errorf("the replaced executable still exists (err %v)", err)
	}
	if _, err := os.Stat(second.Executable); err != nil {
		t.Errorf("the current executable is missing: %v", err)
	}
	if _, _, err := store.Remove("tool"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second.Executable); !os.IsNotExist(err) {
		t.Errorf("the executable of the removed plugin still exists (err %v)", err)
	}
}

func TestStoreKeepsTheExecutableOfALinkedPlugin(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := record("tool", "tool run")
	linked.Linked = true
	linked.Executable = path
	if err := store.Put(linked); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Remove("tool"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the linked executable was removed: %v", err)
	}
}

func TestDigestDirRejectsPathCharacters(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	for _, bad := range []string{"", "sha256", "sha256:../../etc", "sha256:a/b", ":abc"} {
		if dir, err := store.digestDir(bad); err == nil {
			t.Errorf("digestDir(%q) = %s, want an error", bad, dir)
		}
	}
}

func TestDefaultDir(t *testing.T) {
	t.Setenv(DirEnv, "/custom/plugins")
	if dir, _ := DefaultDir(); dir != "/custom/plugins" {
		t.Errorf("DefaultDir() with %s = %s", DirEnv, dir)
	}
	home := t.TempDir()
	t.Setenv(DirEnv, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if dir, _ := DefaultDir(); dir != filepath.Join(home, ".sf", "plugins") {
		t.Errorf("DefaultDir() = %s", dir)
	}
}
