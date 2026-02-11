package analysis

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"graphdb/internal/graph"
)

type JavaParser struct{}

func init() {
	RegisterParser(".java", &JavaParser{})
}

func (p *JavaParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	defQueryStr := `
		(class_declaration name: (identifier) @class.name)
		(interface_declaration name: (identifier) @class.name)
		(enum_declaration name: (identifier) @class.name)
		(record_declaration name: (identifier) @class.name)
		
		(method_declaration name: (identifier) @function.name)
		(constructor_declaration name: (identifier) @function.name)
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), java.GetLanguage())
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

			if !strings.HasSuffix(captureName, ".name") {
				continue
			}

			nodeName := c.Node.Content(content)

			var label string
			if strings.HasPrefix(captureName, "class") {
				label = "Class"
			} else if strings.HasPrefix(captureName, "function") {
				label = "Function"
			} else {
				continue
			}

			n := &graph.Node{
				ID:    fmt.Sprintf("%s:%s", filePath, nodeName),
				Label: label,
				Properties: map[string]interface{}{
					"name": nodeName,
					"file": filePath,
					"line": int(c.Node.StartPoint().Row + 1),
				},
			}
			nodes = append(nodes, n)
		}
	}

	// 2. Reference/Call Query
	refQueryStr := `
		(method_invocation
			name: (identifier) @call.target
		) @call.site

		(object_creation_expression
			type: (type_identifier) @call.target
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), java.GetLanguage())
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
			sourceFunc := findEnclosingJavaFunction(callNode, content)
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

func findEnclosingJavaFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		// method_declaration, constructor_declaration
		if t == "method_declaration" || t == "constructor_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}
