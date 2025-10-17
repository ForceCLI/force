package command

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/ForceCLI/force/error"
	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	packageInstallCmd.Flags().BoolP("activate", "A", false, "keep the isActive state of any Remote Site Settings (RSS) and Content Security Policies (CSP) in package")
	packageInstallCmd.Flags().StringP("password", "p", "", "password for package")
	packageInstallCmd.Flags().StringP("package-version-id", "i", "", "Package version ID (04t) to install via Tooling API")

	packageUninstallCmd.Flags().StringP("package-version-id", "i", "", "Subscriber Package Version ID (04t) to uninstall (required)")
	packageUninstallCmd.MarkFlagRequired("package-version-id")

	packageCreateCmd.Flags().StringP("name", "n", "", "Package name (required)")
	packageCreateCmd.Flags().StringP("type", "t", "", "Package type: Managed or Unlocked (required)")
	packageCreateCmd.Flags().StringP("namespace", "s", "", "Package namespace (required)")
	packageCreateCmd.Flags().StringP("description", "d", "", "Package description (optional)")
	packageCreateCmd.MarkFlagRequired("name")
	packageCreateCmd.MarkFlagRequired("type")
	packageCreateCmd.MarkFlagRequired("namespace")

	packageVersionCreateCmd.Flags().StringP("package-id", "i", "", "Package ID (required if --namespace not provided)")
	packageVersionCreateCmd.Flags().StringP("namespace", "", "", "Package namespace (alternative to --package-id)")
	packageVersionCreateCmd.Flags().StringP("version-number", "n", "", "Version number (required, e.g., 1.0.0.0)")
	packageVersionCreateCmd.Flags().StringP("version-name", "m", "", "Version name (optional, defaults to version-number)")
	packageVersionCreateCmd.Flags().StringP("version-description", "d", "", "Version description (optional, defaults to version-number)")
	packageVersionCreateCmd.Flags().StringP("ancestor-id", "", "", "Ancestor version ID (optional)")
	packageVersionCreateCmd.Flags().StringP("tag", "", "", "Tag to set on the Package2VersionCreateRequest")
	packageVersionCreateCmd.Flags().BoolP("skip-validation", "s", false, "Skip validation")
	packageVersionCreateCmd.Flags().BoolP("async-validation", "y", false, "Async validation")
	packageVersionCreateCmd.Flags().BoolP("code-coverage", "c", true, "Calculate code coverage")
	packageVersionCreateCmd.MarkFlagRequired("version-number")

	packageVersionReleaseCmd.Flags().StringP("version-id", "v", "", "Package Version ID (required)")
	packageVersionReleaseCmd.MarkFlagRequired("version-id")

	packageVersionListCmd.Flags().StringP("package-id", "i", "", "Package ID (optional, filter by package)")
	packageVersionListCmd.Flags().StringP("namespace", "", "", "Package namespace (alternative to --package-id)")
	packageVersionListCmd.Flags().BoolP("released", "r", false, "Show only released versions")
	packageVersionListCmd.Flags().BoolP("verbose", "v", false, "Show detailed information")
	packageVersionListCmd.MarkFlagsMutuallyExclusive("package-id", "namespace")

	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageListCmd)
	packageCmd.AddCommand(packageInstalledCmd)
	packageVersionCmd.AddCommand(packageVersionCreateCmd)
	packageVersionCmd.AddCommand(packageVersionReleaseCmd)
	packageVersionCmd.AddCommand(packageVersionListCmd)
	packageCmd.AddCommand(packageInstallCmd)
	packageCmd.AddCommand(packageUninstallCmd)
	packageCmd.AddCommand(packageVersionCmd)
	RootCmd.AddCommand(packageCmd)
}

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Manage installed packages",
}

var packageVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage package versions",
}

var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new 2GP package",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		packageType, _ := cmd.Flags().GetString("type")
		namespace, _ := cmd.Flags().GetString("namespace")
		description, _ := cmd.Flags().GetString("description")
		runCreatePackage(name, packageType, namespace, description)
	},
}

var packageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all packages in the org",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runListPackages()
	},
}

var packageInstalledCmd = &cobra.Command{
	Use:   "installed",
	Short: "List installed packages in the org",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runListInstalledPackages()
	},
}

