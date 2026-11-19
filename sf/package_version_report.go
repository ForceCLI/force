package sf

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const packageVersionReportFields = "Id,Package2Id,SubscriberPackageVersionId,Name,Description,Tag,Branch,AncestorId,ValidationSkipped,MajorVersion,MinorVersion,PatchVersion,BuildNumber,IsReleased,CodeCoverage,HasPassedCodeCoverageCheck,Package2.IsOrgDependent,ReleaseVersion,BuildDurationInSeconds,HasMetadataRemoved,CreatedById,ConvertedFromVersionId,ValidatedAsync,TotalNumberOfMetadataFiles,TotalSizeOfMetadataFiles"

const packageVersionReportVerboseFields = ",SubscriberPackageVersion.Dependencies,CodeCoveragePercentages,EndToEndBuildDurationInSeconds"

// fixedNumber formats a number with a fixed count of decimals, and nil as
// "".
func fixedNumber(value interface{}, decimals int) string {
	if value == nil {
		return ""
	}
	number, err := strconv.ParseFloat(displayValue(value), 64)
	if err != nil {
		return displayValue(value)
	}
	return strconv.FormatFloat(number, 'f', decimals, 64)
}

// packageVersionReport reads a package version and converts the record to
// the result of "package version report": the record of the query, without
// the fields sf leaves out, and with the values sf replaces for display.
func packageVersionReport(org *targetOrg, versionID string, verbose bool) (*orderedMap, error) {
	fields := packageVersionReportFields
	if verbose {
		fields += packageVersionReportVerboseFields
	}
	records, err := toolingRecords(org, fmt.Sprintf("SELECT %s FROM Package2Version WHERE Id = '%s' AND IsDeprecated != true ORDER BY Package2Id, Branch, MajorVersion, MinorVersion, PatchVersion, BuildNumber", fields, versionID))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, newError("TypeError", "Cannot read properties of undefined (reading 'Package2')")
	}
	record := records[0]
	packageType := ""
	if packages, err := toolingRecords(org, fmt.Sprintf("SELECT Id, ContainerOptions FROM Package2 WHERE Id IN ('%s')", text(record.values["Package2Id"]))); err != nil {
		return nil, err
	} else if len(packages) > 0 {
		packageType = text(packages[0].values["ContainerOptions"])
	}
	managed := packageType == "Managed"
	parent, _ := record.values["Package2"].(*orderedMap)
	orgDependent := parent != nil && boolValue(parent.values["IsOrgDependent"])
	skipsCoverage := orgDependent || boolValue(record.values["ValidationSkipped"])

	result := newOrderedMap()
	for _, key := range record.keys {
		switch key {
		case "CodeCoverage", "HasPassedCodeCoverageCheck", "Package2", "HasMetadataRemoved", "DeveloperUsePkgZip":
			continue
		}
		result.Set(key, record.values[key])
	}
	result.Set("Version", fmt.Sprintf("%s.%s.%s.%s", displayValue(record.values["MajorVersion"]), displayValue(record.values["MinorVersion"]),
		displayValue(record.values["PatchVersion"]), displayValue(record.values["BuildNumber"])))
	if packageType != "" && packageType != "Unlocked" {
		result.Set("PackageType", packageType)
	}
	if !managed {
		result.Set("AncestorVersion", "N/A")
		result.Set("AncestorId", "N/A")
	} else {
		var ancestorVersion interface{}
		if record.values["AncestorId"] != nil {
			versions, err := packageVersionNumbers(org, []*orderedMap{record})
			if err != nil {
				return nil, err
			}
			if found, ok := versions[text(record.values["AncestorId"])]; ok && found != "-" {
				ancestorVersion = found
			}
		}
		result.Set("AncestorVersion", ancestorVersion)
	}
	if skipsCoverage {
		result.Set("CodeCoverage", "N/A")
		result.Set("HasPassedCodeCoverageCheck", "N/A")
	} else {
		result.Set("CodeCoverage", record.values["CodeCoverage"])
		if passed := record.values["HasPassedCodeCoverageCheck"]; passed != nil {
			result.Set("HasPassedCodeCoverageCheck", passed)
		}
	}
	dependent := "No"
	if managed {
		dependent = "N/A"
	} else if orgDependent {
		dependent = "Yes"
	}
	result.Set("Package2", orderedFields("IsOrgDependent", dependent))
	removed := "N/A"
	if managed {
		removed = "No"
		if boolValue(record.values["HasMetadataRemoved"]) {
			removed = "Yes"
		}
	}
	result.Set("HasMetadataRemoved", removed)
	if result.values["ConvertedFromVersionId"] == nil {
		result.Set("ConvertedFromVersionId", " ")
	}
	return result, nil
}

