package command

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
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

	packageVersionCreateCmd.Flags().StringP("package-id", "i", "", "Package ID (required)")
	packageVersionCreateCmd.Flags().StringP("version-number", "n", "", "Version number (required, e.g., 1.0.0.0)")
	packageVersionCreateCmd.Flags().StringP("version-name", "m", "", "Version name (required)")
	packageVersionCreateCmd.Flags().StringP("version-description", "d", "", "Version description (required)")
	packageVersionCreateCmd.Flags().StringP("ancestor-id", "", "", "Ancestor version ID (optional)")
	packageVersionCreateCmd.Flags().BoolP("skip-validation", "s", false, "Skip validation")
	packageVersionCreateCmd.Flags().BoolP("async-validation", "y", false, "Async validation")
	packageVersionCreateCmd.Flags().BoolP("code-coverage", "c", true, "Calculate code coverage")
	packageVersionCreateCmd.MarkFlagRequired("package-id")
	packageVersionCreateCmd.MarkFlagRequired("version-number")
	packageVersionCreateCmd.MarkFlagRequired("version-name")
	packageVersionCreateCmd.MarkFlagRequired("version-description")

	packageVersionReleaseCmd.Flags().StringP("version-id", "v", "", "Package Version ID (required)")
	packageVersionReleaseCmd.MarkFlagRequired("version-id")

	packageVersionListCmd.Flags().StringP("package-id", "i", "", "Package ID (optional, filter by package)")
	packageVersionListCmd.Flags().BoolP("released", "r", false, "Show only released versions")
	packageVersionListCmd.Flags().BoolP("verbose", "v", false, "Show detailed information")

	packageVersionCmd.AddCommand(packageVersionCreateCmd)
	packageVersionCmd.AddCommand(packageVersionReleaseCmd)
	packageVersionCmd.AddCommand(packageVersionListCmd)
	packageCmd.AddCommand(packageInstallCmd)
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

var packageInstallCmd = &cobra.Command{
	Use:   "install [flags] <namespace> <version>",
	Short: "Installed packages",
	Args:  cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		activateRSS, _ := cmd.Flags().GetBool("activate")
		password, _ := cmd.Flags().GetString("password")
		packageNamespace := args[0]
		version := args[1]
		if len(args) > 2 {
			fmt.Println("Warning: Deprecated use of [password] argument.  Use --password flag.")
			password = args[2]
		}
		runInstallPackage(packageNamespace, version, password, activateRSS)
	},
}

var packageVersionCreateCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new package version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		packageId, _ := cmd.Flags().GetString("package-id")
		versionNumber, _ := cmd.Flags().GetString("version-number")
		versionName, _ := cmd.Flags().GetString("version-name")
		versionDescription, _ := cmd.Flags().GetString("version-description")
		ancestorId, _ := cmd.Flags().GetString("ancestor-id")
		skipValidation, _ := cmd.Flags().GetBool("skip-validation")
		asyncValidation, _ := cmd.Flags().GetBool("async-validation")
		codeCoverage, _ := cmd.Flags().GetBool("code-coverage")

		runCreatePackageVersion(path, packageId, versionNumber, versionName,
			versionDescription, ancestorId, skipValidation, asyncValidation, codeCoverage)
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
		releasedOnly, _ := cmd.Flags().GetBool("released")
		verbose, _ := cmd.Flags().GetBool("verbose")
		runListPackageVersions(packageId, releasedOnly, verbose)
	},
}

func runInstallPackage(packageNamespace string, version string, password string, activateRSS bool) {
	if err := force.Metadata.InstallPackageWithRSS(packageNamespace, version, password, activateRSS); err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Package installed")
}

func runCreatePackageVersion(path string, packageId string, versionNumber string,
	versionName string, versionDescription string, ancestorId string,
	skipValidation bool, asyncValidation bool, codeCoverage bool) {

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
			// Skip files that can't be added (non-metadata files)
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

func runListPackageVersions(packageId string, releasedOnly bool, verbose bool) {
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
