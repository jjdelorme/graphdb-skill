package analysis

import (
	"context"
	"fmt"
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

	// 1. Definition Query
	defQueryStr := `
		(function_declaration name: (identifier) @function.name) @function.def
		(generator_function_declaration name: (identifier) @function.name) @function.def
		(method_definition name: (property_identifier) @method.name) @method.def
		(class_declaration name: (type_identifier) @class.name) @class.def
		(interface_declaration name: (type_identifier) @class.name) @class.def
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
            } else if strings.HasPrefix(captureName, "function") || strings.HasPrefix(captureName, "method") {
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

    // 2. Reference/Call Query
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
            sourceFunc := findEnclosingFunction(callNode, content)
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
