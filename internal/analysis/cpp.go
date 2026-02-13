package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Local definitions map: Name -> ID
	localDefs := make(map[string]string)
	// Includes list
	includes := []string{}

	// 1. Structure Query: Definitions, Classes, Includes
	structureQueryStr := `
		(function_definition
			declarator: (function_declarator
				declarator: (identifier) @function.name
			)
		)
		(function_definition
			declarator: (function_declarator
				declarator: (field_identifier) @function.name
			)
		)
		(function_definition
			declarator: (function_declarator
				declarator: (qualified_identifier) @function.name
			)
		)

		(translation_unit
			(declaration
				declarator: (init_declarator
					declarator: (identifier) @global.name
				)
			)
		)
		(translation_unit
			(declaration
				declarator: (identifier) @global.name
			)
		)

		(field_declaration
			declarator: (field_identifier) @field.name
		)

		(class_specifier
			name: (type_identifier) @class.name
		)

		(preproc_include
			path: (string_literal) @include.path
		)
		(preproc_include
			path: (system_lib_string) @include.system
		)
	`
	qStruct, err := sitter.NewQuery([]byte(structureQueryStr), cpp.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid structure query: %w", err)
	}
	defer qStruct.Close()

	qcStruct := sitter.NewQueryCursor()
	defer qcStruct.Close()
	qcStruct.Exec(qStruct, tree.RootNode())

	for {
		m, ok := qcStruct.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			name := qStruct.CaptureNameForId(c.Index)
			nodeContent := c.Node.Content(content)
			
			// Normalize include paths
			if name == "include.path" || name == "include.system" {
				// Remove quotes or brackets
				incPath := strings.Trim(nodeContent, "\"<>")
				includes = append(includes, incPath)
				continue
			}

			var label string
			if name == "function.name" {
				label = "Function"
			} else if name == "global.name" {
				label = "Global"
			} else if name == "field.name" {
				label = "Field"
			} else if name == "class.name" {
				label = "Class"
			} else {
				continue
			}

			nodeID := fmt.Sprintf("%s:%s", filePath, nodeContent)
			nodes = append(nodes, &graph.Node{
				ID:    nodeID,
				Label: label,
				Properties: map[string]interface{}{
					"name": nodeContent,
					"file": filePath,
					"line": c.Node.StartPoint().Row + 1,
				},
			})
			localDefs[nodeContent] = nodeID
		}
	}

	// 2. Inheritance Query
	inheritQueryStr := `
		(class_specifier
			name: (type_identifier) @src
			(base_class_clause
				(type_identifier) @dst
			)
		)
		(class_specifier
			name: (type_identifier) @src
			(base_class_clause
				(_ (type_identifier) @dst)
			)
		)
	`
	qInherit, err := sitter.NewQuery([]byte(inheritQueryStr), cpp.GetLanguage())
	if err != nil {
		// Log error but continue? Or fail.
		// Fail is safer for now.
		return nodes, edges, fmt.Errorf("invalid inheritance query: %w", err)
	}
	defer qInherit.Close()

	qcInherit := sitter.NewQueryCursor()
	defer qcInherit.Close()
	qcInherit.Exec(qInherit, tree.RootNode())

	for {
		m, ok := qcInherit.NextMatch()
		if !ok {
			break
		}
		var src, dst string
		for _, c := range m.Captures {
			name := qInherit.CaptureNameForId(c.Index)
			if name == "src" {
				src = c.Node.Content(content)
			} else if name == "dst" {
				dst = c.Node.Content(content)
			}
		}
		if src != "" && dst != "" {
			// Try to resolve dst locally, else assume header/external
			// But for INHERITS, we usually want to link to the Class node if we found it.
			// If not, we still create the edge to a potential ID.
			
			// Check local
			targetID := localDefs[dst]
			if targetID == "" {
				// Resolve from includes
				targetID = resolveFromIncludes(dst, includes, filePath)
			}

			edges = append(edges, &graph.Edge{
				SourceID: fmt.Sprintf("%s:%s", filePath, src),
				TargetID: targetID,
				Type:     "INHERITS",
			})
		}
	}

	// 3. Usage/Reference Query
	// Captures calls and variable usages
	usageQueryStr := `
		(call_expression
			function: (identifier) @call.target
		) @call.site

		(call_expression
			function: (field_expression field: (field_identifier) @call.target)
		) @call.site
		
		(call_expression
			function: (qualified_identifier) @call.target
		) @call.site

		(assignment_expression
			left: (identifier) @usage.write
		) @usage.site

		(assignment_expression
			right: (identifier) @usage.read
		) @usage.site
		
		(binary_expression
			left: (identifier) @usage.read
		) @usage.site

		(binary_expression
			right: (identifier) @usage.read
		) @usage.site
		
		(unary_expression
			argument: (identifier) @usage.read
		) @usage.site

		(update_expression
			argument: (identifier) @usage.write
		) @usage.site
        
        (field_expression
            field: (field_identifier) @usage.read
        ) @usage.site
	`
	qUsage, err := sitter.NewQuery([]byte(usageQueryStr), cpp.GetLanguage())
	if err != nil {
		return nodes, edges, fmt.Errorf("invalid usage query: %w", err)
	}
	defer qUsage.Close()

	qcUsage := sitter.NewQueryCursor()
	defer qcUsage.Close()
	qcUsage.Exec(qUsage, tree.RootNode())

	for {
		m, ok := qcUsage.NextMatch()
		if !ok {
			break
		}
		
		var targetName string
		var siteNode *sitter.Node
		var edgeType string = "USES" // default

		for _, c := range m.Captures {
			name := qUsage.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
				edgeType = "CALLS"
			} else if name == "usage.read" || name == "usage.write" {
				targetName = c.Node.Content(content)
				edgeType = "USES"
			} else if name == "call.site" || name == "usage.site" {
				siteNode = c.Node
			}
		}

		if targetName != "" && siteNode != nil {
			sourceFunc := findEnclosingCppFunction(siteNode, content)
			if sourceFunc != "" {
				// Resolve Target
				targetID := localDefs[targetName]
				
				// Handle qualified names (e.g. Math::Add)
				if targetID == "" && strings.Contains(targetName, "::") {
					// Check if the qualification matches a known class or namespace?
					// Or just treat the whole thing as a symbol to resolve against includes
				}

				if targetID == "" {
					targetID = resolveFromIncludes(targetName, includes, filePath)
				}

				edges = append(edges, &graph.Edge{
					SourceID: fmt.Sprintf("%s:%s", filePath, sourceFunc),
					TargetID: targetID,
					Type:     edgeType,
				})
			}
		}
	}

	return nodes, edges, nil
}