var packageInstallCmd = &cobra.Command{
	Use:   "install [flags] [<namespace> <version>]",
	Short: "Install packages",
	Args:  cobra.RangeArgs(0, 3),
	Run: func(cmd *cobra.Command, args []string) {
		activateRSS, _ := cmd.Flags().GetBool("activate")
		password, _ := cmd.Flags().GetString("password")
		packageVersionId, _ := cmd.Flags().GetString("package-version-id")

		if packageVersionId != "" {
			// Install using package version ID (04t)
			if len(args) > 0 {
				ErrorAndExit("Cannot specify namespace/version when using --package-version-id")
			}
			runInstallPackageById(packageVersionId, password, activateRSS)
		} else {
			// Traditional installation using namespace and version
			if len(args) < 2 {
				ErrorAndExit("Must provide <namespace> and <version> arguments when not using --package-version-id")
			}
			packageNamespace := args[0]
			version := args[1]
			if len(args) > 2 {
				fmt.Println("Warning: Deprecated use of [password] argument.  Use --password flag.")
				password = args[2]
			}
			runInstallPackage(packageNamespace, version, password, activateRSS)
		}
	},
}

var packageUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall a 2GP package",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		packageVersionId, _ := cmd.Flags().GetString("package-version-id")
		runUninstallPackage(packageVersionId)
	},
}

var packageVersionCreateCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new package version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		packageId, _ := cmd.Flags().GetString("package-id")
		namespace, _ := cmd.Flags().GetString("namespace")
		versionNumber, _ := cmd.Flags().GetString("version-number")
		versionName, _ := cmd.Flags().GetString("version-name")
		versionDescription, _ := cmd.Flags().GetString("version-description")
		ancestorId, _ := cmd.Flags().GetString("ancestor-id")
		tag, _ := cmd.Flags().GetString("tag")
		skipValidation, _ := cmd.Flags().GetBool("skip-validation")
		asyncValidation, _ := cmd.Flags().GetBool("async-validation")
		codeCoverage, _ := cmd.Flags().GetBool("code-coverage")

		runCreatePackageVersion(path, packageId, namespace, versionNumber, versionName,
			versionDescription, ancestorId, tag, skipValidation, asyncValidation, codeCoverage)
	},
}

var packageVersionReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a package version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		versionId, _ := cmd.Flags().GetString("version-id")
		runReleasePackageVersion(versionId)
	},
}

var packageVersionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List package versions",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		packageId, _ := cmd.Flags().GetString("package-id")
		namespace, _ := cmd.Flags().GetString("namespace")
		releasedOnly, _ := cmd.Flags().GetBool("released")
		verbose, _ := cmd.Flags().GetBool("verbose")

		runListPackageVersions(packageId, namespace, releasedOnly, verbose)
	},
}

func runCreatePackage(name string, packageType string, namespace string, description string) {
	if name == "" {
		ErrorAndExit("Package name is required")
	}
	if packageType == "" {
		ErrorAndExit("Package type is required")
	}
	if namespace == "" {
		ErrorAndExit("Package namespace is required")
	}
	if packageType != "Managed" && packageType != "Unlocked" {
		ErrorAndExit("Package type must be either 'Managed' or 'Unlocked'")
	}

	attrs := map[string]string{
		"Name":             name,
		"ContainerOptions": packageType,
		"NamespacePrefix":  namespace,
	}

	if description != "" {
		attrs["Description"] = description
	}

	result, err := force.CreateToolingRecord("Package2", attrs)
	if err != nil {
		ErrorAndExit("Failed to create package: " + err.Error())
	}

	fmt.Printf("Package created successfully: %s\n", result.Id)
}

func runListPackages() {
	query := "SELECT Id, Name, NamespacePrefix FROM Package2 ORDER BY Name"

	result, err := force.Query(query, func(options *lib.QueryOptions) {
		options.IsTooling = true
	})
	if err != nil {
		ErrorAndExit("Failed to query packages: " + err.Error())
	}

	if len(result.Records) == 0 {
		fmt.Println("No packages found")
		return
	}

	fmt.Printf("%-18s %-40s %-20s\n", "ID", "Name", "Namespace Prefix")
	fmt.Println(strings.Repeat("-", 80))

	for _, record := range result.Records {
		id := ""
		if recordId, ok := record["Id"].(string); ok {
			id = recordId
		}

		name := ""
		if recordName, ok := record["Name"].(string); ok {
			name = recordName
		}
		if len(name) > 40 {
			name = name[:37] + "..."
		}

		namespace := ""
		if recordNamespace, ok := record["NamespacePrefix"]; ok && recordNamespace != nil {
			namespace = recordNamespace.(string)
		}

		fmt.Printf("%-18s %-40s %-20s\n", id, name, namespace)
	}
}

