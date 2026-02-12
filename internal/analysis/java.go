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
	var edges []*graph.Edge

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
			var id string
			if strings.HasPrefix(captureName, "class") {
				label = "Class"
				id = nodeName // Global ID for Classes
			} else if strings.HasPrefix(captureName, "function") {
				label = "Function"
				id = fmt.Sprintf("%s:%s", filePath, nodeName) // Scoped ID for Functions
				
				// Create HAS_METHOD edge
				parentClass := findEnclosingClass(c.Node, content)
				if parentClass != "" {
					edges = append(edges, &graph.Edge{
						SourceID: parentClass, // Global Class ID
						TargetID: id,
						Type:     "HAS_METHOD",
					})
				}
			} else {
				continue
			}

			n := &graph.Node{
				ID:    id,
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
			object: (identifier)? @call.scope
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

	for {
		m, ok := qcRef.NextMatch()
		if !ok {
			break
		}

		var targetName string
		var scopeName string
		var callNode *sitter.Node

		for _, c := range m.Captures {
			name := qRef.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
			}
			if name == "call.scope" {
				scopeName = c.Node.Content(content)
			}
			if name == "call.site" {
				callNode = c.Node
			}
		}

		if callNode != nil {
			sourceFunc := findEnclosingJavaFunction(callNode, content)
			if sourceFunc != "" {
				sourceID := fmt.Sprintf("%s:%s", filePath, sourceFunc)
				
				// 1. Handle Constructor Calls (new Type())
				if callNode.Type() == "object_creation_expression" && targetName != "" {
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: targetName, // Links to Global Class ID
						Type:     "CALLS",
					})
				}

				// 2. Handle Method Calls with Scope (Scope.method())
				if callNode.Type() == "method_invocation" {
					// Link to the Scope (Dependency on the class/object)
					if scopeName != "" {
						edges = append(edges, &graph.Edge{
							SourceID: sourceID,
							TargetID: scopeName, // Links to Global Class ID (heuristic)
							Type:     "USES",      // Differentiate from direct CALLS? Or keep CALLS?
						})
					} else if targetName != "" {
						// Internal call or statically imported call
						edges = append(edges, &graph.Edge{
							SourceID: sourceID,
							TargetID: fmt.Sprintf("%s:%s", filePath, targetName),
							Type:     "CALLS",
						})
					}
				}
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

func findEnclosingClass(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "class_declaration" || t == "interface_declaration" || t == "enum_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}