// resolveFromIncludes attempts to map a symbol to an included file
func resolveFromIncludes(symbol string, includes []string, currentFile string) string {
	// Simple heuristic: 
	// 1. If symbol contains "::", the prefix might match a header file name.
	// 2. Or just map to the first include that 'looks like' the symbol.
	// 3. Fallback: Unknown external
	
	// Check if symbol matches a header name (case insensitive)
	// e.g. Math -> math.h
	// e.g. Math::Add -> math.h
	
	symbolBase := symbol
	if idx := strings.Index(symbol, "::"); idx != -1 {
		symbolBase = symbol[:idx]
	}

	for _, inc := range includes {
		// inc is "math.h" or "vector"
		base := filepath.Base(inc)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		
		if strings.EqualFold(name, symbolBase) {
			// Found a potential match
			// Construct an ID that represents the external definition
			// Ideally this matches the ID that would be generated if we parsed that file.
			// Parser generates: filePath:SymbolName
			// So we try to construct: resolvedIncludePath:Symbol
			
			// Since we don't have the absolute path of the include here easily without resolving against include paths...
			// We will make a best effort relative resolution or just use the include string.
			
			// Resolve relative to current file if starts with quote (not system)
			// But for now, let's just use the include path as the file prefix.
			// This might not match exactly if the include is relative, but it's "Import-Inferred Linking"
			
			dir := filepath.Dir(currentFile)
			// Naive join - in reality we should check if file exists
			resolvedPath := filepath.Join(dir, inc) 
			
			return fmt.Sprintf("%s:%s", resolvedPath, symbol)
		}
	}
	
	// Fallback: If it looks like a system include or we can't find it, 
	// we create a "Ghost" node ID but maybe marked as external?
	// The prompt says "link to UNRESOLVED:List" or "potential header file-based ID".
	
	return fmt.Sprintf("UNKNOWN:%s", symbol)
}

func findEnclosingCppFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "function_definition" {
			decl := curr.ChildByFieldName("declarator")
			if decl != nil {
				innerDecl := decl.ChildByFieldName("declarator")
				if innerDecl != nil {
					if innerDecl.Type() == "identifier" || innerDecl.Type() == "field_identifier" {
						return innerDecl.Content(content)
					}
					// Handle qualified identifier (Class::Func)
					if innerDecl.Type() == "qualified_identifier" {
						// We probably just want the name part or the full qualified name?
						// Let's take the full content for uniqueness
						return innerDecl.Content(content)
					}
				}
			}
		}
		curr = curr.Parent()
	}
	return ""
}
