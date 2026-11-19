package sf

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// package version displayancestry follows the sf plugin source.  Only its
// errors for a package and a package version that do not exist were
// compared with sf: the Dev Hub used had no version with an ancestor.

const ancestryVersionQuery = "SELECT AncestorId, SubscriberPackageVersionId, MajorVersion, MinorVersion, PatchVersion, BuildNumber FROM Package2Version"

// ancestryNode is one package version in the ancestry graph.  sf keys the
// graph by version number, and so does this.
type ancestryNode struct {
	record   *orderedMap
	id       string
	ancestor string
	version  string
	numbers  [4]int
	children []*ancestryNode
}

func newAncestryNode(record *orderedMap) *ancestryNode {
	node := &ancestryNode{record: record, id: text(record.values["SubscriberPackageVersionId"]), ancestor: text(record.values["AncestorId"])}
	for i, key := range []string{"MajorVersion", "MinorVersion", "PatchVersion", "BuildNumber"} {
		if record.values[key] != nil {
			if number, err := strconv.ParseFloat(displayValue(record.values[key]), 64); err == nil {
				node.numbers[i] = int(number)
			}
		}
	}
	node.version = fmt.Sprintf("%d.%d.%d.%d", node.numbers[0], node.numbers[1], node.numbers[2], node.numbers[3])
	return node
}

func (n *ancestryNode) less(other *ancestryNode) bool {
	for i := range n.numbers {
		if n.numbers[i] != other.numbers[i] {
			return n.numbers[i] < other.numbers[i]
		}
	}
	return false
}

// ancestryGraph holds the nodes that have an ancestor or a descendant, in
// the order sf adds them.  A version with neither is not in the graph.
type ancestryGraph struct {
	nodes []*ancestryNode
	byKey map[string]*ancestryNode
}

func (g *ancestryGraph) node(candidate *ancestryNode) *ancestryNode {
	if existing, ok := g.byKey[candidate.version]; ok {
		return existing
	}
	g.byKey[candidate.version] = candidate
	g.nodes = append(g.nodes, candidate)
	return candidate
}

func (g *ancestryGraph) addEdge(ancestor, descendant *ancestryNode) {
	from, to := g.node(ancestor), g.node(descendant)
	for _, child := range from.children {
		if child == to {
			return
		}
	}
	from.children = append(from.children, to)
}

// ancestryTree is the result of the depth-first traversal sf makes.  It
// visits the descendants of a node from the last one added to the first.
type ancestryTree struct {
	node     *ancestryNode
	depth    int
	children []*ancestryTree
}

func (g *ancestryGraph) tree(start *ancestryNode) *ancestryTree {
	top := &ancestryTree{node: start}
	seen := map[*ancestryNode]bool{}
	var visit func(parent *ancestryTree, node *ancestryNode, depth int)
	visit = func(parent *ancestryTree, node *ancestryNode, depth int) {
		item := &ancestryTree{node: node, depth: depth}
		parent.children = append(parent.children, item)
		var pending []*ancestryNode
		for _, child := range node.children {
			if !seen[child] {
				seen[child] = true
				pending = append(pending, child)
			}
		}
		for i := len(pending) - 1; i >= 0; i-- {
			visit(item, pending[i], depth+1)
		}
	}
	if start != nil {
		seen[start] = true
		visit(top, start, 0)
		return top
	}
	for _, node := range g.nodes {
		if !seen[node] {
			seen[node] = true
			visit(top, node, 0)
		}
	}
	return top
}

func (t *ancestryTree) sorted() []*ancestryTree {
	children := append([]*ancestryTree(nil), t.children...)
	sort.SliceStable(children, func(i, j int) bool { return children[i].node.less(children[j].node) })
	return children
}

// json returns the result of sf: the nodes in the order of the traversal.
func (t *ancestryTree) json() *orderedMap {
	children := make([]*orderedMap, 0, len(t.children))
	for _, child := range t.children {
		children = append(children, child.json())
	}
	return orderedFields("children", children, "data", orderedFields(
		"SubscriberPackageVersionId", t.node.id, "MajorVersion", t.node.numbers[0], "MinorVersion", t.node.numbers[1],
		"PatchVersion", t.node.numbers[2], "BuildNumber", t.node.record.values["BuildNumber"], "depthCounter", t.depth))
}

