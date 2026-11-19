package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func directoryFixture(t *testing.T, source string) *orderedMap {
	t.Helper()
	parsed, err := parseOrderedJSON([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return parsed.(*orderedMap)
}

func TestPackageVersionDependenciesResolveToSubscriberVersionIds(t *testing.T) {
	fake := devHubSetup(t)
	var queries []string
	fake.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		queries = append(queries, q)
		switch {
		case strings.Contains(q, "FROM Package2 "):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"0Ho000000000002AAA"}]}`)
		case strings.Contains(q, "MAX(BuildNumber)"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"expr0":7}]}`)
		case strings.Contains(q, "BuildNumber = 7 AND Branch = 'dev'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"SubscriberPackageVersionId":"04t000000000007AAA"}]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		}
	}
	org, err := loadOrg("hub")
	if err != nil {
		t.Fatal(err)
	}
	aliases := directoryFixture(t, `{"base": "0Ho000000000002AAA", "tools@1.0.0-1": "04t000000000005AAA"}`)
	directory := directoryFixture(t, `{"dependencies": [
		{"subscriberPackageVersionId": "04t000000000004AAA"},
		{"package": "tools@1.0.0-1"},
		{"package": "base", "versionNumber": "2.1.0.LATEST"}
	]}`)
	ids, err := packageVersionDependencies(org, directory, aliases, "dev")
	if err != nil || strings.Join(ids, ",") != "04t000000000004AAA,04t000000000005AAA,04t000000000007AAA" {
		t.Errorf("ids %v, err %v, queries %q", ids, err, queries)
	}

	_, err = packageVersionDependencies(org, directoryFixture(t, `{"dependencies": [{"package": "base"}]}`), aliases, "")
	if err == nil || !strings.Contains(err.Error(), `Dependency must specify either a subscriberPackageVersionId or both packageId and versionNumber: {"package":"base"}`) {
		t.Errorf("missing version number: %v", err)
	}
	_, err = packageVersionDependencies(org, directoryFixture(t, `{"dependencies": [{"package": "base", "versionNumber": "9.0.0.1"}]}`), aliases, "")
	if err == nil || !strings.Contains(err.Error(), "No version number was found in Dev Hub for package id 0Ho000000000002AAA and branch null and version number 9.0.0.1 that resolved to build number 1.") {
		t.Errorf("unknown version: %v", err)
	}
}

func TestPackageVersionAncestorMustBeTheHighestReleasedVersion(t *testing.T) {
	fake := devHubSetup(t)
	highest := `{"Id":"05i000000000002AAA","SubscriberPackageVersionId":"04t000000000002AAA","MajorVersion":1,"MinorVersion":2,"PatchVersion":0}`
	fake.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		switch {
		case strings.Contains(q, "IsReleased = True") && highest != "":
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[`+highest+`]}`)
		case strings.Contains(q, "MajorVersion = 1 AND MinorVersion = 2 AND PatchVersion = 0"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"05i000000000002AAA","IsReleased":true}]}`)
		case strings.Contains(q, "MajorVersion = 1 AND MinorVersion = 1 AND PatchVersion = 0"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"05i000000000001AAA","IsReleased":true}]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		}
	}
	org, err := loadOrg("hub")
	if err != nil {
		t.Fatal(err)
	}
	aliases := newOrderedMap()
	ancestor := func(directory string, skip bool) (string, error) {
		return packageVersionAncestor(org, directoryFixture(t, directory), aliases, "0Ho000000000001AAA", "1.3.0.NEXT", skip)
	}
	for _, directory := range []string{`{"ancestorVersion": "HIGHEST"}`, `{"ancestorVersion": "1.2.0"}`} {
		if id, err := ancestor(directory, false); err != nil || id != "05i000000000002AAA" {
			t.Errorf("%s: id %q, err %v", directory, id, err)
		}
	}
	if _, err := ancestor(`{"ancestorVersion": "1.1.0"}`, false); err == nil || !strings.Contains(err.Error(), "The ancestor version [1.1.0] you specified isn’t the highest released package version. Set the ancestor version to 1.2.0") {
		t.Errorf("older ancestor: %v", err)
	}
	if id, err := ancestor(`{"ancestorVersion": "1.1.0"}`, true); err != nil || id != "05i000000000001AAA" {
		t.Errorf("older ancestor without the check: id %q, err %v", id, err)
	}
	if _, err := ancestor(`{"ancestorVersion": "NONE"}`, false); err == nil || !strings.Contains(err.Error(), "you didn’t specify a package ancestor. Set the ancestor version to 1.2.0") {
		t.Errorf("no ancestor: %v", err)
	}
	if id, err := ancestor(`{"ancestorVersion": "NONE"}`, true); err != nil || id != "" {
		t.Errorf("no ancestor without the check: id %q, err %v", id, err)
	}
	if _, err := ancestor(`{"ancestorId": "HIGHEST", "ancestorVersion": "NONE"}`, false); err == nil || !strings.Contains(err.Error(), "the values disagree") {
		t.Errorf("mismatch: %v", err)
	}
	// The first version of a package has no released version to descend
	// from.
	highest = ""
	if id, err := ancestor(`{}`, false); err != nil || id != "" {
		t.Errorf("first version: id %q, err %v", id, err)
	}
}

