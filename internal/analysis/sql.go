package analysis

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/sql"
	"graphdb/internal/graph"
)

type SqlParser struct{}

func init() {
	p := &SqlParser{}
	RegisterParser(".sql", p)
}

func (p *SqlParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(sql.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	defQueryStr := `
		(create_function
			(object_reference
				(identifier) @function.name
			)
		) @function.def
	`
    // Note: create_procedure caused invalid node type error, so it's not in the grammar or named differently.
    // We will inspect if we can support it later or if we need to rely on ERROR recovery.

	qDef, err := sitter.NewQuery([]byte(defQueryStr), sql.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()

	qcDef.Exec(qDef, tree.RootNode())

	var nodes []*graph.Node

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)

			if captureName != "function.name" {
				continue
			}

			nodeName := c.Node.Content(content)

			n := &graph.Node{
				ID:    fmt.Sprintf("%s:%s", filePath, nodeName),
				Label: "Function",
				Properties: map[string]interface{}{
					"name": nodeName,
					"file": filePath,
					"line": c.Node.StartPoint().Row + 1,
				},
			}
			nodes = append(nodes, n)
		}
	}

	// 2. Reference/Call Query
	// Matches `SELECT CalculateTotal()` which parses as (invocation (object_reference (identifier)))
	refQueryStr := `
		(invocation
			(object_reference
				(identifier) @call.target
			)
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), sql.GetLanguage())
	if err != nil {
		return nodes, nil, fmt.Errorf("invalid reference query: %w", err)
	}
	defer qRef.Close()

	qcRef := sitter.NewQueryCursor()
	defer qcRef.Close()

	qcRef.Exec(qRef, tree.RootNode())

	var edges []*graph.Edge

	for {
		m, ok := qcRef.NextMatch()
		if !ok {
			break
		}

		var targetName string
		var callNode *sitter.Node

		for _, c := range m.Captures {
			name := qRef.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
			}
			if name == "call.site" {
				callNode = c.Node
			}
		}

		if targetName != "" && callNode != nil {
			sourceFunc := findEnclosingSqlFunction(callNode, content)
			if sourceFunc != "" {
				edges = append(edges, &graph.Edge{
					SourceID: fmt.Sprintf("%s:%s", filePath, sourceFunc),
					TargetID: fmt.Sprintf("%s:%s", filePath, targetName),
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges, nil
}

func findEnclosingSqlFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "create_function" {
            // Find the object_reference child, then its identifier
            count := int(curr.ChildCount())
            for i := 0; i < count; i++ {
                child := curr.Child(i)
                if child.Type() == "object_reference" {
                     if child.ChildCount() > 0 {
                         // Assuming the first child is the identifier
                         return child.Child(0).Content(content)
                     }
                }
            }
		}
		curr = curr.Parent()
	}
	return ""
}
