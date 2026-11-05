package lib

import (
	"strings"
	"testing"
)

func TestMetadataDirectoryIndex(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"src/classes/Foo.cls", "classes"},
		{"src/classes/Foo.cls-meta.xml", "classes"},
		{"classes/triggers/FooHandler.cls", "classes"},
		{"triggers/classes/Foo.trigger-meta.xml", "triggers"},
		{"force-app/reports/main/default/classes/Foo.cls", "classes"},
		{"force-app/classes/main/default/classes/Foo.cls", "classes"},
		{"myproject/src/classes/triggers/Foo.cls", "classes"},
		{"reports/Sales/Pipeline.report-meta.xml", "reports"},
		{"reports/dashboards/Pipeline.report", "reports"},
		{"reports/Sales.reportFolder-meta.xml", "reports"},
		{"customMetadata/settings/Type.Record.md-meta.xml", "customMetadata"},
		{"layouts/flows/Account-Sales.layout-meta.xml", "layouts"},

		{"lwc/settings/settings.js", "lwc"},
		{"force-app/reports/main/default/lwc/settings/settings.js", "lwc"},
		{"aura/pages/pagesController.js", "aura"},
		{"staticresources/app/pages/index.html", "staticresources"},
		{"experiences/Site/views/home.json", "experiences"},
		{"documents/settings/logo.png", "documents"},

		{"objects/Account/fields/Name__c.field-meta.xml", "objects"},
		{"force-app/reports/main/default/objects/Account/fields/Name__c.field-meta.xml", "objects"},
		{"objectTranslations/Account-es/fields/Name__c.fieldTranslation-meta.xml", "objectTranslations"},
		{"src/classes", "classes"},
		{"reports", "reports"},
	}
	for _, c := range cases {
		parts := strings.Split(c.path, "/")
		i := MetadataDirectoryIndex(parts)
		if i < 0 || parts[i] != c.want {
			t.Errorf("MetadataDirectoryIndex(%q) = %d, want the index of %s", c.path, i, c.want)
		}
	}
}

func TestMetadataDirectoryIndexWithoutMetadataDirectory(t *testing.T) {
	for _, path := range []string{"", "src", "force-app/main/default", "src/Foo.cls"} {
		if i := MetadataDirectoryIndex(strings.Split(path, "/")); i != -1 {
			t.Errorf("MetadataDirectoryIndex(%q) = %d, want -1", path, i)
		}
	}
}

func TestMetadataDirectoryIndexBySuffixRequiresMatchingSuffix(t *testing.T) {
	for _, path := range []string{
		"classes",
		"lwc/settings/settings.js",
		"objects/Account/fields/Name__c.field-meta.xml",
		"classes/README",
		"pages/Foo.cls",
	} {
		if i := MetadataDirectoryIndexBySuffix(strings.Split(path, "/")); i != -1 {
			t.Errorf("MetadataDirectoryIndexBySuffix(%q) = %d, want -1", path, i)
		}
	}
}

func TestEveryMetadataTypeWithNamedFilesHasASuffix(t *testing.T) {
	for _, mp := range metapaths {
		if mp.extension == "" && !mp.onlyFolder && mp.name != "Document" {
			t.Errorf("%s (%s) has no extension, so a path cannot be matched to it by suffix", mp.name, mp.path)
		}
	}
}
