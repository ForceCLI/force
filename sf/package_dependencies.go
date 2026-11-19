package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// package version displaydependencies follows the sf plugin source and was
// not compared with sf.

const versionBeingBuilt = "VERSION_BEING_BUILT"

const dependencyRequestQuery = "SELECT CalcTransitiveDependencies, DependencyGraphJson FROM Package2VersionCreateRequest"

type dependencyGraph struct {
	Nodes []struct {
		ID string `json:"id"`
	} `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
	} `json:"edges"`
}

func invalidDependencyVersion(id string) error {
	return newError("InvalidPackageVersionIdError", "Can't display package dependencies. The package version ID %s you specified is invalid. Review the package version ID and then retry this command.", id)
}

func invalidDependencyGraph() error {
	return newError("InvalidDependencyGraphError", "Can't display package dependencies. There's an issue generating the dependency graph. Before retrying this command, make sure you added the calculateTransitiveDependencies parameter to the sfdx-project.json file and set the value to \"true\". After setting the attribute and before retrying this command, you must create a new package version.")
}

// versionLabel formats version numbers as sf does: a missing number is 0.
func versionLabel(record *orderedMap) string {
	parts := make([]string, 4)
	for i, key := range []string{"MajorVersion", "MinorVersion", "PatchVersion", "BuildNumber"} {
		parts[i] = "0"
		if value := record.values[key]; value != nil && displayValue(value) != "" {
			parts[i] = displayValue(value)
		}
	}
	return strings.Join(parts, ".")
}

func hasNullValue(record *orderedMap) bool {
	for _, key := range record.keys {
		if record.values[key] == nil {
			return true
		}
	}
	return false
}

// dependencyDotCode returns the DOT code of the dependency graph of a
// package version (04t) or a package version create request (08c).
func dependencyDotCode(hub *targetOrg, given string, verbose bool, rootLast bool) (string, error) {
	id := packageID(given)
	requestID, versionID := "", ""
	switch {
	case strings.HasPrefix(id, "08c"):
		requestID = id
	case strings.HasPrefix(id, "04t"):
		versionID = id
		versions, err := toolingRecords(hub, fmt.Sprintf("SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", id))
		if err != nil {
			return "", err
		}
		if len(versions) != 1 {
			return "", invalidDependencyVersion(given)
		}
		requests, err := toolingRecords(hub, fmt.Sprintf("SELECT Id FROM Package2VersionCreateRequest WHERE Package2VersionId = '%s'", text(versions[0].values["Id"])))
		if err != nil {
			return "", err
		}
		if len(requests) != 1 {
			return "", invalidDependencyVersion(given)
		}
		requestID = text(requests[0].values["Id"])
	default:
		return "", invalidDependencyVersion(given)
	}
	requests, err := toolingRecords(hub, fmt.Sprintf("%s WHERE Id = '%s'", dependencyRequestQuery, requestID))
	if err != nil {
		return "", err
	}
	if len(requests) == 0 {
		return "", invalidDependencyVersion(given)
	}
	var graph dependencyGraph
	if len(requests) == 1 && boolValue(requests[0].values["CalcTransitiveDependencies"]) {
		source, ok := requests[0].values["DependencyGraphJson"].(string)
		if !ok {
			return "", invalidDependencyGraph()
		}
		if err := json.Unmarshal([]byte(source), &graph); err != nil {
			return "", err
		}
	} else {
		// Without calculated transitive dependencies sf shows the
		// direct dependencies of the version.
		if versionID == "" {
			return "", newError("NoDependencyGraphJsonMustProvideVersionError", "Can't display package dependencies. This Package2VersionCreateRequest does not have CalcTransitiveDependencies set to true. To display package dependencies, either specify a package version id (starts with 04t or 05i) or create a new package version with the calculateTransitiveDependencies parameter in the sfdx-project.json file to \"true\".")
		}
		subscribers, err := toolingRecords(hub, fmt.Sprintf("SELECT Dependencies FROM SubscriberPackageVersion WHERE Id = '%s'", versionID))
		if err != nil {
			return "", err
		}
		if len(subscribers) != 1 {
			return "", invalidDependencyVersion(given)
		}
		graph.Nodes = append(graph.Nodes, struct {
			ID string `json:"id"`
		}{versionID})
		for _, dependency := range dependencyIDs(subscribers[0]) {
			graph.Nodes = append(graph.Nodes, struct {
				ID string `json:"id"`
			}{dependency})
			graph.Edges = append(graph.Edges, struct {
				Source string `json:"source"`
				Target string `json:"target"`
			}{dependency, versionID})
		}
	}

	type node struct{ id, name, version string }
	var nodes []node
	builtID := versionBeingBuilt
	for _, item := range graph.Nodes {
		switch {
		case item.ID == versionBeingBuilt:
			records, err := toolingRecords(hub, fmt.Sprintf("SELECT Package2Version.SubscriberPackageVersionId, Package2.Name, Package2Version.MajorVersion, Package2Version.MinorVersion, Package2Version.PatchVersion, Package2Version.BuildNumber FROM Package2VersionCreateRequest WHERE Id = '%s'", requestID))
			if err != nil {
				return "", err
			}
			if len(records) != 1 {
				return "", invalidDependencyGraph()
			}
			parent, _ := records[0].values["Package2"].(*orderedMap)
			if parent == nil || text(parent.values["Name"]) == "" {
				return "", invalidDependencyGraph()
			}
			entry := node{id: versionBeingBuilt, name: text(parent.values["Name"]), version: "0.0.0.0"}
			if version, ok := records[0].values["Package2Version"].(*orderedMap); ok && text(version.values["SubscriberPackageVersionId"]) != "" {
				if hasNullValue(version) {
					return "", invalidDependencyGraph()
				}
				entry.id = text(version.values["SubscriberPackageVersionId"])
				entry.version = versionLabel(version)
				builtID = entry.id
			}
			nodes = append(nodes, entry)
		case strings.HasPrefix(item.ID, "04t"):
			records, err := toolingRecords(hub, fmt.Sprintf("SELECT SubscriberPackageVersionId, Package2.Name, MajorVersion, MinorVersion, PatchVersion, BuildNumber FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", item.ID))
			if err != nil {
				return "", err
			}
			if len(records) != 1 || hasNullValue(records[0]) {
				return "", invalidDependencyGraph()
			}
			parent, _ := records[0].values["Package2"].(*orderedMap)
			if parent == nil {
				return "", invalidDependencyGraph()
			}
			nodes = append(nodes, node{id: item.ID, name: text(parent.values["Name"]), version: versionLabel(records[0])})
		default:
			return "", invalidDependencyGraph()
		}
	}

	// sf colors the requested version and its direct dependencies green.
	selected := map[string]bool{builtID: true}
	if strings.HasPrefix(builtID, "04t") {
		subscribers, err := toolingRecords(hub, fmt.Sprintf("SELECT Dependencies FROM SubscriberPackageVersion WHERE Id = '%s'", builtID))
		if err != nil {
			return "", invalidDependencyVersion(builtID)
		}
		if len(subscribers) == 1 {
			for _, dependency := range dependencyIDs(subscribers[0]) {
				if strings.HasPrefix(dependency, "04t") {
					selected[dependency] = true
				}
			}
		}
	}

	var lines []string
	for _, entry := range nodes {
		label := entry.name + "@" + entry.version
		if entry.id == versionBeingBuilt {
			label = entry.name + "@" + versionBeingBuilt
		}
		if verbose {
			label += " (" + entry.id + ")"
		}
		color := ""
		if selected[entry.id] {
			color = ` color="green"`
		}
		lines = append(lines, fmt.Sprintf("\t node_%s [label=\"%s\"%s]", entry.id, label, color))
	}
	for _, edge := range graph.Edges {
		if edge.Source == "" || edge.Target == "" {
			return "", invalidDependencyGraph()
		}
		source, target := edge.Source, edge.Target
		if source == versionBeingBuilt {
			source = builtID
		}
		if target == versionBeingBuilt {
			target = builtID
		}
		if rootLast {
			lines = append(lines, fmt.Sprintf("\t node_%s -> node_%s", source, target))
		} else {
			lines = append(lines, fmt.Sprintf("\t node_%s -> node_%s", target, source))
		}
	}
	return "strict digraph G {\n" + strings.Join(lines, "\n") + "\n}", nil
}

// dependencyIDs returns the 04t Ids in the Dependencies field of a
// SubscriberPackageVersion record.
func dependencyIDs(record *orderedMap) []string {
	var ids []string
	dependencies, _ := record.values["Dependencies"].(*orderedMap)
	if dependencies == nil {
		return nil
	}
	list, _ := dependencies.values["ids"].([]interface{})
	for _, item := range list {
		if entry, ok := item.(*orderedMap); ok {
			ids = append(ids, text(entry.values["subscriberPackageVersionId"]))
		}
	}
	return ids
}

func newPackageVersionDisplayDependenciesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "displaydependencies",
		Short:       "Display the dependency graph for an unlocked or 2GP managed package version.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionDisplayDependenciesCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			direction, _ := cmd.Flags().GetString("edge-direction")
			if direction != "root-first" && direction != "root-last" {
				return &Error{Message: fmt.Sprintf("Expected --edge-direction=%s to be one of: root-first, root-last\nSee more help with --help", direction), ExitCode: 1}
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			code, err := dependencyDotCode(hub, packageFlag, verbose, direction == "root-last")
			if err != nil {
				return err
			}
			return emit(cmd, code, func(w io.Writer) { fmt.Fprintln(w, code) })
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID or alias of the package version (starts with 04t) or the package version create request (starts with 08c) to display the dependency graph for.")
	cmd.Flags().String("edge-direction", "root-first", "Order (root-first or root-last) in which the dependencies are displayed.")
	cmd.Flags().Bool("verbose", false, "Display both the package version ID (starts with 04t) and the version number (major.minor.patch.build) in each node.")
	addTargetDevHubFlag(cmd)
	return cmd
}
