package analysis

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"graphdb/internal/graph"
)

type CSharpParser struct{}

func init() {
	RegisterParser(".cs", &CSharpParser{})
}

func (p *CSharpParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(csharp.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	defQueryStr := `
		(class_declaration name: (identifier) @class.name) @class.def
		(interface_declaration name: (identifier) @class.name) @class.def
		(struct_declaration name: (identifier) @class.name) @class.def
		(record_declaration name: (identifier) @class.name) @class.def
		
		(method_declaration name: (identifier) @function.name) @function.def
		(constructor_declaration name: (identifier) @function.name) @function.def
		(local_function_statement name: (identifier) @function.name) @function.def
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), csharp.GetLanguage())
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

			// Only process the name capture to create the node
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
					"line": c.Node.StartPoint().Row + 1,
				},
			}
			nodes = append(nodes, n)
		}
	}

	// 2. Reference/Call Query - Basic implementation for now
    // NOTE: This might need refinement based on exact C# grammar for calls
	refQueryStr := `
		(invocation_expression
			function: (identifier) @call.target
		) @call.site

		(invocation_expression
			function: (member_access_expression name: (identifier) @call.target)
		) @call.site

		(object_creation_expression
			type: (identifier) @call.target
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), csharp.GetLanguage())
	if err != nil {
		// Just log error or return partial?
        // Since we are adding edges, if query fails, maybe we just return nodes.
        // But for now, let's error out to be safe in dev.
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
			sourceFunc := findEnclosingCSharpFunction(callNode, content)
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

func findEnclosingCSharpFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		// method_declaration, constructor_declaration, local_function_statement
		if t == "method_declaration" || t == "constructor_declaration" || t == "local_function_statement" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}
