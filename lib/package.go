package lib

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// packagePollInterval is the wait between status queries while a package
// version is created or a package is installed.
var packagePollInterval = 5 * time.Second

// PackageVersionRequest describes a 2GP package version to create from a
// directory of metadata.
type PackageVersionRequest struct {
	// Path is the directory whose contents (classes/, objects/, ...) make up
	// the package.
	Path string
	// PackageId is the Package2 (0Ho) Id.
	PackageId     string
	VersionNumber string
	// VersionName and VersionDescription default to VersionNumber.
	VersionName        string
	VersionDescription string
	// AncestorId is a Package2Version (05i) or SubscriberPackageVersion (04t)
	// Id, or empty for no ancestor.
	AncestorId string
	// Dependencies are SubscriberPackageVersion (04t) Ids.
	Dependencies    []string
	Tag             string
	SkipValidation  bool
	AsyncValidation bool
	CodeCoverage    bool
}

// PackageVersion identifies a created package version.
type PackageVersion struct {
	// Id is the Package2Version (05i) Id.
	Id string
	// SubscriberPackageVersionId is the 04t Id used to install the version.
	SubscriberPackageVersionId string
}

// CreatePackage creates a 2GP package and returns its Package2 (0Ho) Id.
func (f *Force) CreatePackage(name string, packageType string, namespace string, description string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("Package name is required")
	}
	if namespace == "" {
		return "", fmt.Errorf("Package namespace is required")
	}
	if packageType != "Managed" && packageType != "Unlocked" {
		return "", fmt.Errorf("Package type must be either 'Managed' or 'Unlocked'")
	}
	attrs := map[string]string{
		"Name":             name,
		"ContainerOptions": packageType,
		"NamespacePrefix":  namespace,
	}
	if description != "" {
		attrs["Description"] = description
	}
	result, err := f.CreateToolingRecord("Package2", attrs)
	if err != nil {
		return "", fmt.Errorf("Failed to create package: %w", err)
	}
	if result.Id == "" {
		return "", fmt.Errorf("Failed to get package ID from response")
	}
	return result.Id, nil
}

// CreatePackageVersion builds a package version from req.Path, waits for the
// request to finish, and returns the version's Ids. Progress is written to
// progress, which may be nil.
func (f *Force) CreatePackageVersion(req PackageVersionRequest, progress io.Writer) (PackageVersion, error) {
	if progress == nil {
		progress = io.Discard
	}
	if req.PackageId == "" {
		return PackageVersion{}, fmt.Errorf("Package ID is required")
	}
	if req.VersionNumber == "" {
		return PackageVersion{}, fmt.Errorf("Version number is required")
	}
	if req.VersionName == "" {
		req.VersionName = req.VersionNumber
	}
	if req.VersionDescription == "" {
		req.VersionDescription = req.VersionNumber
	}
	for _, dependency := range req.Dependencies {
		if !strings.HasPrefix(dependency, "04t") {
			return PackageVersion{}, fmt.Errorf("Invalid dependency ID: %s. Must be a Subscriber Package Version ID (04t)", dependency)
		}
	}

	// The descriptor's ancestorId must be a Package2Version (05i) Id.
	ancestorId := req.AncestorId
	if strings.HasPrefix(ancestorId, "04t") {
		id, err := f.queryToolingId(fmt.Sprintf("SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", ancestorId))
		if err != nil {
			return PackageVersion{}, fmt.Errorf("Failed to resolve ancestor version: %w", err)
		}
		if id == "" {
			return PackageVersion{}, fmt.Errorf("No Package2Version found for ancestor: %s", ancestorId)
		}
		fmt.Fprintf(progress, "Resolved ancestor %s to Package2Version %s\n", ancestorId, id)
		ancestorId = id
	}

	descriptor := buildPackageVersionDescriptor(req.VersionName, req.VersionNumber, req.VersionDescription, req.PackageId, ancestorId, req.Dependencies)
	descriptorJson, err := json.Marshal(descriptor)
	if err != nil {
		return PackageVersion{}, fmt.Errorf("Failed to create descriptor JSON: %w", err)
	}
	packageZip, err := buildPackageZip(req.Path, progress)
	if err != nil {
		return PackageVersion{}, err
	}
	body, contentType, err := packageVersionCreateBody(packageZip, descriptorJson, req)
	if err != nil {
		return PackageVersion{}, err
	}

	result, err := f.CreateToolingRecordMultipart("Package2VersionCreateRequest", body, contentType)
	if err != nil {
		return PackageVersion{}, fmt.Errorf("Failed to upload package version: %w", err)
	}
	if result.Id == "" {
		return PackageVersion{}, fmt.Errorf("Failed to get request ID from response")
	}
	fmt.Fprintf(progress, "Package version creation request submitted: %s\n", result.Id)

	versionId, err := f.pollPackageVersionCreateRequest(result.Id, progress)
	if err != nil {
		return PackageVersion{}, err
	}
	subscriberVersionId, err := f.queryToolingField(
		fmt.Sprintf("SELECT SubscriberPackageVersionId FROM Package2Version WHERE Id = '%s'", versionId),
		"SubscriberPackageVersionId")
	if err != nil {
		return PackageVersion{}, fmt.Errorf("Failed to query subscriber package version: %w", err)
	}
	if subscriberVersionId == "" {
		return PackageVersion{}, fmt.Errorf("No SubscriberPackageVersionId for Package2Version %s", versionId)
	}
	return PackageVersion{Id: versionId, SubscriberPackageVersionId: subscriberVersionId}, nil
}

