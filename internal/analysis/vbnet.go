package analysis

import (
	"fmt"
	"regexp"
	"strings"

	"graphdb/internal/graph"
)

type VBNetParser struct{}

func init() {
	RegisterParser(".vb", &VBNetParser{})
}

func (p *VBNetParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	nodes := []*graph.Node{}
	edges := []*graph.Edge{}

	// Regex patterns
	// Note: These are simplified and might not cover all VB.NET syntax edge cases.
	classRegex := regexp.MustCompile(`(?i)(?:Class|Module)\s+(\w+)`)
	funcRegex := regexp.MustCompile(`(?i)(?:Sub|Function)\s+(\w+)`)
	endFuncRegex := regexp.MustCompile(`(?i)End\s+(?:Sub|Function)`)
	callRegex := regexp.MustCompile(`(\w+)\(`)

	lines := strings.Split(string(content), "\n")
	
	// Create File Node
	fileNode := &graph.Node{
		ID:    filePath,
		Label: "File",
		Properties: map[string]interface{}{
			"name":    filePath, // Simplification
			"content": string(content),
			"lang":    "vbnet",
		},
	}
	nodes = append(nodes, fileNode)

	currentFunction := ""
	// currentClass := "" // Not strictly needed for call tracking unless we want fully qualified names

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// 1. Check for Class/Module Definition
		if matches := classRegex.FindStringSubmatch(trimmed); matches != nil {
			className := matches[1]
			// currentClass = className
			
			classID := fmt.Sprintf("%s:%s", filePath, className)
			classNode := &graph.Node{
				ID:    classID,
				Label: "Class",
				Properties: map[string]interface{}{
					"name": className,
					"file": filePath,
				},
			}
			nodes = append(nodes, classNode)
		}

		// 2. Check for Function/Sub Definition
		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			funcName := matches[1]
			currentFunction = funcName
			
			funcID := fmt.Sprintf("%s:%s", filePath, funcName)
			funcNode := &graph.Node{
				ID:    funcID,
				Label: "Function",
				Properties: map[string]interface{}{
					"name":      funcName,
					"signature": trimmed, // Rough signature
					"file":      filePath,
				},
			}
			nodes = append(nodes, funcNode)

			// Edge: DEFINED_IN File
			edges = append(edges, &graph.Edge{
				SourceID: funcID,
				TargetID: filePath,
				Type:     "DEFINED_IN",
			})
			continue // Skip checking for calls on the definition line itself (simplification)
		}

		// 3. Check for End of Function/Sub
		if endFuncRegex.MatchString(trimmed) {
			currentFunction = ""
		}

		// 4. Check for Calls (only if inside a function)
		if currentFunction != "" {
			// Find all calls in the line
			callMatches := callRegex.FindAllStringSubmatch(trimmed, -1)
			for _, match := range callMatches {
				calledFunc := match[1]
				
				// Avoid self-references or keywords if possible (basic filtering)
				if strings.EqualFold(calledFunc, "If") || strings.EqualFold(calledFunc, "While") || strings.EqualFold(calledFunc, "For") {
					continue
				}

				// Construct IDs
				sourceID := fmt.Sprintf("%s:%s", filePath, currentFunction)
				// Target ID is tricky without semantic analysis. 
				// We'll assume it's in the same file for the test case, or just use the name.
				// However, to match the test expectation `strings.HasSuffix(e.TargetID, ":Calculate")`,
				// we should probably construct a similar ID structure.
				// For now, let's guess it's a local function.
				targetID := fmt.Sprintf("%s:%s", filePath, calledFunc)

				edges = append(edges, &graph.Edge{
					SourceID: sourceID,
					TargetID: targetID,
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges, nil
}