func TestPackageVersionDescriptorTakesFeaturesFromTheDefinitionFile(t *testing.T) {
	directory := directoryFixture(t, `{"path": "force-app", "default": true, "package": "billing", "features": ["OldFeature"], "postInstallScript": "Setup",
		"apexTestAccess": {"permissionSets": "Admin, Tester"}, "packageMetadataAccess": {"permissionSets": ["Builder"]}}`)
	descriptor, files, err := packageVersionDescriptor(directory, "", true)
	if err != nil || files != nil {
		t.Fatalf("files %v, err %v", files, err)
	}
	encoded, _ := json.Marshal(descriptor)
	if string(encoded) != `{"features":["OldFeature"],"packageMetadataPermissionSetNames":["Builder"],"permissionSetNames":["Admin","Tester"],"postInstallScript":"Setup"}` {
		t.Errorf("descriptor without a definition file = %s", encoded)
	}

	definition := filepath.Join(t.TempDir(), "def.json")
	os.WriteFile(definition, []byte(`{"orgName": "Acme", "Edition": "Enterprise", "features": ["Communities"], "settings": {"lightningExperienceSettings": {"enableS1DesktopEnabled": true}}}`), 0o644)
	descriptor, files, err = packageVersionDescriptor(directory, definition, false)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(descriptor)
	if string(encoded) != `{"edition":"Enterprise","features":["Communities"],"packageMetadataPermissionSetNames":["Builder"],"postInstallScript":"Setup"}` {
		t.Errorf("descriptor with a definition file = %s", encoded)
	}
	if !strings.Contains(string(files["scratch-org-definition.json"]), `"orgName": "Acme"`) {
		t.Errorf("definition file = %s", files["scratch-org-definition.json"])
	}
	settings, err := unzipFiles(files["settings.zip"])
	if err != nil || !strings.Contains(string(settings["settings/LightningExperience.settings"]), "<enableS1DesktopEnabled>true</enableS1DesktopEnabled>") || settings["package.xml"] == nil {
		t.Errorf("settings.zip = %v, err %v", settings, err)
	}

	os.WriteFile(definition, []byte(`{"orgPreferences": {"enabled": ["S1DesktopEnabled"]}, "settings": {"lightningExperienceSettings": {"enableS1DesktopEnabled": true}}}`), 0o644)
	if _, _, err = packageVersionDescriptor(directory, definition, false); err == nil || !strings.Contains(err.Error(), "You cannot use 'settings' and 'orgPreferences'") {
		t.Errorf("settings with orgPreferences: %v", err)
	}
}
