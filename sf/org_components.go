package sf

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/ForceCLI/force/lib"
)

// packageStates are the manageable states of components that belong to a
// package, by the value of --include-packages that keeps them.
var packageStates = map[string][]string{
	"managed":  {"beta", "deleted", "deprecated", "installed", "released"},
	"unlocked": {"deprecatedEditable", "installedEditable"},
}

// listOrgMetadata sends listMetadata queries three to a request and returns
// the components found.  A request that the org refuses is repeated with
// one query at a time, and a refused query finds nothing: sf sends one
// query in each request and ignores those that fail.
func listOrgMetadata(org *targetOrg, queries []lib.ListMetadataQuery) []*orderedMap {
	var found []*orderedMap
	send := func(batch []lib.ListMetadataQuery) bool {
		body, err := org.Force.Metadata.ListMetadataQueries(batch)
		if err != nil {
			return false
		}
		results, err := soapResults(body)
		if err != nil {
			return false
		}
		found = append(found, results...)
		return true
	}
	for start := 0; start < len(queries); start += 3 {
		end := start + 3
		if end > len(queries) {
			end = len(queries)
		}
		if send(queries[start:end]) || end-start == 1 {
			continue
		}
		for _, query := range queries[start:end] {
			send([]lib.ListMetadataQuery{query})
		}
	}
	return found
}

// orgComponents lists the metadata components of an org, as sf does for
// "project generate manifest --from-org": the components of each type sf
// knows, then the contents of the folders found, the child components of
// the types found, and the standard value sets that have values.  typeNames
// limits the types; without it every type is listed.  Each component has
// the properties of the Metadata API FileProperties.
//
// sf asks the org about each of the types in its registry.  This asks the
// org which types it has first, and then about those.
func orgComponents(org *targetOrg, typeNames []string) ([]*orderedMap, error) {
	if len(typeNames) == 0 {
		typeNames = registryTypeNames
	}
	known := map[string]bool{}
	for _, name := range registryTypeNames {
		known[name] = true
	}
	describe, err := org.Force.Metadata.DescribeMetadata()
	if err != nil {
		return nil, apiError(err)
	}
	inOrg := map[string]bool{}
	for _, object := range describe.MetadataObjects {
		inOrg[object.XmlName] = true
		for _, child := range object.ChildXmlNames {
			inOrg[child] = true
		}
	}
	var queries []lib.ListMetadataQuery
	standardValueSets := false
	for _, name := range typeNames {
		switch {
		case !known[name]:
		case name == "StandardValueSet":
			// listMetadata returns nothing for this type.
			standardValueSets = true
		case name == "BotVersion":
		case inOrg[name] || registryFolderContent[name] != "":
			queries = append(queries, lib.ListMetadataQuery{Type: name})
		}
	}
	components := listOrgMetadata(org, queries)

	var folderQueries []lib.ListMetadataQuery
	childTypes := map[string]bool{}
	var childOrder []string
	for _, component := range components {
		typeName := registryType(text(component.values["type"]))
		if content := registryFolderContent[typeName]; content != "" {
			folderQueries = append(folderQueries, lib.ListMetadataQuery{Type: content, Folder: text(component.values["fullName"])})
		}
		for _, child := range registryChildTypes[typeName] {
			if !childTypes[child] {
				childTypes[child] = true
				childOrder = append(childOrder, child)
			}
		}
	}
	components = append(components, listOrgMetadata(org, folderQueries)...)
	var childQueries []lib.ListMetadataQuery
	for _, child := range childOrder {
		childQueries = append(childQueries, lib.ListMetadataQuery{Type: child})
	}
	components = append(components, listOrgMetadata(org, childQueries)...)

	if standardValueSets {
		for _, name := range registryStandardValueSets {
			records, err := toolingRecords(org, fmt.Sprintf("SELECT Id, MasterLabel, Metadata FROM StandardValueSet WHERE MasterLabel = '%s'", name))
			if err != nil || len(records) != 1 {
				continue
			}
			metadata, _ := records[0].values["Metadata"].(*orderedMap)
			if metadata == nil {
				continue
			}
			if values, _ := metadata.values["standardValue"].([]interface{}); len(values) > 0 {
				label := text(records[0].values["MasterLabel"])
				components = append(components, orderedFields("fullName", label, "fileName", "standardValueSets/"+label+".standardValueSet", "type", "StandardValueSet"))
			}
		}
	}
	var valid []*orderedMap
	for _, component := range components {
		if text(component.values["type"]) != "" && text(component.values["fullName"]) != "" {
			valid = append(valid, component)
		}
	}
	return valid, nil
}

// registryType returns the type that sf uses for a type name.
func registryType(name string) string {
	if alias, ok := registryTypeAliases[name]; ok {
		return alias
	}
	return name
}

