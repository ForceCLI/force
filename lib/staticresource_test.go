package lib

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const staticResourceTestMeta = `<?xml version="1.0" encoding="UTF-8"?>
<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata">
	<cacheControl>Public</cacheControl>
	<contentType>application/zip</contentType>
</StaticResource>
`

func writeStaticResourceFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// expandedLeaflet is the layout of the GitHub issue: a zip resource expanded
// into a directory with a nested directory.
var expandedLeaflet = map[string]string{
	"staticresources/Leaflet.resource-meta.xml":    staticResourceTestMeta,
	"staticresources/Leaflet/leaflet.js":           "js",
	"staticresources/Leaflet/leaflet.css":          "css",
	"staticresources/Leaflet/images/marker.png":    "png",
	"staticresources/Leaflet/.DS_Store":            "finder",
	"staticresources/Leaflet/.hidden/ignored.txt":  "hidden",
	"staticresources/logo.resource":                "logo",
	"staticresources/logo.resource-meta.xml":       "<StaticResource/>",
	"staticresources/Theme.resource-meta.xml":      "<StaticResource/>",
	"staticresources/Theme.css":                    "body {}",
	"classes/Unrelated.cls":                        "public class Unrelated {}",
	"classes/Unrelated.cls-meta.xml":               "<ApexClass/>",
	"staticresources/Leaflet/images/shadow.png":    "shadow",
	"staticresources/Leaflet/images/sub/layer.png": "layer",
}

func sortedFileNames(files ForceMetadataFiles) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, filepath.ToSlash(name))
	}
	sort.Strings(names)
	return names
}

func zipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	entries := make(map[string]string)
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[f.Name] = string(content)
	}
	return entries
}

func staticResourceMembers(pb PackageBuilder) []string {
	members := append([]string(nil), pb.Metadata["StaticResource"].Members...)
	sort.Strings(members)
	return members
}

func TestPushBuilder_StaticResources(t *testing.T) {
	wantLeafletZip := map[string]string{
		"leaflet.js":           "js",
		"leaflet.css":          "css",
		"images/marker.png":    "png",
		"images/shadow.png":    "shadow",
		"images/sub/layer.png": "layer",
	}
	cases := []struct {
		name        string
		add         func(pb *PackageBuilder, root string) error
		wantFiles   []string
		wantMembers []string
	}{
		{
			name: "pushing_the_type_zips_each_expanded_resource_and_renames_single_file_resources",
			add: func(pb *PackageBuilder, root string) error {
				return pb.AddMetadataType("StaticResource")
			},
			wantFiles: []string{
				"staticresources/Leaflet.resource",
				"staticresources/Leaflet.resource-meta.xml",
				"staticresources/Theme.resource",
				"staticresources/Theme.resource-meta.xml",
				"staticresources/logo.resource",
				"staticresources/logo.resource-meta.xml",
			},
			wantMembers: []string{"Leaflet", "Theme", "logo"},
		},
		{
			name: "pushing_by_name_includes_the_meta_xml_of_an_expanded_resource",
			add: func(pb *PackageBuilder, root string) error {
				return pb.AddMetadataItem("StaticResource", "Leaflet")
			},
			wantFiles: []string{
				"staticresources/Leaflet.resource",
				"staticresources/Leaflet.resource-meta.xml",
			},
			wantMembers: []string{"Leaflet"},
		},
		{
			name: "pushing_a_file_inside_an_expanded_resource_pushes_the_whole_resource",
			add: func(pb *PackageBuilder, root string) error {
				return pb.AddFile(filepath.Join(root, "staticresources", "Leaflet", "images", "marker.png"))
			},
			wantFiles: []string{
				"staticresources/Leaflet.resource",
				"staticresources/Leaflet.resource-meta.xml",
			},
			wantMembers: []string{"Leaflet"},
		},
		{
			name: "pushing_a_nested_directory_of_an_expanded_resource_pushes_the_whole_resource",
			add: func(pb *PackageBuilder, root string) error {
				return pb.AddDirectory(filepath.Join(root, "staticresources", "Leaflet", "images"))
			},
			wantFiles: []string{
				"staticresources/Leaflet.resource",
				"staticresources/Leaflet.resource-meta.xml",
			},
			wantMembers: []string{"Leaflet"},
		},
		{
			name: "pushing_the_meta_xml_of_a_single_file_resource_includes_its_content_file",
			add: func(pb *PackageBuilder, root string) error {
				return pb.AddFile(filepath.Join(root, "staticresources", "Theme.resource-meta.xml"))
			},
			wantFiles: []string{
				"staticresources/Theme.resource",
				"staticresources/Theme.resource-meta.xml",
			},
			wantMembers: []string{"Theme"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeStaticResourceFixture(t, root, expandedLeaflet)
			pb := NewPushBuilder()
			pb.Root = root
			if err := tc.add(&pb, root); err != nil {
				t.Fatal(err)
			}
			if got := sortedFileNames(pb.Files); !reflect.DeepEqual(got, tc.wantFiles) {
				t.Errorf("files:\n got %v\nwant %v", got, tc.wantFiles)
			}
			if got := staticResourceMembers(pb); !reflect.DeepEqual(got, tc.wantMembers) {
				t.Errorf("members = %v, want %v", got, tc.wantMembers)
			}
			if data, ok := pb.Files[filepath.Join("staticresources", "Leaflet.resource")]; ok {
				if got := zipEntries(t, data); !reflect.DeepEqual(got, wantLeafletZip) {
					t.Errorf("Leaflet.resource entries:\n got %v\nwant %v", got, wantLeafletZip)
				}
			}
			if data, ok := pb.Files[filepath.Join("staticresources", "Theme.resource")]; ok && string(data) != "body {}" {
				t.Errorf("Theme.resource = %q, want the content of Theme.css", data)
			}
			if data, ok := pb.Files[filepath.Join("staticresources", "logo.resource")]; ok && string(data) != "logo" {
				t.Errorf("logo.resource = %q, want it deployed as it is", data)
			}
			if _, err := os.Stat(filepath.Join(root, "staticresources", "Leaflet.resource")); !os.IsNotExist(err) {
				t.Error("expected no .resource file to be written to the source directory")
			}
		})
	}
}

