package sf

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The output and the generated files of these commands were compared, file
// by file, with what sf 2.138.6 generates.

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			relative, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func TestTemplateGenerateProject(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("USER", "ada")
	expectRun(t, 0, "target dir = "+home+`
  create std/config/project-scratch-def.json
  create std/README.md
  create std/sfdx-project.json
  create std/.husky/pre-commit
  create std/.vscode/extensions.json
  create std/.vscode/launch.json
  create std/.vscode/settings.json
  create std/eslint.config.js
  create std/scripts/soql/account.soql
  create std/scripts/apex/hello.apex
  create std/.forceignore
  create std/.gitignore
  create std/.prettierignore
  create std/.prettierrc
  create std/jest.config.js
  create std/package.json

`, "template", "generate", "project", "-n", "std")
	project, _ := os.ReadFile("std/sfdx-project.json")
	want := `{
  "packageDirectories": [
    {
      "path": "force-app",
      "default": true
    }
  ],
  "name": "std",
  "namespace": "",
  "sfdcLoginUrl": "https://login.salesforce.com",
  "sourceApiVersion": "66.0"
}
`
	if string(project) != want {
		t.Errorf("sfdx-project.json =\n%s\nwant\n%s", project, want)
	}
	scratch, _ := os.ReadFile("std/config/project-scratch-def.json")
	if !strings.Contains(string(scratch), `"orgName": "ada company"`) {
		t.Errorf("project-scratch-def.json = %s", scratch)
	}
	for _, folder := range []string{"applications", "aura", "classes", "lwc", "objects", "triggers"} {
		if info, err := os.Stat(filepath.Join("std", "force-app", "main", "default", folder)); err != nil || !info.IsDir() {
			t.Errorf("the metadata folder %s was not created", folder)
		}
	}
}

func TestTemplateGenerateProjectVariants(t *testing.T) {
	useTempHome(t)
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "outputDir": "`+os.Getenv("HOME")+`",
    "created": [
      "emp/config/project-scratch-def.json",
      "emp/README.md",
      "emp/sfdx-project.json",
      "emp/.forceignore"
    ],
    "rawOutput": "target dir = `+os.Getenv("HOME")+`\n  create emp/config/project-scratch-def.json\n  create emp/README.md\n  create emp/sfdx-project.json\n  create emp/.forceignore\n"
  },
  "warnings": [
    "The \"force project create\" command has been deprecated. Use \"template generate project\" instead."
  ]
}
`, "force:project:create", "-n", "emp", "-t", "empty", "--json")

	if _, errOut, status := run("template", "generate", "project", "-n", "custom", "-s", "myns", "-p", "src", "-x", "--api-version", "60.0", "--lwc-language", "typescript", "-d", "sub"); status != 0 {
		t.Fatalf("status %d %s", status, errOut)
	}
	project, _ := os.ReadFile("sub/custom/sfdx-project.json")
	for _, want := range []string{`"path": "src"`, `"namespace": "myns"`, `"sourceApiVersion": "60.0",`, `"defaultLwcLanguage": "typescript"`} {
		if !strings.Contains(string(project), want) {
			t.Errorf("sfdx-project.json lacks %s:\n%s", want, project)
		}
	}
	manifest, _ := os.ReadFile("sub/custom/manifest/package.xml")
	if !strings.Contains(string(manifest), "<version>60.0</version>") {
		t.Errorf("manifest = %s", manifest)
	}
	if _, err := os.Stat("sub/custom/tsconfig.json"); err != nil {
		t.Errorf("a TypeScript project has no tsconfig.json: %v", err)
	}
	if _, err := os.Stat("sub/custom/src/main/default/lwc"); err != nil {
		t.Errorf("the package directory was not created under src: %v", err)
	}

	if _, _, status := run("template", "generate", "project", "-n", "ana", "-t", "analytics"); status != 0 {
		t.Fatalf("analytics: status %d", status)
	}
	extensions, _ := os.ReadFile("ana/.vscode/extensions.json")
	if !strings.Contains(string(extensions), `"salesforce.analyticsdx-vscode"`) || strings.HasSuffix(string(extensions), "\n") {
		t.Errorf("analytics extensions.json = %q", extensions)
	}
	if _, err := os.Stat("ana/scripts"); !os.IsNotExist(err) {
		t.Error("the analytics template has the scripts of the standard template")
	}

	_, errOut, status := run("template", "generate", "project", "-n", "x", "-t", "nosuch")
	if status != 1 || errOut != "Error (1): Expected --template=nosuch to be one of: standard, empty, analytics, reactinternalapp, reactexternalapp, agent\nSee more help with --help\n\n\n" {
		t.Errorf("unknown template: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("template", "generate", "project", "-n", "x", "-t", "agent")
	if status != 1 || !strings.Contains(errOut, "The project template agent is not supported") {
		t.Errorf("unsupported template: status %d, stderr %q", status, errOut)
	}
	_, errOut, status = run("template", "generate", "project")
	if status != 2 || errOut != "Error (2): The following error occurred:\n  Missing required flag name\nSee more help with --help\n\n\n" {
		t.Errorf("missing name: status %d, stderr %q", status, errOut)
	}
}

func TestKebabWords(t *testing.T) {
	cases := map[string]string{
		"myLwc":         "my-lwc",
		"MyUpper":       "my-upper",
		"myHTTPClient2": "my-http-client2",
		"dashOne":       "dash-one",
		"XMLParser":     "xml-parser",
		"parseXML":      "parse-xml",
		"item2Go":       "item2-go",
		"a_b":           "a-b",
	}
	for name, want := range cases {
		if got := strings.ToLower(strings.Join(kebabWords(name), "-")); got != want {
			t.Errorf("kebabWords(%s) = %s, want %s", name, got, want)
		}
	}
}

func TestTemplateGenerateLightningComponent(t *testing.T) {
	home := useTempHome(t)
	os.MkdirAll(filepath.Join(home, "lwc"), 0o755)
	os.MkdirAll(filepath.Join(home, "aura"), 0o755)
	expectRun(t, 0, "target dir = "+filepath.Join(home, "lwc")+`
  create lwc/myHTTPClient2/myHTTPClient2.js
  create lwc/myHTTPClient2/myHTTPClient2.html
  create lwc/myHTTPClient2/__tests__/myHTTPClient2.test.js
  create lwc/myHTTPClient2/myHTTPClient2.js-meta.xml