func runListInstalledPackages() {
	// Query InstalledSubscriberPackage to get installed packages
	query := "SELECT Id, SubscriberPackage.Name, SubscriberPackage.NamespacePrefix, " +
		"SubscriberPackageVersion.Id, SubscriberPackageVersion.Name, " +
		"SubscriberPackageVersion.MajorVersion, SubscriberPackageVersion.MinorVersion, " +
		"SubscriberPackageVersion.PatchVersion, SubscriberPackageVersion.BuildNumber " +
		"FROM InstalledSubscriberPackage " +
		"ORDER BY SubscriberPackage.Name"

	result, err := force.Query(query, func(options *lib.QueryOptions) {
		options.IsTooling = true
	})
	if err != nil {
		ErrorAndExit("Failed to query installed packages: " + err.Error())
	}

	if len(result.Records) == 0 {
		fmt.Println("No installed packages found")
		return
	}

	fmt.Printf("%-18s %-30s %-15s %-18s %-20s %-10s\n", "ID", "Package Name", "Namespace", "Version ID", "Version Name", "Version")
	fmt.Println(strings.Repeat("-", 120))

	for _, record := range result.Records {
		id := ""
		if recordId, ok := record["Id"].(string); ok {
			id = recordId
		}

		packageName := ""
		namespace := ""
		if subscriberPackage, ok := record["SubscriberPackage"].(map[string]interface{}); ok {
			if name, ok := subscriberPackage["Name"].(string); ok {
				packageName = name
				if len(packageName) > 30 {
					packageName = packageName[:27] + "..."
				}
			}
			if ns, ok := subscriberPackage["NamespacePrefix"]; ok && ns != nil {
				namespace = ns.(string)
			}
		}

		versionId := ""
		versionName := ""
		versionNumber := ""
		if subscriberPackageVersion, ok := record["SubscriberPackageVersion"].(map[string]interface{}); ok {
			if vId, ok := subscriberPackageVersion["Id"].(string); ok {
				versionId = vId
			}
			if vName, ok := subscriberPackageVersion["Name"].(string); ok {
				versionName = vName
				if len(versionName) > 20 {
					versionName = versionName[:17] + "..."
				}
			}
			// Construct version number from individual components
			major := subscriberPackageVersion["MajorVersion"]
			minor := subscriberPackageVersion["MinorVersion"]
			patch := subscriberPackageVersion["PatchVersion"]
			build := subscriberPackageVersion["BuildNumber"]
			if major != nil && minor != nil && patch != nil && build != nil {
				versionNumber = fmt.Sprintf("%v.%v.%v.%v", major, minor, patch, build)
			}
		}

		fmt.Printf("%-18s %-30s %-15s %-18s %-20s %-10s\n",
			id, packageName, namespace, versionId, versionName, versionNumber)
	}
}

func runInstallPackage(packageNamespace string, version string, password string, activateRSS bool) {
	if err := force.Metadata.InstallPackageWithRSS(packageNamespace, version, password, activateRSS); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Package installed")
}

func runInstallPackageById(packageVersionId string, password string, activateRSS bool) {
	// Validate that the ID starts with 04t (SubscriberPackageVersion prefix)
	if !strings.HasPrefix(packageVersionId, "04t") {
		ErrorAndExit("Invalid package version ID. Must be a Subscriber Package Version ID (04t)")
	}

	// Create the PackageInstallRequest using Tooling API
	attrs := map[string]string{
		"SubscriberPackageVersionKey": packageVersionId,
		"EnableRss":                   fmt.Sprintf("%t", activateRSS),
		"NameConflictResolution":      "Block",
		"SecurityType":                "None",
	}

	if password != "" {
		attrs["Password"] = password
	}

	fmt.Printf("Installing package version: %s\n", packageVersionId)

	result, err := force.CreateToolingRecord("PackageInstallRequest", attrs)
	if err != nil {
		ErrorAndExit("Failed to create package install request: " + err.Error())
	}

	requestId := result.Id
	if requestId == "" {
		ErrorAndExit("Failed to get request ID from response")
	}

	fmt.Printf("Package install request submitted: %s\n", requestId)

	// Poll for completion
	pollPackageInstallStatus(requestId)
}

