package snippet

import (
	"bufio"
	"os"
	"strings"
)

// Line represents a single line of code with its line number.
type Line struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

// Match represents a collection of lines (match + context).
type Match struct {
	Lines []Line `json:"lines"`
}

// SliceFile reads a file and returns lines between start and end (inclusive, 1-based).
func SliceFile(path string, startLine, endLine int) (string, error) {
	if startLine > endLine {
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		if currentLine >= startLine && currentLine <= endLine {
			lines = append(lines, scanner.Text())
		}
		if currentLine > endLine {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}

// FindPatternInScope searches content for pattern and returns matches with context.
// startLineOffset is the line number of the first line in content.
func FindPatternInScope(content, pattern string, contextLines int, startLineOffset int) ([]Match, error) {
	lines := strings.Split(content, "\n")
	var matches []Match

	for i, line := range lines {
		if strings.Contains(line, pattern) {
			match := Match{
				Lines: []Line{},
			}

			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= len(lines) {
				end = len(lines) - 1
			}

			for j := start; j <= end; j++ {
				match.Lines = append(match.Lines, Line{
					Number:  startLineOffset + j,
					Content: lines[j],
				})
			}
			matches = append(matches, match)
		}
	}

	return matches, nil
}