func TestPushBuilder_expanded_directory_is_used_instead_of_a_stale_resource_file(t *testing.T) {
	root := t.TempDir()
	writeStaticResourceFixture(t, root, map[string]string{
		"staticresources/app.resource":          "stale zip",
		"staticresources/app.resource-meta.xml": staticResourceTestMeta,
		"staticresources/app/index.html":        "<html/>",
	})
	pb := NewPushBuilder()
	pb.Root = root
	if err := pb.AddMetadataType("StaticResource"); err != nil {
		t.Fatal(err)
	}
	got := zipEntries(t, pb.Files[filepath.Join("staticresources", "app.resource")])
	if want := map[string]string{"index.html": "<html/>"}; !reflect.DeepEqual(got, want) {
		t.Errorf("app.resource entries = %v, want %v", got, want)
	}
	if members := staticResourceMembers(pb); !reflect.DeepEqual(members, []string{"app"}) {
		t.Errorf("members = %v, want [app]", members)
	}
}

func TestFetchBuilder_file_inside_an_expanded_resource_fetches_the_resource(t *testing.T) {
	root := t.TempDir()
	writeStaticResourceFixture(t, root, expandedLeaflet)
	pb := NewFetchBuilder()
	pb.Root = root
	if err := pb.AddFile(filepath.Join(root, "staticresources", "Leaflet", "leaflet.js")); err != nil {
		t.Fatal(err)
	}
	if members := staticResourceMembers(pb); !reflect.DeepEqual(members, []string{"Leaflet"}) {
		t.Errorf("members = %v, want [Leaflet]", members)
	}
	if len(pb.Files) != 0 {
		t.Errorf("expected no files in a fetch, got %v", sortedFileNames(pb.Files))
	}
}

func TestHasSourceFormatFiles_static_resources(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{
			name: "expanded_resource_is_source_format",
			files: map[string]string{
				"staticresources/app.resource-meta.xml": staticResourceTestMeta,
				"staticresources/app/index.html":        "<html/>",
			},
			want: true,
		},
		{
			name: "single_file_resource_with_its_content_type_extension_is_source_format",
			files: map[string]string{
				"staticresources/app.resource-meta.xml": "<StaticResource/>",
				"staticresources/app.js":                "js",
			},
			want: true,
		},
		{
			name: "resource_file_is_metadata_format",
			files: map[string]string{
				"staticresources/app.resource":          "zip",
				"staticresources/app.resource-meta.xml": staticResourceTestMeta,
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeStaticResourceFixture(t, root, tc.files)
			got, err := HasSourceFormatFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("HasSourceFormatFiles = %v, want %v", got, tc.want)
			}
		})
	}
}