func pollPackageInstallStatus(requestId string) {
	query := fmt.Sprintf("SELECT Id, Status, Errors, CreatedDate FROM PackageInstallRequest WHERE Id = '%s'", requestId)

	for i := 0; i < 240; i++ { // Poll for up to 20 minutes (5 seconds * 240)
		time.Sleep(5 * time.Second)

		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			fmt.Printf("Error querying status: %s\n", err.Error())
			continue
		}

		if len(result.Records) == 0 {
			fmt.Println("No records found")
			continue
		}

		record := result.Records[0]
		status := record["Status"].(string)

		fmt.Printf("Status: %s\n", status)

		if status == "SUCCESS" {
			fmt.Println("Package installed successfully")
			return
		} else if status == "ERROR" {
			var errorMessages []string
			if errors, ok := record["Errors"]; ok && errors != nil {
				// Errors field is typically a JSON array within the response
				if errorsMap, ok := errors.(map[string]interface{}); ok {
					if errorArray, ok := errorsMap["errors"].([]interface{}); ok {
						for _, err := range errorArray {
							if errMap, ok := err.(map[string]interface{}); ok {
								if message, ok := errMap["message"].(string); ok {
									errorMessages = append(errorMessages, message)
								}
							}
						}
					}
				} else if errorsArray, ok := errors.([]interface{}); ok {
					for _, err := range errorsArray {
						if errMap, ok := err.(map[string]interface{}); ok {
							if message, ok := errMap["message"].(string); ok {
								errorMessages = append(errorMessages, message)
							}
						}
					}
				}
			}

			if len(errorMessages) > 0 {
				ErrorAndExit("Package installation failed with errors:\n" + strings.Join(errorMessages, "\n"))
			} else {
				ErrorAndExit("Package installation failed with status: ERROR")
			}
		} else if status == "IN_PROGRESS" {
			// Continue polling
		} else {
			// Handle other statuses if any
			fmt.Printf("Unexpected status: %s. Continuing to poll...\n", status)
		}
	}

	ErrorAndExit("Package installation timed out after 20 minutes")
}

func runUninstallPackage(packageVersionId string) {
	// Validate that the ID starts with 04t (SubscriberPackageVersion prefix)
	if !strings.HasPrefix(packageVersionId, "04t") {
		ErrorAndExit("Invalid package version ID. Must be a Subscriber Package Version ID (04t)")
	}

	// Query InstalledSubscriberPackage to find the installed package ID
	query := fmt.Sprintf("SELECT Id, SubscriberPackage.Name, SubscriberPackageVersion.Name "+
		"FROM InstalledSubscriberPackage "+
		"WHERE SubscriberPackageVersionId = '%s'", packageVersionId)

	result, err := force.Query(query, func(options *lib.QueryOptions) {
		options.IsTooling = true
	})
	if err != nil {
		ErrorAndExit("Failed to query installed package: " + err.Error())
	}

	if len(result.Records) == 0 {
		ErrorAndExit(fmt.Sprintf("Package version %s is not installed in this org", packageVersionId))
	}

	record := result.Records[0]

	packageName := ""
	versionName := ""
	if subscriberPackage, ok := record["SubscriberPackage"].(map[string]interface{}); ok {
		if name, ok := subscriberPackage["Name"].(string); ok {
			packageName = name
		}
	}
	if subscriberPackageVersion, ok := record["SubscriberPackageVersion"].(map[string]interface{}); ok {
		if name, ok := subscriberPackageVersion["Name"].(string); ok {
			versionName = name
		}
	}

	fmt.Printf("Uninstalling package: %s (%s)\n", packageName, versionName)
	fmt.Printf("Package Version ID: %s\n", packageVersionId)

	// Create the SubscriberPackageVersionUninstallRequest using Tooling API
	attrs := map[string]string{
		"SubscriberPackageVersionId": packageVersionId,
	}

	requestResult, err := force.CreateToolingRecord("SubscriberPackageVersionUninstallRequest", attrs)
	if err != nil {
		ErrorAndExit("Failed to create package uninstall request: " + err.Error())
	}

	requestId := requestResult.Id
	if requestId == "" {
		ErrorAndExit("Failed to get request ID from response")
	}

	fmt.Printf("Package uninstall request submitted: %s\n", requestId)

	// Poll for completion
	pollPackageUninstallStatus(requestId)
}

