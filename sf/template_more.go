package sf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/sf/templates"
)

var (
	nameWord        = regexp.MustCompile(`^\w+$`)
	nameStartLetter = regexp.MustCompile(`^[A-Za-z]`)
)

// checkTemplateName applies the rules sf has for the names of generated
// components.
func checkTemplateName(name string) error {
	switch {
	case !nameWord.MatchString(name):
		return &Error{Message: "Name must contain only alphanumeric characters.", ExitCode: 1}
	case !nameStartLetter.MatchString(name):
		return &Error{Message: "Name must start with a letter.", ExitCode: 1}
	case strings.HasSuffix(name, "_"):
		return &Error{Message: "Name can't end with an underscore.", ExitCode: 1}
	case strings.Contains(name, "__"):
		return &Error{Message: "Name can't contain 2 consecutive underscores.", ExitCode: 1}
	}
	return nil
}

// requireParentFolder returns an error unless a folder of the given name
// is on the path of --output-dir.
func requireParentFolder(cmd *cobra.Command, folder, message string) error {
	outputDir, _ := cmd.Flags().GetString("output-dir")
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	for _, part := range strings.Split(absolute, string(os.PathSeparator)) {
		if part == folder {
			return nil
		}
	}
	return &Error{Message: message, ExitCode: 1}
}

const missingAuraDir = "Lightning bundles must have a parent folder named 'aura'."

var (
	upperThenLower = regexp.MustCompile(`^[A-Z][a-z]+`)
	wordPattern    = regexp.MustCompile(`^[A-Z]?[a-z]+[0-9]*`)
	digitsPattern  = regexp.MustCompile(`^[0-9]+`)
)

// kebabWords splits a component name into the words sf joins with hyphens
// for the tag name of a Lightning web component: myHTTPClient2 gives my,
// HTTP, Client2.  It follows the expression sf uses,
//
//	[A-Z]{2,}(?=[A-Z][a-z]+[0-9]*|\b)|[A-Z]?[a-z]+[0-9]*|[A-Z]|[0-9]+
//
// which Go's regexp package cannot express because of the lookahead.
func kebabWords(name string) []string {
	var words []string
	isUpper := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	for i := 0; i < len(name); {
		rest := name[i:]
		run := 0
		for run < len(rest) && isUpper(rest[run]) {
			run++
		}
		matched := ""
		// A run of capitals is a word when a capitalized word or the end
		// of the name follows it.
		for length := run; length >= 2 && matched == ""; length-- {
			if after := rest[length:]; after == "" || upperThenLower.MatchString(after) {
				matched = rest[:length]
			}
		}
		if matched == "" {
			matched = wordPattern.FindString(rest)
		}
		if matched == "" && isUpper(rest[0]) {
			matched = rest[:1]
		}
		if matched == "" {
			matched = digitsPattern.FindString(rest)
		}
		if matched == "" {
			i++
			continue
		}
		words = append(words, matched)
		i += len(matched)
	}
	return words
}

// titleFromCamelCase turns myComponentName into "My Component Name".
func titleFromCamelCase(name string) string {
	var b strings.Builder
	for i, char := range name {
		if i > 0 && char >= 'A' && char <= 'Z' {
			b.WriteByte(' ')
		}
		if i == 0 {
			b.WriteString(strings.ToUpper(string(char)))
			continue
		}
		b.WriteRune(char)
	}
	return b.String()
}

// Values of --template for a Lightning web component.  The typescript value
// selects the typeScript template directory.
var lwcTemplates = []string{"default", "analyticsDashboard", "analyticsDashboardWithStep", "typescript"}

func newTemplateLightningComponentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "component",
		Short:       "Generate a bundle for an Aura component or a Lightning web component.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "LightningComponent"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			kind, _ := cmd.Flags().GetString("type")
			template, _ := cmd.Flags().GetString("template")
			if kind != "aura" && kind != "lwc" {
				return &Error{Message: fmt.Sprintf("Expected --type=%s to be one of: aura, lwc\nSee more help with --help", kind), ExitCode: 1}
			}
			validTemplate := false
			for _, known := range lwcTemplates {
				if template == known {
					validTemplate = true
				}
			}
			if !validTemplate {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: %s\nSee more help with --help", template, strings.Join(lwcTemplates, ", ")), ExitCode: 1}
			}
			if template == "typescript" {
				template = "typeScript"
			}
			if kind == "aura" {
				if err := requireParentFolder(cmd, "aura", missingAuraDir); err != nil {
					return err
				}
				if template != "default" {
					return &Error{Message: fmt.Sprintf("Template %s not available for component type %s.", template, kind), ExitCode: 1}
				}
				base := "aura/default/default"
				variables := map[string]string{"componentname": name, "description": "A Lightning Component Bundle", "apiVersion": apiVersion}
				pairs := [][2]string{{name + "/" + name + ".cmp-meta.xml", base + ".cmp-meta.xml"}}
				for _, suffix := range []string{".auradoc", ".cmp", ".css", ".design", ".svg", "Controller.js", "Helper.js", "Renderer.js"} {
					pairs = append(pairs, [2]string{name + "/" + name + suffix, base + suffix})
				}
				files, err := renderFiles("lightningcomponent", variables, pairs...)
				if err != nil {
					return err
				}
				return writeTemplates(cmd, files)
			}
			if err := requireParentFolder(cmd, "lwc", "Lightning bundles must have a parent folder named 'lwc'."); err != nil {
				return err
			}
			pascal := strings.ToUpper(name[:1]) + name[1:]
			camel := strings.ToLower(name[:1]) + name[1:]
			extension := "js"
			if template == "typeScript" {
				extension = "ts"
			}
			base := "lwc/" + template + "/"
			variables := map[string]string{
				"pascalCaseComponentName": pascal,
				"camelCaseComponentName":  camel,
				"kebabCaseComponentName":  strings.ToLower(strings.Join(kebabWords(name), "-")),
				"apiVersion":              apiVersion,
				"masterLabel":             strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(titleFromCamelCase(name)),
			}
			pairs := [][2]string{
				{camel + "/" + camel + "." + extension, base + template + "." + extension},
				{camel + "/" + camel + ".html", base + template + ".html"},
				{camel + "/__tests__/" + camel + ".test." + extension, base + "__tests__/" + template + ".test." + extension},
			}
			if template == "typeScript" {
				pairs = append(pairs, [2]string{camel + "/.gitignore", base + "gitignore"})
			}
			pairs = append(pairs, [2]string{camel + "/" + camel + ".js-meta.xml", base + template + ".js-meta.xml"})
			files, err := renderFiles("lightningcomponent", variables, pairs...)
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", "default", "Template to use for file creation.")
	cmd.Flags().String("type", "aura", "Type of the component bundle.")
	return cmd
}

// newTemplateAuraBundleCmd builds the commands that generate an Aura app,
// event or interface bundle.
func newTemplateAuraBundleCmd(kind, short, className string) *cobra.Command {
	type bundle struct {
		directory, extension, description, template string
		others                                      [][2]string
	}
	bundles := map[string]bundle{
		"app": {"lightningapp", "app", "A Lightning Application Bundle", "DefaultLightningApp", [][2]string{
			{".auradoc", "DefaultLightningAuradoc.auradoc"}, {"Controller.js", "DefaultLightningController.js"}, {".css", "DefaultLightningCss.css"},
			{"Helper.js", "DefaultLightningHelper.js"}, {"Renderer.js", "DefaultLightningRenderer.js"}, {".svg", "DefaultLightningSVG.svg"},
		}},
		"event":     {"lightningevent", "evt", "A Lightning Event Bundle", "DefaultLightningEvt", nil},
		"interface": {"lightninginterface", "intf", "A Lightning Interface Bundle", "DefaultLightningIntf", nil},
	}
	shape := bundles[kind]
	cmd := &cobra.Command{
		Use:         kind,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: className},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			if err := requireParentFolder(cmd, "aura", missingAuraDir); err != nil {
				return err
			}
			if template, _ := cmd.Flags().GetString("template"); template != shape.template {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: %s\nSee more help with --help", template, shape.template), ExitCode: 1}
			}
			variables := map[string]string{"apiVersion": apiVersion, "description": shape.description, "eventname": name}
			pairs := [][2]string{
				{name + "/" + name + "." + shape.extension + "-meta.xml", "_auradefinitionbundle." + shape.extension + "-meta.xml"},
				{name + "/" + name + "." + shape.extension, shape.template + "." + shape.extension},
			}
			for _, other := range shape.others {
				pairs = append(pairs, [2]string{name + "/" + name + other[0], other[1]})
			}
			files, err := renderFiles(shape.directory, variables, pairs...)
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", shape.template, "Template to use for file creation.")
	return cmd
}

func newTemplateLightningTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "test",
		Short:       "Generate a Lightning test.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "LightningTest"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			if template, _ := cmd.Flags().GetString("template"); template != "DefaultLightningTest" {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: DefaultLightningTest\nSee more help with --help", template), ExitCode: 1}
			}
			files, err := renderFiles("lightningtest", map[string]string{"apiName": name, "description": "A Lightning Test"},
				[2]string{name + ".resource-meta.xml", "_staticresource.resource-meta.xml"}, [2]string{name + ".resource", "DefaultLightningTest.resource"})
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", "DefaultLightningTest", "Template to use for file creation.")
	return cmd
}

// staticResourceExtensions maps a content type to the file extension of
// the resource.  Types with a template in this list get a file with that
// extension; application/zip gets a directory; other types get a .resource
// file.
var staticResourceExtensions = map[string]string{
	"application/zip": "zip", "application/javascript": "js", "text/javascript": "js", "text/css": "css",
	"application/json": "json", "text/plain": "txt",
}

var mimeType = regexp.MustCompile(`^[a-z]+/[a-z0-9][a-z0-9.+-]*$`)

func newTemplateStaticResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "static-resource",
		Short:       "Generate a static resource.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "StaticResource"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			contentType, _ := cmd.Flags().GetString("type")
			if !mimeType.MatchString(strings.ToLower(contentType)) {
				return &Error{Message: "The value of this argument must be a valid MIME type.", ExitCode: 1}
			}
			var content [2]string
			switch extension := staticResourceExtensions[strings.ToLower(contentType)]; extension {
			case "zip":
				content = [2]string{name + "/.gitkeep", "_gitkeep"}
			case "":
				content = [2]string{name + ".resource", "empty.resource"}
			default:
				content = [2]string{name + "." + extension, "empty." + extension}
			}
			files, err := renderFiles("staticresource", map[string]string{"contentType": contentType},
				content, [2]string{name + ".resource-meta.xml", "_staticresource.resource-meta.xml"})
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().String("type", "application/zip", "Content type (mime type) of the generated static resource.")
	return cmd
}

func newTemplateAnalyticsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template",
		Short:       "Generate a simple Analytics template.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "AnalyticsTemplate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			if err := requireParentFolder(cmd, "waveTemplates", "Analytics templates must have a parent folder named 'waveTemplates'."); err != nil {
				return err
			}
			base := "waveTemplates/DefaultAnalyticsTemplate/"
			pairs := [][2]string{{name + "/dashboards/" + name + "Dashboard.json", base + "dashboards/basicDashboard.json"}}
			for _, file := range []string{"app-to-template-rules.json", "folder.json", "releaseNotes.html", "template-info.json", "template-to-app-rules.json", "ui.json", "variables.json"} {
				pairs = append(pairs, [2]string{name + "/" + file, base + file})
			}
			files, err := renderFiles("analytics", map[string]string{"templateName": name, "sourceApiVersion": apiVersion}, pairs...)
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	return cmd
}

// Metadata folders sf creates, empty, in a new project.
var projectFolders = map[string][]string{
	"standard":  {"applications", "aura", "classes", "contentassets", "flexipages", "layouts", "lwc", "objects", "permissionsets", "staticresources", "tabs", "triggers"},
	"empty":     {"aura", "lwc"},
	"analytics": {"aura", "classes", "lwc", "waveTemplates"},
}

// Project templates that sf has and this mode does not generate.
var unsupportedProjectTemplates = []string{"reactinternalapp", "reactexternalapp", "agent", "nativemobile"}

func newTemplateProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "project",
		Short:       "Generate a Salesforce DX project.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Project"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				return missingFlagError("name")
			}
			template, _ := cmd.Flags().GetString("template")
			folders, known := projectFolders[template]
			if !known {
				for _, unsupported := range unsupportedProjectTemplates {
					if template == unsupported {
						return newError("UnsupportedFlagError", "The project template %s is not supported by the sf-compatible mode of force.", template)
					}
				}
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: standard, empty, analytics, reactinternalapp, reactexternalapp, agent\nSee more help with --help", template), ExitCode: 1}
			}
			language, _ := cmd.Flags().GetString("lwc-language")
			if language != "" && language != "javascript" && language != "typescript" {
				return &Error{Message: fmt.Sprintf("Expected --lwc-language=%s to be one of: javascript, typescript\nSee more help with --help", language), ExitCode: 1}
			}
			typeScript := language == "typescript"
			apiVersion, _ := cmd.Flags().GetString("api-version")
			if apiVersion == "" {
				apiVersion = templateAPIVersion
			}
			namespace, _ := cmd.Flags().GetString("namespace")
			packageDir, _ := cmd.Flags().GetString("default-package-dir")
			loginURL, _ := cmd.Flags().GetString("login-url")
			manifest, _ := cmd.Flags().GetBool("manifest")
			user := os.Getenv("USER")
			if user == "" {
				user = "Demo"
			}
			variables := map[string]string{
				"company": user + " company", "projectname": name, "defaultpackagedir": packageDir, "namespace": namespace,
				"loginurl": loginURL, "apiversion": apiVersion, "name": name, "lwcLanguage": language,
			}
			// A file with a TypeScript variant is read from the typescript
			// directory for a TypeScript project.
			variant := func(file string) string {
				if typeScript {
					switch file {
					case "package.json", ".forceignore", "gitignore", "project.eslint.config.js", "settings.json":
						return "typescript/" + file
					}
				}
				return file
			}
			pairs := [][2]string{
				{"config/project-scratch-def.json", template + "/ScratchDef.json"},
				{"README.md", template + "/README.md"},
				{"sfdx-project.json", "sfdx-project.json"},
			}
			if manifest {
				pairs = append(pairs, [2]string{"manifest/package.xml", template + "/Manifest.xml"})
			}
			tooling := func() {
				pairs = append(pairs,
					[2]string{".husky/pre-commit", ".husky/pre-commit"},
					[2]string{".vscode/extensions.json", "extensions.json"},
					[2]string{".vscode/launch.json", "launch.json"},
					[2]string{".vscode/settings.json", variant("settings.json")},
					[2]string{"eslint.config.js", variant("project.eslint.config.js")},
				)
			}
			projectFiles := func() {
				for _, file := range []string{".forceignore", "gitignore", ".prettierignore", ".prettierrc", "jest.config.js", "package.json"} {
					out := file
					if file == "gitignore" {
						out = ".gitignore"
					}
					pairs = append(pairs, [2]string{out, variant(file)})
				}
				if typeScript {
					pairs = append(pairs, [2]string{"tsconfig.json", "tsconfig.json"})
				}
			}
			switch template {
			case "standard":
				tooling()
				pairs = append(pairs, [2]string{"scripts/soql/account.soql", "standard/account.soql"}, [2]string{"scripts/apex/hello.apex", "standard/hello.apex"})
				projectFiles()
			case "empty":
				if typeScript {
					tooling()
					projectFiles()
				} else {
					pairs = append(pairs, [2]string{".forceignore", ".forceignore"})
				}
			case "analytics":
				tooling()
				projectFiles()
			}
			for i := range pairs {
				pairs[i][0] = name + "/" + pairs[i][0]
			}
			// A template directory without its own file uses the file of the
			// standard template.
			available, err := templates.Files("project")
			if err != nil {
				return err
			}
			for i := range pairs {
				if _, ok := available[pairs[i][1]]; !ok && strings.HasPrefix(pairs[i][1], template+"/") {
					pairs[i][1] = "standard/" + strings.TrimPrefix(pairs[i][1], template+"/")
				}
			}
			files, err := renderFiles("project", variables, pairs...)
			if err != nil {
				return err
			}
			if template == "analytics" {
				// The analytics template recommends one more editor
				// extension.
				for i := range files {
					if files[i].name == name+"/.vscode/extensions.json" {
						// sf rewrites the file as JSON, which drops the final
						// newline.
						files[i].content = strings.TrimRight(strings.Replace(files[i].content, "\n  ]", ",\n    \"salesforce.analyticsdx-vscode\"\n  ]", 1), "\n")
					}
				}
			}
			outputDir, _ := cmd.Flags().GetString("output-dir")
			for _, folder := range folders {
				if err := os.MkdirAll(filepath.Join(outputDir, name, packageDir, "main", "default", folder), 0o755); err != nil {
					return err
				}
			}
			return writeTemplates(cmd, files)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the generated project.")
	cmd.Flags().StringP("template", "t", "standard", "Template to use for project creation.")
	cmd.Flags().StringP("output-dir", "d", ".", "Directory for saving the created files.")
	cmd.Flags().StringP("namespace", "s", "", "Namespace associated with this project and any connected scratch orgs.")
	cmd.Flags().StringP("default-package-dir", "p", "force-app", "Default package directory name.")
	cmd.Flags().BoolP("manifest", "x", false, "Generate a manifest (package.xml) for change-set based development.")
	cmd.Flags().String("api-version", "", "Will set this version as sourceApiVersion in the sfdx-project.json file")
	cmd.Flags().String("lwc-language", "", "Default language for Lightning Web Components.")
	cmd.Flags().StringP("login-url", "l", "https://login.salesforce.com", "Salesforce instance login URL.")
	cmd.Flags().MarkHidden("login-url")
	return cmd
}

func newTemplateFlexipageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "flexipage",
		Short:       "Generate a FlexiPage, also known as a Lightning page.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "FlexipageGenerate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			warn(cmd, betaWarning)
			if hasUnescapedComma(currentArgs) {
				warn(cmd, commaListWarning)
			}
			name, _ := cmd.Flags().GetString("name")
			kind, _ := cmd.Flags().GetString("template")
			var missing []string
			if name == "" {
				missing = append(missing, "name")
			}
			if kind == "" {
				missing = append(missing, "template")
			}
			if len(missing) > 0 {
				return missingFlagsError(missing...)
			}
			if kind != "RecordPage" && kind != "AppPage" && kind != "HomePage" {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: RecordPage, AppPage, HomePage\nSee more help with --help", kind), ExitCode: 1}
			}
			sobject, _ := cmd.Flags().GetString("sobject")
			primary, _ := cmd.Flags().GetString("primary-field")
			secondary := splitList(cmd, "secondary-fields")
			detail := splitList(cmd, "detail-fields")
			if kind == "RecordPage" && sobject == "" {
				return &Error{Message: "RecordPage template requires the --sobject flag to specify the Salesforce object API name (e.g., 'Account', 'Opportunity', 'Custom_Object__c').", ExitCode: 1}
			}
			if kind != "RecordPage" {
				for _, flag := range []string{"primary-field", "secondary-fields", "detail-fields"} {
					if cmd.Flags().Changed(flag) {
						return &Error{Message: fmt.Sprintf("The --%s flag can only be used with --template RecordPage.", flag), ExitCode: 1}
					}
				}
			}
			if len(secondary) > 11 {
				return &Error{Message: fmt.Sprintf("Too many secondary fields specified (%d). The Dynamic Highlights Panel supports a maximum of 11 secondary fields.", len(secondary)), ExitCode: 1}
			}
			if err := checkTemplateName(name); err != nil {
				return err
			}
			label, _ := cmd.Flags().GetString("label")
			if label == "" {
				label = name
			}
			description, _ := cmd.Flags().GetString("description")
			content, err := templates.File("flexipage", kind+"/_flexipage.flexipage-meta.xml")
			if err != nil {
				return err
			}
			scope := &ejsScope{
				values: map[string]string{"flexipagename": name, "masterlabel": label, "description": description, "entityName": sobject, "primaryField": primary},
				lists:  map[string][]string{"secondaryFields": secondary, "detailFields": detail},
			}
			rendered, err := renderEJS(string(content), scope)
			if err != nil {
				return err
			}
			// A FlexiPage belongs in a directory named flexipages.
			outputDir, _ := cmd.Flags().GetString("output-dir")
			if !strings.Contains(outputDir, "flexipages") {
				cmd.Flags().Set("output-dir", filepath.Join(outputDir, "flexipages"))
			}
			return writeTemplates(cmd, []generatedFile{{name + ".flexipage-meta.xml", rendered}})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the FlexiPage.")
	cmd.Flags().StringP("template", "t", "", "Template type for the FlexiPage.")
	cmd.Flags().StringP("output-dir", "d", ".", "Directory for saving the created files.")
	cmd.Flags().String("api-version", "", "Override the api version used for api requests made by this command")
	cmd.Flags().String("label", "", "Label of this FlexiPage; if not specified, uses the FlexiPage name as the label.")
	cmd.Flags().String("description", "", "Description for the FlexiPage, which provides context about its purpose.")
	cmd.Flags().StringP("sobject", "s", "", "API name of the Salesforce object; required when creating a RecordPage.")
	cmd.Flags().String("primary-field", "", "Primary field for the dynamic highlights header; typically 'Name'. Used only with RecordPage.")
	cmd.Flags().StringArray("secondary-fields", nil, "Secondary fields shown in the dynamic highlights header.")
	cmd.Flags().StringArray("detail-fields", nil, "Fields to display in the Details tab.")
	return cmd
}

// betaWarning is the warning sf gives for a command that is in beta.
const betaWarning = "This command is currently in beta. Any aspect of this command can change without advanced notice. Don't use beta commands in your scripts."
