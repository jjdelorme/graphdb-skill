package analysis

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"graphdb/internal/graph"
)

type CppParser struct{}

func init() {
	p := &CppParser{}
	RegisterParser(".c", p)
	RegisterParser(".h", p)
	RegisterParser(".cpp", p)
	RegisterParser(".hpp", p)
	RegisterParser(".cc", p)
}

func (p *CppParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(cpp.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	// Matches standalone functions and methods defined inline
	defQueryStr := `
		(function_definition
			declarator: (function_declarator
				declarator: (identifier) @function.name
			)
		) @function.def

		(function_definition
			declarator: (function_declarator
				declarator: (field_identifier) @function.name
			)
		) @function.def
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), cpp.GetLanguage())
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
	refQueryStr := `
		(call_expression
			function: (identifier) @call.target
		) @call.site

		(call_expression
			function: (field_expression field: (field_identifier) @call.target)
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), cpp.GetLanguage())
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
			sourceFunc := findEnclosingCppFunction(callNode, content)
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

func findEnclosingCppFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "function_definition" {
			// Try to find the name in the declarator
			// declarator -> function_declarator -> declarator (identifier/field_identifier)
			
			// Simple traversal to find the identifier node
			// This is a bit brute force but works for standard structures
			
			// We need to look for the "declarator" child
			decl := curr.ChildByFieldName("declarator")
			if decl != nil {
				// Inside declarator (function_declarator), find "declarator"
				innerDecl := decl.ChildByFieldName("declarator")
				if innerDecl != nil {
					// This might be the identifier or field_identifier
					if innerDecl.Type() == "identifier" || innerDecl.Type() == "field_identifier" {
						return innerDecl.Content(content)
					}
				}
			}
		}
		curr = curr.Parent()
	}
	return ""
}
