package sf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/sf/templates"
)

// templateAPIVersion is the API version sf 2.138.6 writes into generated
// metadata files when --api-version is not given.
const templateAPIVersion = "66.0"

func newTemplateCmd() *cobra.Command {
	return newGroup("template", "Commands to create metadata from templates.",
		newGroup("generate", "Create metadata from templates.",
			newGroup("analytics", "Create Analytics templates.", newTemplateAnalyticsCmd()),
			newGroup("apex", "Create Apex classes and triggers.",
				newTemplateApexClassCmd(),
				newTemplateApexTriggerCmd(),
			),
			newGroup("lightning", "Create Lightning bundles.",
				newTemplateLightningComponentCmd(),
				newTemplateAuraBundleCmd("app", "Generate a Lightning App.", "LightningApp"),
				newTemplateAuraBundleCmd("event", "Generate a Lightning Event.", "LightningEvent"),
				newTemplateAuraBundleCmd("interface", "Generate a Lightning Interface.", "LightningInterface"),
				newTemplateLightningTestCmd(),
			),
			newTemplateProjectCmd(),
			newTemplateStaticResourceCmd(),
			newTemplateFlexipageCmd(),
			newGroup("visualforce", "Create Visualforce pages and components.",
				newTemplateVisualforceCmd("page", "Generate a Visualforce Page.", "visualforcepage", "DefaultVFPage.page", "_page.page-meta.xml"),
				newTemplateVisualforceCmd("component", "Generate a Visualforce Component.", "visualforcecomponent", "DefaultVFComponent.component", "_component.component-meta.xml"),
			),
		),
	)
}

var (
	templateVariable    = regexp.MustCompile(`<%[=-]\s*(\w+)\s*%>`)
	templateConditional = regexp.MustCompile(`(?s)<% if \((\w+)\) \{ %>(.*?)<% \} %>`)
)

// renderTemplate reads a file of the embedded templates and replaces each
// <%= name %> with the value of the variable.
func renderTemplate(directory, file string, variables map[string]string) (string, error) {
	content, err := templates.File(directory, file)
	if err != nil {
		return "", err
	}
	// <% if (name) { %>text<% } %> keeps the text when the variable has a
	// value.  The templates use no other control flow, except the flexipage
	// templates, which no command renders yet.
	text := templateConditional.ReplaceAllStringFunc(string(content), func(match string) string {
		parts := templateConditional.FindStringSubmatch(match)
		if variables[parts[1]] == "" {
			return ""
		}
		return parts[2]
	})
	return templateVariable.ReplaceAllStringFunc(text, func(match string) string {
		return variables[templateVariable.FindStringSubmatch(match)[1]]
	}), nil
}

// renderFiles renders template files and returns them under their output
// names.  Each pair is an output name and a template file.
func renderFiles(directory string, variables map[string]string, pairs ...[2]string) ([]generatedFile, error) {
	files := make([]generatedFile, 0, len(pairs))
	for _, pair := range pairs {
		content, err := renderTemplate(directory, pair[1], variables)
		if err != nil {
			return nil, err
		}
		files = append(files, generatedFile{pair[0], content})
	}
	return files, nil
}

type generatedFile struct {
	name    string
	content string
}

type templateResult struct {
	OutputDir string   `json:"outputDir"`
	Created   []string `json:"created"`
	RawOutput string   `json:"rawOutput"`
}

