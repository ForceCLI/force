package templates

import (
	"os"
	"strings"
	"testing"
)

func TestArchivesHoldTheTemplateFiles(t *testing.T) {
	names := strings.Join(Names(), " ")
	for _, want := range []string{"apexclass", "apextrigger", "lightningcomponent", "project", "visualforcepage"} {
		if !strings.Contains(" "+names+" ", " "+want+" ") {
			t.Errorf("no archive named %s; archives: %s", want, names)
		}
	}
	files, err := Files("apexclass")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["DefaultApexClass.cls"]), "public with sharing class <%= apiName %>") {
		t.Errorf("DefaultApexClass.cls = %q", files["DefaultApexClass.cls"])
	}
	project, err := Files("project")
	if err != nil {
		t.Fatal(err)
	}
	// Files in subdirectories and files whose names start with a dot are
	// kept.
	for _, path := range []string{"standard/README.md", "sfdx-project.json", ".forceignore"} {
		if _, ok := project[path]; !ok {
			t.Errorf("the project templates have no file %s", path)
		}
	}
	if _, err := Files("nosuch"); err == nil {
		t.Error("Files returns no error for an unknown name")
	}
	if _, err := File("apexclass", "nosuch.cls"); err == nil {
		t.Error("File returns no error for an unknown file")
	}
}

// The license of the templates has to accompany the binary.  The release
// zips include third-party-licenses.txt, so it has to contain the license
// and name the source the archives were built from.
func TestThirdPartyLicensesFileCoversTheTemplates(t *testing.T) {
	if !strings.Contains(License, "Redistribution and use in source and binary forms") || !strings.Contains(License, "Salesforce") {
		t.Fatalf("LICENSE.txt is not the license of the templates:\n%s", License)
	}
	notice, err := os.ReadFile("../../third-party-licenses.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(notice), License) {
		t.Error("third-party-licenses.txt does not contain sf/templates/LICENSE.txt; run make update-sf-templates")
	}
	for _, line := range strings.Split(strings.TrimSpace(Source), "\n") {
		_, value, ok := strings.Cut(line, ": ")
		if !ok || !strings.Contains(string(notice), value) {
			t.Errorf("third-party-licenses.txt does not name %q from SOURCE", line)
		}
	}
}

// The release zips are built by Makefile rules that add the license files.
func TestReleaseZipsIncludeTheLicenseFiles(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(makefile)
	if !strings.Contains(text, "LICENSES := LICENSE third-party-licenses.txt") {
		t.Fatal("the Makefile does not define LICENSES as LICENSE and third-party-licenses.txt")
	}
	rules := strings.Count(text, ".zip: ")
	withLicenses := strings.Count(text, "\tzip $@ $< $(LICENSES)\n")
	if rules == 0 || rules != withLicenses {
		t.Errorf("%d zip rules, %d of them add $(LICENSES)", rules, withLicenses)
	}
}

// The license report lists the Go modules compiled into the binary.
func TestThirdPartyLicensesFileCoversGoModules(t *testing.T) {
	notice, err := os.ReadFile("../../third-party-licenses.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"github.com/spf13/cobra", "github.com/spf13/pflag", "golang.org/x/term"} {
		if !strings.Contains(string(notice), "\n"+module+" ") {
			t.Errorf("third-party-licenses.txt has no block for %s; run make third-party-licenses.txt", module)
		}
	}
}
