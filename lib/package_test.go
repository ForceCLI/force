package lib

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Test_buildPackageVersionDescriptor_includes_dependencies_when_provided(t *testing.T) {
	dependencies := []string{"04tKA000000D34QYAS", "04tKA000000D34RYAS"}
	descriptor := buildPackageVersionDescriptor("1.0.0.1", "1.0.0.1", "1.0.0.1", "0Ho000000000001", "05i000000000001", dependencies)

	rawDependencies, ok := descriptor["dependencies"]
	if !ok {
		t.Fatal("Expected dependencies to be present in descriptor")
	}

	gotDependencies, ok := rawDependencies.([]map[string]string)
	if !ok {
		t.Fatalf("Expected dependencies to be []map[string]string, got %T", rawDependencies)
	}

	wantDependencies := []map[string]string{
		{"subscriberPackageVersionId": "04tKA000000D34QYAS"},
		{"subscriberPackageVersionId": "04tKA000000D34RYAS"},
	}
	if !reflect.DeepEqual(gotDependencies, wantDependencies) {
		t.Errorf("Unexpected dependencies. got=%v want=%v", gotDependencies, wantDependencies)
	}
}

func Test_buildPackageVersionDescriptor_omits_dependencies_when_empty(t *testing.T) {
	descriptor := buildPackageVersionDescriptor("1.0.0.1", "1.0.0.1", "1.0.0.1", "0Ho000000000001", "05i000000000001", []string{})

	if _, ok := descriptor["dependencies"]; ok {
		t.Error("Did not expect dependencies to be present in descriptor")
	}
}

func Test_addProfileTypeToManifest_appends_profile_type_when_absent(t *testing.T) {
	manifest := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>GoBridge</members>
        <name>ApexClass</name>
    </types>
    <version>67.0</version>
</Package>`)

	updated, err := addProfileTypeToManifest(manifest)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var p Package
	if err := xml.Unmarshal(updated, &p); err != nil {
		t.Fatalf("Failed to parse updated manifest: %s", err)
	}

	if len(p.Types) != 2 {
		t.Fatalf("Expected 2 types, got %d", len(p.Types))
	}
	if p.Types[1].Name != "Profile" {
		t.Errorf("Expected Profile type to be appended, got %q", p.Types[1].Name)
	}
	if len(p.Types[1].Members) != 0 {
		t.Errorf("Expected Profile type to have no members, got %v", p.Types[1].Members)
	}
	if p.Version != "67.0" {
		t.Errorf("Expected version to be preserved, got %q", p.Version)
	}

	// The Profile <types> element must precede <version> to satisfy the
	// Package schema's element ordering.
	if typesIdx, versionIdx := strings.LastIndex(string(updated), "<types>"), strings.Index(string(updated), "<version>"); typesIdx > versionIdx {
		t.Errorf("Expected all <types> elements before <version>, got types at %d and version at %d", typesIdx, versionIdx)
	}
}

func Test_addProfileTypeToManifest_leaves_manifest_unchanged_when_profile_present(t *testing.T) {
	manifest := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Admin</members>
        <name>Profile</name>
    </types>
    <version>67.0</version>
</Package>`)

	updated, err := addProfileTypeToManifest(manifest)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !reflect.DeepEqual(updated, manifest) {
		t.Errorf("Expected manifest to be unchanged.\ngot=%s\nwant=%s", updated, manifest)
	}
}

func Test_addProfileTypeToManifest_returns_error_for_invalid_xml(t *testing.T) {
	if _, err := addProfileTypeToManifest([]byte("not xml")); err == nil {
		t.Error("Expected an error for invalid package.xml")
	}
}

func Test_collectZipDirectories_returns_sorted_directory_entries(t *testing.T) {
	files := ForceMetadataFiles{
		"package.xml":                        nil,
		"classes/GoBridge.cls":               nil,
		"classes/GoBridge.cls-meta.xml":      nil,
		"objects/Thunder_Settings__c.object": nil,
		"lwc/go/go.js":                       nil,
		"lwc/thunder/thunder.js":             nil,
	}

	got := collectZipDirectories(files)
	want := []string{"classes/", "lwc/", "lwc/go/", "lwc/thunder/", "objects/"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unexpected directories.\ngot=%v\nwant=%v", got, want)
	}
}

