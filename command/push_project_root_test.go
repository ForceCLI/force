package command

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/ForceCLI/force/lib"
)

const sfdxProject = `{"packageDirectories": [{"path": "force-app", "default": true}, {"path": "./packages/billing/"}]}`

// sfdxProjectDir creates an sfdx project in a directory that is itself below a
// directory named for a metadata type, which must not be taken for the project's.
func sfdxProjectDir(t *testing.T) string {
	t.Helper()
	tempDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(tempDir, "email", "project")
	writeTestFile(t, filepath.Join(project, "sfdx-project.json"), sfdxProject)
	return project
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
}

func TestProjectMetadataRoot(t *testing.T) {
	project := sfdxProjectDir(t)
	defaultRoot := filepath.Join(project, "force-app", "main", "default")
	cases := []struct {
		path string
		want string
	}{
		{"force-app/main/default/classes/MyClass.cls", defaultRoot},
		{"force-app/main/default/classes", defaultRoot},
		{"force-app/main/default/classes/triggers/FooHandler.cls", defaultRoot},
		{"force-app/main/default/classes/service/__tests__/FooTest.cls", defaultRoot},
		{"force-app/main/default/lwc/settings/settings.js", defaultRoot},
		{"force-app/main/default/staticresources/app/pages/index.html", defaultRoot},
		{"packages/billing/main/default/classes/flows/Invoice.cls", filepath.Join(project, "packages", "billing", "main", "default")},
		{"packages/billing/classes/Invoice.cls", filepath.Join(project, "packages", "billing")},
	}
	for _, c := range cases {
		got, ok := projectMetadataRoot(filepath.Join(project, filepath.FromSlash(c.path)))
		if !ok || got != c.want {
			t.Errorf("projectMetadataRoot(%q) = %q, %v, want %q", c.path, got, ok, c.want)
		}
	}
}

func TestProjectMetadataRootReportsFalseOutsideMetadata(t *testing.T) {
	project := sfdxProjectDir(t)
	for _, path := range []string{
		"unpackaged/classes/triggers/FooHandler.cls",
		"force-app",
		"force-app/main/default",
	} {
		if got, ok := projectMetadataRoot(filepath.Join(project, filepath.FromSlash(path))); ok {
			t.Errorf("projectMetadataRoot(%q) = %q, want false", path, got)
		}
	}
}

func TestProjectMetadataRootReportsFalseWithoutReadableProject(t *testing.T) {
	missing, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := projectMetadataRoot(filepath.Join(missing, "force-app", "main", "default", "classes", "Foo.cls")); ok {
		t.Errorf("without a project file, got %q", got)
	}

	malformed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(malformed, "sfdx-project.json"), "{")
	if got, ok := projectMetadataRoot(filepath.Join(malformed, "force-app", "classes", "Foo.cls")); ok {
		t.Errorf("with a malformed project file, got %q", got)
	}
}

func TestSourceDirFromPathsIdentifiesTypeInProject(t *testing.T) {
	project := sfdxProjectDir(t)
	defaultRoot := filepath.Join(project, "force-app", "main", "default")
	writeTestFile(t, filepath.Join(defaultRoot, "classes", "triggers", "FooHandler.cls"), "public class FooHandler {}")
	writeTestFile(t, filepath.Join(defaultRoot, "lwc", "settings", "settings.js"), "")

	cases := []struct {
		name       string
		dir        string
		path       string
		wantType   string
		wantMember string
	}{
		{"class from project", project, "force-app/main/default/classes/triggers/FooHandler.cls", "ApexClass", "FooHandler"},
		{"class from source root", defaultRoot, "classes/triggers/FooHandler.cls", "ApexClass", "FooHandler"},
		{"class from its directory", filepath.Join(defaultRoot, "classes", "triggers"), "./FooHandler.cls", "ApexClass", "FooHandler"},
		{"component from project", project, "force-app/main/default/lwc/settings/settings.js", "LightningComponentBundle", "settings"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chdir(t, c.dir)
			root := sourceDirFromPaths([]string{c.path})
			if root != defaultRoot {
				t.Fatalf("sourceDirFromPaths = %q, want %q", root, defaultRoot)
			}
			pb := NewFetchBuilder()
			pb.Root = root
			if err := pb.AddFile(c.path); err != nil {
				t.Fatal(err)
			}
			if len(pb.Metadata) != 1 {
				t.Fatalf("metadata = %v, want only %s", pb.Metadata, c.wantType)
			}
			members := pb.Metadata[c.wantType].Members
			if len(members) != 1 || members[0] != c.wantMember {
				t.Errorf("%s members = %v, want %s", c.wantType, members, c.wantMember)
			}
		})
	}
}

func TestSourceDirFromPathsComparesProjectAndRelativeRoots(t *testing.T) {
	project := sfdxProjectDir(t)
	chdir(t, project)
	got := sourceDirFromPaths([]string{
		"force-app/main/default/classes/triggers/FooHandler.cls",
		"force-app/main/default/pages/Foo.page",
	})
	if want := filepath.Join(project, "force-app", "main", "default"); got != want {
		t.Errorf("sourceDirFromPaths = %q, want %q", got, want)
	}

	got = sourceDirFromPaths([]string{
		"force-app/main/default/classes/Foo.cls",
		"packages/billing/main/default/classes/Bar.cls",
	})
	if got != "" {
		t.Errorf("expected no source dir for two package directories, got %q", got)
	}
}
