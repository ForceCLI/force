package sf

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// package convert follows the source of the sf packaging library.

// packageConvertInterval is the time between checks of a conversion
// request.  Tests shorten it.
var packageConvertInterval = 30 * time.Second

// addPackageAlias sets an alias in the packageAliases of sfdx-project.json
// and keeps the other content of the file.
func addPackageAlias(root, alias, id string) error {
	file := filepath.Join(root, "sfdx-project.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return fmt.Errorf("sfdx-project.json: %w", err)
	}
	project, ok := parsed.(*orderedMap)
	if !ok {
		return fmt.Errorf("sfdx-project.json is not a JSON object")
	}
	aliases, ok := project.values["packageAliases"].(*orderedMap)
	if !ok {
		aliases = newOrderedMap()
		project.Set("packageAliases", aliases)
	}
	aliases.Set(alias, id)
	var buffer strings.Builder
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(project); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(buffer.String()), 0o644)
}

// convertedPackage returns the Id and the name of the second-generation
// package that was converted from a first-generation package, creating the
// package when the Dev Hub has none.
func convertedPackage(hub *targetOrg, seedPackage string) (string, string, error) {
	existing, err := toolingRecords(hub, fmt.Sprintf("SELECT Id, Name FROM Package2 WHERE ConvertedFromPackageId = '%s'", seedPackage))
	if err != nil {
		return "", "", err
	}
	if len(existing) > 1 {
		ids := make([]string, len(existing))
		for i, record := range existing {
			ids[i] = text(record.values["Id"])
		}
		return "", "", newError("ErrorMoreThanOnePackage2WithSeedError", "Only one package in a Dev Hub is allowed per converted from first-generation package, but the following were found:\n%s", strings.Join(ids, ", "))
	}
	if len(existing) == 1 {
		return text(existing[0].values["Id"]), text(existing[0].values["Name"]), nil
	}
	subscribers, err := toolingRecords(hub, fmt.Sprintf("SELECT Name, Description, NamespacePrefix FROM SubscriberPackage WHERE Id = '%s'", seedPackage))
	if err != nil || len(subscribers) != 1 {
		return "", "", newError("ErrorNoSubscriberPackageRecordError", "No subscriber package was found for seed id: %s", seedPackage)
	}
	subscriber := subscribers[0]
	id, err := createToolingRecord(hub, "Package2", orderedFields(
		"Name", subscriber.values["Name"], "Description", subscriber.values["Description"], "NamespacePrefix", subscriber.values["NamespacePrefix"],
		"ContainerOptions", "Managed", "ConvertedFromPackageId", seedPackage))
	if err != nil {
		return "", "", err
	}
	return id, text(subscriber.values["Name"]), nil
}

// conversionVersionInfo builds the zip a conversion request carries: the
// descriptor of the package version, a manifest, and the settings of the
// definition file.
func conversionVersionInfo(packageRecordID, patchVersion, definitionFile, apiVersion string, packageDirectory *orderedMap, codeCoverage bool) ([]byte, error) {
	descriptor := orderedFields("id", packageRecordID)
	if patchVersion != "" {
		descriptor.Set("versionNumber", patchVersion)
	}
	if packageDirectory != nil {
		if access, ok := packageDirectory.Get("apexTestAccess"); ok && access != nil {
			descriptor.Set("apexTestAccess", access)
		}
	}
	settingsFiles := lib.ForceMetadataFiles{}
	if definitionFile != "" {
		data, err := os.ReadFile(definitionFile)
		if err != nil {
			return nil, newError("ErrorReadingDefintionFileError", "There was an error while reading or parsing the provided scratch definition file: %s", definitionFile)
		}
		parsed, err := parseOrderedJSON(data)
		definition, ok := parsed.(*orderedMap)
		if err != nil || !ok {
			return nil, newError("ErrorReadingDefintionFileError", "There was an error while reading or parsing the provided scratch definition file: %s", definitionFile)
		}
		if _, unsupported := definition.Get("objectSettings"); unsupported {
			return nil, newError("UnsupportedDefinitionKey", "The definition file key \"objectSettings\" is not supported by the sf-compatible mode of force.")
		}
		settings, _ := definition.values["settings"].(*orderedMap)
		if settings != nil && len(settings.keys) > 0 {
			if preferences, has := definition.Get("orgPreferences"); has && preferences != nil {
				return nil, newError("SignupDuplicateSettingsSpecifiedError", "You cannot use 'settings' and 'orgPreferences' in your scratch definition file, please specify one or the other.")
			}
			settingsFiles = scratchSettingsFiles(settings)
		}
		// These properties of the definition go into the descriptor.
		for _, property := range []string{"country", "edition", "language", "features", "orgPreferences", "snapshot", "release", "sourceOrg"} {
			for _, key := range definition.keys {
				if strings.EqualFold(key, property) {
					descriptor.Set(property, definition.values[key])
				}
			}
		}
	}
	if _, ok := settingsFiles["package.xml"]; !ok {
		version := ""
		if apiVersion != "" {
			version = "\n    <version>" + apiVersion + "</version>"
		}
		settingsFiles["package.xml"] = []byte("<?xml version='1.0'?>\n<Package xmlns='http://soap.sforce.com/2006/04/metadata'>" + version + "\n</Package>")
	}
	settingsZip, err := zipFiles(settingsFiles, "")
	if err != nil {
		return nil, err
	}
	packageZip, err := zipFiles(lib.ForceMetadataFiles{"package.xml": settingsFiles["package.xml"]}, "")
	if err != nil {
		return nil, err
	}
	// When the code coverage is calculated, the permission sets and
	// permission set licenses of apexTestAccess are also sent as lists
	// under these names.
	if access, ok := descriptor.values["apexTestAccess"].(*orderedMap); ok && codeCoverage {
		for _, pair := range [][2]string{{"permissionSets", "permissionSetNames"}, {"permissionSetLicenses", "permissionSetLicenseDeveloperNames"}} {
			var names []string
			switch value := access.values[pair[0]].(type) {
			case []interface{}:
				for _, item := range value {
					names = append(names, strings.TrimSpace(displayValue(item)))
				}
			case string:
				for _, item := range strings.Split(value, ",") {
					names = append(names, strings.TrimSpace(item))
				}
			}
			if len(names) > 0 {
				descriptor.Set(pair[1], names)
			}
		}
	}
	encoded, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, err
	}
	return zipFiles(lib.ForceMetadataFiles{"package.zip": packageZip, "settings.zip": settingsZip, "package2-descriptor.json": encoded}, "")
}

func newPackageConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "convert",
		Short:       "Convert a managed-released first-generation managed package into a second-generation managed package.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageConvert"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := unsupportedFlags(cmd, "seed-metadata"); err != nil {
				return err
			}
			seedPackage, _ := cmd.Flags().GetString("package")
			if seedPackage == "" {
				return missingFlagError("package")
			}
			if err := salesforceIDFlag("package", seedPackage, "033", true); err != nil {
				return err
			}
			installationKey, _ := cmd.Flags().GetString("installation-key")
			bypass, _ := cmd.Flags().GetBool("installation-key-bypass")
			if (installationKey != "") == bypass {
				return exactlyOneError("installation-key", "installation-key-bypass")
			}
			definitionFile, _ := cmd.Flags().GetString("definition-file")
			if definitionFile != "" {
				if _, err := os.Stat(definitionFile); err != nil {
					return flagParseError("", "definition-file", "No file found at "+definitionFile)
				}
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			minutes, _ := cmd.Flags().GetInt("wait")
			out := cmd.OutOrStdout()
			stop := func(string) {}
			if verbose {
				if !jsonRequested(cmd) {
					fmt.Fprintln(out, "Converting Package")
				}
			} else {
				stop = startSpinner(cmd, "Converting Package")
			}
			fail := func(err error) error {
				stop("")
				return err
			}

			packageRecordID, packageName, err := convertedPackage(hub, seedPackage)
			if err != nil {
				return fail(err)
			}
			if err := addPackageAlias(project.Root, packageName, packageRecordID); err != nil {
				return fail(err)
			}
			// The package directory of the converted package, if the
			// project has one, gives the test access of the build.
			var packageDirectory *orderedMap
			if data, err := os.ReadFile(filepath.Join(project.Root, "sfdx-project.json")); err == nil {
				if parsed, err := parseOrderedJSON(data); err == nil {
					if content, ok := parsed.(*orderedMap); ok {
						directories, _ := content.values["packageDirectories"].([]interface{})
						for _, item := range directories {
							directory, ok := item.(*orderedMap)
							if !ok {
								continue
							}
							if name := text(directory.values["package"]); name != "" && (name == packageRecordID || packageID(name) == packageRecordID) {
								packageDirectory = directory
							}
						}
					}
				}
			}
			patchVersion, _ := cmd.Flags().GetString("patch-version")
			codeCoverage, _ := cmd.Flags().GetBool("code-coverage")
			versionInfo, err := conversionVersionInfo(packageRecordID, patchVersion, definitionFile, project.SourceApiVersion, packageDirectory, codeCoverage)
			if err != nil {
				return fail(err)
			}
			request := orderedFields("Package2Id", packageRecordID, "VersionInfo", base64.StdEncoding.EncodeToString(versionInfo))
			if installationKey != "" {
				request.Set("InstallKey", installationKey)
			}
			if instance, _ := cmd.Flags().GetString("build-instance"); instance != "" {
				request.Set("Instance", instance)
			}
			request.Set("IsConversionRequest", true)
			request.Set("CalculateCodeCoverage", codeCoverage)
			requestID, err := createToolingRecord(hub, "Package2VersionCreateRequest", request)
			if err != nil {
				return fail(err)
			}

			load := func() (*orderedMap, error) {
				records, err := toolingRecords(hub, fmt.Sprintf("SELECT %s FROM Package2VersionCreateRequest WHERE Id = '%s'", createRequestFields, requestID))
				if err != nil {
					return nil, err
				}
				if len(records) == 0 {
					return nil, newError("PackageVersionCreateRequestNotFound", "No package version create request found for ID %s.", requestID)
				}
				return createRequestResult(hub, records[0], true), nil
			}
			result, err := load()
			if err != nil {
				return fail(err)
			}
			// sf 2.138.6 reads the request with the Id of the package
			// when --wait is not given, finds nothing and fails with
			// "Cannot read properties of undefined".  This reports the
			// request.
			retries := 0
			if minutes > 0 {
				retries = int(time.Duration(minutes) * time.Minute / (30 * time.Second))
			}
			for attempt := 0; minutes > 0; attempt++ {
				status := text(result.values["Status"])
				if status == "Success" || status == "Error" {
					break
				}
				if attempt >= retries {
					stop("")
					return &Error{
						Name:     "PollingClientTimeout",
						Message:  fmt.Sprintf("The client has timed out. Run 'sf package version create report -i %s' to check the status.", requestID),
						ExitCode: 1,
						Data:     orderedFields("VersionCreateRequestId", requestID),
					}
				}
				remaining := (retries - attempt) * 30
				if verbose && !jsonRequested(cmd) {
					fmt.Fprintf(out, "Request in progress. Will wait a total of %d more seconds before timing out. Current Status='%s'.\n", remaining, titleCase(status))
				}
				time.Sleep(packageConvertInterval)
				if result, err = load(); err != nil {
					return fail(err)
				}
			}

			switch text(result.values["Status"]) {
			case "Error":
				stop("")
				messages, _ := result.values["Error"].([]interface{})
				lines := make([]string, 0, len(messages))
				for i, message := range messages {
					if len(messages) > 1 {
						lines = append(lines, fmt.Sprintf("(%d) %s", i+1, displayValue(message)))
					} else {
						lines = append(lines, displayValue(message))
					}
				}
				if len(lines) == 0 {
					lines = []string{"An unknown error occurred."}
				}
				return newError("SfError", "%s", strings.Join(lines, "\n"))
			case "Success":
				subscriberID := text(result.values["SubscriberPackageVersionId"])
				// The alias of the new version: <package>@<version>-main.
				if versions, err := toolingRecords(hub, fmt.Sprintf("SELECT MajorVersion, MinorVersion, PatchVersion, BuildNumber FROM Package2Version WHERE Id = '%s'", text(result.values["Package2VersionId"]))); err == nil && len(versions) == 1 && os.Getenv("SF_PROJECT_AUTOUPDATE_DISABLE_FOR_PACKAGE_VERSION_CREATE") != "true" {
					version := versions[0]
					alias := fmt.Sprintf("%s@%s.%s.%s-%s-main", packageName, displayValue(version.values["MajorVersion"]), displayValue(version.values["MinorVersion"]),
						displayValue(version.values["PatchVersion"]), displayValue(version.values["BuildNumber"]))
					if err := addPackageAlias(project.Root, alias, subscriberID); err != nil {
						return fail(err)
					}
				}
				message := fmt.Sprintf("Successfully created the package version [%s]. Subscriber Package Version Id: %s\nPackage Installation URL: https://login.salesforce.com/packaging/installPackage.apexp?p0=%s\nAs an alternative, you can use the \"%s package install\" command.",
					text(result.values["Id"]), subscriberID, subscriberID, binName)
				if !verbose {
					stop(message)
				}
				return emit(cmd, result, func(w io.Writer) {
					if verbose {
						fmt.Fprintln(w, message)
					}
				})
			}
			message := fmt.Sprintf("Package version creation request status is '%s'. Run \"%s package version create report -i %s\" to query for status.", titleCase(text(result.values["Status"])), binName, text(result.values["Id"]))
			if !verbose {
				stop(message)
			}
			return emit(cmd, result, func(w io.Writer) {
				if verbose {
					fmt.Fprintln(w, message)
				}
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID (starts with 033) of the first-generation managed package to convert.")
	cmd.Flags().StringP("installation-key", "k", "", "Installation key for key-protected package.")
	cmd.Flags().StringP("definition-file", "f", "", "Path to a definition file that contains features and org preferences that the metadata of the package version depends on.")
	cmd.Flags().BoolP("installation-key-bypass", "x", false, "Bypass the installation key requirement.")
	cmd.Flags().IntP("wait", "w", 0, "Minutes to wait for the package version to be created.")
	cmd.Flags().StringP("build-instance", "s", "", "Instance where the conversion package version will be created, such as NA50.")
	cmd.Flags().MarkHidden("build-instance")
	cmd.Flags().StringP("seed-metadata", "m", "", "Directory containing metadata to be deployed prior to conversion.")
	cmd.Flags().Bool("verbose", false, "Display verbose command output.")
	cmd.Flags().StringP("patch-version", "a", "", "Specific released patch version to be converted.")
	cmd.Flags().BoolP("code-coverage", "c", false, "Calculate and store the code coverage percentage by running the packaged Apex tests included in this package version.")
	addTargetDevHubFlag(cmd)
	return cmd
}