func pollPackageUninstallStatus(requestId string) {
	query := fmt.Sprintf("SELECT Id, Status FROM SubscriberPackageVersionUninstallRequest WHERE Id = '%s'", requestId)

	for i := 0; i < 240; i++ { // Poll for up to 20 minutes (5 seconds * 240)
		time.Sleep(5 * time.Second)

		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			fmt.Printf("Error querying status: %s\n", err.Error())
			continue
		}

		if len(result.Records) == 0 {
			fmt.Println("No records found")
			continue
		}

		record := result.Records[0]
		status := ""
		if statusValue, ok := record["Status"].(string); ok {
			status = statusValue
		}

		fmt.Printf("Status: %s\n", status)

		if status == "Success" {
			fmt.Println("Package uninstalled successfully")
			return
		} else if status == "Error" {
			ErrorAndExit("Package uninstall failed with status: Error")
		} else if status == "InProgress" {
			// Continue polling
		} else {
			// Handle other statuses if any
			fmt.Printf("Status: %s, continuing to poll...\n", status)
		}
	}

	ErrorAndExit("Package uninstall timed out after 20 minutes")
}

func runCreatePackageVersion(path string, packageId string, namespace string, versionNumber string,
	versionName string, versionDescription string, ancestorId string, tag string,
	skipValidation bool, asyncValidation bool, codeCoverage bool) {

	// Use version-number as default for version-name and version-description if not provided
	if versionName == "" {
		versionName = versionNumber
	}
	if versionDescription == "" {
		versionDescription = versionNumber
	}

	// Validate that either package-id or namespace is provided
	if packageId == "" && namespace == "" {
		ErrorAndExit("Either --package-id or --namespace must be provided")
	}
	if packageId != "" && namespace != "" {
		ErrorAndExit("Cannot specify both --package-id and --namespace")
	}

	// If namespace is provided, query for the package ID
	if namespace != "" {
		query := fmt.Sprintf("SELECT Id, Name FROM Package2 WHERE NamespacePrefix = '%s'", namespace)
		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			ErrorAndExit("Failed to query package by namespace: " + err.Error())
		}
		if len(result.Records) == 0 {
			ErrorAndExit(fmt.Sprintf("No package found with namespace: %s", namespace))
		}
		if len(result.Records) > 1 {
			ErrorAndExit(fmt.Sprintf("Multiple packages found with namespace: %s", namespace))
		}

		if id, ok := result.Records[0]["Id"].(string); ok {
			packageId = id
			if name, ok := result.Records[0]["Name"].(string); ok {
				fmt.Printf("Found package '%s' with ID: %s\n", name, packageId)
			}
		} else {
			ErrorAndExit("Failed to get package ID from query result")
		}
	}

	// If ancestor-id is not provided, query for the last released version
	if ancestorId == "" {
		query := fmt.Sprintf("SELECT Id FROM Package2Version WHERE Package2Id = '%s' AND IsReleased = true AND PatchVersion = 0 ORDER BY MajorVersion DESC, MinorVersion DESC, PatchVersion DESC, BuildNumber DESC LIMIT 1", packageId)
		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err == nil && len(result.Records) > 0 {
			if id, ok := result.Records[0]["Id"].(string); ok {
				ancestorId = id
				fmt.Printf("Using ancestor version: %s\n", ancestorId)
			}
		}
	}

	// Create package2-descriptor.json
	descriptor := map[string]interface{}{
		"versionName":        versionName,
		"versionNumber":      versionNumber,
		"path":               "salesforce",
		"versionDescription": versionDescription,
		"id":                 packageId,
		"ancestorId":         ancestorId,
	}

	descriptorJson, err := json.Marshal(descriptor)
	if err != nil {
		ErrorAndExit("Failed to create descriptor JSON: " + err.Error())
	}

	// Create package.zip from the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		ErrorAndExit("Failed to get absolute path: " + err.Error())
	}

	// Validate that the path is a directory
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		ErrorAndExit("Failed to access path: " + err.Error())
	}
	if !fileInfo.IsDir() {
		ErrorAndExit("Path must be a directory, not a file: " + path)
	}

	pb := lib.NewPushBuilder()
	pb.Root = absPath

	// Add all subdirectories and files from the path
	files, err := filepath.Glob(filepath.Join(absPath, "*"))
	if err != nil {
		ErrorAndExit("Failed to list directory contents: " + err.Error())
	}

	for _, file := range files {
		err = pb.Add(file)
		if err != nil {
			fmt.Printf("Warning: Could not add %s: %s\n", file, err.Error())
			continue
		}
	}

	// ForceMetadataFiles() adds package.xml automatically
	packageFiles := pb.ForceMetadataFiles()
	packageZipBuffer := new(bytes.Buffer)
	packageZipWriter := zip.NewWriter(packageZipBuffer)

	// Set a proper timestamp (current time)
	modTime := time.Now()

	for filePath, fileData := range packageFiles {
		// Create a zip header with proper settings
		header := &zip.FileHeader{
			Name:           filePath,
			Method:         zip.Deflate,
			Modified:       modTime,
			CreatorVersion: 0x14, // Version 2.0 made by (FAT filesystem)
			ReaderVersion:  0x0A, // Version 1.0 needed to extract
			ExternalAttrs:  0,    // FAT attributes
		}

		w, err := packageZipWriter.CreateHeader(header)
		if err != nil {
			ErrorAndExit("Failed to create zip entry: " + err.Error())
		}
		_, err = w.Write(fileData)
		if err != nil {
			ErrorAndExit("Failed to write zip entry: " + err.Error())
		}
	}
	err = packageZipWriter.Close()
	if err != nil {
		ErrorAndExit("Failed to close package zip: " + err.Error())
	}

	// Create the final package2.zip containing package.zip and package2-descriptor.json
	finalZipBuffer := new(bytes.Buffer)
	finalZipWriter := zip.NewWriter(finalZipBuffer)

	// Add package.zip with proper header
	packageHeader := &zip.FileHeader{
		Name:           "package.zip",
		Method:         zip.Deflate,
		Modified:       modTime,
		CreatorVersion: 0x14, // Version 2.0 made by (FAT filesystem)
		ReaderVersion:  0x0A, // Version 1.0 needed to extract
		ExternalAttrs:  0,    // FAT attributes
	}

	packageEntry, err := finalZipWriter.CreateHeader(packageHeader)
	if err != nil {
		ErrorAndExit("Failed to create package.zip entry: " + err.Error())
	}
	_, err = packageEntry.Write(packageZipBuffer.Bytes())
	if err != nil {
		ErrorAndExit("Failed to write package.zip: " + err.Error())
	}

	// Add package2-descriptor.json with proper header
	descriptorHeader := &zip.FileHeader{
		Name:           "package2-descriptor.json",
		Method:         zip.Deflate,
		Modified:       modTime,
		CreatorVersion: 0x14, // Version 2.0 made by (FAT filesystem)
		ReaderVersion:  0x0A, // Version 1.0 needed to extract
		ExternalAttrs:  0,    // FAT attributes
	}

	descriptorEntry, err := finalZipWriter.CreateHeader(descriptorHeader)
	if err != nil {
		ErrorAndExit("Failed to create descriptor entry: " + err.Error())
	}
	_, err = descriptorEntry.Write(descriptorJson)
	if err != nil {
		ErrorAndExit("Failed to write descriptor: " + err.Error())
	}

	err = finalZipWriter.Close()
	if err != nil {
		ErrorAndExit("Failed to close final zip: " + err.Error())
	}

	// Create multipart form data
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Add VersionInfo file field (first part)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="VersionInfo"; filename="package-version-info.zip"`)
	h.Set("Content-Type", "application/zip")
	part, err := writer.CreatePart(h)
	if err != nil {
		ErrorAndExit("Failed to create form file: " + err.Error())
	}
	_, err = io.Copy(part, bytes.NewReader(finalZipBuffer.Bytes()))
	if err != nil {
		ErrorAndExit("Failed to copy zip data: " + err.Error())
	}

	// Add Package2VersionCreateRequest field (second part)
	request := map[string]interface{}{
		"Package2Id":            packageId,
		"CalculateCodeCoverage": codeCoverage,
		"SkipValidation":        skipValidation,
		"AsyncValidation":       asyncValidation,
	}
	if tag != "" {
		request["Tag"] = tag
	}
	requestJson, _ := json.Marshal(request)

	h = make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="Package2VersionCreateRequest"`)
	h.Set("Content-Type", "application/json")
	part, err = writer.CreatePart(h)
	if err != nil {
		ErrorAndExit("Failed to create request part: " + err.Error())
	}
	_, err = part.Write(requestJson)
	if err != nil {
		ErrorAndExit("Failed to write request data: " + err.Error())
	}

	err = writer.Close()
	if err != nil {
		ErrorAndExit("Failed to close multipart writer: " + err.Error())
	}

	// Post to /tooling/sobjects/Package2VersionCreateRequest
	result, err := force.CreateToolingRecordMultipart("Package2VersionCreateRequest", body.Bytes(), writer.FormDataContentType())
	if err != nil {
		ErrorAndExit("Failed to upload package version: " + err.Error())
	}

	requestId := result.Id
	if requestId == "" {
		ErrorAndExit("Failed to get request ID from response")
	}

	fmt.Printf("Package version creation request submitted: %s\n", requestId)

	// Poll for completion
	packageVersionId := pollPackageVersionStatus(requestId)
	if packageVersionId != "" {
		fmt.Printf("Package version created successfully: %s\n", packageVersionId)
	}
}