// writeTemplates writes files into --output-dir and reports them the way
// sf does, with paths relative to the working directory.
func writeTemplates(cmd *cobra.Command, files []generatedFile) error {
	outputDir, _ := cmd.Flags().GetString("output-dir")
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	result := templateResult{OutputDir: absolute, Created: []string{}}
	for _, file := range files {
		path := filepath.Join(absolute, filepath.FromSlash(file.name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return err
		}
		relative, err := filepath.Rel(wd, path)
		if err != nil {
			relative = path
		}
		result.Created = append(result.Created, filepath.ToSlash(relative))
	}
	var raw strings.Builder
	fmt.Fprintf(&raw, "target dir = %s\n", absolute)
	for _, created := range result.Created {
		fmt.Fprintf(&raw, "  create %s\n", created)
	}
	result.RawOutput = raw.String()
	return emit(cmd, result, func(w io.Writer) {
		fmt.Fprintln(w, result.RawOutput)
	})
}

func addTemplateFlags(cmd *cobra.Command) {
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the generated metadata.")
	cmd.Flags().StringP("output-dir", "d", ".", "Directory for saving the created files.")
	cmd.Flags().String("api-version", "", "Override the api version used for api requests made by this command")
}

func templateFlags(cmd *cobra.Command) (name string, apiVersion string, err error) {
	name, _ = cmd.Flags().GetString("name")
	if name == "" {
		return "", "", missingFlagError("name")
	}
	if err := checkTemplateName(name); err != nil {
		return "", "", err
	}
	apiVersion, _ = cmd.Flags().GetString("api-version")
	if apiVersion == "" {
		apiVersion = templateAPIVersion
	}
	return name, apiVersion, nil
}

func newTemplateApexClassCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "class",
		Short:       "Generate an Apex class.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ApexClass"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			template, _ := cmd.Flags().GetString("template")
			valid := false
			for _, known := range apexClassTemplates {
				if template == known {
					valid = true
				}
			}
			if !valid {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: %s\nSee more help with --help", template, strings.Join(apexClassTemplates, ", ")), ExitCode: 1}
			}
			files, err := renderFiles("apexclass", map[string]string{"apiName": name, "apiVersion": apiVersion},
				[2]string{name + ".cls", template + ".cls"}, [2]string{name + ".cls-meta.xml", "_class.cls-meta.xml"})
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", "DefaultApexClass", "Template to use for file creation.")
	return cmd
}

var apexClassTemplates = []string{"ApexException", "ApexUnitTest", "BasicUnitTest", "DefaultApexClass", "InboundEmailService"}

var triggerEvents = []string{"before insert", "before update", "before delete", "after insert", "after update", "after delete", "after undelete"}

func newTemplateApexTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "trigger",
		Short:       "Generate an Apex trigger.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ApexTrigger"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			if template, _ := cmd.Flags().GetString("template"); template != "ApexTrigger" {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: ApexTrigger\nSee more help with --help", template), ExitCode: 1}
			}
			sobject, _ := cmd.Flags().GetString("sobject")
			events := splitList(cmd, "event")
			if len(events) == 0 {
				events = []string{"before insert"}
			}
			for _, event := range events {
				valid := false
				for _, known := range triggerEvents {
					if event == known {
						valid = true
					}
				}
				if !valid {
					return &Error{Message: fmt.Sprintf("Expected --event=%s to be one of: %s\nSee more help with --help", event, strings.Join(triggerEvents, ", ")), ExitCode: 1}
				}
			}
			files, err := renderFiles("apextrigger", map[string]string{"triggername": name, "sobject": sobject, "triggerEvents": strings.Join(events, ","), "apiVersion": apiVersion},
				[2]string{name + ".trigger", "ApexTrigger.trigger"}, [2]string{name + ".trigger-meta.xml", "_trigger.trigger-meta.xml"})
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", "ApexTrigger", "Template to use for file creation.")
	cmd.Flags().StringP("sobject", "s", "SOBJECT", "Salesforce object to generate a trigger on.")
	cmd.Flags().StringArrayP("event", "e", nil, "Events that fire the trigger.")
	return cmd
}

func newTemplateVisualforceCmd(kind, short, directory, body, meta string) *cobra.Command {
	// The command has one template, named after its body file.
	templateName := strings.TrimSuffix(body, filepath.Ext(body))
	cmd := &cobra.Command{
		Use:         kind,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Visualforce" + strings.ToUpper(kind[:1]) + kind[1:]},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, apiVersion, err := templateFlags(cmd)
			if err != nil {
				return err
			}
			label, _ := cmd.Flags().GetString("label")
			if label == "" {
				return missingFlagError("label")
			}
			if template, _ := cmd.Flags().GetString("template"); template != templateName {
				return &Error{Message: fmt.Sprintf("Expected --template=%s to be one of: %s\nSee more help with --help", template, templateName), ExitCode: 1}
			}
			files, err := renderFiles(directory, map[string]string{"apiVersion": apiVersion, "vfLabel": label},
				[2]string{name + "." + kind, body}, [2]string{name + "." + kind + "-meta.xml", meta})
			if err != nil {
				return err
			}
			return writeTemplates(cmd, files)
		},
	}
	addTemplateFlags(cmd)
	cmd.Flags().StringP("template", "t", templateName, "Template to use for file creation.")
	cmd.Flags().StringP("label", "l", "", "Visualforce "+strings.ToUpper(kind[:1])+kind[1:]+" label.")
	return cmd
}
