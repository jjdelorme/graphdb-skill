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

		(field_declaration) @field.declarator
		(property_declaration name: (identifier) @field.name)

		(using_directive (qualified_name) @using.namespace)
		(using_directive (identifier) @using.namespace)
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
	var extraEdges []*graph.Edge // Store inheritance edges here
	var usings []string

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)

			if captureName == "using.namespace" {
				usings = append(usings, c.Node.Content(content))
				continue
			}

			var nodeNames []string
			if strings.HasSuffix(captureName, ".name") {
				nodeNames = append(nodeNames, c.Node.Content(content))
			} else if captureName == "field.declarator" {
				// c.Node is field_declaration
				count := c.Node.ChildCount()
				for i := 0; i < int(count); i++ {
					child := c.Node.Child(i)
					if child.Type() == "variable_declarator" {
						name := extractNameFromDeclarator(child, content)
						if name != "" {
							nodeNames = append(nodeNames, name)
						}
					} else if child.Type() == "variable_declaration" {
						vCount := child.ChildCount()
						for k := 0; k < int(vCount); k++ {
							vChild := child.Child(k)
							if vChild.Type() == "variable_declarator" {
								name := extractNameFromDeclarator(vChild, content)
								if name != "" {
									nodeNames = append(nodeNames, name)
								}
							}
						}
					}
				}
			}

			if len(nodeNames) == 0 {
				continue
			}

			namespace := findEnclosingNamespace(c.Node, content)

			for _, nodeName := range nodeNames {
				var label string
				var fullID string
				var properties = map[string]interface{}{
					"name": nodeName,
					"file": filePath,
					"line": c.Node.StartPoint().Row + 1,
				}

				if strings.HasPrefix(captureName, "class") {
					label = "Class"
					if namespace != "" {
						fullID = fmt.Sprintf("%s:%s.%s", filePath, namespace, nodeName)
						properties["namespace"] = namespace
					} else {
						fullID = fmt.Sprintf("%s:%s", filePath, nodeName)
					}

					// Check for Inheritance (base_list)
					// The captured node is the identifier (name). Parent is the declaration.
					parent := c.Node.Parent()
					if parent != nil {
						var baseList *sitter.Node
						baseList = parent.ChildByFieldName("base_list")
						if baseList == nil {
							count := parent.ChildCount()
							for i := 0; i < int(count); i++ {
								child := parent.Child(i)
								if child.Type() == "base_list" {
									baseList = child
									break
								}
							}
						}

						if baseList != nil {
							// base_list children are usually: (colon) (simple_type) ...
							// We iterate named children to skip punctuation
							count := baseList.NamedChildCount()
							for i := 0; i < int(count); i++ {
								child := baseList.NamedChild(i)
								// We blindly take all types in base list as INHERITS for now
								baseName := child.Content(content)
								targetID := fmt.Sprintf("%s:%s", filePath, baseName) // Simple local assumption
								extraEdges = append(extraEdges, &graph.Edge{
									SourceID: fullID,
									TargetID: targetID,
									Type:     "INHERITS",
								})
							}
						}
					}

				} else if strings.HasPrefix(captureName, "function") {
					label = "Function"
					fullID = fmt.Sprintf("%s:%s", filePath, nodeName)
				} else if strings.HasPrefix(captureName, "field") {
					label = "Field"
					fullID = fmt.Sprintf("%s:%s", filePath, nodeName)
				} else {
					continue
				}

				n := &graph.Node{
					ID:         fullID,
					Label:      label,
					Properties: properties,
				}
				nodes = append(nodes, n)
			}
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

		(object_creation_expression
			type: (generic_name (identifier) @call.target)
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

	var edges []*graph.Edge = extraEdges

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
				ns := findEnclosingNamespace(callNode, content)
				candidates := resolveCandidates(targetName, usings, ns)
				for _, cand := range candidates {
					edges = append(edges, &graph.Edge{
						SourceID: fmt.Sprintf("%s:%s", filePath, sourceFunc),
						TargetID: cand, // Use the resolved candidate as ID
						Type:     "CALLS",
					})
				}
			}
		}
	}

	return nodes, edges, nil
}

func extractNameFromDeclarator(n *sitter.Node, content []byte) string {
	nameChild := n.ChildByFieldName("name")
	if nameChild != nil {
		return nameChild.Content(content)
	}
	count := n.ChildCount()
	for i := 0; i < int(count); i++ {
		if n.Child(i).Type() == "identifier" {
			return n.Child(i).Content(content)
		}
	}
	return ""
}

func resolveCandidates(name string, usings []string, currentNamespace string) []string {
	if strings.Contains(name, ".") {
		return []string{name}
	}

	var candidates []string
	// Local namespace
	if currentNamespace != "" {
		candidates = append(candidates, fmt.Sprintf("%s.%s", currentNamespace, name))
	} else {
		// Global namespace
		candidates = append(candidates, name)
	}

	for _, u := range usings {
		candidates = append(candidates, fmt.Sprintf("%s.%s", u, name))
	}

	return candidates
}

func findEnclosingNamespace(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		if curr.Type() == "namespace_declaration" || curr.Type() == "file_scoped_namespace_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
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
