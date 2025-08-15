package command

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"

	. "github.com/ForceCLI/force/lib"
)

// DestructivePackage represents a destructiveChangesPost.xml structure
type DestructivePackage struct {
	XMLName xml.Name          `xml:"Package"`
	Xmlns   string            `xml:"xmlns,attr"`
	Types   []DestructiveType `xml:"types"`
	Version string            `xml:"version"`
}

// DestructiveType lists metadata members to delete
type DestructiveType struct {
	Members []string `xml:"members"`
	Name    string   `xml:"name"`
}

// flowQuerier abstracts the Query method needed to fetch flow versions
type flowQuerier interface {
	Query(string, ...func(*QueryOptions)) (ForceQueryResult, error)
}

// hasNonVersionedFlowsInDestructive checks if any destructive changes files contain non-versioned flows
func hasNonVersionedFlowsInDestructive(files ForceMetadataFiles) bool {
	destructiveFiles := []string{
		"destructiveChanges.xml",
		"destructiveChangesPre.xml",
		"destructiveChangesPost.xml",
	}

	for _, fileName := range destructiveFiles {
		if fileContent, exists := files[fileName]; exists {
			var pkg DestructivePackage
			if err := xml.Unmarshal(fileContent, &pkg); err != nil {
				continue
			}

			for _, metaType := range pkg.Types {
				if metaType.Name == "Flow" {
					for _, member := range metaType.Members {
						// Check if this is an unversioned flow (no -N suffix)
						if !regexp.MustCompile(`.+-\d+$`).MatchString(member) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// processDestructiveFlows handles flows in destructiveChanges files.
// For flows without versions, it queries the org for existing versions,
// adds them individually to the destructive changes, and adds a FlowDefinition with version 0.
func processDestructiveFlows(q flowQuerier, files ForceMetadataFiles) (ForceMetadataFiles, error) {
	destructiveFiles := []string{
		"destructiveChanges.xml",
		"destructiveChangesPre.xml",
		"destructiveChangesPost.xml",
	}

	for _, fileName := range destructiveFiles {
		if fileContent, exists := files[fileName]; exists {
			modified, _, err := processDestructiveFile(q, fileContent, files)
			if err != nil {
				return files, fmt.Errorf("processing %s: %w", fileName, err)
			}
			if modified != nil {
				files[fileName] = modified
			}
		}
	}

	return files, nil
}

// processDestructiveFile processes a single destructive changes file
// Returns the modified content (or nil if not modified), whether non-versioned flows were found, and any error
func processDestructiveFile(q flowQuerier, fileContent []byte, files ForceMetadataFiles) ([]byte, bool, error) {
	var pkg DestructivePackage
	if err := xml.Unmarshal(fileContent, &pkg); err != nil {
		return nil, false, fmt.Errorf("parse destructive changes: %w", err)
	}

	modified := false
	foundNonVersioned := false
	flowDefinitionsToAdd := make(map[string]struct{})

	// Track which versioned flows we've already added to avoid duplicates
	addedVersions := make(map[string]struct{})

	for typeIdx, metaType := range pkg.Types {
		if metaType.Name == "Flow" {
			var newMembers []string
			var flowsToRemove []string

			for _, member := range metaType.Members {
				// Check if this is an unversioned flow (no -N suffix)
				if !regexp.MustCompile(`.+-\d+$`).MatchString(member) {
					foundNonVersioned = true
					// Query for existing versions using LIKE for case-insensitive matching
					// Since Salesforce doesn't allow flows with names differing only by case,
					// this is safe and handles mismatched casing in destructiveChanges files
					soql := fmt.Sprintf("SELECT Status, FlowDefinitionView.ApiName, VersionNumber FROM FlowVersionView WHERE FlowDefinitionView.ApiName LIKE '%s'", member)
					qr, err := q.Query(soql)
					if err != nil {
						return nil, false, fmt.Errorf("query Flow versions for %s: %w", member, err)
					}

					if len(qr.Records) > 0 {
						// First pass: Get the actual API name from the org (with correct casing)
						var actualApiName string
						for _, rec := range qr.Records {
							if actualApiName == "" && rec["FlowDefinitionView.ApiName"] != nil {
								actualApiName = rec["FlowDefinitionView.ApiName"].(string)
								break
							}
						}

						// If we got an actual API name, process the versions
						if actualApiName != "" {
							// Second pass: Process versions using the actual API name
							hasActiveVersion := false
							for _, rec := range qr.Records {
								if rec["VersionNumber"] != nil {
									vf := rec["VersionNumber"].(float64)
									v := int(vf)

									// Check if this version is active
									if status, ok := rec["Status"].(string); ok && status == "Active" {
										hasActiveVersion = true
									}

									// Use the actual API name from the org for versioned flows
									versionedName := fmt.Sprintf("%s-%d", actualApiName, v)
									// Only add if we haven't already added this specific version
									if _, exists := addedVersions[versionedName]; !exists {
										newMembers = append(newMembers, versionedName)
										addedVersions[versionedName] = struct{}{}
									}
								}
							}

							// Only add FlowDefinition if there's at least one active version
							// Always use the actual API name from org for FlowDefinition
							if hasActiveVersion {
								flowDefinitionsToAdd[actualApiName] = struct{}{}
							}
						}

						// Remove the original member name from destructiveChanges
						flowsToRemove = append(flowsToRemove, member)
						modified = true
					} else {
						// No versions in org - remove from destructive changes
						flowsToRemove = append(flowsToRemove, member)
						modified = true
					}
				} else {
					// Already versioned - keep as is (but check for duplicates)
					if _, exists := addedVersions[member]; !exists {
						newMembers = append(newMembers, member)
						addedVersions[member] = struct{}{}
					}
				}
			}

			// Remove unversioned flows and replace with versioned ones
			if modified {
				// Keep only members not marked for removal
				var finalMembers []string
				for _, m := range metaType.Members {
					remove := false
					for _, toRemove := range flowsToRemove {
						if m == toRemove {
							remove = true
							break
						}
					}
					if !remove {
						finalMembers = append(finalMembers, m)
					}
				}
				// Add new versioned members
				finalMembers = append(finalMembers, newMembers...)

				// Sort for consistency
				sort.Strings(finalMembers)
				pkg.Types[typeIdx].Members = finalMembers
			}
		}
	}

	// Add FlowDefinition entries with version 0 to package.xml
	if len(flowDefinitionsToAdd) > 0 {
		addFlowDefinitionsToPackage(files, flowDefinitionsToAdd)
	}

	if modified {
		out, err := xml.MarshalIndent(pkg, "", "    ")
		if err != nil {
			return nil, false, fmt.Errorf("marshal destructive changes: %w", err)
		}
		return append([]byte(xml.Header), out...), foundNonVersioned, nil
	}

	return nil, foundNonVersioned, nil
}

// addFlowDefinitionsToPackage adds FlowDefinition entries with version 0 to package.xml
func addFlowDefinitionsToPackage(files ForceMetadataFiles, flowsToAdd map[string]struct{}) {
	if pkgXml, ok := files["package.xml"]; ok {
		type pkgType struct {
			Members []string `xml:"members"`
			Name    string   `xml:"name"`
		}
		type pkgStruct struct {
			XMLName xml.Name  `xml:"Package"`
			Xmlns   string    `xml:"xmlns,attr"`
			Types   []pkgType `xml:"types"`
			Version string    `xml:"version"`
		}
		var pkg pkgStruct
		if err := xml.Unmarshal(pkgXml, &pkg); err == nil {
			// Find or create FlowDefinition type
			flowDefFound := false
			for i, t := range pkg.Types {
				if t.Name == "FlowDefinition" {
					// Add new flows to existing FlowDefinition
					for flow := range flowsToAdd {
						// Check if not already present
						found := false
						for _, m := range t.Members {
							if m == flow {
								found = true
								break
							}
						}
						if !found {
							pkg.Types[i].Members = append(pkg.Types[i].Members, flow)
						}
					}
					flowDefFound = true
					break
				}
			}

			if !flowDefFound && len(flowsToAdd) > 0 {
				// Create new FlowDefinition type
				var members []string
				for flow := range flowsToAdd {
					members = append(members, flow)
				}
				sort.Strings(members)
				pkg.Types = append(pkg.Types, pkgType{
					Name:    "FlowDefinition",
					Members: members,
				})
			}

			// Marshal back
			out, err := xml.MarshalIndent(pkg, "", "    ")
			if err == nil {
				files["package.xml"] = append([]byte(xml.Header), out...)
			}
		}
	}

	// Also create FlowDefinition files with version 0
	for flow := range flowsToAdd {
		flowDefContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
    <activeVersionNumber>0</activeVersionNumber>
</FlowDefinition>`)
		files[fmt.Sprintf("flowDefinitions/%s.flowDefinition-meta.xml", flow)] = []byte(flowDefContent)
	}
}

// handleDestructiveFlows processes any non-versioned flows in destructive changes files,
// expanding them to specific versions. This happens automatically, independent of --smart-flow-version flag.
func handleDestructiveFlows(q flowQuerier, files ForceMetadataFiles) (ForceMetadataFiles, error) {
	// Check if there are any non-versioned flows in destructive changes
	if !hasNonVersionedFlowsInDestructive(files) {
		return files, nil
	}

	fmt.Println("Info: Non-versioned flows detected in destructiveChanges files. Automatically expanding to specific versions.")

	files, err := processDestructiveFlows(q, files)
	if err != nil {
		return files, fmt.Errorf("processing destructive flows: %w", err)
	}

	return files, nil
}

// processSmartFlowVersion auto-assigns version numbers to unversioned flows
// and generates a destructiveChangesPost.xml to remove inactive versions.
func processSmartFlowVersion(q flowQuerier, files ForceMetadataFiles) (ForceMetadataFiles, error) {
	// Identify local unversioned flows: flows/Name.flow
	reFlow := regexp.MustCompile(`^flows/([^/]+)\.flow$`)
	unversioned := map[string]struct{}{}
	for path := range files {
		if m := reFlow.FindStringSubmatch(path); m != nil {
			base := m[1]
			// skip if base already contains a version suffix
			if !regexp.MustCompile(`.+-\d+$`).MatchString(base) {
				unversioned[base] = struct{}{}
			}
		}
	}
	if len(unversioned) == 0 {
		return files, nil
	}
	// Prepare destructive type
	var dest DestructiveType
	dest.Name = "Flow"

	// Track new versioned names for package.xml update
	newNames := map[string]string{}
	for name := range unversioned {
		// Query existing Flow versions via Tooling API using LIKE for case-insensitive matching
		soql := fmt.Sprintf("SELECT Status, FlowDefinitionView.ApiName, VersionNumber FROM FlowVersionView WHERE FlowDefinitionView.ApiName LIKE '%s'", name)
		qr, err := q.Query(soql)
		if err != nil {
			return files, fmt.Errorf("query Flow versions for %s: %w", name, err)
		}

		// Get the actual API name from org if it exists
		var actualApiName string
		if len(qr.Records) > 0 && qr.Records[0]["FlowDefinitionView.ApiName"] != nil {
			actualApiName = qr.Records[0]["FlowDefinitionView.ApiName"].(string)
		}

		existing := map[int]struct{}{}
		statuses := map[int]string{}
		for _, rec := range qr.Records {
			if rec["VersionNumber"] == nil {
				continue
			}
			// VersionNumber comes back as float64
			vf := rec["VersionNumber"].(float64)
			v := int(vf)
			existing[v] = struct{}{}
			if s, ok := rec["Status"].(string); ok {
				statuses[v] = s
			}
		}
		// Compute new version: smallest positive integer not in existing
		newVer := 1
		for {
			if _, ok := existing[newVer]; !ok {
				break
			}
			newVer++
		}
		// Collect inactive versions for deletion
		// Use the actual API name from org if available
		nameForDeletion := name
		if actualApiName != "" {
			nameForDeletion = actualApiName
		}
		for v, s := range statuses {
			if s != "Active" {
				dest.Members = append(dest.Members, fmt.Sprintf("%s-%d", nameForDeletion, v))
			}
		}
		// Record new versioned name and rename local flow files
		member := fmt.Sprintf("%s-%d", name, newVer)
		newNames[name] = member
		// Rename files
		oldFlow := fmt.Sprintf("flows/%s.flow", name)
		oldMeta := fmt.Sprintf("flows/%s.flow-meta.xml", name)
		newFlow := fmt.Sprintf("flows/%s.flow", member)
		newMeta := fmt.Sprintf("flows/%s.flow-meta.xml", member)
		if data, ok := files[oldFlow]; ok {
			files[newFlow] = data
			delete(files, oldFlow)
		}
		// Update package.xml members for Flow entries
		if pkgXml, ok := files["package.xml"]; ok {
			// Unmarshal into struct
			type pkgType struct {
				Members []string `xml:"members"`
				Name    string   `xml:"name"`
			}
			type pkgStruct struct {
				XMLName xml.Name  `xml:"Package"`
				Xmlns   string    `xml:"xmlns,attr"`
				Types   []pkgType `xml:"types"`
				Version string    `xml:"version"`
			}
			var pkg pkgStruct
			if err := xml.Unmarshal(pkgXml, &pkg); err == nil {
				// Replace unversioned member names
				for ti, t := range pkg.Types {
					if t.Name == "Flow" {
						for mi, m := range t.Members {
							if replacement, found := newNames[m]; found {
								pkg.Types[ti].Members[mi] = replacement
							}
						}
					}
				}
				// Marshal back
				out, err := xml.MarshalIndent(pkg, "", "    ")
				if err == nil {
					out = append([]byte(xml.Header), out...)
					files["package.xml"] = out
				}
			}
		}
		if meta, ok := files[oldMeta]; ok {
			// Update <fullName> tag inside meta xml
			reFull := regexp.MustCompile(`<fullName>[^<]+</fullName>`)
			member := fmt.Sprintf("%s-%d", name, newVer)
			updated := reFull.ReplaceAll(meta, []byte(fmt.Sprintf("<fullName>%s</fullName>", member)))
			files[newMeta] = updated
			delete(files, oldMeta)
		}
	}
	// If any inactive versions, generate or merge destructiveChangesPost.xml
	if len(dest.Members) > 0 {
		// sort members for consistency
		sort.Strings(dest.Members)

		var pkg DestructivePackage

		// Check if destructiveChangesPost.xml already exists
		if existing, ok := files["destructiveChangesPost.xml"]; ok {
			// Parse existing destructiveChangesPost.xml
			if err := xml.Unmarshal(existing, &pkg); err != nil {
				return files, fmt.Errorf("parse existing destructiveChangesPost.xml: %w", err)
			}

			// Find existing Flow type or create new one
			flowTypeFound := false
			for i, t := range pkg.Types {
				if t.Name == "Flow" {
					// Merge with existing Flow members, avoiding duplicates
					memberSet := make(map[string]struct{})
					for _, m := range t.Members {
						memberSet[m] = struct{}{}
					}
					for _, m := range dest.Members {
						memberSet[m] = struct{}{}
					}

					// Convert back to slice and sort
					var mergedMembers []string
					for m := range memberSet {
						mergedMembers = append(mergedMembers, m)
					}
					sort.Strings(mergedMembers)
					pkg.Types[i].Members = mergedMembers
					flowTypeFound = true
					break
				}
			}

			// If no Flow type found, add it
			if !flowTypeFound {
				pkg.Types = append(pkg.Types, dest)
			}
		} else {
			// Create new destructiveChangesPost.xml
			pkg = DestructivePackage{
				Xmlns:   "http://soap.sforce.com/2006/04/metadata",
				Types:   []DestructiveType{dest},
				Version: strings.TrimPrefix(ApiVersion(), "v"),
			}
		}

		out, err := xml.MarshalIndent(pkg, "", "    ")
		if err != nil {
			return files, fmt.Errorf("marshal destructiveChangesPost: %w", err)
		}
		out = append([]byte(xml.Header), out...)
		files["destructiveChangesPost.xml"] = out
	}
	return files, nil
}