// printPackageVersionReport prints the tables of "package version report".
func printPackageVersionReport(w io.Writer, result *orderedMap, verbose bool) {
	coverage := "N/A"
	switch value := result.values["CodeCoverage"].(type) {
	case string:
		coverage = value
	case *orderedMap:
		coverage = fixedNumber(value.values["apexCodeCoveragePercentage"], 2) + "%"
	}
	size := ""
	if bytes, err := strconv.ParseFloat(resultCell(result, "TotalSizeOfMetadataFiles"), 64); err == nil && bytes > 0 {
		size = fmt.Sprintf("%d MB", int(math.Ceil(bytes/(1024*1024))))
	}
	dependencies := " "
	if version, ok := result.values["SubscriberPackageVersion"].(*orderedMap); ok && verbose {
		if detail, ok := version.values["Dependencies"].(*orderedMap); ok {
			var ids []string
			items, _ := detail.values["ids"].([]interface{})
			for _, item := range items {
				if dependency, ok := item.(*orderedMap); ok {
					ids = append(ids, text(dependency.values["subscriberPackageVersionId"]))
				}
			}
			dependencies = strings.Join(ids, ", ")
		}
	}
	parent, _ := result.values["Package2"].(*orderedMap)
	rows := [][]string{
		{"Name", resultCell(result, "Name")},
		{"Subscriber Package Version Id", resultCell(result, "SubscriberPackageVersionId")},
	}
	if verbose {
		rows = append(rows, []string{"Id", resultCell(result, "Id")})
	}
	rows = append(rows, [][]string{
		{"Package Id", resultCell(result, "Package2Id")},
		{"Version", resultCell(result, "Version")},
		{"Description", resultCell(result, "Description")},
		{"Branch", resultCell(result, "Branch")},
		{"Tag", resultCell(result, "Tag")},
		{"Released", resultCell(result, "IsReleased") + " "},
		{"Validation Skipped", resultCell(result, "ValidationSkipped")},
		{"Validated Async", resultCell(result, "ValidatedAsync")},
		{"Ancestor", resultCell(result, "AncestorId")},
		{"Ancestor Version", resultCell(result, "AncestorVersion")},
		{"Code Coverage", coverage},
		{"Code Coverage Met", resultCell(result, "HasPassedCodeCoverageCheck")},
	}...)
	if verbose || strings.TrimSpace(resultCell(result, "ConvertedFromVersionId")) != "" {
		rows = append(rows, []string{"Converted From Version Id", resultCell(result, "ConvertedFromVersionId")})
	}
	rows = append(rows, [][]string{
		{"Org-Dependent Unlocked Package", resultCell(parent, "IsOrgDependent")},
		{"Release Version", fixedNumber(result.values["ReleaseVersion"], 1)},
		{"Build Duration in Seconds", fixedNumber(result.values["BuildDurationInSeconds"], 1)},
		{"Managed Metadata Removed", resultCell(result, "HasMetadataRemoved")},
	}...)
	if verbose {
		rows = append(rows, []string{"Dependencies", dependencies})
	}
	rows = append(rows, [][]string{
		{"Created By", resultCell(result, "CreatedById")},
		{"# MetadataFiles", resultCell(result, "TotalNumberOfMetadataFiles")},
		{"Metadata File Size", size},
	}...)
	var coverageRows [][]string
	if verbose {
		details := "N/A"
		if percentages, ok := result.values["CodeCoveragePercentages"].(*orderedMap); ok {
			if classes, ok := percentages.values["codeCovPercentages"].([]interface{}); ok {
				details = "The code coverage details are too large to display. To request code coverage details for this package version, log a case in the Salesforce Partner Community."
				// sf shows the first 15 classes.
				for i, item := range classes {
					class, ok := item.(*orderedMap)
					if !ok || i >= 15 {
						break
					}
					coverageRows = append(coverageRows, []string{text(class.values["className"]), displayValue(class.values["codeCoveragePercentage"]) + "%"})
				}
				if len(coverageRows) > 0 {
					details = "..."
				}
			}
		}
		rows = append(rows,
			[]string{"End To End Build Duration In Seconds", fixedNumber(result.values["EndToEndBuildDurationInSeconds"], 1)},
			[]string{"Code Coverage Details", details})
	}
	printTable(w, paint(w, style{"\x1b[34m", "\x1b[39m"}, "Package Version"), []string{"Key", "Value"}, rows)
	if len(coverageRows) > 0 {
		printTable(w, "", []string{"Key", "Value"}, coverageRows)
	}
}

func runPackageVersionReport(cmd *cobra.Command, org *targetOrg, versionID string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	result, err := packageVersionReport(org, versionID, verbose)
	if err != nil {
		return err
	}
	return emit(cmd, result, func(w io.Writer) {
		printPackageVersionReport(w, result, verbose)
	})
}