// InstallPackageVersion installs a package version (04t) and waits for the
// install to finish. Progress is written to progress, which may be nil.
func (f *Force) InstallPackageVersion(subscriberPackageVersionId string, password string, activateRSS bool, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	if !strings.HasPrefix(subscriberPackageVersionId, "04t") {
		return fmt.Errorf("Invalid package version ID. Must be a Subscriber Package Version ID (04t)")
	}
	attrs := map[string]string{
		"SubscriberPackageVersionKey": subscriberPackageVersionId,
		"EnableRss":                   fmt.Sprintf("%t", activateRSS),
		"NameConflictResolution":      "Block",
		"SecurityType":                "None",
	}
	if password != "" {
		attrs["Password"] = password
	}
	fmt.Fprintf(progress, "Installing package version: %s\n", subscriberPackageVersionId)
	result, err := f.CreateToolingRecord("PackageInstallRequest", attrs)
	if err != nil {
		return fmt.Errorf("Failed to create package install request: %w", err)
	}
	if result.Id == "" {
		return fmt.Errorf("Failed to get request ID from response")
	}
	fmt.Fprintf(progress, "Package install request submitted: %s\n", result.Id)
	return f.pollPackageInstallRequest(result.Id, progress)
}

func (f *Force) pollPackageVersionCreateRequest(requestId string, progress io.Writer) (string, error) {
	query := fmt.Sprintf("SELECT Id, Status, Package2VersionId FROM Package2VersionCreateRequest WHERE Id = '%s'", requestId)
	for i := 0; i < 360; i++ { // 30 minutes at the default interval
		time.Sleep(packagePollInterval)
		result, err := f.Query(query, func(options *QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			fmt.Fprintf(progress, "Error querying status: %s\n", err.Error())
			continue
		}
		if len(result.Records) == 0 {
			fmt.Fprintln(progress, "No records found")
			continue
		}
		record := result.Records[0]
		status, _ := record["Status"].(string)
		fmt.Fprintf(progress, "Status: %s\n", status)
		switch status {
		case "Success":
			versionId, ok := record["Package2VersionId"].(string)
			if !ok || versionId == "" {
				return "", fmt.Errorf("Package version creation succeeded without a Package2VersionId")
			}
			return versionId, nil
		case "Error":
			errorResult, err := f.Query(
				fmt.Sprintf("SELECT Message FROM Package2VersionCreateRequestError WHERE ParentRequest.Id = '%s'", requestId),
				func(options *QueryOptions) {
					options.IsTooling = true
				})
			var messages []string
			if err == nil {
				for _, errorRecord := range errorResult.Records {
					if message, ok := errorRecord["Message"].(string); ok {
						messages = append(messages, message)
					}
				}
			}
			if len(messages) > 0 {
				return "", fmt.Errorf("Package version creation failed with errors:\n%s", strings.Join(messages, "\n"))
			}
			return "", fmt.Errorf("Package version creation failed with status: Error")
		}
	}
	return "", fmt.Errorf("Package version creation timed out after %s", 360*packagePollInterval)
}

func (f *Force) pollPackageInstallRequest(requestId string, progress io.Writer) error {
	query := fmt.Sprintf("SELECT Id, Status, Errors, CreatedDate FROM PackageInstallRequest WHERE Id = '%s'", requestId)
	for i := 0; i < 240; i++ { // 20 minutes at the default interval
		time.Sleep(packagePollInterval)
		result, err := f.Query(query, func(options *QueryOptions) {
			options.IsTooling = true
		})
		if err != nil {
			fmt.Fprintf(progress, "Error querying status: %s\n", err.Error())
			continue
		}
		if len(result.Records) == 0 {
			fmt.Fprintln(progress, "No records found")
			continue
		}
		record := result.Records[0]
		status, _ := record["Status"].(string)
		fmt.Fprintf(progress, "Status: %s\n", status)
		switch status {
		case "SUCCESS":
			return nil
		case "ERROR":
			messages := packageInstallErrorMessages(record["Errors"])
			if len(messages) > 0 {
				return fmt.Errorf("Package installation failed with errors:\n%s", strings.Join(messages, "\n"))
			}
			return fmt.Errorf("Package installation failed with status: ERROR")
		case "IN_PROGRESS":
		default:
			fmt.Fprintf(progress, "Unexpected status: %s. Continuing to poll...\n", status)
		}
	}
	return fmt.Errorf("Package installation timed out after %s", 240*packagePollInterval)
}

// packageInstallErrorMessages reads the messages from a PackageInstallRequest's
// Errors field, which is either an object holding an "errors" array or the
// array itself.
func packageInstallErrorMessages(errorsValue interface{}) []string {
	var entries []interface{}
	switch v := errorsValue.(type) {
	case map[string]interface{}:
		entries, _ = v["errors"].([]interface{})
	case []interface{}:
		entries = v
	}
	var messages []string
	for _, entry := range entries {
		if errMap, ok := entry.(map[string]interface{}); ok {
			if message, ok := errMap["message"].(string); ok {
				messages = append(messages, message)
			}
		}
	}
	return messages
}

func (f *Force) queryToolingId(query string) (string, error) {
	return f.queryToolingField(query, "Id")
}

// queryToolingField returns field of the first record a tooling query
// returns, or "" when it returns none.
func (f *Force) queryToolingField(query string, field string) (string, error) {
	result, err := f.Query(query, func(options *QueryOptions) {
		options.IsTooling = true
	})
	if err != nil {
		return "", err
	}
	if len(result.Records) == 0 {
		return "", nil
	}
	value, _ := result.Records[0][field].(string)
	return value, nil
}

// buildPackageZip builds the package.zip of a package version from the
// contents of path.
func buildPackageZip(path string, progress io.Writer) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to get absolute path: %w", err)
	}
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to access path: %w", err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("Path must be a directory, not a file: %s", path)
	}

	pb := NewPushBuilder()
	pb.Root = absPath
	files, err := filepath.Glob(filepath.Join(absPath, "*"))
	if err != nil {
		return nil, fmt.Errorf("Failed to list directory contents: %w", err)
	}
	for _, file := range files {
		if err := pb.Add(file); err != nil {
			fmt.Fprintf(progress, "Warning: Could not add %s: %s\n", file, err.Error())
		}
	}

	// ForceMetadataFiles() adds package.xml automatically
	packageFiles := pb.ForceMetadataFiles()

	// The Salesforce CLI always includes an (often empty) Profile type in the
	// manifest so that profile settings are recalculated for the package's
	// components. Add it when no profiles are present.
	if manifest, ok := packageFiles["package.xml"]; ok {
		updated, err := addProfileTypeToManifest(manifest)
		if err != nil {
			return nil, err
		}
		packageFiles["package.xml"] = updated
	}

	buffer := new(bytes.Buffer)
	writer := zip.NewWriter(buffer)
	modTime := time.Now()

	// Collect directory entries so the zip matches the layout the Salesforce
	// CLI produces (each directory is written as its own zero-length entry).
	for _, dir := range collectZipDirectories(packageFiles) {
		header := fatZipHeader(dir, zip.Store, modTime)
		header.SetMode(os.ModeDir | 0755)
		if _, err := writer.CreateHeader(header); err != nil {
			return nil, fmt.Errorf("Failed to create zip directory entry: %w", err)
		}
	}
	for filePath, fileData := range packageFiles {
		w, err := writer.CreateHeader(fatZipHeader(filePath, zip.Deflate, modTime))
		if err != nil {
			return nil, fmt.Errorf("Failed to create zip entry: %w", err)
		}
		if _, err := w.Write(fileData); err != nil {
			return nil, fmt.Errorf("Failed to write zip entry: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("Failed to close package zip: %w", err)
	}
	return buffer.Bytes(), nil
}

func fatZipHeader(name string, method uint16, modTime time.Time) *zip.FileHeader {
	return &zip.FileHeader{
		Name:           name,
		Method:         method,
		Modified:       modTime,
		CreatorVersion: 0x14, // Version 2.0 made by (FAT filesystem)
		ReaderVersion:  0x0A, // Version 1.0 needed to extract
		ExternalAttrs:  0,    // FAT attributes
	}
}

// packageVersionCreateBody builds the multipart body of a
// Package2VersionCreateRequest: a zip holding package.zip and
// package2-descriptor.json, followed by the request fields.
func packageVersionCreateBody(packageZip []byte, descriptorJson []byte, req PackageVersionRequest) ([]byte, string, error) {
	modTime := time.Now()
	finalZipBuffer := new(bytes.Buffer)
	finalZipWriter := zip.NewWriter(finalZipBuffer)
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{"package.zip", packageZip},
		{"package2-descriptor.json", descriptorJson},
	} {
		w, err := finalZipWriter.CreateHeader(fatZipHeader(entry.name, zip.Deflate, modTime))
		if err != nil {
			return nil, "", fmt.Errorf("Failed to create %s entry: %w", entry.name, err)
		}
		if _, err := w.Write(entry.data); err != nil {
			return nil, "", fmt.Errorf("Failed to write %s: %w", entry.name, err)
		}
	}
	if err := finalZipWriter.Close(); err != nil {
		return nil, "", fmt.Errorf("Failed to close final zip: %w", err)
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="VersionInfo"; filename="package-version-info.zip"`)
	h.Set("Content-Type", "application/zip")
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create form file: %w", err)
	}
	if _, err := part.Write(finalZipBuffer.Bytes()); err != nil {
		return nil, "", fmt.Errorf("Failed to copy zip data: %w", err)
	}

	request := map[string]interface{}{
		"Package2Id":                 req.PackageId,
		"CalculateCodeCoverage":      req.CodeCoverage,
		"SkipValidation":             req.SkipValidation,
		"AsyncValidation":            req.AsyncValidation,
		"CalcTransitiveDependencies": false,
		"IsDevUsePkgZipRequested":    false,
	}
	if req.Tag != "" {
		request["Tag"] = req.Tag
	}
	requestJson, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create request JSON: %w", err)
	}
	h = make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="Package2VersionCreateRequest"`)
	h.Set("Content-Type", "application/json")
	part, err = writer.CreatePart(h)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create request part: %w", err)
	}
	if _, err := part.Write(requestJson); err != nil {
		return nil, "", fmt.Errorf("Failed to write request data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("Failed to close multipart writer: %w", err)
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

// addProfileTypeToManifest ensures the package.xml manifest includes a Profile
// type, matching the Salesforce CLI which always adds one (often empty) so that
// profile settings are recalculated for the package's components. The manifest
// is re-marshaled so the Profile <types> element is correctly ordered before
// <version>. If a Profile type is already present the manifest is returned
// unchanged.
func addProfileTypeToManifest(manifest []byte) ([]byte, error) {
	var p Package
	if err := xml.Unmarshal(manifest, &p); err != nil {
		return nil, fmt.Errorf("Failed to parse package.xml: %w", err)
	}
	for _, t := range p.Types {
		if t.Name == "Profile" {
			return manifest, nil
		}
	}
	p.Types = append(p.Types, MetaType{Name: "Profile"})
	byteXml, err := xml.MarshalIndent(p, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal package.xml: %w", err)
	}
	return append([]byte(xml.Header), byteXml...), nil
}

// collectZipDirectories returns the sorted set of directory entries (each with a
// trailing slash) implied by the file paths, so the package zip mirrors the
// layout the Salesforce CLI produces, which writes each directory as its own
// zero-length entry.
func collectZipDirectories(files ForceMetadataFiles) []string {
	dirSet := make(map[string]bool)
	for filePath := range files {
		parts := strings.Split(filePath, "/")
		for i := 1; i < len(parts); i++ {
			dirSet[strings.Join(parts[:i], "/")+"/"] = true
		}
	}
	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func buildPackageVersionDescriptor(versionName string, versionNumber string, versionDescription string, packageId string, ancestorId string, dependencies []string) map[string]interface{} {
	descriptor := map[string]interface{}{
		"versionName":        versionName,
		"versionNumber":      versionNumber,
		"path":               "salesforce",
		"versionDescription": versionDescription,
		"id":                 packageId,
		"ancestorId":         ancestorId,
	}

	if len(dependencies) > 0 {
		descriptorDependencies := make([]map[string]string, 0, len(dependencies))
		for _, dependency := range dependencies {
			descriptorDependencies = append(descriptorDependencies, map[string]string{
				"subscriberPackageVersionId": dependency,
			})
		}
		descriptor["dependencies"] = descriptorDependencies
	}

	return descriptor
}