func Test_collectZipDirectories_returns_empty_when_no_subdirectories(t *testing.T) {
	files := ForceMetadataFiles{
		"package.xml": nil,
	}

	if got := collectZipDirectories(files); len(got) != 0 {
		t.Errorf("Expected no directories, got %v", got)
	}
}

// packageTestServer serves the tooling endpoints the package functions use.
// created receives each record creation request; query answers each tooling
// query.
func packageTestServer(t *testing.T, created func(object string, r *http.Request) string, query func(q string) string) *Force {
	t.Helper()
	previous := packagePollInterval
	packagePollInterval = time.Millisecond
	t.Cleanup(func() { packagePollInterval = previous })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/tooling/query"):
			_, _ = w.Write([]byte(query(r.URL.Query().Get("q"))))
		case strings.Contains(r.URL.Path, "/tooling/sobjects/") && r.Method == http.MethodPost:
			object := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"success":true,"errors":[]}`, created(object, r))))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
	}))
	t.Cleanup(server.Close)
	return &Force{Credentials: &ForceSession{InstanceUrl: server.URL, AccessToken: "token"}}
}

func readZipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading zip: %v", err)
	}
	entries := make(map[string][]byte)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = content
	}
	return entries
}

func Test_CreatePackageVersion_uploads_the_package_and_returns_both_version_ids(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "classes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "classes", "Greeter.cls"), []byte("global class Greeter {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>66.0</apiVersion>
    <status>Active</status>
</ApexClass>
`
	if err := os.WriteFile(filepath.Join(dir, "classes", "Greeter.cls-meta.xml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	var packageZip, descriptor, request []byte
	polls := 0
	f := packageTestServer(t,
		func(object string, r *http.Request) string {
			if object != "Package2VersionCreateRequest" {
				t.Fatalf("unexpected record creation: %s", object)
			}
			reader, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("reading multipart body: %v", err)
			}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(part)
				if err != nil {
					t.Fatal(err)
				}
				switch part.FormName() {
				case "VersionInfo":
					entries := readZipEntries(t, content)
					packageZip = entries["package.zip"]
					descriptor = entries["package2-descriptor.json"]
				case "Package2VersionCreateRequest":
					request = content
				}
			}
			return "08cRp0000007EflIAE"
		},
		func(q string) string {
			switch {
			case strings.Contains(q, "FROM Package2Version WHERE SubscriberPackageVersionId = '04tRp0000002ancAAA'"):
				return `{"totalSize":1,"done":true,"records":[{"Id":"05iRp000001AncAAA"}]}`
			case strings.Contains(q, "FROM Package2VersionCreateRequest"):
				polls++
				if polls == 1 {
					return `{"totalSize":1,"done":true,"records":[{"Id":"08cRp0000007EflIAE","Status":"InProgress"}]}`
				}
				return `{"totalSize":1,"done":true,"records":[{"Id":"08cRp0000007EflIAE","Status":"Success","Package2VersionId":"05iRp000001EoxFIAS"}]}`
			case strings.Contains(q, "SELECT SubscriberPackageVersionId FROM Package2Version WHERE Id = '05iRp000001EoxFIAS'"):
				return `{"totalSize":1,"done":true,"records":[{"SubscriberPackageVersionId":"04tRp0000002iJVIAY"}]}`
			}
			t.Fatalf("unexpected query: %s", q)
			return ""
		})

	var progress strings.Builder
	version, err := f.CreatePackageVersion(PackageVersionRequest{
		Path:           dir,
		PackageId:      "0HoRp000000C6b7KAC",
		VersionNumber:  "0.1.0.0",
		AncestorId:     "04tRp0000002ancAAA",
		SkipValidation: true,
	}, &progress)
	if err != nil {
		t.Fatalf("CreatePackageVersion: %v", err)
	}
	want := PackageVersion{Id: "05iRp000001EoxFIAS", SubscriberPackageVersionId: "04tRp0000002iJVIAY"}
	if version != want {
		t.Errorf("version = %+v, want %+v", version, want)
	}

	var gotDescriptor map[string]interface{}
	if err := json.Unmarshal(descriptor, &gotDescriptor); err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	if gotDescriptor["id"] != "0HoRp000000C6b7KAC" || gotDescriptor["ancestorId"] != "05iRp000001AncAAA" ||
		gotDescriptor["versionName"] != "0.1.0.0" || gotDescriptor["versionDescription"] != "0.1.0.0" {
		t.Errorf("descriptor = %v", gotDescriptor)
	}
	var gotRequest map[string]interface{}
	if err := json.Unmarshal(request, &gotRequest); err != nil {
		t.Fatalf("request: %v", err)
	}
	if gotRequest["Package2Id"] != "0HoRp000000C6b7KAC" || gotRequest["SkipValidation"] != true || gotRequest["CalculateCodeCoverage"] != false {
		t.Errorf("request = %v", gotRequest)
	}
	entries := readZipEntries(t, packageZip)
	if string(entries["classes/Greeter.cls"]) != "global class Greeter {}" {
		t.Errorf("package.zip entries: %v", keysOf(entries))
	}
	if !strings.Contains(string(entries["package.xml"]), "<name>Profile</name>") {
		t.Errorf("package.xml has no Profile type:\n%s", entries["package.xml"])
	}
	if !strings.Contains(progress.String(), "Status: Success") {
		t.Errorf("progress output: %q", progress.String())
	}
}

