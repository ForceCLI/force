package apex

import (
	"context"
	"fmt"
	"os"
	"strings"

	sfapex "github.com/octoberswimmer/go-tree-sitter-sfapex/apex"
	sitter "github.com/smacker/go-tree-sitter"
)

// Validate that the apex parses successfully
func ValidateAnonymous(code []byte) error {
	apex := wrap(string(code))
	return Validate(apex)
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

// Wrap anonymous apex in class and method because tree-sitter-sfapex doesn't
// support anonymous apex yet
func wrap(anon string) []byte {
	wrapped := []byte(fmt.Sprintf(`public class Temp {
		public void run() {
		%s
		}
	}`, anon))
	return wrapped
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

func printNodes(n *sitter.Node, code []byte) {
	tree := sitter.NewTreeCursor(n)
	defer tree.Close()

	rootNode := tree.CurrentNode()
Tree:
	for {
		switch tree.CurrentNode().Type() {
		case "(", ")", "{", "}", ";", ".", ",", "+", "<", ">":
		default:
			fmt.Fprintf(os.Stderr, "TYPE: %s\nFIELD NAME: %s\nNODE: %s\nCONTENT: %s\n\n", tree.CurrentNode().Type(), tree.CurrentFieldName(), tree.CurrentNode().String(), tree.CurrentNode().Content([]byte(code)))
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