func (t *ancestryTree) write(w io.Writer, prefix string, verbose bool) {
	children := t.sorted()
	for i, child := range children {
		branch, indent := "├─ ", "│  "
		if i == len(children)-1 {
			branch, indent = "└─ ", "   "
		}
		label := child.node.version
		if verbose && child.node.id != "" {
			label += " (" + child.node.id + ")"
		}
		fmt.Fprintln(w, prefix+branch+label)
		child.write(w, prefix+indent, verbose)
	}
}

// dot returns the graph in the DOT language as sf writes it: the nodes
// breadth first, then the edges.  The first node line is for the requested
// version, or "root" for a package.
func (t *ancestryTree) dot() string {
	name := func(item *ancestryTree) string {
		if item.node == nil {
			return "nodeunknown"
		}
		return "node" + item.node.id
	}
	label := func(item *ancestryTree) string {
		if item.node == nil {
			return "root"
		}
		return item.node.version
	}
	var lines []string
	queue := []*ancestryTree{t}
	var order []*ancestryTree
	sortedChildren := map[*ancestryTree][]*ancestryTree{}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		order = append(order, item)
		lines = append(lines, fmt.Sprintf("\t %s [label=\"%s\"]", name(item), label(item)))
		sortedChildren[item] = item.sorted()
		queue = append(queue, sortedChildren[item]...)
	}
	for _, item := range order {
		for _, child := range sortedChildren[item] {
			lines = append(lines, fmt.Sprintf("\t %s -- %s", name(item), name(child)))
		}
	}
	return "strict graph G {\n" + strings.Join(lines, "\n") + "\n}"
}

func ancestryIDError(label, value, prefix string) error {
	return newError("InvalidIdOrAliasError", "The %s: %s isn't defined in the sfdx-project.json. Add it to the packageDirectories section and add the alias to packageAliases with its %s ID.", label, value, prefix)
}

func ancestryNodes(hub *targetOrg, condition string) ([]*ancestryNode, error) {
	records, err := toolingRecords(hub, ancestryVersionQuery+" WHERE "+condition)
	if err != nil {
		return nil, err
	}
	nodes := make([]*ancestryNode, 0, len(records))
	for _, record := range records {
		nodes = append(nodes, newAncestryNode(record))
	}
	return nodes, nil
}

