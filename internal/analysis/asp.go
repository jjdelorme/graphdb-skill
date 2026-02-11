package analysis

import (
	"path/filepath"
	"regexp"
	"strings"

	"graphdb/internal/graph"
)

type AspParser struct{}

func init() {
	p := &AspParser{}
	RegisterParser(".asp", p)
	RegisterParser(".aspx", p)
	RegisterParser(".ascx", p)
}

// Regex for code blocks
var (
	// Matches <% ... %>
	// We use (?s) for dot matches newline
	reAspBlock = regexp.MustCompile(`(?s)<%.*?%>`)
	
	// Matches <script runat="server"> ... </script>
	// We use (?si) for case insensitive tag matching
	reScriptBlock = regexp.MustCompile(`(?si)<script[^>]*runat=["']server["'][^>]*>(.*?)</script>`)
	
	// Matches language directive
	// <%@ Language="VBScript" %> or <%@ Page Language="C#" %>
	reLangDirective = regexp.MustCompile(`(?i)<%@\s*(?:Page\s+)?Language=["']([^"']+)["']`)
)

func (p *AspParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	lang := p.detectLanguage(filePath, content)
	
	maskedContent := p.maskHTML(content)

	if lang == "vb" {
		// VB.NET needs a Module wrapper for the parser to work correctly (if it expects full file structure)
		// Our current VB parser is Regex based, so it might not strictly need it, but the plan says to add it.
		// Also we need to handle line offsets.
		
		// The plan says: prepend "Module AspWrapper\n" and append "\nEnd Module"
		// This shifts everything by 1 line.
		
		wrappedContent := "Module AspWrapper\n" + string(maskedContent) + "\nEnd Module"
		
		// Get VB Parser
		vbParser := &VBNetParser{}
		nodes, edges, err := vbParser.Parse(filePath, []byte(wrappedContent))
		if err != nil {
			return nil, nil, err
		}
		
		// Adjust line numbers: -1
		for _, n := range nodes {
			if line, ok := n.Properties["line"].(int); ok {
				n.Properties["line"] = line - 1
			}
		}
		
		return nodes, edges, nil
	} else {
		// C#
		csParser := &CSharpParser{}
		return csParser.Parse(filePath, maskedContent)
	}
}

func (p *AspParser) detectLanguage(filePath string, content []byte) string {
	// 1. Check directive
	matches := reLangDirective.FindSubmatch(content)
	if len(matches) > 1 {
		lang := strings.ToLower(string(matches[1]))
		if lang == "c#" || lang == "cs" || lang == "csharp" {
			return "cs"
		}
		if lang == "vb" || lang == "vbscript" || lang == "vbs" || lang == "vb.net" {
			return "vb"
		}
	}

	// 2. Check extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".aspx" || ext == ".ascx" {
		return "cs" // Default for .NET web forms usually
	}
	
	return "vb" // Default for .asp
}

func (p *AspParser) maskHTML(content []byte) []byte {
	// Strategy:
	// 1. Create a buffer of spaces/newlines matching the original content length
	// 2. Iterate through "Code Blocks" and copy the code into the buffer at the correct positions.
	// This ensures everything outside code blocks is masked, and line numbers are preserved.
	
	originalStr := string(content)
	masked := make([]byte, len(content))
	
	// Initialize with spaces and original newlines
	for i, b := range content {
		if b == '\n' || b == '\r' {
			masked[i] = b
		} else {
			masked[i] = ' '
		}
	}
	
	// Helper to copy slice
	copyRange := func(start, end int) {
		if start < 0 || end > len(content) || start >= end {
			return
		}
		copy(masked[start:end], content[start:end])
	}
	
	// Find <% ... %> blocks
	// FindAllIndex returns [[start, end], [start, end], ...]
	aspBlocks := reAspBlock.FindAllStringIndex(originalStr, -1)
	for _, loc := range aspBlocks {
		start, end := loc[0], loc[1]
		
		// If it is a directive <%@ ... %>, we treat it as NOT code (mask it), 
		// because C# parser doesn't know what `@ Page` means. 
		if strings.HasPrefix(originalStr[start:], "<%@") {
			continue // Keep it masked (spaces)
		}
		
		// Copy the content inside delimiters
		// Check for `<%=` (expression) or `<%` (code)
		innerStart := start + 2
		innerEnd := end - 2
		
		if strings.HasPrefix(originalStr[start:], "<%=") {
			innerStart = start + 3
		}
		
		copyRange(innerStart, innerEnd)
	}
	
	// Find <script runat="server"> ... </script> blocks
	// The regex `reScriptBlock` captures the inner content in group 1.
	// FindAllStringSubmatchIndex returns [start, end, g1Start, g1End, ...]
	scriptBlocks := reScriptBlock.FindAllStringSubmatchIndex(originalStr, -1)
	for _, loc := range scriptBlocks {
		if len(loc) >= 4 {
			// loc[2] is start of group 1, loc[3] is end of group 1
			copyRange(loc[2], loc[3])
		}
	}
	
	return masked
}
