package apex

import (
	"context"
	"fmt"
	"strings"

	sfapex "github.com/octoberswimmer/go-tree-sitter-sfapex/apex"
	sitter "github.com/smacker/go-tree-sitter"
)

// Validate that the apex parses successfully
func ValidateAnonymous(code []byte) error {
	return Validate(code)
}

func Validate(code []byte) error {
	n, err := sitter.ParseCtx(context.Background(), code, sfapex.GetLanguage())
	if err != nil {
		return err
	}
	if !n.HasError() {
		return nil
	}
	apexError, err := getError(n, code)
	if err != nil {
		return fmt.Errorf("Apex error: %w", err)
	}
	return fmt.Errorf("failed to parse apex: %s", apexError)
}

func getError(node *sitter.Node, apex []byte) (string, error) {
	e := `(ERROR) @node`
	errorQuery, err := sitter.NewQuery([]byte(e), sfapex.GetLanguage())
	if err != nil {
		return "", err
	}
	defer errorQuery.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(errorQuery, node)
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range match.Captures {
			v := c.Node.Content(apex)
			return v, nil
		}
	}
	// If no error, find MISSING Node, then parent expression_statement Node
	if missing := findMissing(node, apex); missing != "" {
		return missing, nil
	}
	return "", fmt.Errorf("unknown error")
}

// Find MISSING node by walking tree since querying for missing nodes isn't
// supported yet.  See https://github.com/tree-sitter/tree-sitter/issues/606
func findMissing(n *sitter.Node, code []byte) string {
	tree := sitter.NewTreeCursor(n)
	defer tree.Close()

	rootNode := tree.CurrentNode()
	missing := ""
Tree:
	for {
		if strings.HasPrefix(tree.CurrentNode().String(), "(MISSING") {
			missing = tree.CurrentNode().String()
			break Tree
		}
		ok := tree.GoToFirstChild()
		if !ok {
			ok := tree.GoToNextSibling()
			if !ok {
			Sibling:
				for {
					tree.GoToParent()
					if tree.CurrentNode() == rootNode {
						break Tree
					}
					if tree.GoToNextSibling() {
						break Sibling
					}
				}
			}
		}
	}
	for {
		if tree.CurrentNode().Type() == "expression_statement" {
			missing = fmt.Sprintf("%s in %s", missing, tree.CurrentNode().Content(code))
			break
		}
		if tree.CurrentNode() == rootNode {
			break
		}
		tree.GoToParent()
	}
	return missing
}