// buildAncestry reads the versions related to a package (0Ho) or a package
// version (04t) and returns the graph and the requested version, which is
// nil for a package.
func buildAncestry(hub *targetOrg, id string) (*ancestryGraph, *ancestryNode, error) {
	const unlocked = "Can’t display package ancestry. Package ancestry is available only for second-generation managed packages. Retry this command and specify a second-generation managed package or package version."
	graph := &ancestryGraph{byKey: map[string]*ancestryNode{}}
	validLength := len(id) == 15 || len(id) == 18
	var roots []*ancestryNode
	var requested *ancestryNode
	switch {
	case strings.HasPrefix(id, "0Ho"):
		if !validLength {
			return nil, nil, ancestryIDError("Package Id", id, "0Ho")
		}
		packages, err := toolingRecords(hub, fmt.Sprintf("SELECT ContainerOptions FROM Package2 WHERE Id = '%s'", id))
		if err != nil {
			return nil, nil, err
		}
		if len(packages) == 0 {
			return nil, nil, newError("IdOrAliasNotFoundError", "Can't find package with id or alias: %s", id)
		}
		if text(packages[0].values["ContainerOptions"]) != "Managed" {
			return nil, nil, newError("UnlockedPackageError", unlocked)
		}
		if roots, err = ancestryNodes(hub, fmt.Sprintf("AncestorId = NULL AND Package2Id = '%s'  AND IsReleased = true", id)); err != nil {
			return nil, nil, err
		}
		if len(roots) == 0 {
			return nil, nil, newError("NoVersionsError", "Can’t display package ancestry. The specified package has no associated released package versions. Retry this command after you create and promote at least one package version.")
		}
	case strings.HasPrefix(id, "04t"):
		if !validLength {
			return nil, nil, ancestryIDError("Subscriber Package Version Id", id, "04t")
		}
		version := func(versionID string) (*ancestryNode, error) {
			found, err := ancestryNodes(hub, fmt.Sprintf("SubscriberPackageVersionId = '%s'", versionID))
			if err != nil {
				return nil, err
			}
			if len(found) == 0 {
				return nil, newError("VersionNotFoundError", "Can’t display the ancestry tree for %s. Verify the package version number (starts with 04t) or the package version alias listed in the sfdx-project.json file, and try creating the ancestry tree again.", versionID)
			}
			return found[0], nil
		}
		node, err := version(id)
		if err != nil {
			return nil, nil, err
		}
		requested = node
		for node.ancestor != "" {
			ancestor, err := version(node.ancestor)
			if err != nil {
				return nil, nil, err
			}
			graph.addEdge(ancestor, node)
			node = ancestor
		}
		roots = []*ancestryNode{node}
		types, err := toolingRecords(hub, fmt.Sprintf("SELECT Package2.ContainerOptions FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", id))
		if err != nil {
			return nil, nil, err
		}
		packageType := ""
		if len(types) > 0 {
			if parent, ok := types[0].values["Package2"].(*orderedMap); ok {
				packageType = text(parent.values["ContainerOptions"])
			}
		}
		if packageType != "Managed" {
			return nil, nil, newError("UnlockedPackageError", unlocked)
		}
	default:
		return nil, nil, newError("IdOrAliasNotFoundError", "Can't find package with id or alias: %s", id)
	}
	for len(roots) > 0 {
		ancestor := roots[0]
		roots = roots[1:]
		descendants, err := ancestryNodes(hub, fmt.Sprintf("AncestorId = '%s'  AND IsReleased = true", ancestor.id))
		if err != nil {
			return nil, nil, err
		}
		for _, descendant := range descendants {
			graph.addEdge(ancestor, descendant)
		}
		roots = append(roots, descendants...)
	}
	if requested != nil {
		// The node in the graph, if the version has relatives.
		requested = graph.byKey[requested.version]
		if requested == nil {
			// sf fails here: it looks for the path from the version to
			// the root in a graph that does not contain the version.
			return nil, nil, &Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'map')", ExitCode: 10, Label: "10"}
		}
	}
	return graph, requested, nil
}

func newPackageVersionDisplayAncestryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "displayancestry",
		Short:       "Display the ancestry tree for a 2GP managed package version.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackageVersionDisplayAncestryCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package")
			if packageFlag == "" {
				return missingFlagError("package")
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			graph, requested, err := buildAncestry(hub, packageID(packageFlag))
			if err != nil {
				return err
			}
			tree := graph.tree(requested)
			if dot, _ := cmd.Flags().GetBool("dot-code"); dot {
				code := tree.dot()
				return emit(cmd, code, func(w io.Writer) { fmt.Fprintln(w, code) })
			}
			var result interface{}
			if len(tree.children) > 0 {
				result = tree.children[0].json()
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			return emit(cmd, result, func(w io.Writer) {
				if requested != nil {
					path := []string{requested.version}
					for node := requested; node.ancestor != ""; {
						var parent *ancestryNode
						for _, candidate := range graph.nodes {
							if candidate.id == node.ancestor {
								parent = candidate
								break
							}
						}
						if parent == nil {
							break
						}
						path = append(path, parent.version)
						node = parent
					}
					fmt.Fprintf(w, "%s (root)\n\n", strings.Join(path, " -> "))
				}
				var text strings.Builder
				tree.write(&text, "", verbose)
				fmt.Fprintln(w, strings.TrimSuffix(text.String(), "\n"))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID or alias of the package (starts with 0Ho) or package version (starts with 04t) to display ancestry for.")
	cmd.Flags().Bool("dot-code", false, "Display the ancestry tree in DOT code.")
	cmd.Flags().Bool("verbose", false, "Display both the package version ID (starts with 04t) and the version number (major.minor.patch.build) in the ancestry tree.")
	addTargetDevHubFlag(cmd)
	return cmd
}