func pollPackageVersionStatus(requestId string) string {
	query := fmt.Sprintf("SELECT Id, Status, Package2VersionId FROM Package2VersionCreateRequest WHERE Id = '%s'", requestId)

	for i := 0; i < 120; i++ { // Poll for up to 10 minutes
		time.Sleep(5 * time.Second)

		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			fmt.Printf("Error querying status: %s\n", err.Error())
			continue
		}

		if len(result.Records) == 0 {
			fmt.Println("No records found")
			continue
		}

		record := result.Records[0]
		status := record["Status"].(string)

		fmt.Printf("Status: %s\n", status)

		if status == "Success" {
			if versionId, ok := record["Package2VersionId"].(string); ok {
				return versionId
			}
		} else if status == "Error" {
			errorQuery := fmt.Sprintf("SELECT Message FROM Package2VersionCreateRequestError WHERE ParentRequest.Id = '%s'", requestId)
			errorResult, err := force.Query(errorQuery, func(options *lib.QueryOptions) {
				options.IsTooling = true
			})
			if err == nil && len(errorResult.Records) > 0 {
				var errorMessages []string
				for _, errorRecord := range errorResult.Records {
					if message, ok := errorRecord["Message"].(string); ok {
						errorMessages = append(errorMessages, message)
					}
				}
				if len(errorMessages) > 0 {
					ErrorAndExit("Package version creation failed with errors:\n" + strings.Join(errorMessages, "\n"))
				}
			}
			ErrorAndExit("Package version creation failed with status: Error")
		}
	}

	ErrorAndExit("Package version creation timed out")
	return ""
}

