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

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Context state
	packageName := ""
	imports := make(map[string]string)               // Alias -> Full Name
	classFields := make(map[string]map[string]string) // ClassName -> FieldName -> TypeName

	// 1. Definition Query
	defQueryStr := `
		(package_declaration
			(scoped_identifier) @package.name
		)

		(import_declaration
			[(scoped_identifier) (identifier)] @import.name
		)

		(class_declaration 
			name: (identifier) @class.name
			superclass: (superclass (type_identifier) @class.extends)?
			interfaces: (super_interfaces (type_list (type_identifier) @class.implements))?
		)
		(interface_declaration 
			name: (identifier) @class.name
			(extends_interfaces (type_list (type_identifier) @class.extends))?
		)
		(enum_declaration name: (identifier) @class.name)
		(record_declaration name: (identifier) @class.name)
		
		(field_declaration
			type: (_) @field.type
			declarator: (variable_declarator name: (identifier) @field.name)
		)
		
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

	// Helper to resolve type name using imports
	resolveType := func(typeName string) string {
		if fq, ok := imports[typeName]; ok {
			return fq
		}
		// Default to package prefix if not imported (heuristic)
		// Or return simple name if we can't be sure
		return typeName
	}

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)
			nodeContent := c.Node.Content(content)

			switch captureName {
			case "package.name":
				packageName = nodeContent
			case "import.name":
				// nodeContent is like "java.util.List"
				// Alias is "List"
				parts := strings.Split(nodeContent, ".")
				if len(parts) > 0 {
					alias := parts[len(parts)-1]
					imports[alias] = nodeContent
				}
			case "class.name":
				// Handle Class/Interface/Enum
				// Construct ID: package.ClassName
				// But we need to handle inner classes? For now assume top level or unique names.
				// NOTE: nodeContent is just the class Name.
				
				// We need to know which capture group this belongs to (Class, Interface, etc)
				// But here we are iterating captures.
				// We can check the node type of the parent to determine Label.
				parentType := c.Node.Parent().Type()
				label := "Class"
				if parentType == "interface_declaration" {
					label = "Interface"
				} else if parentType == "enum_declaration" {
					label = "Enum"
				}

				// Fully Qualified ID if possible
				id := nodeContent
				if packageName != "" {
					id = fmt.Sprintf("%s.%s", packageName, nodeContent)
				}
				
				nodes = append(nodes, &graph.Node{
					ID:    id,
					Label: label,
					Properties: map[string]interface{}{
						"name": nodeContent,
						"file": filePath,
						"line": int(c.Node.StartPoint().Row + 1),
					},
				})

			case "class.extends", "class.implements":
				// This capture is the Type Identifier of the superclass/interface
				// e.g. "Base" or "Worker"
				// We need to find the Source Class (the one being defined).
				sourceClass := findEnclosingClass(c.Node, content)
				if sourceClass != "" {
					sourceID := sourceClass
					if packageName != "" {
						sourceID = fmt.Sprintf("%s.%s", packageName, sourceClass)
					}
					
					targetType := resolveType(nodeContent)
					// If resolved type doesn't have dots, and package exists, assume same package?
					// Or keep as is.
					
					edgeType := "EXTENDS"
					if captureName == "class.implements" {
						edgeType = "IMPLEMENTS"
					}
					
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: targetType,
						Type:     edgeType,
					})
				}

			case "field.name":
				// Capture field definition
				// We need the type. The query captures type separately as @field.type.
				// But we process captures linearly.
				// We need to find the sibling @field.type in the same match?
				// Matches contain all captures for the pattern.
				// Let's iterate captures in the match to find pairs.
			case "function.name":
				// Method definition
				parentClass := findEnclosingClass(c.Node, content)
				if parentClass != "" {
					classID := parentClass
					if packageName != "" {
						classID = fmt.Sprintf("%s.%s", packageName, parentClass)
					}
					
					methodID := fmt.Sprintf("%s:%s", classID, nodeContent)
					
					nodes = append(nodes, &graph.Node{
						ID:    methodID,
						Label: "Function",
						Properties: map[string]interface{}{
							"name": nodeContent,
							"file": filePath,
							"line": int(c.Node.StartPoint().Row + 1),
						},
					})
					
					edges = append(edges, &graph.Edge{
						SourceID: classID,
						TargetID: methodID,
						Type:     "HAS_METHOD",
					})
				}
			}
		}

		// Handle Fields specifically within the match to pair name and type
		var fieldName, fieldType string
		for _, c := range m.Captures {
			name := qDef.CaptureNameForId(c.Index)
			if name == "field.name" {
				fieldName = c.Node.Content(content)
			}
			if name == "field.type" {
				fieldType = c.Node.Content(content)
				// Clean up generics: List<String> -> List
				if idx := strings.Index(fieldType, "<"); idx != -1 {
					fieldType = fieldType[:idx]
				}
			}
		}
		if fieldName != "" && fieldType != "" {
			parentClass := findEnclosingClass(m.Captures[0].Node, content) // Approximate location
			if parentClass != "" {
				// Initialize map
				if classFields[parentClass] == nil {
					classFields[parentClass] = make(map[string]string)
				}
				resolvedType := resolveType(fieldType)
				classFields[parentClass][fieldName] = resolvedType
				
				classID := parentClass
				if packageName != "" {
					classID = fmt.Sprintf("%s.%s", packageName, parentClass)
				}
				
				// Create Field Node
				fieldID := fmt.Sprintf("%s:%s", classID, fieldName)
				nodes = append(nodes, &graph.Node{
					ID: fieldID,
					Label: "Field",
					Properties: map[string]interface{}{
						"name": fieldName,
						"type": resolvedType,
						"file": filePath,
					},
				})
				
				edges = append(edges, &graph.Edge{
					SourceID: classID,
					TargetID: fieldID,
					Type:     "DEFINES",
				})
			}
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
			sourceClass := findEnclosingClass(callNode, content)
			
			if sourceFunc != "" && sourceClass != "" {
				classID := sourceClass
				if packageName != "" {
					classID = fmt.Sprintf("%s.%s", packageName, sourceClass)
				}
				sourceID := fmt.Sprintf("%s:%s", classID, sourceFunc)
				
				// 1. Constructor Calls (new Type())
				if callNode.Type() == "object_creation_expression" && targetName != "" {
					resolvedType := resolveType(targetName)
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: resolvedType,
						Type:     "CALLS",
					})
				}

				// 2. Method Calls (scope.method() or method())
				if callNode.Type() == "method_invocation" {
					if scopeName != "" {
						// e.g. helper.doWork()
						// Check if scopeName is a known field in the current class
						if typeName, ok := classFields[sourceClass][scopeName]; ok {
							// Link to Type:Method
							// If typeName is fully qualified "com.example.Worker"
							// TargetID = "com.example.Worker:doWork" (heuristic)
							targetID := fmt.Sprintf("%s:%s", typeName, targetName)
							edges = append(edges, &graph.Edge{
								SourceID: sourceID,
								TargetID: targetID,
								Type:     "CALLS",
							})
						} else {
							// Scope not found in fields (maybe local var or static class)
							// Fallback: Link to scopeName:targetName (Ghost Node potentially, but better than nothing)
							// Or try to resolve scopeName as a Class (Static call)
							resolvedScope := resolveType(scopeName)
							targetID := fmt.Sprintf("%s:%s", resolvedScope, targetName)
							edges = append(edges, &graph.Edge{
								SourceID: sourceID,
								TargetID: targetID,
								Type:     "CALLS",
							})
						}
					} else {
						// Implicit scope (this.method() or static import)
						// Assume internal call to current class
						targetID := fmt.Sprintf("%s:%s", classID, targetName)
						edges = append(edges, &graph.Edge{
							SourceID: sourceID,
							TargetID: targetID,
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
