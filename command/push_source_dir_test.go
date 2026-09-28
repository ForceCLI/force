package command

import "testing"

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
		{"force-app/main/default/classes/MyClass.cls", "force-app/main/default"},
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
