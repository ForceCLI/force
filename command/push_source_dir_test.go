package command

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataRootForPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"src/classes/MyClass.cls", "src"},
		{"src/classes", "src"},
		{"src", "src"},
		{"src/objects/Account/fields/Status__c.field-meta.xml", "src"},
		{"testdata/everything/cachePartitions/aerreg.cachePartition-meta.xml", "testdata/everything"},
		{"testdata/life-sciences-cloud/territory2Types/State.territory2Type", "testdata/life-sciences-cloud"},
		{"force-app/main/default/classes/MyClass.cls", "force-app/main/default"},
		{"force-app/core/main/default/objects/Account/fields/Email_ID__c.field-meta.xml", "force-app/core/main/default"},
		{"/home/user/project/src/classes/MyClass.cls", "/home/user/project/src"},
		{"classes/MyClass.cls", "."},
	}
	for _, c := range cases {
		if got := metadataRootForPath(c.path); got != c.want {
			t.Errorf("metadataRootForPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestSourceDirFromPathsRequiresOneRoot(t *testing.T) {
	if got := sourceDirFromPaths([]string{"a/classes/One.cls", "b/classes/Two.cls"}); got != "" {
		t.Errorf("expected no source dir for paths under different roots, got %q", got)
	}
}

func TestSourceDirFromPathsAcceptsTypesUnderOneRoot(t *testing.T) {
	got := sourceDirFromPaths([]string{
		"testdata/life-sciences-cloud/settings/Territory2.settings",
		"testdata/life-sciences-cloud/territory2Types/State.territory2Type",
	})
	if !strings.HasSuffix(got, filepath.Join("testdata", "life-sciences-cloud")) {
		t.Errorf("expected the source dir testdata/life-sciences-cloud, got %q", got)
	}
}