// manifestMember returns the type and the member name under which a
// component of an org is listed in a manifest, or "" for a component that
// cannot be named in one.
func manifestMember(typeName, fullName string) (string, string) {
	if registryNotAddressable[typeName] {
		return "", ""
	}
	if content := registryFolderContent[typeName]; content != "" {
		// A folder is listed with the components it holds.  The folders
		// of these two types are named with a slash at the end.
		switch typeName {
		case "DashboardFolder", "ReportFolder":
			if !strings.HasSuffix(fullName, "/") {
				fullName += "/"
			}
		}
		return content, fullName
	}
	if registryNamedWithoutParent[typeName] {
		if parts := strings.Split(fullName, "."); len(parts) > 1 {
			fullName = parts[1]
		}
	}
	return typeName, fullName
}

// orgManifestTypes returns the members of a manifest for the metadata of an
// org.  metadata are the --metadata entries, which limit the components to
// those types and names; excluded are the --excluded-metadata entries; and
// includePackages are the kinds of packages whose components are kept.
func orgManifestTypes(org *targetOrg, metadata, excluded, includePackages []string) (map[string][]string, error) {
	wanted := map[string][]string{}
	var wantedOrder []string
	for _, entry := range metadata {
		typeName, member, hasMember := strings.Cut(entry, ":")
		typeName = strings.TrimSpace(typeName)
		if !hasMember {
			member = "*"
		}
		if _, ok := wanted[typeName]; !ok {
			wantedOrder = append(wantedOrder, typeName)
		}
		wanted[typeName] = append(wanted[typeName], strings.TrimSpace(member))
	}
	typeNames := wantedOrder
	if len(metadata) == 0 && len(excluded) > 0 {
		// Every type but the excluded ones.  Excluding a type whose
		// components are in folders excludes its folder type too.
		skip := map[string]bool{}
		for _, entry := range excluded {
			typeName, member, hasMember := strings.Cut(entry, ":")
			typeName = strings.TrimSpace(typeName)
			if !hasMember || strings.TrimSpace(member) == "*" {
				skip[typeName] = true
			}
			// sf names one folder type for a type: the one the registry
			// gives, which for EmailTemplate is EmailFolder.
			for folder, content := range registryFolderContent {
				if content == typeName && registryType(folder) == folder {
					skip[folder] = true
				}
			}
		}
		typeNames = nil
		for _, name := range registryTypeNames {
			if !skip[name] {
				typeNames = append(typeNames, name)
			}
		}
	}
	components, err := orgComponents(org, typeNames)
	if err != nil {
		return nil, err
	}
	// Without --metadata, the components of packages are left out, unless
	// --include-packages names the kind of package.
	excludedStates := map[string]bool{}
	for kind, states := range packageStates {
		if !containsString(includePackages, kind) {
			for _, state := range states {
				excludedStates[state] = true
			}
		}
	}
	types := map[string][]string{}
	for _, component := range components {
		typeName, fullName := registryType(text(component.values["type"])), text(component.values["fullName"])
		if len(metadata) > 0 {
			matched := false
			for _, pattern := range wanted[typeName] {
				if ok, _ := path.Match(pattern, fullName); ok || pattern == fullName {
					matched = true
				}
			}
			if !matched {
				continue
			}
		} else if state := text(component.values["manageableState"]); state != "" && excludedStates[state] {
			continue
		}
		manifestType, member := manifestMember(typeName, fullName)
		if manifestType == "" {
			continue
		}
		if !containsString(types[manifestType], member) {
			types[manifestType] = append(types[manifestType], member)
		}
	}
	for _, members := range types {
		sort.Strings(members)
	}
	return types, nil
}

// typeNameSuggestions returns the lines sf adds to the error for a metadata
// type name it does not know: the known names that differ from it the
// least, and an empty line.
func typeNameSuggestions(name string) []string {
	// The names are in the order of the registry, which is the order sf
	// lists names that are equally close in.
	lowest := -1
	var closest []string
	for _, candidate := range registryTypeNames {
		// sf compares the letters without regard to case.
		distance := editDistance(strings.ToLower(name), strings.ToLower(candidate))
		switch {
		case lowest < 0 || distance < lowest:
			lowest, closest = distance, []string{candidate}
		case distance == lowest:
			closest = append(closest, candidate)
		}
	}
	if len(closest) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("Did you mean one of the following types? [%s]", strings.Join(closest, ",")), ""}
}

// registryKnowsType reports whether sf knows a metadata type name, as a
// type or as a child type such as CustomField.  sf compares the names
// without regard to case.
func registryKnowsType(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, known := range registryTypeNames {
		if strings.ToLower(known) == lower {
			return true
		}
	}
	for _, children := range registryChildTypes {
		for _, child := range children {
			if strings.ToLower(child) == lower {
				return true
			}
		}
	}
	return false
}
