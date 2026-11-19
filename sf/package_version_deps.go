package sf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// packageDirectoryEntry returns the entry of packageDirectories in
// sfdx-project.json for a package alias or a path, and the packageAliases.
func packageDirectoryEntry(root, packageFlag, path string) (*orderedMap, *orderedMap) {
	aliases := newOrderedMap()
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		return nil, aliases
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return nil, aliases
	}
	project, ok := parsed.(*orderedMap)
	if !ok {
		return nil, aliases
	}
	if found, ok := project.values["packageAliases"].(*orderedMap); ok {
		aliases = found
	}
	directories, _ := project.values["packageDirectories"].([]interface{})
	for _, item := range directories {
		directory, ok := item.(*orderedMap)
		if !ok {
			continue
		}
		if (packageFlag != "" && text(directory.values["package"]) == packageFlag) ||
			(path != "" && filepath.Clean(text(directory.values["path"])) == filepath.Clean(path)) {
			return directory, aliases
		}
	}
	return nil, aliases
}

func aliasValue(aliases *orderedMap, name string) string {
	if id := text(aliases.values[name]); id != "" {
		return id
	}
	return name
}

// packageVersionDependencies resolves the dependencies of a package
// directory to subscriber package version (04t) Ids, as sf does.  A
// dependency names a 04t Id or alias, or a package with a version number
// whose build may be LATEST or RELEASED.
func packageVersionDependencies(org *targetOrg, directory, aliases *orderedMap, branch string) ([]string, error) {
	var ids []string
	dependencies, _ := directory.values["dependencies"].([]interface{})
	for _, item := range dependencies {
		dependency, ok := item.(*orderedMap)
		if !ok {
			continue
		}
		pair := func() error {
			encoded, _ := json.Marshal(dependency)
			return newError("ErrorDependencyPairError", "Dependency must specify either a subscriberPackageVersionId or both packageId and versionNumber: %s", encoded)
		}
		if id := text(dependency.values["subscriberPackageVersionId"]); id != "" {
			ids = append(ids, id)
			continue
		}
		name, legacyID := text(dependency.values["package"]), text(dependency.values["packageId"])
		if name != "" && legacyID != "" {
			return nil, newError("ErrorPackageAndPackageIdCollisionError", "You can’t have both \"package\" and \"packageId\" (deprecated) defined as dependencies in sfdx-project.json.")
		}
		if legacyID != "" {
			name = legacyID
		}
		if name == "" {
			return nil, newError("ErrorPackageOrPackageIdMissingError", "You must provide either \"package\" or \"packageId\" (deprecated) defined as dependencies in sfdx-project.json.")
		}
		id := aliasValue(aliases, name)
		if strings.HasPrefix(id, "04t") {
			ids = append(ids, id)
			continue
		}
		versionNumber := text(dependency.values["versionNumber"])
		if versionNumber == "" {
			return nil, pair()
		}
		parts := strings.Split(versionNumber, ".")
		if len(parts) != 4 {
			return nil, newError("ErrorInvalidVersionNumberError", "The version number %s is not in the format major.minor.patch.build.", versionNumber)
		}
		if found, err := toolingRecords(org, fmt.Sprintf("SELECT Id FROM Package2 WHERE Id = '%s'", id)); err != nil {
			return nil, err
		} else if len(found) != 1 {
			return nil, newError("ErrorNoIdInHubError", "No package ID was found in Dev Hub for package ID: %s.", id)
		}
		dependencyBranch := branch
		if own, ok := dependency.values["branch"].(string); ok {
			dependencyBranch = own
		}
		branchText := "null"
		if dependencyBranch != "" {
			branchText = "'" + dependencyBranch + "'"
		}
		build := parts[3]
		if build == "LATEST" || build == "RELEASED" {
			condition := "AND Branch = " + branchText + " "
			if build == "RELEASED" {
				condition = " AND IsReleased = true"
			}
			records, err := toolingRecords(org, fmt.Sprintf("SELECT MAX(BuildNumber) FROM Package2Version WHERE Package2Id = '%s' AND IsDeprecated != true AND MajorVersion = %s AND MinorVersion = %s AND PatchVersion = %s %s", id, parts[0], parts[1], parts[2], condition))
			if err != nil {
				return nil, err
			}
			if len(records) == 0 || records[0].values["expr0"] == nil {
				if build == "RELEASED" {
					return nil, newError("NoReleaseVersionFoundError", "No released version was found in Dev Hub for package id %s and version number %s.", id, versionNumber)
				}
				return nil, newError("NoReleaseVersionFoundForBranchError", "No version number was found in Dev Hub for package id %s and branch %s and version number %s.", id, dependencyBranch, versionNumber)
			}
			build = displayValue(records[0].values["expr0"])
		}
		condition := "AND Branch = " + branchText
		if parts[3] == "RELEASED" {
			condition = "AND IsReleased = true"
		}
		records, err := toolingRecords(org, fmt.Sprintf("SELECT SubscriberPackageVersionId FROM Package2Version WHERE Package2Id = '%s' AND MajorVersion = %s AND MinorVersion = %s AND PatchVersion = %s AND BuildNumber = %s %s", id, parts[0], parts[1], parts[2], build, condition))
		if err != nil {
			return nil, err
		}
		if len(records) != 1 {
			return nil, newError("VersionNumberNotFoundInDevHubError", "No version number was found in Dev Hub for package id %s and branch %s and version number %s that resolved to build number %s.", id, branchText, versionNumber, build)
		}
		ids = append(ids, text(records[0].values["SubscriberPackageVersionId"]))
	}
	return ids, nil
}

