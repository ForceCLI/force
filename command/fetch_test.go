package command

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchUnpacker_WritesEntriesAndCounts(t *testing.T) {
	root := t.TempDir()
	unpacker := newFetchUnpacker(root)

	entries := map[string]string{
		"classes/Foo.cls": "public class Foo {}",
		"pages/Bar.page":  "<apex:page/>",
		"package.xml":     "<Package/>",
	}
	for _, name := range []string{"classes/Foo.cls", "pages/Bar.page", "package.xml"} {
		if err := unpacker.handle(name, strings.NewReader(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if unpacker.count != 2 {
		t.Errorf("count = %d, want 2 (package.xml does not count)", unpacker.count)
	}
	for name, content := range entries {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != content {
			t.Errorf("%s = %q, want %q", name, data, content)
		}
	}
}

func TestFetchUnpacker_PreservesExistingPackageXml(t *testing.T) {
	root := t.TempDir()
	existing := "<Package>mine</Package>"
	if err := os.WriteFile(filepath.Join(root, "package.xml"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	unpacker := newFetchUnpacker(root)
	if err := unpacker.handle("package.xml", strings.NewReader("<Package>server</Package>")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "package.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Errorf("existing package.xml was overwritten: %q", data)
	}
}

func buildResourceZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchUnpacker_ExpandsZipResources(t *testing.T) {
	oldUnpack := unpack
	unpack = true
	defer func() { unpack = oldUnpack }()

	root := t.TempDir()
	unpacker := newFetchUnpacker(root)

	resource := buildResourceZip(t, map[string]string{"js/app.js": "console.log(1)"})
	meta := `<?xml version="1.0" encoding="UTF-8"?>
<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata">
    <cacheControl>Public</cacheControl>
    <contentType>application/zip</contentType>
</StaticResource>`

	if err := unpacker.handle("staticresources/app.resource", bytes.NewReader(resource)); err != nil {
		t.Fatal(err)
	}
	if err := unpacker.handle("staticresources/app.resource-meta.xml", strings.NewReader(meta)); err != nil {
		t.Fatal(err)
	}
	if len(unpacker.resourceMetas) != 1 {
		t.Fatalf("resourceMetas = %v, want the meta file recorded", unpacker.resourceMetas)
	}
	unpacker.expandResources()

	data, err := os.ReadFile(filepath.Join(root, "staticresources", "app", "js", "app.js"))
	if err != nil {
		t.Fatalf("expected expanded resource: %v", err)
	}
	if string(data) != "console.log(1)" {
		t.Errorf("expanded content = %q", data)
	}
}

func TestFetchUnpacker_DoesNotExpandNonZipResources(t *testing.T) {
	oldUnpack := unpack
	unpack = true
	defer func() { unpack = oldUnpack }()

	root := t.TempDir()
	unpacker := newFetchUnpacker(root)

	meta := `<?xml version="1.0" encoding="UTF-8"?>
<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata">
    <contentType>image/png</contentType>
</StaticResource>`
	if err := unpacker.handle("staticresources/logo.resource", strings.NewReader("png-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := unpacker.handle("staticresources/logo.resource-meta.xml", strings.NewReader(meta)); err != nil {
		t.Fatal(err)
	}
	unpacker.expandResources()

	if _, err := os.Stat(filepath.Join(root, "staticresources", "logo")); !os.IsNotExist(err) {
		t.Error("non-zip resource should not be expanded into a bundle folder")
	}
}

func TestFetchUnpacker_NoResourceCollectionWithoutUnpackFlag(t *testing.T) {
	oldUnpack := unpack
	unpack = false
	defer func() { unpack = oldUnpack }()

	unpacker := newFetchUnpacker(t.TempDir())
	meta := `<StaticResource><contentType>application/zip</contentType></StaticResource>`
	if err := unpacker.handle("staticresources/app.resource-meta.xml", strings.NewReader(meta)); err != nil {
		t.Fatal(err)
	}
	if len(unpacker.resourceMetas) != 0 {
		t.Errorf("resourceMetas = %v, want none without --unpack", unpacker.resourceMetas)
	}
}