func keysOf(m map[string][]byte) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Test_CreatePackageVersion_returns_the_request_errors_when_creation_fails(t *testing.T) {
	dir := t.TempDir()
	f := packageTestServer(t,
		func(object string, r *http.Request) string { return "08cRp0000007EflIAE" },
		func(q string) string {
			switch {
			case strings.Contains(q, "FROM Package2VersionCreateRequestError"):
				return `{"totalSize":1,"done":true,"records":[{"Message":"Greeter: Invalid type: Missing"}]}`
			case strings.Contains(q, "FROM Package2VersionCreateRequest"):
				return `{"totalSize":1,"done":true,"records":[{"Id":"08cRp0000007EflIAE","Status":"Error"}]}`
			}
			t.Fatalf("unexpected query: %s", q)
			return ""
		})

	_, err := f.CreatePackageVersion(PackageVersionRequest{Path: dir, PackageId: "0HoRp000000C6b7KAC", VersionNumber: "0.1.0.0"}, nil)
	if err == nil || !strings.Contains(err.Error(), "Greeter: Invalid type: Missing") {
		t.Fatalf("err = %v", err)
	}
}

func Test_InstallPackageVersion_waits_for_the_install_to_succeed(t *testing.T) {
	var attrs map[string]string
	polls := 0
	f := packageTestServer(t,
		func(object string, r *http.Request) string {
			if object != "PackageInstallRequest" {
				t.Fatalf("unexpected record creation: %s", object)
			}
			if err := json.NewDecoder(r.Body).Decode(&attrs); err != nil {
				t.Fatal(err)
			}
			return "0HfRp0000000001AAA"
		},
		func(q string) string {
			polls++
			if polls == 1 {
				return `{"totalSize":1,"done":true,"records":[{"Id":"0HfRp0000000001AAA","Status":"IN_PROGRESS"}]}`
			}
			return `{"totalSize":1,"done":true,"records":[{"Id":"0HfRp0000000001AAA","Status":"SUCCESS"}]}`
		})

	if err := f.InstallPackageVersion("04tRp0000002iJVIAY", "", false, nil); err != nil {
		t.Fatalf("InstallPackageVersion: %v", err)
	}
	if attrs["SubscriberPackageVersionKey"] != "04tRp0000002iJVIAY" || attrs["SecurityType"] != "None" {
		t.Errorf("install request = %v", attrs)
	}
	if polls != 2 {
		t.Errorf("polled %d times, want 2", polls)
	}
}

func Test_InstallPackageVersion_returns_the_install_errors(t *testing.T) {
	f := packageTestServer(t,
		func(object string, r *http.Request) string { return "0HfRp0000000001AAA" },
		func(q string) string {
			return `{"totalSize":1,"done":true,"records":[{"Id":"0HfRp0000000001AAA","Status":"ERROR","Errors":{"errors":[{"message":"Package is not available"}]}}]}`
		})

	err := f.InstallPackageVersion("04tRp0000002iJVIAY", "", false, nil)
	if err == nil || !strings.Contains(err.Error(), "Package is not available") {
		t.Fatalf("err = %v", err)
	}
}

func Test_InstallPackageVersion_rejects_an_id_that_is_not_a_subscriber_package_version(t *testing.T) {
	f := &Force{Credentials: &ForceSession{InstanceUrl: "http://127.0.0.1:0"}}
	if err := f.InstallPackageVersion("05iRp000001EoxFIAS", "", false, nil); err == nil {
		t.Fatal("expected an error for a 05i Id")
	}
}
