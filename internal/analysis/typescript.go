package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"graphdb/internal/graph"
)

type TypeScriptParser struct{}

func init() {
	RegisterParser(".ts", &TypeScriptParser{})
}

func (p *TypeScriptParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge
	
	// Map of local alias -> resolved target ID
	imports := make(map[string]string)

	// 1. Import Query
	importQueryStr := `
		(import_statement
			(import_clause 
				(named_imports 
					(import_specifier) @import.specifier
				)
			)
			(string) @import.source
		)
		(import_statement
			(import_clause 
				(identifier) @import.default
			)
			(string) @import.source
		)
		(import_statement
			(import_clause 
				(namespace_import (identifier) @import.namespace)
			)
			(string) @import.source
		)
	`
	qImport, err := sitter.NewQuery([]byte(importQueryStr), typescript.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid import query: %w", err)
	}
	defer qImport.Close()
	
	qcImport := sitter.NewQueryCursor()
	defer qcImport.Close()
	qcImport.Exec(qImport, tree.RootNode())
	
	for {
		m, ok := qcImport.NextMatch()
		if !ok {
			break
		}
		
		var sourcePath string
		var defaultNode, specifierNode, namespaceNode *sitter.Node
		
		for _, c := range m.Captures {
			name := qImport.CaptureNameForId(c.Index)
			if name == "import.source" {
				sourcePath = c.Node.Content(content)
				sourcePath = strings.Trim(sourcePath, "\"'`")
			} else if name == "import.default" {
				defaultNode = c.Node
			} else if name == "import.specifier" {
				specifierNode = c.Node
			} else if name == "import.namespace" {
				namespaceNode = c.Node
			}
		}
		
		if sourcePath != "" {
			resolvedPath := resolveTSPath(filePath, sourcePath)
			
			if defaultNode != nil {
				localName := defaultNode.Content(content)
				imports[localName] = fmt.Sprintf("%s:default", resolvedPath)
			}
			
			if namespaceNode != nil {
				localName := namespaceNode.Content(content)
				imports[localName] = resolvedPath
			}
			
			if specifierNode != nil {
				var localName, remoteName string
				nameNode := specifierNode.ChildByFieldName("name")
				aliasNode := specifierNode.ChildByFieldName("alias")
				
				if nameNode != nil {
					remoteName = nameNode.Content(content)
				}
				if aliasNode != nil {
					localName = aliasNode.Content(content)
				} else {
					localName = remoteName
				}
				
				if localName != "" && remoteName != "" {
					imports[localName] = fmt.Sprintf("%s:%s", resolvedPath, remoteName)
				}
			}
		}
	}

	// 2. Definition & Field Query
	defQueryStr := `
		(function_declaration name: (identifier) @function.name) @function.def
		(generator_function_declaration name: (identifier) @function.name) @function.def
		(method_definition name: (property_identifier) @method.name) @method.def
		(class_declaration name: (type_identifier) @class.name) @class.def
		(interface_declaration name: (type_identifier) @class.name) @class.def
		(public_field_definition name: (property_identifier) @field.name) @field.def
		(variable_declarator 
			name: (identifier) @function.name 
			value: [(arrow_function) (function_expression)]
		) @function.def
	`
	qDef, err := sitter.NewQuery([]byte(defQueryStr), typescript.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()
	qcDef.Exec(qDef, tree.RootNode())
	
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
			} else if strings.HasPrefix(captureName, "function") || strings.HasPrefix(captureName, "method") {
				label = "Function"
			} else if strings.HasPrefix(captureName, "field") {
				label = "Field"
			} else {
				continue
			}
			
			id := fmt.Sprintf("%s:%s", filePath, nodeName)
			
			n := &graph.Node{
				ID:    id,
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

	// 3. Inheritance Query
    // Using generic capture for extends target and filtering in Go
	inheritanceQueryStr := `
		(class_declaration
			name: (type_identifier) @class.name
			(class_heritage
				(extends_clause (_) @extends.target)?
				(implements_clause (_) @implements.target)*
			)?
		)
	`
    
	qInh, err := sitter.NewQuery([]byte(inheritanceQueryStr), typescript.GetLanguage())
	if err != nil {
        return nil, nil, fmt.Errorf("invalid inheritance query: %w", err)
    }
    defer qInh.Close()
    
    qcInh := sitter.NewQueryCursor()
    defer qcInh.Close()
    qcInh.Exec(qInh, tree.RootNode())
    
    for {
        m, ok := qcInh.NextMatch()
        if !ok {
            break
        }
        
        var className string
        var extendsTarget string
        var implementsTargets []string
        
        for _, c := range m.Captures {
            name := qInh.CaptureNameForId(c.Index)
            contentStr := c.Node.Content(content)
            
            if name == "class.name" {
                className = contentStr
            } else if name == "extends.target" {
                if contentStr != "extends" {
                    extendsTarget = contentStr
                }
            } else if name == "implements.target" {
                if contentStr != "implements" && contentStr != "," {
                     implementsTargets = append(implementsTargets, contentStr)
                }
            }
        }
        
        if className != "" {
            sourceID := fmt.Sprintf("%s:%s", filePath, className)
            
            if extendsTarget != "" {
                if idx := strings.Index(extendsTarget, "<"); idx != -1 {
                    extendsTarget = extendsTarget[:idx]
                }
                extendsTarget = strings.TrimSpace(extendsTarget)

                targetID := resolveTargetID(extendsTarget, imports, filePath)
                edges = append(edges, &graph.Edge{
                    SourceID: sourceID,
                    TargetID: targetID,
                    Type:     "EXTENDS",
                })
            }
            
            for _, imp := range implementsTargets {
                if idx := strings.Index(imp, "<"); idx != -1 {
                    imp = imp[:idx]
                }
                imp = strings.TrimSpace(imp)
                targetID := resolveTargetID(imp, imports, filePath)
                edges = append(edges, &graph.Edge{
                    SourceID: sourceID,
                    TargetID: targetID,
                    Type:     "IMPLEMENTS",
                })
            }
        }
    }

	// 4. Reference/Call Query
	refQueryStr := `
		(call_expression
		  function: [
			(identifier) @call.target
			(member_expression property: (property_identifier) @call.target)
		  ]
		) @call.site
		
		(new_expression
		  constructor: (identifier) @call.target
		) @call.site
	`
	qRef, err := sitter.NewQuery([]byte(refQueryStr), typescript.GetLanguage())
	if err != nil {
		return nodes, edges, fmt.Errorf("invalid reference query: %w", err)
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
			sourceFunc := findEnclosingFunction(callNode, content)
			if sourceFunc != "" {
				targetID := resolveTargetID(targetName, imports, filePath)
				edges = append(edges, &graph.Edge{
					SourceID: fmt.Sprintf("%s:%s", filePath, sourceFunc),
					TargetID: targetID,
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges, nil
}

func resolveTSPath(currentFile, importPath string) string {
    importPath = strings.Trim(importPath, "\"'`")
    if strings.HasPrefix(importPath, ".") {
        dir := filepath.Dir(currentFile)
        resolved := filepath.Join(dir, importPath)
        // Add extension if missing
        if filepath.Ext(resolved) == "" {
            resolved += ".ts"
        }
        return resolved
    }
    return importPath
}

func resolveTargetID(symbol string, imports map[string]string, currentFile string) string {
	if resolved, ok := imports[symbol]; ok {
		return resolved
	}
	return fmt.Sprintf("%s:%s", currentFile, symbol)
}

func findEnclosingFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "function_declaration" || t == "generator_function_declaration" || t == "method_definition" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		if t == "arrow_function" || t == "function_expression" {
			 if curr.Parent() != nil && curr.Parent().Type() == "variable_declarator" {
				 nameNode := curr.Parent().ChildByFieldName("name")
				 if nameNode != nil {
					 return nameNode.Content(content)
				 }
			 }
		}
		curr = curr.Parent()
	}
	return ""
}