var ancestorVersionNumber = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(\.[^.]*)?$`)

// packageVersionAncestor returns the Package2Version (05i) Id of the
// ancestor of a new version of a managed package, from the ancestorId and
// ancestorVersion of the package directory, as sf does.  Unless the check is
// skipped, the ancestor of a version that is not a patch must be the highest
// released version.
func packageVersionAncestor(org *targetOrg, directory, aliases *orderedMap, packageRecordID, versionNumber string, skipCheck bool) (string, error) {
	ancestorID, ancestorVersion := text(directory.values["ancestorId"]), text(directory.values["ancestorVersion"])
	highest := ancestorID == "HIGHEST" || ancestorVersion == "HIGHEST"
	none := ancestorID == "NONE" || ancestorVersion == "NONE"
	if (highest || none) && ancestorID != "" && ancestorVersion != "" && ancestorID != ancestorVersion {
		return "", newError("ErrorAncestorIdVersionHighestOrNoneMismatchError", "Both ancestorId (%s) and ancestorVersion (%s) specified, HIGHEST and/or NONE are used, the values disagree", ancestorID, ancestorVersion)
	}
	if none && skipCheck {
		return "", nil
	}
	parts := strings.Split(versionNumber, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	isPatch := parts[2] != "0"
	var highestReleased *orderedMap
	if isPatch && highest {
		records, err := toolingRecords(org, fmt.Sprintf("SELECT Id FROM Package2Version WHERE Package2Id = '%s' AND IsReleased = True AND IsDeprecated = False AND PatchVersion = 0 AND MajorVersion = %s AND MinorVersion = %s ORDER BY MajorVersion Desc, MinorVersion Desc, PatchVersion Desc, BuildNumber Desc LIMIT 1", packageRecordID, parts[0], parts[1]))
		if err != nil {
			return "", err
		}
		if len(records) != 1 {
			return "", newError("ErrorNoMatchingMajorMinorForPatchError", "No matching major.minor version found for the given patch version (%s.%s.0)", parts[0], parts[1])
		}
		return text(records[0].values["Id"]), nil
	}
	if !isPatch && (highest || !skipCheck) {
		records, err := toolingRecords(org, fmt.Sprintf("SELECT Id, SubscriberPackageVersionId, MajorVersion, MinorVersion, PatchVersion FROM Package2Version WHERE Package2Id = '%s' AND IsReleased = True AND IsDeprecated = False AND PatchVersion = 0 ORDER BY MajorVersion Desc, MinorVersion Desc, PatchVersion Desc, BuildNumber Desc LIMIT 1", packageRecordID))
		if err != nil {
			return "", err
		}
		if len(records) > 0 {
			highestReleased = records[0]
			if highest {
				return text(highestReleased.values["Id"]), nil
			}
		} else if highest {
			return "", newError("ErrorNoMatchingAncestorError", "No matching ancestor found for the given ancestorVersion (%s) in package %s", versionNumber, packageRecordID)
		}
	}
	resolved, specified := "", ancestorID
	if !none && ancestorID != "" {
		resolved = aliasValue(aliases, ancestorID)
		if strings.HasPrefix(resolved, "04t") {
			records, err := toolingRecords(org, fmt.Sprintf("SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", resolved))
			if err != nil {
				return "", err
			}
			if len(records) == 0 {
				return "", newError("ErrorInvalidIdNoMatchingVersionIdError", "The Subscriber Package Version ID %s is invalid, as a corresponding Package Version ID was not found", resolved)
			}
			resolved = text(records[0].values["Id"])
		}
	}
	if !none && ancestorVersion != "" {
		if !ancestorVersionNumber.MatchString(ancestorVersion) {
			return "", newError("Error", "The given ancestorVersion (%s) is not in the correct format", ancestorVersion)
		}
		numbers := strings.Split(ancestorVersion, ".")
		records, err := toolingRecords(org, fmt.Sprintf("SELECT Id, IsReleased FROM Package2Version WHERE Package2Id = '%s' AND MajorVersion = %s AND MinorVersion = %s AND PatchVersion = %s", packageRecordID, numbers[0], numbers[1], numbers[2]))
		if err != nil {
			return "", err
		}
		if len(records) == 0 {
			return "", newError("ErrorNoMatchingAncestorError", "No matching ancestor found for the given ancestorVersion (%s) in package %s", ancestorVersion, packageRecordID)
		}
		released := ""
		for _, record := range records {
			if record.values["IsReleased"] == true {
				released = text(record.values["Id"])
				break
			}
		}
		if released == "" {
			return "", newError("ErrorAncestorNotReleasedError", "The given ancestor version (%s) has not been released", ancestorVersion)
		}
		if ancestorID != "" && resolved != released {
			return "", newError("ErrorAncestorIdVersionMismatchError", "No matching ancestor version (%s) found for the given ancestorId (%s)", ancestorVersion, ancestorID)
		}
		resolved, specified = released, ancestorVersion
	}
	highestNumber := ""
	if highestReleased != nil {
		highestNumber = fmt.Sprintf("%s.%s.%s", displayValue(highestReleased.values["MajorVersion"]), displayValue(highestReleased.values["MinorVersion"]), displayValue(highestReleased.values["PatchVersion"]))
	}
	if none {
		if highestReleased == nil {
			return "", nil
		}
		return "", newError("ErrorAncestorNoneNotAllowedError", "Can’t create package version because you didn’t specify a package ancestor. Set the ancestor version to %s and try creating the package version again. You can also specify --skip-ancestor-check to override the ancestry requirement.", highestNumber)
	}
	if !isPatch && !skipCheck {
		if highestReleased == nil {
			// The first version of the package has no ancestor.
			return "", nil
		}
		if text(highestReleased.values["Id"]) != resolved {
			return "", newError("ErrorAncestorNotHighestError", "Can’t create package version. The ancestor version [%s] you specified isn’t the highest released package version. Set the ancestor version to %s and try creating the package version again. You can also specify --skip-ancestor-check to override the ancestry requirement.", specified, highestNumber)
		}
	}
	return resolved, nil
}

// trimmedList returns the entries of a list given as an array or as a
// comma-separated string.
func trimmedList(value interface{}) []interface{} {
	var entries []interface{}
	switch list := value.(type) {
	case string:
		for _, entry := range strings.Split(list, ",") {
			entries = append(entries, strings.TrimSpace(entry))
		}
	case []interface{}:
		for _, entry := range list {
			entries = append(entries, strings.TrimSpace(text(entry)))
		}
	}
	return entries
}

// packageVersionDescriptor returns the properties of the descriptor of a new
// package version that come from the package directory and from a
// definition file, and the files that go with them: the definition file and
// a zip file of the settings it has.  As in sf, the features, org
// preferences and snapshot come from the definition file when there is one,
// and from the package directory otherwise.
func packageVersionDescriptor(directory *orderedMap, definitionFile string, codeCoverage bool) (map[string]interface{}, map[string][]byte, error) {
	handled := map[string]bool{
		// Used by this program only, as in sf.
		"default": true, "includeProfileUserLicenses": true, "unpackagedMetadata": true, "seedMetadata": true, "branch": true,
		"fullPath": true, "name": true, "scopeProfiles": true,
		// Set from flags and resolved values.
		"path": true, "package": true, "versionName": true, "versionNumber": true, "versionDescription": true, "dependencies": true,
		"ancestorId": true, "ancestorVersion": true, "definitionFile": true, "language": true, "apexTestAccess": true, "packageMetadataAccess": true,
	}
	descriptor := map[string]interface{}{}
	for _, key := range directory.keys {
		if !handled[key] {
			descriptor[key] = directory.values[key]
		}
	}
	if access, ok := directory.values["apexTestAccess"].(*orderedMap); ok && codeCoverage {
		if sets := trimmedList(access.values["permissionSets"]); sets != nil {
			descriptor["permissionSetNames"] = sets
		}
		if licenses := trimmedList(access.values["permissionSetLicenses"]); licenses != nil {
			descriptor["permissionSetLicenseDeveloperNames"] = licenses
		}
	}
	if access, ok := directory.values["packageMetadataAccess"].(*orderedMap); ok {
		if sets := trimmedList(access.values["permissionSets"]); sets != nil {
			descriptor["packageMetadataPermissionSetNames"] = sets
		}
		if licenses := trimmedList(access.values["permissionSetLicenses"]); licenses != nil {
			descriptor["packageMetadataPermissionSetLicenseNames"] = licenses
		}
	}
	if definitionFile == "" {
		return descriptor, nil, nil
	}
	data, err := os.ReadFile(definitionFile)
	if err != nil {
		return nil, nil, newError("Error", "ENOENT: no such file or directory, open '%s'", definitionFile)
	}
	parsed, err := parseOrderedJSON(data)
	definition, ok := parsed.(*orderedMap)
	if err != nil || !ok {
		return nil, nil, newError("SyntaxError", "The definition file %s is not a JSON object.", definitionFile)
	}
	if definition.values["objectSettings"] != nil {
		return nil, nil, newError("UnsupportedDefinitionKeyError", "The objectSettings key of the definition file is not supported.")
	}
	settings, _ := definition.values["settings"].(*orderedMap)
	hasSettings := settings != nil && len(settings.keys) > 0
	if hasSettings && definition.values["orgPreferences"] != nil {
		return nil, nil, newError("SignupDuplicateSettingsSpecifiedError", "You cannot use 'settings' and 'orgPreferences' in your scratch definition file, please specify one or the other.")
	}
	for _, key := range []string{"features", "orgPreferences", "snapshot"} {
		delete(descriptor, key)
	}
	for _, property := range []string{"country", "edition", "language", "features", "orgPreferences", "snapshot", "release", "sourceOrg"} {
		for _, key := range definition.keys {
			if strings.EqualFold(key, property) {
				descriptor[property] = definition.values[key]
				break
			}
		}
	}
	files := map[string][]byte{"scratch-org-definition.json": data}
	if hasSettings {
		archive, err := zipFiles(scratchSettingsFiles(settings), "")
		if err != nil {
			return nil, nil, err
		}
		files["settings.zip"] = archive
	}
	return descriptor, files, nil
}
