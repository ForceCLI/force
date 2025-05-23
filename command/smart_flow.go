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
		// Query existing Flow versions via Tooling API
		soql := fmt.Sprintf("SELECT Status, FlowDefinitionView.ApiName, VersionNumber FROM FlowVersionView WHERE FlowDefinitionView.ApiName = '%s'", name)
		qr, err := q.Query(soql)
		if err != nil {
			return files, fmt.Errorf("query Flow versions for %s: %w", name, err)
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
		for v, s := range statuses {
			if s != "Active" {
				dest.Members = append(dest.Members, fmt.Sprintf("%s-%d", name, v))
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