func runReleasePackageVersion(versionId string) {
	// Update the Package2Version record to set IsReleased = true using Tooling API
	updateData := map[string]string{
		"IsReleased": "true",
	}

	err := force.UpdateToolingRecord("Package2Version", versionId, updateData)
	if err != nil {
		ErrorAndExit("Failed to release package version: " + err.Error())
	}

	fmt.Printf("Package version released successfully: %s\n", versionId)
}

func runListPackageVersions(packageId string, namespace string, releasedOnly bool, verbose bool) {
	if namespace != "" {
		query := fmt.Sprintf("SELECT Id, Name FROM Package2 WHERE NamespacePrefix = '%s'", namespace)
		result, err := force.Query(query, func(options *lib.QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			ErrorAndExit("Failed to query package by namespace: " + err.Error())
		}
		if len(result.Records) == 0 {
			ErrorAndExit(fmt.Sprintf("No package found with namespace: %s", namespace))
		}
		if len(result.Records) > 1 {
			ErrorAndExit(fmt.Sprintf("Multiple packages found with namespace: %s", namespace))
		}

		if id, ok := result.Records[0]["Id"].(string); ok {
			packageId = id
			if name, ok := result.Records[0]["Name"].(string); ok {
				fmt.Printf("Found package '%s' with ID: %s\n", name, packageId)
			}
		} else {
			ErrorAndExit("Failed to get package ID from query result")
		}
	}

	// Build the SOQL query
	query := "SELECT Id, MajorVersion, MinorVersion, PatchVersion, BuildNumber, Name, Description, IsReleased, CreatedDate, Package2Id, Package2.Name, SubscriberPackageVersionId, AncestorId"
	query += " FROM Package2Version"

	var whereClauses []string
	if packageId != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("Package2Id = '%s'", packageId))
	}
	if releasedOnly {
		whereClauses = append(whereClauses, "IsReleased = true")
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " ORDER BY Package2.Name, MajorVersion DESC, MinorVersion DESC, PatchVersion DESC, BuildNumber DESC"

	result, err := force.Query(query, func(options *lib.QueryOptions) {
		options.IsTooling = true
	})
	if err != nil {
		ErrorAndExit("Failed to query package versions: " + err.Error())
	}

	if len(result.Records) == 0 {
		fmt.Println("No package versions found")
		return
	}

	if verbose {
		// Detailed output
		for _, record := range result.Records {
			fmt.Printf("ID: %s\n", record["Id"].(string))
			if p2Id, ok := record["Package2Id"]; ok && p2Id != nil {
				fmt.Printf("Package2 ID: %s\n", p2Id.(string))
			}
			if packageName, ok := record["Package2"].(map[string]interface{}); ok {
				if name, exists := packageName["Name"]; exists {
					fmt.Printf("Package Name: %s\n", name.(string))
				}
			}
			// Construct version number from individual fields
			versionNumber := fmt.Sprintf("%v.%v.%v.%v",
				record["MajorVersion"],
				record["MinorVersion"],
				record["PatchVersion"],
				record["BuildNumber"])
			fmt.Printf("Version: %s\n", versionNumber)
			fmt.Printf("Name: %s\n", record["Name"].(string))
			if desc, ok := record["Description"]; ok && desc != nil {
				fmt.Printf("Description: %s\n", desc.(string))
			}
			fmt.Printf("Released: %t\n", record["IsReleased"].(bool))
			fmt.Printf("Created: %s\n", record["CreatedDate"].(string))

			// Always show Subscriber Package Version ID (may be empty)
			subscriberPkgVerId := ""
			if spvId, ok := record["SubscriberPackageVersionId"]; ok && spvId != nil {
				subscriberPkgVerId = spvId.(string)
			}
			fmt.Printf("Subscriber Package Version ID: %s\n", subscriberPkgVerId)

			// Always show Ancestor ID (may be empty)
			ancestorId := ""
			if aId, ok := record["AncestorId"]; ok && aId != nil {
				ancestorId = aId.(string)
			}
			fmt.Printf("Ancestor ID: %s\n", ancestorId)
			fmt.Println("---")
		}
	} else {
		// Simple table output
		fmt.Printf("%-18s %-18s %-18s %-18s %-25s %-13s %-20s %-8s\n", "ID", "Package2Id", "SubscriberPkgVerId", "AncestorId", "Package", "Version", "Name", "Released")
		fmt.Println(strings.Repeat("-", 146))
		for _, record := range result.Records {
			packageName := ""
			if pkg, ok := record["Package2"].(map[string]interface{}); ok {
				if name, exists := pkg["Name"]; exists {
					packageName = name.(string)
				}
			}
			if len(packageName) > 25 {
				packageName = packageName[:22] + "..."
			}

			versionName := record["Name"].(string)
			if len(versionName) > 20 {
				versionName = versionName[:17] + "..."
			}

			// Construct version number from individual fields
			versionNumber := fmt.Sprintf("%v.%v.%v.%v",
				record["MajorVersion"],
				record["MinorVersion"],
				record["PatchVersion"],
				record["BuildNumber"])

			subscriberPkgVerId := ""
			if spvId, ok := record["SubscriberPackageVersionId"]; ok && spvId != nil {
				subscriberPkgVerId = spvId.(string)
			}

			ancestorId := ""
			if aId, ok := record["AncestorId"]; ok && aId != nil {
				ancestorId = aId.(string)
			}

			package2Id := ""
			if p2Id, ok := record["Package2Id"]; ok && p2Id != nil {
				package2Id = p2Id.(string)
			}

			fmt.Printf("%-18s %-18s %-18s %-18s %-25s %-13s %-20s %-8t\n",
				record["Id"].(string),
				package2Id,
				subscriberPkgVerId,
				ancestorId,
				packageName,
				versionNumber,
				versionName,
				record["IsReleased"].(bool))
		}
	}
}