`, "template", "generate", "lightning", "component", "-n", "MyHTTPClient2", "--type", "lwc", "-d", "lwc")
	js, _ := os.ReadFile("lwc/myHTTPClient2/myHTTPClient2.js")
	if !strings.Contains(string(js), "export default class MyHTTPClient2 extends LightningElement {}") {
		t.Errorf("component js = %s", js)
	}
	test, _ := os.ReadFile("lwc/myHTTPClient2/__tests__/myHTTPClient2.test.js")
	if !strings.Contains(string(test), "describe('c-my-http-client2'") || !strings.Contains(string(test), "import MyHTTPClient2 from 'c/myHTTPClient2';") {
		t.Errorf("component test = %s", test)
	}

	if _, _, status := run("template", "generate", "lightning", "component", "-n", "tsComp", "--type", "lwc", "-t", "typescript", "-d", "lwc"); status != 0 {
		t.Fatalf("typescript: status %d", status)
	}
	if got := strings.Join(listFiles(t, "lwc/tsComp"), " "); got != ".gitignore __tests__/tsComp.test.ts tsComp.html tsComp.js-meta.xml tsComp.ts" {
		t.Errorf("typescript component files = %s", got)
	}
	if _, _, status := run("template", "generate", "lightning", "component", "-n", "salesBoard", "--type", "lwc", "-t", "analyticsDashboard", "-d", "lwc"); status != 0 {
		t.Fatalf("analyticsDashboard: status %d", status)
	}
	meta, _ := os.ReadFile("lwc/salesBoard/salesBoard.js-meta.xml")
	if !strings.Contains(string(meta), "<masterLabel>Sales Board</masterLabel>") {
		t.Errorf("analytics component meta = %s", meta)
	}

	if _, _, status := run("template", "generate", "lightning", "component", "-n", "myAura", "-d", "aura"); status != 0 {
		t.Fatalf("aura: status %d", status)
	}
	want := "myAura.auradoc myAura.cmp myAura.cmp-meta.xml myAura.css myAura.design myAura.svg myAuraController.js myAuraHelper.js myAuraRenderer.js"
	if got := strings.Join(listFiles(t, "aura/myAura"), " "); got != want {
		t.Errorf("aura files = %s", got)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-n", "x", "-d", home}, "Error (1): Lightning bundles must have a parent folder named 'aura'.\n\n\n"},
		{[]string{"-n", "x", "--type", "lwc", "-d", "aura"}, "Error (1): Lightning bundles must have a parent folder named 'lwc'.\n\n\n"},
		{[]string{"-n", "1bad", "--type", "lwc", "-d", "lwc"}, "Error (1): Name must start with a letter.\n\n\n"},
		{[]string{"-n", "bad_", "-d", "aura"}, "Error (1): Name can't end with an underscore.\n\n\n"},
		{[]string{"-n", "ba__d", "-d", "aura"}, "Error (1): Name can't contain 2 consecutive underscores.\n\n\n"},
		{[]string{"-n", "ba d", "-d", "aura"}, "Error (1): Name must contain only alphanumeric characters.\n\n\n"},
		{[]string{"-n", "x", "--type", "lwc", "-t", "nosuch", "-d", "lwc"}, "Error (1): Expected --template=nosuch to be one of: default, analyticsDashboard, analyticsDashboardWithStep, typescript\nSee more help with --help\n\n\n"},
	} {
		_, errOut, status := run(append([]string{"template", "generate", "lightning", "component"}, c.args...)...)
		if status != 1 || errOut != c.want {
			t.Errorf("%v: status %d, stderr %q, want %q", c.args, status, errOut, c.want)
		}
	}
}

func TestTemplateGenerateAuraBundlesAndTest(t *testing.T) {
	home := useTempHome(t)
	os.MkdirAll(filepath.Join(home, "aura"), 0o755)
	expectRun(t, 0, "target dir = "+filepath.Join(home, "aura")+`
  create aura/myApp/myApp.app-meta.xml
  create aura/myApp/myApp.app
  create aura/myApp/myApp.auradoc
  create aura/myApp/myAppController.js
  create aura/myApp/myApp.css
  create aura/myApp/myAppHelper.js
  create aura/myApp/myAppRenderer.js
  create aura/myApp/myApp.svg

