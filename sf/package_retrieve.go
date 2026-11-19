package sf

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// package version retrieve follows the sf plugin source and was not
// compared with sf: it needs a package version whose developer package zip
// can be downloaded.

// effectivelyEmpty reports whether a directory holds no files other than
// .eslintrc.json, at any depth.
func effectivelyEmpty(directory string) bool {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if !effectivelyEmpty(filepath.Join(directory, entry.Name())) {
				return false
			}
		} else if entry.Name() != ".eslintrc.json" {
			return false
		}
	}
	return true
}

// unzipFiles returns the files of a zip archive by name.
func unzipFiles(archive []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		content, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			return nil, err
		}
		files[entry.Name] = data
	}
	return files, nil
}

// recordRetrievedPackage adds the package directory of a retrieved package
// version and the alias of the version to sfdx-project.json, as sf does.
func recordRetrievedPackage(hub *targetOrg, root string, version *orderedMap, subscriberID, folder string, dependencies []string) error {
	packageRecordID := text(version.values["Package2Id"])
	packages, err := toolingRecords(hub, fmt.Sprintf("SELECT Id, Name, Description, NamespacePrefix, ContainerOptions FROM Package2 WHERE Id = '%s'", packageRecordID))
	if err != nil || len(packages) == 0 {
		return err
	}
	pkg := packages[0]
	file := filepath.Join(root, "sfdx-project.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return err
	}
	project, ok := parsed.(*orderedMap)
	if !ok {
		return fmt.Errorf("sfdx-project.json is not a JSON object")
	}
	directories, _ := project.values["packageDirectories"].([]interface{})
	entry := orderedFields("versionName", "ver 0.1", "versionNumber", "0.1.0.NEXT")
	index, hasDefault := -1, false
	for i, item := range directories {
		directory, ok := item.(*orderedMap)
		if !ok {
			continue
		}
		if boolValue(directory.values["default"]) {
			hasDefault = true
		}
		if index < 0 && text(directory.values["path"]) == folder {
			index = i
		}
	}
	if index >= 0 {
		existing := directories[index].(*orderedMap)
		if _, packaged := existing.Get("package"); !packaged {
			for _, key := range existing.keys {
				entry.Set(key, existing.values[key])
			}
		}
	}
	if _, ok := entry.Get("path"); !ok {
		entry.Set("path", folder)
		entry.Set("default", !hasDefault)
	}
	entry.Set("package", pkg.values["Name"])
	entry.Set("versionDescription", pkg.values["Description"])
	entry.Set("versionNumber", "<set version number>")
	entry.Set("versionName", "<set version name>")
	entry.Set("ancestorVersion", "<set ancestor version>")
	required := make([]*orderedMap, 0, len(dependencies))
	for _, dependency := range dependencies {
		required = append(required, orderedFields("package", dependency))
	}
	entry.Set("dependencies", required)
	if index >= 0 {
		directories[index] = entry
	} else {
		directories = append(directories, entry)
	}
	project.Set("packageDirectories", directories)

	aliases, ok := project.values["packageAliases"].(*orderedMap)
	if !ok {
		aliases = newOrderedMap()
		project.Set("packageAliases", aliases)
	}
	name := text(pkg.values["Name"])
	for _, alias := range aliases.keys {
		if aliases.values[alias] == packageRecordID {
			name = alias
			break
		}
	}
	aliases.Set(fmt.Sprintf("%s@%s.%s.%s-%s-main", name, displayValue(version.values["MajorVersion"]), displayValue(version.values["MinorVersion"]),
		displayValue(version.values["PatchVersion"]), displayValue(version.values["BuildNumber"])), subscriberID)
	if text(pkg.values["ContainerOptions"]) == "Managed" && text(project.values["namespace"]) == "" {
		project.Set("namespace", pkg.values["NamespacePrefix"])
	}
	var buffer strings.Builder
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(project); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(buffer.String()), 0o644)
}

func newPackageVersionRetrieveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "retrieve",
		Short:       "Retrieve package metadata for a specified package version. Package metadata can be retrieved for only second-generation managed package versions or unlocked packages.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionRetrieveCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			folder, _ := cmd.Flags().GetString("output-dir")
			if filepath.IsAbs(folder) {
				return newError("SourcesDownloadDirectoryMustBeRelativeError", "Can't retrieve package version metadata. The specified directory must be relative to your Salesforce DX project directory, and not an absolute path.")
			}
			destination := filepath.Join(project.Root, folder)
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			if !effectivelyEmpty(destination) {
				return newError("SourcesDownloadDirectoryNotEmptyError", "Can't retrieve package version metadata. The specified directory isn't empty. Empty the directory, or create a new one and try again.")
			}
			noPermission := newError("DeveloperUsePkgZipFieldUnavailableError", "Can't retrieve package metadata. To use this feature, you must first assign yourself the DownloadPackageVersionZips user permission. Then retry retrieving your package metadata.")
			subscriberID := packageID(packageFlag)
			versions, err := toolingRecords(hub, fmt.Sprintf("SELECT Id, Package2Id, SubscriberPackageVersionId, MajorVersion, MinorVersion, PatchVersion, BuildNumber, ConvertedFromVersionId, DeveloperUsePkgZip FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", subscriberID))
			if err != nil {
				message := err.Error()
				switch {
				case strings.Contains(message, "No such column 'DeveloperUsePkgZip' on entity 'Package2Version'"):
					return noPermission
				case strings.Contains(message, "sObject type 'Package2Version' is not supported."):
					return newError("PackagingNotEnabledOnOrgError", "Can't retrieve package metadata. The org you specified doesn't have the required second-generation packaging permission enabled. Enable this permission on your Dev Hub org, and try again.")
				}
				return err
			}
			if len(versions) == 0 || text(versions[0].values["DeveloperUsePkgZip"]) == "" {
				return noPermission
			}
			version := versions[0]
			archive, err := hub.Force.GetAbsoluteBytes(text(version.values["DeveloperUsePkgZip"]))
			if err != nil {
				return apiError(err)
			}
			if len(archive) == 0 {
				if version.values["ConvertedFromVersionId"] != nil {
					return newError("DownloadDeveloperPackageZipHasNoDataConverted2GPError", "Can't retrieve package metadata. The developer package zip for this converted 2GP package version is unretrievable. To resolve, retry conversion to produce a new converted package version. Then retry retrieving your package metadata.")
				}
				return newError("DownloadDeveloperPackageZipHasNoDataNative2GPError", "Can't retrieve package metadata. The developer package zip for this native 2GP package version is unretrievable. To resolve, create a new package version with the --generate-pkg-zip flag. Then retry retrieving your package metadata.")
			}
			contents, err := unzipFiles(archive)
			if err != nil {
				return err
			}
			var dependencies []string
			if data, ok := contents["dependency-ids.json"]; ok {
				var listed struct {
					IDs []string `json:"ids"`
				}
				if json.Unmarshal(data, &listed) == nil {
					dependencies = listed.IDs
				}
			}
			// A 2GP package has its package.zip inside the downloaded zip.
			if inner, ok := contents["package.zip"]; ok {
				if contents, err = unzipFiles(inner); err != nil {
					return err
				}
			}
			files := lib.ForceMetadataFiles{}
			prefix := ""
			if _, atRoot := contents["package.xml"]; !atRoot {
				for name := range contents {
					if strings.HasSuffix(name, "/package.xml") && strings.Count(name, "/") == 1 {
						prefix = strings.TrimSuffix(name, "package.xml")
					}
				}
			}
			for name, data := range contents {
				if strings.HasPrefix(name, prefix) {
					files[strings.TrimPrefix(name, prefix)] = data
				}
			}
			query := wildcardQuery
			if manifestData, ok := files["package.xml"]; ok {
				staging, err := os.MkdirTemp("", "sf-package-retrieve")
				if err != nil {
					return err
				}
				defer os.RemoveAll(staging)
				manifestPath := filepath.Join(staging, "package.xml")
				if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
					return err
				}
				if manifest, err := readManifest(manifestPath); err == nil {
					query = manifest.query()
				}
			}
			written, err := writeSourceFormat(filepath.Join(destination, "main", "default"), files, query)
			if err != nil {
				return err
			}
			if os.Getenv("SF_PROJECT_AUTOUPDATE_DISABLE_FOR_PACKAGE_VERSION_RETRIEVE") != "true" {
				// sf logs a failure to update the file and goes on.
				_ = recordRetrievedPackage(hub, project.Root, version, subscriberID, folder, dependencies)
			}
			results := make([]*orderedMap, 0, len(written))
			rows := make([][]string, 0, len(written))
			for _, item := range written {
				results = append(results, orderedFields("fullName", item.FullName, "type", item.Type, "filePath", item.FilePath))
				rows = append(rows, []string{item.FullName, item.Type, item.FilePath})
			}
			return emit(cmd, results, func(w io.Writer) {
				printTable(w, "", []string{"FULL NAME", "TYPE", "PROJECT PATH"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "Subscriber package version ID (starts with 04t).")
	cmd.Flags().StringP("output-dir", "d", "force-app", "Path within your Salesforce DX project directory in which to download the metadata. This directory must be empty.")
	addTargetDevHubFlag(cmd)
	return cmd
}
