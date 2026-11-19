package sf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The output and the generated files were compared with those of sf
// 2.138.6.

func TestSchemaGenerateTab(t *testing.T) {
	home := useTempHome(t)
	if _, errOut, status := run("schema", "generate", "tab", "-o", "A__c", "-d", "."); status != 1 || !strings.HasPrefix(errOut, "    RequiresProjectError: ") {
		t.Errorf("outside a project: status %d, stderr %q", status, errOut)
	}
	os.WriteFile("sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644)
	tabs := filepath.Join("force-app", "main", "default", "tabs")
	os.MkdirAll(tabs, 0o755)
	os.MkdirAll("other", 0o755)
	absolute := filepath.Join(home, tabs)

	expectRun(t, 0, "Generated a tab file at "+filepath.Join(absolute, "MyObject__c.tab-meta.xml")+".\n", "schema", "generate", "tab", "-o", "MyObject__c", "-d", tabs)
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomTab>\n    <customObject>true</customObject>\n    <motif>Custom1: Heart</motif>\n</CustomTab>"
	if got := readFile(t, filepath.Join(tabs, "MyObject__c.tab-meta.xml")); got != want {
		t.Errorf("tab file = %q", got)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "tab": {
      "customObject": true,
      "motif": "Custom54: Boat"
    },
    "path": "`+filepath.Join(absolute, "Other__c.tab-meta.xml")+`"
  },
  "warnings": []
}
`, "schema", "generate", "tab", "-o", "Other__c", "-d", tabs, "-i", "54", "--json")
	for icon, motif := range map[string]string{"100": "Custom100: TV Widescreen", "4": "Custom4: Hexagon ", "10": "Custom10: Moon"} {
		if _, errOut, status := run("schema", "generate", "tab", "-o", "Icon__c", "-d", tabs+"/", "-i", icon); status != 0 {
			t.Fatalf("icon %s: status %d %s", icon, status, errOut)
		}
		if got := readFile(t, filepath.Join(tabs, "Icon__c.tab-meta.xml")); !strings.Contains(got, "<motif>"+motif+"</motif>") {
			t.Errorf("icon %s: %s", icon, got)
		}
	}
	if len(tabMotifs) != 100 {
		t.Errorf("%d tab styles, want 100", len(tabMotifs))
	}

	missing := filepath.Join(home, "nosuch", "tabs", "Bad__c.tab-meta.xml")
	for _, c := range []struct {
		args   []string
		status int
		want   string
	}{
		{[]string{"-o", "Zero__c", "-d", tabs, "-i", "0"}, 2, "Error (2): Parsing --icon \n\tExpected an integer greater than or equal to 1 but received: 0\nSee more help with --help\n\n\n"},
		{[]string{"-o", "Big__c", "-d", tabs, "-i", "101"}, 2, "Error (2): Parsing --icon \n\tExpected an integer less than or equal to 100 but received: 101\nSee more help with --help\n\n\n"},
		{[]string{"-o", "Word__c", "-d", tabs, "-i", "abc"}, 2, "Error (2): Parsing --icon \n\tExpected an integer but received: abc\nSee more help with --help\n\n\n"},
		{[]string{"-o", "Bad__c", "-d", "other"}, 2, "Error (SfError): Parsing --directory \n\tInvalid tabs path: other. Choose a folder ending with /tabs in your project.\nSee more help with --help\n\n\n"},
		{[]string{"-o", "Bad__c", "-d", "nosuch/tabs"}, 1, "Error (ENOENT): ENOENT: no such file or directory, open '" + missing + "'\n\n\n"},
		{[]string{"-d", tabs}, 2, "Error (2): The following error occurred:\n  Missing required flag object\nSee more help with --help\n\n\n"},
		{[]string{"-o", "Bad__c"}, 2, "Error (2): The following error occurred:\n  Missing required flag directory\nSee more help with --help\n\n\n"},
	} {
		_, errOut, status := run(append([]string{"schema", "generate", "tab"}, c.args...)...)
		if status != c.status || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q, want %q", c.args, status, errOut, c.want)
		}
	}
	_, errOut, status := run("generate:metadata:tab", "-o", "Legacy__c", "-d", tabs)
	if status != 0 || errOut != "Warning: The \"generate metadata tab\" command has been deprecated. Use \"schema generate tab\" instead.\n" {
		t.Errorf("legacy id: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateFlagAcceptsOnlyTheTemplateOfTheCommand(t *testing.T) {
	useTempHome(t)
	for _, c := range []struct {
		args     []string
		template string
	}{
		{[]string{"apex", "trigger", "-n", "T1"}, "ApexTrigger"},
		{[]string{"visualforce", "page", "-n", "P1", "-l", "L"}, "DefaultVFPage"},
		{[]string{"visualforce", "component", "-n", "C1", "-l", "L"}, "DefaultVFComponent"},
	} {
		args := append([]string{"template", "generate"}, c.args...)
		if _, errOut, status := run(append(args, "-t", c.template)...); status != 0 {
			t.Errorf("%v with its template: status %d %s", c.args, status, errOut)
		}
		_, errOut, status := run(append(args, "-t", "Other")...)
		if want := "Error (1): Expected --template=Other to be one of: " + c.template + "\nSee more help with --help\n\n\n"; status != 1 || errOut != want {
			t.Errorf("%v with another template: status %d, stderr %q", c.args, status, errOut)
		}
	}
}
