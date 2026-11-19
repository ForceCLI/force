package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// packagePollInterval is the time between checks of a package install or
// uninstall request.  Tests shorten it.
var packagePollInterval = 2 * time.Second

func newPackageCmd() *cobra.Command {
	return newGroup("package", "Commands to develop and install unlocked packages and managed 2GP packages.",
		newPackageCreateCmd(),
		newPackageConvertCmd(),
		newPackageDeleteCmd(false),
		newPackageListCmd(),
		newPackageUpdateCmd(),
		newPackagePushUpgradeCmd(),
		withSubcommands(newPackageInstallCmd(), newPackageInstallReportCmd()),
		newGroup("installed", "List installed packages.", newPackageInstalledListCmd()),
		withSubcommands(newPackageUninstallCmd(), newPackageUninstallReportCmd()),
		newGroup("version", "Commands to manage package versions.",
			withSubcommands(newPackageVersionCreateCmd(), newPackageVersionCreateListCmd(), newPackageVersionCreateReportCmd()),
			newPackageDeleteCmd(true),
			newPackageVersionListCmd(),
			newPackageVersionReportCmd(),
			newPackageVersionPromoteCmd(),
			newPackageVersionUpdateCmd(),
			newPackageVersionDisplayAncestryCmd(),
			newPackageVersionDisplayDependenciesCmd(),
			newPackageVersionRetrieveCmd(),
		),
	)
}

// packageAliases returns the packageAliases of sfdx-project.json, or an
// empty map outside a project.
func packageAliases() map[string]string {
	aliases := map[string]string{}
	wd, err := os.Getwd()
	if err != nil {
		return aliases
	}
	root, ok := lib.SFProjectRoot(wd)
	if !ok {
		return aliases
	}
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		return aliases
	}
	var project struct {
		PackageAliases map[string]string `json:"packageAliases"`
	}
	if json.Unmarshal(data, &project) == nil && project.PackageAliases != nil {
		return project.PackageAliases
	}
	return aliases
}

// packageID returns the Id for a package alias, or the argument when it is
// not an alias.
func packageID(aliasOrID string) string {
	if id, ok := packageAliases()[aliasOrID]; ok {
		return id
	}
	return aliasOrID
}

