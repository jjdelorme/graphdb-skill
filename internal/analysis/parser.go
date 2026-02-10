package analysis

import "graphdb/internal/graph"

// LanguageParser defines the interface for parsing source code files.
type LanguageParser interface {
	Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error)
}

var parsers = make(map[string]LanguageParser)

// RegisterParser registers a parser for a specific file extension (e.g., ".go").
func RegisterParser(ext string, p LanguageParser) {
	parsers[ext] = p
}

// GetParser retrieves the parser for the given extension.
func GetParser(ext string) (LanguageParser, bool) {
	p, ok := parsers[ext]
	return p, ok
}