`, "template", "generate", "lightning", "app", "-n", "myApp", "-d", "aura")
	expectRun(t, 0, "target dir = "+filepath.Join(home, "aura")+"\n  create aura/myEvent/myEvent.evt-meta.xml\n  create aura/myEvent/myEvent.evt\n\n",
		"template", "generate", "lightning", "event", "-n", "myEvent", "-d", "aura", "--api-version", "55.0")
	meta, _ := os.ReadFile("aura/myEvent/myEvent.evt-meta.xml")
	if !strings.Contains(string(meta), "<apiVersion>55.0</apiVersion>") || !strings.Contains(string(meta), "<description>A Lightning Event Bundle</description>") {
		t.Errorf("event meta = %s", meta)
	}
	expectRun(t, 0, "target dir = "+filepath.Join(home, "aura")+"\n  create aura/myInterface/myInterface.intf-meta.xml\n  create aura/myInterface/myInterface.intf\n\n",
		"lightning:generate:interface", "-n", "myInterface", "-d", "aura")
	expectRun(t, 0, "target dir = "+home+"\n  create myTest.resource-meta.xml\n  create myTest.resource\n\n", "template", "generate", "lightning", "test", "-n", "myTest")
	test, _ := os.ReadFile("myTest.resource")
	if !strings.HasPrefix(string(test), `describe("myTest", function(){`) {
		t.Errorf("test resource = %s", test)
	}
	_, errOut, status := run("template", "generate", "lightning", "app", "-n", "a1", "-d", home)
	if status != 1 || errOut != "Error (1): Lightning bundles must have a parent folder named 'aura'.\n\n\n" {
		t.Errorf("app outside aura: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateGenerateStaticResource(t *testing.T) {
	home := useTempHome(t)
	expectRun(t, 0, "target dir = "+home+"\n  create myRes/.gitkeep\n  create myRes.resource-meta.xml\n\n", "template", "generate", "static-resource", "-n", "myRes")
	for contentType, file := range map[string]string{
		"application/javascript": "a.js", "text/javascript": "a.js", "text/css": "a.css", "application/json": "a.json",
		"text/plain": "a.txt", "image/png": "a.resource", "application/octet-stream": "a.resource",
	} {
		os.RemoveAll(filepath.Join(home, file))
		if _, errOut, status := run("template", "generate", "static-resource", "-n", "a", "--type", contentType); status != 0 {
			t.Fatalf("%s: status %d %s", contentType, status, errOut)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("%s did not create %s", contentType, file)
		}
		meta, _ := os.ReadFile("a.resource-meta.xml")
		if !strings.Contains(string(meta), "<contentType>"+contentType+"</contentType>") {
			t.Errorf("%s meta = %s", contentType, meta)
		}
	}
	_, errOut, status := run("template", "generate", "static-resource", "-n", "rbad", "--type", "notamime")
	if status != 1 || errOut != "Error (1): The value of this argument must be a valid MIME type.\n\n\n" {
		t.Errorf("bad type: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateGenerateAnalyticsTemplate(t *testing.T) {
	home := useTempHome(t)
	os.MkdirAll(filepath.Join(home, "waveTemplates"), 0o755)
	if _, errOut, status := run("template", "generate", "analytics", "template", "-n", "sales", "-d", "waveTemplates", "--api-version", "58.0"); status != 0 {
		t.Fatalf("status %d %s", status, errOut)
	}
	want := "app-to-template-rules.json dashboards/salesDashboard.json folder.json releaseNotes.html template-info.json template-to-app-rules.json ui.json variables.json"
	if got := strings.Join(listFiles(t, "waveTemplates/sales"), " "); got != want {
		t.Errorf("files = %s", got)
	}
	info, _ := os.ReadFile("waveTemplates/sales/template-info.json")
	// The template does not use the API version.
	if !strings.Contains(string(info), `"name": "sales"`) || !strings.Contains(string(info), `"file": "dashboards/salesDashboard.json"`) {
		t.Errorf("template-info.json = %s", info)
	}
	_, errOut, status := run("template", "generate", "analytics", "template", "-n", "a1", "-d", home)
	if status != 1 || errOut != "Error (1): Analytics templates must have a parent folder named 'waveTemplates'.\n\n\n" {
		t.Errorf("outside waveTemplates: status %d, stderr %q", status, errOut)
	}
}

func TestTemplateGenerateFlexipage(t *testing.T) {
	// The expected files were written by sf 2.138.6 for the same flags.
	wantApp, err := os.ReadFile(filepath.Join("testdata", "flexipage-app.xml"))
	if err != nil {
		t.Fatal(err)
	}
	wantRecord, err := os.ReadFile(filepath.Join("testdata", "flexipage-record.xml"))
	if err != nil {
		t.Fatal(err)
	}
	home := useTempHome(t)
	project := newProject(t, home)
	out, errOut, status := run("template", "generate", "flexipage", "-n", "App1", "-t", "AppPage")
	target := filepath.Join(project, "flexipages")
	if status != 0 || out != "target dir = "+target+"\n  create flexipages/App1.flexipage-meta.xml\n\n" || errOut != "Warning: "+betaWarning+"\n" {
		t.Errorf("app page: status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if got := readFile(t, filepath.Join(target, "App1.flexipage-meta.xml")); got != string(wantApp) {
		t.Errorf("app page file differs from the file of sf:\n%s", got)
	}

	_, errOut, status = run("template", "generate", "flexipage", "-n", "Rec2", "-t", "RecordPage", "-s", "Account", "--primary-field", "Name",
		"--secondary-fields", "Phone,Owner.Name", "--secondary-fields", "Industry", "--detail-fields", "Name,Phone,Website", "-d", "out")
	if status != 0 || errOut != "Warning: "+betaWarning+"\nWarning: "+commaListWarning+"\n" {
		t.Errorf("record page: status %d, stderr %q", status, errOut)
	}
	if got := readFile(t, filepath.Join(project, "out", "flexipages", "Rec2.flexipage-meta.xml")); got != string(wantRecord) {
		t.Errorf("record page file differs from the file of sf:\n%s", got)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-n", "R", "-t", "RecordPage"}, "RecordPage template requires the --sobject flag to specify the Salesforce object API name (e.g., 'Account', 'Opportunity', 'Custom_Object__c')."},
		{[]string{"-n", "A", "-t", "AppPage", "--primary-field", "Name"}, "The --primary-field flag can only be used with --template RecordPage."},
		{[]string{"-n", "A", "-t", "Nope"}, "Expected --template=Nope to be one of: RecordPage, AppPage, HomePage\nSee more help with --help"},
	} {
		_, errOut, status := run(append([]string{"template", "generate", "flexipage"}, c.args...)...)
		if status != 1 || !strings.HasSuffix(errOut, "Error (1): "+c.want+"\n\n\n") {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
}