// aliasesOf returns the aliases that refer to an Id, joined by commas.
func aliasesOf(ids ...string) string {
	var names []string
	for alias, id := range packageAliases() {
		for _, wanted := range ids {
			if id == wanted && wanted != "" {
				names = append(names, alias)
			}
		}
	}
	sortStrings(names)
	return strings.Join(names, ",")
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func boolValue(value interface{}) bool {
	b, _ := value.(bool)
	return b
}

func newPackageListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all packages in the Dev Hub org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageListCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			records, err := toolingRecords(org, "SELECT Id, SubscriberPackageId, Name, Description, NamespacePrefix, ContainerOptions, ConvertedFromPackageId, IsOrgDependent, PackageErrorUsername, AppAnalyticsEnabled, CreatedById, IsDeprecated FROM Package2 WHERE IsDeprecated != true ORDER BY NamespacePrefix, Name")
			if err != nil {
				return err
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			results := make([]*orderedMap, 0, len(records))
			rows := make([][]string, 0, len(records))
			cell := func(value interface{}) string {
				if value == nil {
					return ""
				}
				return displayValue(value)
			}
			for _, record := range records {
				orgDependent := "No"
				if record.values["ContainerOptions"] == "Managed" {
					orgDependent = "N/A"
				} else if boolValue(record.values["IsOrgDependent"]) {
					orgDependent = "Yes"
				}
				id := text(record.values["Id"])
				result := newOrderedMap()
				result.Set("Id", id)
				result.Set("SubscriberPackageId", record.values["SubscriberPackageId"])
				result.Set("Name", record.values["Name"])
				result.Set("Description", record.values["Description"])
				result.Set("NamespacePrefix", record.values["NamespacePrefix"])
				result.Set("ContainerOptions", record.values["ContainerOptions"])
				result.Set("ConvertedFromPackageId", record.values["ConvertedFromPackageId"])
				result.Set("Alias", aliasesOf(id))
				result.Set("IsOrgDependent", orgDependent)
				result.Set("PackageErrorUsername", record.values["PackageErrorUsername"])
				result.Set("AppAnalyticsEnabled", record.values["AppAnalyticsEnabled"])
				result.Set("CreatedBy", record.values["CreatedById"])
				results = append(results, result)
				row := []string{cell(record.values["NamespacePrefix"]), cell(record.values["Name"]), id, aliasesOf(id), cell(record.values["Description"]), cell(record.values["ContainerOptions"])}
				if verbose {
					row = append(row, cell(record.values["SubscriberPackageId"]), cell(record.values["ConvertedFromPackageId"]), orgDependent,
						cell(record.values["PackageErrorUsername"]), cell(record.values["CreatedById"]), cell(record.values["AppAnalyticsEnabled"]))
				}
				rows = append(rows, row)
			}
			return emit(cmd, results, func(w io.Writer) {
				headers := []string{"Namespace Prefix", "Name", "Id", "Alias", "Description", "Container Options"}
				if verbose {
					headers = append(headers, "Package Id", "Converted From Package Id", "Org Dependent Unlocked Package", "Error Notification Username", "Created By", "App Analytics Enabled")
				}
				printTable(w, paint(w, style{"\x1b[34m", "\x1b[39m"}, fmt.Sprintf("Packages [%d]", len(rows))), headers, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("verbose", false, "Display extended package detail.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

// recordPackageInProject adds a new package to sfdx-project.json: the
// alias in packageAliases, and the package name and first version on the
// package directory.  The other content of the file is kept.
func recordPackageInProject(name, path, id string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, ok := lib.SFProjectRoot(wd)
	if !ok {
		return nil
	}
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
	if directories, ok := project.values["packageDirectories"].([]interface{}); ok {
		for _, item := range directories {
			directory, ok := item.(*orderedMap)
			if !ok || filepath.Clean(text(directory.values["path"])) != filepath.Clean(path) {
				continue
			}
			directory.Set("package", name)
			if _, present := directory.Get("versionName"); !present {
				directory.Set("versionName", "ver 0.1")
			}
			if _, present := directory.Get("versionNumber"); !present {
				directory.Set("versionNumber", "0.1.0.NEXT")
			}
		}
	}
	aliases, ok := project.values["packageAliases"].(*orderedMap)
	if !ok {
		aliases = newOrderedMap()
		project.Set("packageAliases", aliases)
	}
	aliases.Set(name, id)
	var buffer strings.Builder
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(project); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(buffer.String()), 0o644)
}

func newPackageCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a package.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageCreateCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			packageType, _ := cmd.Flags().GetString("package-type")
			path, _ := cmd.Flags().GetString("path")
			description, _ := cmd.Flags().GetString("description")
			for flag, value := range map[string]string{"name": name, "package-type": packageType, "path": path} {
				if value == "" {
					return missingFlagError(flag)
				}
			}
			if packageType != "Managed" && packageType != "Unlocked" {
				return &Error{Message: fmt.Sprintf("Expected --package-type=%s to be one of: Managed, Unlocked\nSee more help with --help", packageType), ExitCode: 1}
			}
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			// The namespace is the one registered in sfdx-project.json.
			namespace := ""
			if wd, err := os.Getwd(); err == nil {
				if root, ok := lib.SFProjectRoot(wd); ok {
					if data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json")); err == nil {
						var project struct {
							Namespace string `json:"namespace"`
						}
						json.Unmarshal(data, &project)
						namespace = project.Namespace
					}
				}
			}
			request := newOrderedMap()
			request.Set("Name", name)
			request.Set("ContainerOptions", packageType)
			if noNamespace, _ := cmd.Flags().GetBool("no-namespace"); noNamespace {
				namespace = ""
			}
			request.Set("NamespacePrefix", namespace)
			if description != "" {
				request.Set("Description", description)
			}
			if cmd.Flags().Changed("org-dependent") {
				orgDependent, _ := cmd.Flags().GetBool("org-dependent")
				request.Set("IsOrgDependent", orgDependent)
			}
			if cmd.Flags().Changed("error-notification-username") {
				username, _ := cmd.Flags().GetString("error-notification-username")
				request.Set("PackageErrorUsername", username)
			}
			content, err := encodeJSON(request)
			if err != nil {
				return err
			}
			body, err := org.Force.PostREST("/tooling/sobjects/Package2", content)
			if err != nil {
				return apiError(err)
			}
			var created struct {
				Id string `json:"id"`
			}
			if err := json.Unmarshal([]byte(body), &created); err != nil || created.Id == "" {
				return fmt.Errorf("the org did not return a package Id")
			}
			if err := recordPackageInProject(name, path, created.Id); err != nil {
				return err
			}
			result := newOrderedMap()
			result.Set("Id", created.Id)
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Ids", []string{"Name", "Value"}, [][]string{{"Package Id", created.Id}})
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the package to create.")
	cmd.Flags().StringP("package-type", "t", "", "Type of package.")
	cmd.Flags().StringP("description", "d", "", "Description of the package.")
	cmd.Flags().BoolP("no-namespace", "e", false, "Create the package with no namespace; available only for unlocked packages.")
	cmd.Flags().StringP("path", "r", "", "Path to directory that contains the contents of the package.")
	cmd.Flags().Bool("org-dependent", false, "Depends on unpackaged metadata in the installation org; applies to unlocked packages only.")
	cmd.Flags().StringP("error-notification-username", "o", "", "Active Dev Hub user designated to receive email notifications for package errors.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

func newPackageInstalledListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the org’s installed packages.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageInstalledListCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			records, err := toolingRecords(org, "SELECT Id, SubscriberPackageId, SubscriberPackage.NamespacePrefix, SubscriberPackage.Name, SubscriberPackageVersion.Id, SubscriberPackageVersion.Name, SubscriberPackageVersion.MajorVersion, SubscriberPackageVersion.MinorVersion, SubscriberPackageVersion.PatchVersion, SubscriberPackageVersion.BuildNumber, SubscriberPackageVersion.IsManaged, SubscriberPackageVersion.Package2ContainerOptions FROM InstalledSubscriberPackage ORDER BY SubscriberPackageId")
			if err != nil {
				return err
			}
			results := make([]*orderedMap, 0, len(records))
			rows := make([][]string, 0, len(records))
			cell := func(result *orderedMap, key string) string {
				if value, ok := result.Get(key); ok && value != nil {
					return displayValue(value)
				}
				return ""
			}
			for _, record := range records {
				result := newOrderedMap()
				result.Set("Id", record.values["Id"])
				result.Set("SubscriberPackageId", record.values["SubscriberPackageId"])
				if subscriberPackage, ok := record.values["SubscriberPackage"].(*orderedMap); ok {
					result.Set("SubscriberPackageName", subscriberPackage.values["Name"])
					result.Set("SubscriberPackageNamespace", subscriberPackage.values["NamespacePrefix"])
				}
				settings := ""
				if version, ok := record.values["SubscriberPackageVersion"].(*orderedMap); ok {
					result.Set("SubscriberPackageVersionId", version.values["Id"])
					result.Set("SubscriberPackageVersionName", version.values["Name"])
					result.Set("SubscriberPackageVersionNumber", fmt.Sprintf("%s.%s.%s.%s",
						displayValue(version.values["MajorVersion"]), displayValue(version.values["MinorVersion"]),
						displayValue(version.values["PatchVersion"]), displayValue(version.values["BuildNumber"])))
					containerOptions := text(version.values["Package2ContainerOptions"])
					switch {
					case boolValue(version.values["IsManaged"]) && containerOptions == "":
						settings = "namespace"
					case containerOptions == "Managed":
						settings = "packageId"
					}
				}
				result.Set("VersionSettings", settings)
				results = append(results, result)
				rows = append(rows, []string{
					cell(result, "Id"), cell(result, "SubscriberPackageId"), cell(result, "SubscriberPackageName"), cell(result, "SubscriberPackageNamespace"),
					settings, cell(result, "SubscriberPackageVersionId"), cell(result, "SubscriberPackageVersionName"), cell(result, "SubscriberPackageVersionNumber"),
				})
			}
			return emit(cmd, results, func(w io.Writer) {
				printTable(w, "", []string{"ID", "Package ID", "Package Name", "Namespace", "Version Settings", "Package Version ID", "Version Name", "Version"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// toolingRecord reads one Tooling API record.
func toolingRecord(org *targetOrg, sobject, id string) (*orderedMap, error) {
	body, err := org.Force.GetREST("/tooling/sobjects/" + sobject + "/" + id)
	if err != nil {
		return nil, apiError(err)
	}
	parsed, err := parseOrderedJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	record, ok := parsed.(*orderedMap)
	if !ok {
		return nil, fmt.Errorf("unexpected response for %s %s", sobject, id)
	}
	return record, nil
}

// createToolingRecord creates a Tooling API record and returns its Id.
func createToolingRecord(org *targetOrg, sobject string, fields *orderedMap) (string, error) {
	content, err := encodeJSON(fields)
	if err != nil {
		return "", err
	}
	body, err := org.Force.PostREST("/tooling/sobjects/"+sobject, content)
	if err != nil {
		return "", apiError(err)
	}
	var created struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.Id == "" {
		return "", fmt.Errorf("the org did not return an Id for the new %s", sobject)
	}
	return created.Id, nil
}

// waitForRequest polls a Tooling API request record until its Status is
// one of the final values or the timeout passes.
func waitForRequest(org *targetOrg, sobject, id string, timeout time.Duration, final ...string) (*orderedMap, error) {
	deadline := time.Now().Add(timeout)
	for {
		record, err := toolingRecord(org, sobject, id)
		if err != nil {
			return nil, err
		}
		status := text(record.values["Status"])
		for _, value := range final {
			if strings.EqualFold(status, value) {
				return record, nil
			}
		}
		if time.Now().After(deadline) {
			return record, nil
		}
		time.Sleep(packagePollInterval)
	}
}

// packagePublishInterval is the time between checks of whether a package
// version is available in the target org.  Tests shorten it.
var packagePublishInterval = 10 * time.Second

// packagePublishSleep waits between two of those checks.  Tests replace it.
var packagePublishSleep = time.Sleep

// waitForPackagePublish waits until a package version can be installed in
// the org: a new version takes some time to reach the instance of the org.
func waitForPackagePublish(cmd *cobra.Command, org *targetOrg, versionID, installationKey string, timeout time.Duration) error {
	stop := startSpinner(cmd, fmt.Sprintf("%d minutes remaining until timeout. Publish status: Querying Status", int(timeout.Minutes())))
	soql := fmt.Sprintf("SELECT Id, SubscriberPackageId, InstallValidationStatus FROM SubscriberPackageVersion WHERE Id ='%s'", versionID)
	if installationKey != "" {
		soql += fmt.Sprintf(" AND InstallationKey ='%s'", strings.ReplaceAll(strings.ReplaceAll(installationKey, `\`, `\\`), "'", `\'`))
	}
	waiting := map[string]bool{"PACKAGE_UNAVAILABLE": true, "PACKAGE_UNAVAILABLE_CRC": true, "PACKAGE_UNAVAILABLE_ZIP": true, "UNINSTALL_IN_PROGRESS": true}
	var last *orderedMap
	for elapsed := time.Duration(0); ; elapsed += packagePublishInterval {
		status := "PACKAGE_UNAVAILABLE"
		records, err := toolingRecords(org, soql)
		if err != nil {
			// A wrong installation key is an error; a version the org
			// cannot see yet is not.
			if failure, ok := err.(*Error); ok && strings.Contains(failure.Name, "INSTALL_KEY") {
				stop("")
				return err
			}
		} else if len(records) > 0 {
			last = records[0]
			status = text(last.values["InstallValidationStatus"])
		}
		if !waiting[status] {
			stop("done")
			return nil
		}
		if elapsed+packagePublishInterval > timeout {
			stop("")
			notPublished := &Error{Name: "SubscriberPackageVersionNotPublishedError", Message: "The subscriber package version is not fully available.", ExitCode: 1}
			if last != nil {
				notPublished.Data = last
			}
			return notPublished
		}
		packagePublishSleep(packagePublishInterval)
	}
}

func newPackageInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "install",
		Short:       "Install or upgrade a version of a package in the target org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Install"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			securityType, _ := cmd.Flags().GetString("security-type")
			upgradeType, _ := cmd.Flags().GetString("upgrade-type")
			apexCompile, _ := cmd.Flags().GetString("apex-compile")
			securityTypes := map[string]string{"AllUsers": "full", "AdminsOnly": "none"}
			upgradeTypes := map[string]string{"DeprecateOnly": "deprecate-only", "Mixed": "mixed-mode", "Delete": "delete-only"}
			if _, ok := securityTypes[securityType]; !ok {
				return &Error{Message: fmt.Sprintf("Expected --security-type=%s to be one of: AllUsers, AdminsOnly\nSee more help with --help", securityType), ExitCode: 1}
			}
			if _, ok := upgradeTypes[upgradeType]; !ok {
				return &Error{Message: fmt.Sprintf("Expected --upgrade-type=%s to be one of: DeprecateOnly, Mixed, Delete\nSee more help with --help", upgradeType), ExitCode: 1}
			}
			if apexCompile != "all" && apexCompile != "package" {
				return &Error{Message: fmt.Sprintf("Expected --apex-compile=%s to be one of: all, package\nSee more help with --help", apexCompile), ExitCode: 1}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			installationKey, _ := cmd.Flags().GetString("installation-key")
			if minutes, _ := cmd.Flags().GetInt("publish-wait"); minutes > 0 {
				if err := waitForPackagePublish(cmd, org, packageID(packageFlag), installationKey, time.Duration(minutes)*time.Minute); err != nil {
					return err
				}
			}
			request := newOrderedMap()
			request.Set("SubscriberPackageVersionKey", packageID(packageFlag))
			request.Set("Password", installationKey)
			request.Set("ApexCompileType", apexCompile)
			request.Set("SecurityType", securityTypes[securityType])
			if handlers, _ := cmd.Flags().GetStringArray("skip-handlers"); len(handlers) > 0 {
				for _, handler := range handlers {
					if handler != "FeatureEnforcement" {
						return &Error{Message: fmt.Sprintf("Expected --skip-handlers=%s to be one of: FeatureEnforcement\nSee more help with --help", handler), ExitCode: 1}
					}
				}
				request.Set("SkipHandlers", strings.Join(handlers, ","))
			}
			request.Set("UpgradeType", upgradeTypes[upgradeType])
			request.Set("NameConflictResolution", "Block")
			request.Set("EnableRss", true)
			id, err := createToolingRecord(org, "PackageInstallRequest", request)
			if err != nil {
				return err
			}
			var record *orderedMap
			if cmd.Flags().Changed("wait") {
				minutes, _ := cmd.Flags().GetInt("wait")
				stop := startSpinner(cmd, fmt.Sprintf("Waiting %d minutes for package install to complete.", minutes))
				record, err = waitForRequest(org, "PackageInstallRequest", id, time.Duration(minutes)*time.Minute, "SUCCESS", "ERROR")
				stop("done")
			} else {
				record, err = toolingRecord(org, "PackageInstallRequest", id)
			}
			if err != nil {
				return err
			}
			return emitInstallRequest(cmd, org, record)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for installation status.")
	cmd.Flags().StringP("installation-key", "k", "", "Installation key for key-protected package (default: null).")
	cmd.Flags().IntP("publish-wait", "b", 0, "Maximum number of minutes to wait for the Subscriber Package Version ID to become available in the target org before canceling the install request.")
	cmd.Flags().BoolP("no-prompt", "r", false, "Don't prompt for confirmation.")
	cmd.Flags().StringP("package", "p", "", "ID (starts with 04t) or alias of the package version to install.")
	cmd.Flags().StringP("apex-compile", "a", "all", "Compile all Apex in the org and package, or only Apex in the package; unlocked packages only.")
	cmd.Flags().StringP("security-type", "s", "AdminsOnly", "Security access type for the installed package.")
	cmd.Flags().StringP("upgrade-type", "t", "Mixed", "Upgrade type for the package installation; available only for unlocked packages.")
	cmd.Flags().StringArrayP("skip-handlers", "l", nil, "Skip install handlers.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newPackageUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "uninstall",
		Short:       "Uninstall a second-generation package from the target org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageUninstallCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			request := newOrderedMap()
			request.Set("SubscriberPackageVersionId", packageID(packageFlag))
			id, err := createToolingRecord(org, "SubscriberPackageVersionUninstallRequest", request)
			if err != nil {
				return err
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			record, err := waitForRequest(org, "SubscriberPackageVersionUninstallRequest", id, time.Duration(minutes)*time.Minute, "Success", "Error")
			if err != nil {
				return err
			}
			return emitUninstallRequest(cmd, org, record)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for uninstall status.")
	cmd.Flags().StringP("package", "p", "", "ID (starts with 04t) or alias of the package version to uninstall.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

const packageVersionFields = "Id, Package2Id, SubscriberPackageVersionId, Name, Package2.Name, Package2.NamespacePrefix, Package2.IsOrgDependent, Package2.ContainerOptions, Description, Tag, Branch, MajorVersion, MinorVersion, PatchVersion, BuildNumber, IsReleased, CreatedDate, LastModifiedDate, IsPasswordProtected, AncestorId, ValidationSkipped, ValidatedAsync, CreatedById, ConvertedFromVersionId, ReleaseVersion, BuildDurationInSeconds, HasMetadataRemoved"

// packageVersionVerboseFields are the fields "package version list" reads in
// addition to packageVersionFields when --verbose is given.
const packageVersionVerboseFields = ", CodeCoverage, HasPassedCodeCoverageCheck, Language"

// packageVersionResult converts a Package2Version record to the result sf
// reports.  sf reads the code coverage and the language only for --verbose,
// and leaves the keys it has no value for out of the JSON result.
func packageVersionResult(record *orderedMap, asJSON bool, verbose bool, ancestorVersions map[string]string) *orderedMap {
	parent, _ := record.values["Package2"].(*orderedMap)
	if parent == nil {
		parent = newOrderedMap()
	}
	managed := parent.values["ContainerOptions"] == "Managed"
	version := fmt.Sprintf("%s.%s.%s.%s", displayValue(record.values["MajorVersion"]), displayValue(record.values["MinorVersion"]),
		displayValue(record.values["PatchVersion"]), displayValue(record.values["BuildNumber"]))
	date := func(value interface{}) string {
		converted := isoTime(text(value))
		if len(converted) >= 16 {
			return strings.Replace(converted[:16], "T", " ", 1)
		}
		return converted
	}
	flag := func(value interface{}) interface{} {
		if asJSON {
			return value
		}
		return displayValue(value)
	}
	ancestorID := record.values["AncestorId"]
	var ancestorVersion interface{}
	if ancestorID != nil {
		if found, ok := ancestorVersions[text(ancestorID)]; ok {
			ancestorVersion = found
		}
	} else if !managed {
		ancestorID, ancestorVersion = "N/A", "N/A"
	}
	orgDependent := "No"
	if managed {
		orgDependent = "N/A"
	} else if boolValue(parent.values["IsOrgDependent"]) {
		orgDependent = "Yes"
	}
	metadataRemoved := "N/A"
	if managed {
		metadataRemoved = "No"
		if boolValue(record.values["HasMetadataRemoved"]) {
			metadataRemoved = "Yes"
		}
	}
	subscriberID := text(record.values["SubscriberPackageVersionId"])
	result := newOrderedMap()
	result.Set("Package2Id", record.values["Package2Id"])
	result.Set("Branch", record.values["Branch"])
	result.Set("Tag", record.values["Tag"])
	result.Set("MajorVersion", record.values["MajorVersion"])
	result.Set("MinorVersion", record.values["MinorVersion"])
	result.Set("PatchVersion", record.values["PatchVersion"])
	result.Set("BuildNumber", record.values["BuildNumber"])
	result.Set("Id", record.values["Id"])
	result.Set("SubscriberPackageVersionId", subscriberID)
	result.Set("ConvertedFromVersionId", record.values["ConvertedFromVersionId"])
	result.Set("Name", record.values["Name"])
	result.Set("NamespacePrefix", parent.values["NamespacePrefix"])
	result.Set("Package2Name", parent.values["Name"])
	result.Set("Description", record.values["Description"])
	result.Set("Version", version)
	result.Set("IsPasswordProtected", flag(record.values["IsPasswordProtected"]))
	result.Set("IsReleased", flag(record.values["IsReleased"]))
	result.Set("CreatedDate", date(record.values["CreatedDate"]))
	result.Set("LastModifiedDate", date(record.values["LastModifiedDate"]))
	result.Set("InstallUrl", "https://login.salesforce.com/packaging/installPackage.apexp?p0="+subscriberID)
	skipsCoverage := boolValue(parent.values["IsOrgDependent"]) || boolValue(record.values["ValidationSkipped"])
	coverage := "use --verbose for code coverage"
	if verbose {
		coverage = ""
		if skipsCoverage {
			coverage = "N/A"
		}
		if detail, ok := record.values["CodeCoverage"].(*orderedMap); ok && detail.values["apexCodeCoveragePercentage"] != nil {
			coverage = displayValue(detail.values["apexCodeCoveragePercentage"]) + "%"
		}
	}
	result.Set("CodeCoverage", coverage)
	if skipsCoverage {
		result.Set("HasPassedCodeCoverageCheck", "N/A")
	} else if verbose {
		result.Set("HasPassedCodeCoverageCheck", record.values["HasPassedCodeCoverageCheck"])
	}
	result.Set("ValidationSkipped", record.values["ValidationSkipped"])
	result.Set("ValidatedAsync", record.values["ValidatedAsync"])
	result.Set("AncestorId", ancestorID)
	if ancestorVersion != nil {
		result.Set("AncestorVersion", ancestorVersion)
	}
	result.Set("Alias", aliasesOf(text(record.values["Id"]), subscriberID))
	result.Set("IsOrgDependent", orgDependent)
	releaseVersion := ""
	switch value := record.values["ReleaseVersion"].(type) {
	case nil:
	case float64:
		releaseVersion = strconv.FormatFloat(value, 'f', 1, 64)
	default:
		releaseVersion = displayValue(value)
		if number, err := strconv.ParseFloat(releaseVersion, 64); err == nil {
			releaseVersion = strconv.FormatFloat(number, 'f', 1, 64)
		}
	}
	result.Set("ReleaseVersion", releaseVersion)
	var buildDuration interface{} = ""
	if record.values["BuildDurationInSeconds"] != nil {
		buildDuration = record.values["BuildDurationInSeconds"]
	}
	result.Set("BuildDurationInSeconds", buildDuration)
	result.Set("HasMetadataRemoved", metadataRemoved)
	result.Set("CreatedBy", record.values["CreatedById"])
	if verbose {
		result.Set("Language", record.values["Language"])
	}
	return result
}

// packageVersionNumbers returns the version numbers of the ancestors of the
// given package versions, by the 04t Id of the ancestor.
func packageVersionNumbers(org *targetOrg, records []*orderedMap) (map[string]string, error) {
	versions := map[string]string{}
	var ids []string
	for _, record := range records {
		if id := text(record.values["AncestorId"]); id != "" && versions[id] == "" {
			versions[id] = "-"
			ids = append(ids, id)
		}
	}
	const chunk = 200
	for start := 0; start < len(ids); start += chunk {
		end := start + chunk
		if end > len(ids) {
			end = len(ids)
		}
		found, err := toolingRecords(org, fmt.Sprintf("SELECT SubscriberPackageVersionId, MajorVersion, MinorVersion, PatchVersion, BuildNumber FROM Package2Version WHERE SubscriberPackageVersionId IN (%s)", quoteIds(ids[start:end])))
		if err != nil {
			return nil, err
		}
		for _, record := range found {
			versions[text(record.values["SubscriberPackageVersionId"])] = fmt.Sprintf("%s.%s.%s.%s", displayValue(record.values["MajorVersion"]),
				displayValue(record.values["MinorVersion"]), displayValue(record.values["PatchVersion"]), displayValue(record.values["BuildNumber"]))
		}
	}
	for id, version := range versions {
		if version == "-" {
			delete(versions, id)
		}
	}
	return versions, nil
}

func resultCell(result *orderedMap, key string) string {
	value, ok := result.Get(key)
	if !ok || value == nil {
		return ""
	}
	return displayValue(value)
}

func newPackageVersionListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all package versions in the Dev Hub org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionListCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			conditions := []string{"IsDeprecated != true"}
			var packages []string
			list, _ := cmd.Flags().GetString("packages")
			for _, item := range strings.Split(list, ",") {
				if item = strings.TrimSpace(item); item != "" {
					packages = append(packages, item)
				}
			}
			if len(packages) > 0 {
				ids := make([]string, len(packages))
				for i, name := range packages {
					ids[i] = packageID(name)
				}
				conditions = append(conditions, fmt.Sprintf("Package2Id IN (%s)", quoteIds(ids)))
			}
			if released, _ := cmd.Flags().GetBool("released"); released {
				conditions = append(conditions, "IsReleased = true")
			}
			if days, _ := cmd.Flags().GetInt("created-last-days"); cmd.Flags().Changed("created-last-days") {
				conditions = append(conditions, fmt.Sprintf("CreatedDate = LAST_N_DAYS:%d", days))
			}
			if days, _ := cmd.Flags().GetInt("modified-last-days"); cmd.Flags().Changed("modified-last-days") {
				conditions = append(conditions, fmt.Sprintf("LastModifiedDate = LAST_N_DAYS:%d", days))
			}
			conversions, _ := cmd.Flags().GetBool("show-conversions-only")
			if conversions {
				conditions = append(conditions, "ConvertedFromVersionId != null")
			}
			if branch, _ := cmd.Flags().GetString("branch"); branch != "" {
				conditions = append(conditions, fmt.Sprintf("Branch = '%s'", strings.ReplaceAll(branch, "'", `\'`)))
			}
			orderBy, _ := cmd.Flags().GetString("order-by")
			if orderBy == "" {
				orderBy = "Package2Id, Branch, MajorVersion, MinorVersion, PatchVersion, BuildNumber"
			}
			concise, _ := cmd.Flags().GetBool("concise")
			verbose, _ := cmd.Flags().GetBool("verbose")
			fields := packageVersionFields
			if verbose {
				fields += packageVersionVerboseFields
			}
			records, err := toolingRecords(org, fmt.Sprintf("SELECT %s FROM Package2Version WHERE %s ORDER BY %s", fields, strings.Join(conditions, " AND "), orderBy))
			if err != nil {
				return err
			}
			columns := [][2]string{
				{"Package2Name", "Package Name"}, {"NamespacePrefix", "Namespace"}, {"Name", "Version Name"}, {"Version", "Version"},
				{"SubscriberPackageVersionId", "Subscriber Package Version Id"}, {"Alias", "Alias"}, {"IsPasswordProtected", "Installation Key"},
				{"IsReleased", "Released"}, {"ValidationSkipped", "Validation Skipped"}, {"ValidatedAsync", "Validated Async"},
				{"AncestorId", "Ancestor"}, {"AncestorVersion", "Ancestor Version"}, {"Branch", "Branch"},
			}
			if conversions && !verbose {
				columns = append(columns, [2]string{"ConvertedFromVersionId", "Converted From Version Id"})
			}
			if concise {
				columns = [][2]string{{"Package2Id", "Package Id"}, {"Version", "Version"}, {"SubscriberPackageVersionId", "Subscriber Package Version Id"}, {"IsReleased", "Released"}}
			} else if verbose {
				columns = append(columns, [][2]string{
					{"Package2Id", "Package Id"}, {"InstallUrl", "Installation URL"}, {"Id", "Package Version Id"}, {"CreatedDate", "Created Date"},
					{"LastModifiedDate", "Last Modified Date"}, {"Tag", "Tag"}, {"Description", "Description"}, {"CodeCoverage", "Code Coverage"},
					{"HasPassedCodeCoverageCheck", "Code Coverage Met"},
					{"ConvertedFromVersionId", "Converted From Version Id"}, {"IsOrgDependent", "Org-Dependent Unlocked Package"},
					{"ReleaseVersion", "Release Version"}, {"BuildDurationInSeconds", "Build Duration in Seconds"},
					{"HasMetadataRemoved", "Managed Metadata Removed"}, {"CreatedBy", "Created By"}, {"Language", "Language"},
				}...)
			}
			ancestorVersions, err := packageVersionNumbers(org, records)
			if err != nil {
				return err
			}
			results := make([]*orderedMap, 0, len(records))
			rows := make([][]string, 0, len(records))
			for _, record := range records {
				results = append(results, packageVersionResult(record, true, verbose, ancestorVersions))
				human := packageVersionResult(record, false, verbose, ancestorVersions)
				row := make([]string, len(columns))
				for i, column := range columns {
					row[i] = resultCell(human, column[0])
				}
				rows = append(rows, row)
			}
			return emit(cmd, results, func(w io.Writer) {
				if len(rows) == 0 {
					warn(cmd, "No results found")
					return
				}
				headers := make([]string, len(columns))
				for i, column := range columns {
					headers[i] = column[1]
				}
				printTable(w, fmt.Sprintf("Package Versions [%d]", len(rows)), headers, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().IntP("created-last-days", "c", 0, "Number of days since the request was created, starting at 00:00:00 of first day to now.")
	cmd.Flags().Bool("concise", false, "Display limited package version details.")
	cmd.Flags().Bool("show-conversions-only", false, "Filter the list output to display only converted package version.")
	cmd.Flags().IntP("modified-last-days", "m", 0, "Number of days since the items were modified, starting at 00:00:00 of first day to now.")
	cmd.Flags().StringP("packages", "p", "", "Comma-delimited list of packages (aliases or 0Ho IDs) to list.")
	cmd.Flags().BoolP("released", "r", false, "Display released versions only (IsReleased=true).")
	cmd.Flags().StringP("branch", "b", "", "Branch in source control repository on which the package version is based.")
	cmd.Flags().StringP("order-by", "o", "", "Package version fields used to order the list.")
	cmd.Flags().Bool("verbose", false, "Display extended package version details.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

// findPackageVersion reads the Package2Version for a 05i or 04t Id or an
// alias.
func findPackageVersion(org *targetOrg, aliasOrID string) (*orderedMap, error) {
	id := packageID(aliasOrID)
	field := "Id"
	if strings.HasPrefix(id, "04t") {
		field = "SubscriberPackageVersionId"
	}
	records, err := toolingRecords(org, fmt.Sprintf("SELECT %s, Language FROM Package2Version WHERE %s = '%s'", packageVersionFields, field, strings.ReplaceAll(id, "'", `\'`)))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		searched, other := "Subscriber Package Version Id", "Package Version Id"
		if field == "Id" {
			searched, other = other, searched
		}
		return nil, newError("ErrorInvalidIdNoMatchingVersionIdError", "The %s %s is invalid, as a corresponding %s was not found", searched, id, other)
	}
	return records[0], nil
}

func newPackageVersionReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Retrieve details about a package version in the Dev Hub org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionReportCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			record, err := findPackageVersion(org, packageFlag)
			if err != nil {
				return err
			}
			return runPackageVersionReport(cmd, org, text(record.values["Id"]))
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID (starts with 04t) or alias of the package to retrieve details for.")
	cmd.Flags().Bool("verbose", false, "Display extended package version details.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

func newPackageVersionPromoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "promote",
		Short:       "Promote a package version to released.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionPromoteCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			record, err := findPackageVersion(org, packageFlag)
			if err != nil {
				return err
			}
			if !confirm(cmd, fmt.Sprintf("Are you sure you want to release package version %s? You can't undo this action. Release package (y/n)?", packageFlag)) {
				return newError("promote-deny", "Promote operation denied")
			}
			if _, err := org.Force.PatchREST("/tooling/sobjects/Package2Version/"+text(record.values["Id"]), `{"IsReleased":true}`); err != nil {
				converted := apiError(err)
				if e, ok := converted.(*Error); ok && e.Name == "DUPLICATE_VALUE" && strings.Contains(e.Message, "previously released") {
					e.Message = "You already promoted a package version with this major.minor.patch version number. For a given major.minor.patch number, you can promote only one version."
					e.Actions = []string{"Create a new package version with a different --version-number, then promote the package version.\nsf package:version:create -p <name> -n <versionnum> -k <key>\nsf package:version:promote -p 05ixxx"}
				}
				return converted
			}
			subscriberID := text(record.values["SubscriberPackageVersionId"])
			result := newOrderedMap()
			result.Set("id", subscriberID)
			result.Set("success", true)
			result.Set("errors", []interface{}{})
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Successfully promoted the package version, ID: %s, to released.\n", subscriberID)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID (starts with 04t) or alias of the package version to promote.")
	cmd.Flags().BoolP("no-prompt", "n", false, "Don't prompt to confirm setting the package version as released.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

// flagText and flagSet return the value of a string flag and of a boolean
// flag.
func flagText(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func flagSet(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func newPackageVersionCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a package version in the Dev Hub org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionCreateCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			installationKey, _ := cmd.Flags().GetString("installation-key")
			if bypass, _ := cmd.Flags().GetBool("installation-key-bypass"); (installationKey != "") == bypass {
				return exactlyOneError("installation-key", "installation-key-bypass")
			}
			postInstallURL, _ := cmd.Flags().GetString("post-install-url")
			if postInstallURL != "" {
				if parsed, err := url.Parse(postInstallURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
					return newError("MalformedUrlError", "The %s value \"%s\" from the command line or sfdx-project.json is not in the correct format for a URL. It must be a valid URL in the format \"http://salesforce.com\". More information: https://nodejs.org/api/url.html#url_url_strings_and_url_objects", "postInstallUrl", postInstallURL)
				}
			}
			packageFlag, _ := cmd.Flags().GetString("package")
			path, _ := cmd.Flags().GetString("path")
			versionNumber, _ := cmd.Flags().GetString("version-number")
			versionName, _ := cmd.Flags().GetString("version-name")
			project, err := loadProject()
			if err != nil {
				return err
			}
			// Missing values come from the package directory entry in
			// sfdx-project.json.
			data, _ := os.ReadFile(filepath.Join(project.Root, "sfdx-project.json"))
			var config struct {
				PackageDirectories []struct {
					Path          string `json:"path"`
					Package       string `json:"package"`
					VersionName   string `json:"versionName"`
					VersionNumber string `json:"versionNumber"`
				} `json:"packageDirectories"`
			}
			json.Unmarshal(data, &config)
			for _, directory := range config.PackageDirectories {
				matches := (packageFlag != "" && directory.Package == packageFlag) || (path != "" && filepath.Clean(directory.Path) == filepath.Clean(path))
				if !matches {
					continue
				}
				if packageFlag == "" {
					packageFlag = directory.Package
				}
				if path == "" {
					path = directory.Path
				}
				if versionNumber == "" {
					versionNumber = directory.VersionNumber
				}
				if versionName == "" {
					versionName = directory.VersionName
				}
			}
			if packageFlag == "" || path == "" {
				return newError("PackageOrPathRequiredError", "Either --package or --path is required and must name a package directory in sfdx-project.json.")
			}
			if versionNumber == "" {
				return missingFlagError("version-number")
			}
			org, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			roots, err := metadataRoots(filepath.Join(project.Root, path))
			if err != nil || len(roots) == 0 {
				return newError("ErrorPathNotFound", "The directory [%s] doesn’t exist in the current directory.", path)
			}
			description, _ := cmd.Flags().GetString("version-description")
			if description == "" {
				if entry, _ := packageDirectoryEntry(project.Root, packageFlag, path); entry != nil {
					description = text(entry.values["versionDescription"])
				}
			}
			tag, _ := cmd.Flags().GetString("tag")
			skipValidation, _ := cmd.Flags().GetBool("skip-validation")
			asyncValidation, _ := cmd.Flags().GetBool("async-validation")
			codeCoverage, _ := cmd.Flags().GetBool("code-coverage")
			if flagText(cmd, "path") != "" && flagText(cmd, "package") != "" {
				warn(cmd, "Starting in v59.0 or later, specifying both the --package and --path flag will no longer be supported. Only one is required.")
			}
			if skipValidation {
				warn(cmd, "Skipping validation suppresses errors that usually surface during package version creation. Instead, these errors surface at a later stage, such as installation or post-installation. If you encounter errors that are difficult to debug, retry package version creation without the --skip-validation flag.")
			}
			packageRecordID := packageID(packageFlag)
			directory, aliases := packageDirectoryEntry(project.Root, packageFlag, path)
			if directory == nil {
				directory = newOrderedMap()
			}
			for _, key := range []string{"unpackagedMetadata", "seedMetadata"} {
				if directory.values[key] != nil {
					return newError("UnsupportedPackageDirectoryKeyError", "The %s key of the package directory in sfdx-project.json is not supported.", key)
				}
			}
			packages, err := toolingRecords(org, fmt.Sprintf("SELECT Id, ContainerOptions FROM Package2 WHERE Id = '%s'", strings.ReplaceAll(packageRecordID, "'", `\'`)))
			if err != nil {
				return err
			}
			ancestor := ""
			if len(packages) > 0 && text(packages[0].values["ContainerOptions"]) == "Unlocked" {
				if flagText(cmd, "post-install-script") != "" || flagText(cmd, "uninstall-script") != "" {
					return newError("ErrorScriptsNotApplicableToUnlockedPackageError", "We can’t create the package version. This parameter is available only for second-generation managed packages. Create the package version without the postinstallscript or uninstallscript parameters.")
				}
				if directory.values["ancestorId"] != nil || directory.values["ancestorVersion"] != nil {
					return newError("ErrorAncestorNotApplicableToUnlockedPackageError", "Can’t create package version. Specifying an ancestor is available only for second-generation managed packages. Remove the ancestorId or ancestorVersion from your sfdx-project.json file, and then create the package version again.")
				}
			} else if len(packages) > 0 {
				if ancestor, err = packageVersionAncestor(org, directory, aliases, packageRecordID, versionNumber, flagSet(cmd, "skip-ancestor-check")); err != nil {
					return err
				}
			}
			dependencies, err := packageVersionDependencies(org, directory, aliases, flagText(cmd, "branch"))
			if err != nil {
				return err
			}
			definitionFile := flagText(cmd, "definition-file")
			if definitionFile == "" {
				if fromDirectory := text(directory.values["definitionFile"]); fromDirectory != "" {
					definitionFile = filepath.Join(project.Root, fromDirectory)
				}
			}
			extraDescriptor, extraFiles, err := packageVersionDescriptor(directory, definitionFile, codeCoverage)
			if err != nil {
				return err
			}
			request := lib.PackageVersionRequest{
				KeepEmptyDescription:   true,
				SourceApiVersion:       project.SourceApiVersion,
				ExtraDescriptor:        extraDescriptor,
				ExtraFiles:             extraFiles,
				AncestorId:             ancestor,
				Dependencies:           dependencies,
				TransitiveDependencies: directory.values["calculateTransitiveDependencies"] == true,
				Path:                   roots[0],
				PackageId:              packageRecordID,
				VersionNumber:          versionNumber,
				VersionName:            versionName,
				VersionDescription:     description,
				Tag:                    tag,
				SkipValidation:         skipValidation,
				AsyncValidation:        asyncValidation,
				CodeCoverage:           codeCoverage,
				Branch:                 flagText(cmd, "branch"),
				InstallationKey:        installationKey,
				Language:               flagText(cmd, "language"),
				GeneratePackageZip:     flagSet(cmd, "generate-pkg-zip"),
				ReleaseNotesURL:        flagText(cmd, "releasenotes-url"),
				PostInstallURL:         postInstallURL,
				PostInstallScript:      flagText(cmd, "post-install-script"),
			}
			if request.VersionName == "" {
				// As in sf, the name defaults to the number without .NEXT.
				request.VersionName = strings.TrimSuffix(versionNumber, ".NEXT")
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			frequency := 30 * time.Second
			if minutes > 0 && skipValidation {
				frequency = 5 * time.Second
			}
			// sf prints this warning after the result.
			recordTypes := func() {}
			if _, found, _ := lib.GetSFConfig("org-capitalize-record-types"); !found {
				message := "Record types defined in the scratch org definition file will stop being capitalized by default in a future release.\nSet the `org-capitalize-record-types` config var to `true` to enforce capitalization."
				if jsonRequested(cmd) {
					warnJSON(cmd, message)
				} else {
					recordTypes = func() { warn(cmd, message) }
				}
			}
			defer recordTypes()
			return waitForPackageVersion(cmd, org, project.Root, request, time.Duration(minutes)*time.Minute, frequency, flagSet(cmd, "verbose"))
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID (starts with 0Ho) or alias of the package to create a version of.")
	cmd.Flags().StringP("path", "d", "", "Path to the directory that contains the contents of the package.")
	cmd.Flags().StringP("installation-key", "k", "", "Installation key for key-protected package.")
	cmd.Flags().BoolP("installation-key-bypass", "x", false, "Bypass the installation key requirement.")
	cmd.Flags().BoolP("code-coverage", "c", false, "Calculate and store the code coverage percentage by running the packaged Apex tests included in this package version.")
	cmd.Flags().StringP("definition-file", "f", "", "Path to a definition file similar to scratch org definition file that contains the list of features and org preferences that the metadata of the package version depends on.")
	cmd.Flags().StringP("branch", "b", "", "Name of the branch in your source control system that the package version is based on.")
	cmd.Flags().StringP("tag", "t", "", "Package version’s tag.")
	cmd.Flags().StringP("version-name", "a", "", "Name of the package version to be created; overrides the sfdx-project.json value.")
	cmd.Flags().StringP("version-number", "n", "", "Version number of the package version to be created; overrides the sfdx-project.json value.")
	cmd.Flags().StringP("version-description", "e", "", "Description of the package version to be created; overrides the sfdx-project.json value.")
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for the package version to be created.")
	cmd.Flags().Bool("skip-validation", false, "Skip validation during package version creation; you can’t promote unvalidated package versions.")
	cmd.Flags().Bool("async-validation", false, "Return a new package version before completing package validations.")
	cmd.Flags().Bool("skip-ancestor-check", false, "Overrides ancestry requirements, which allows you to specify a package ancestor that isn’t the highest released package version.")
	cmd.Flags().String("post-install-script", "", "Name of the post-install script; applies to managed packages only.")
	cmd.Flags().String("post-install-url", "", "Post-install instructions URL.")
	cmd.Flags().String("releasenotes-url", "", "Release notes URL.")
	cmd.Flags().String("uninstall-script", "", "Uninstall script name; applies to managed packages only.")
	cmd.Flags().String("language", "", "Language for the package.")
	cmd.Flags().Bool("generate-pkg-zip", false, "Generate package zip.")
	cmd.Flags().Bool("verbose", false, "Display verbose command output.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}
