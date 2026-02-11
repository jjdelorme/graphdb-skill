package snippet

import (
	"os"
	"testing"
)

func TestSliceFile(t *testing.T) {
	// Create a temp file for testing
	content := `line 1
line 2
line 3
line 4
line 5`
	tmpfile, err := os.CreateTemp("", "snippet_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		start     int
		end       int
		want      string
		wantErr   bool
	}{
		{"full range", 1, 5, "line 1\nline 2\nline 3\nline 4\nline 5", false},
		{"partial range", 2, 4, "line 2\nline 3\nline 4", false},
		{"single line", 3, 3, "line 3", false},
		{"out of bounds end", 4, 10, "line 4\nline 5", false},
		{"out of bounds start", 10, 15, "", false},
		{"invalid range", 5, 2, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SliceFile(tmpfile.Name(), tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Errorf("SliceFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SliceFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindPatternInScope(t *testing.T) {
	content := `line 1
line 2: target
line 3
line 4: target
line 5`
	
	tests := []struct {
		name         string
		pattern      string
		context      int
		startOffset  int
		wantMatches  int
		wantLines    int // lines in the first match
	}{
		{"no context", "target", 0, 1, 2, 1},
		{"with context", "target", 1, 1, 2, 3},
		{"start offset", "target", 0, 10, 2, 1},
		{"overlap context", "target", 2, 1, 2, 4},
		{"not found", "missing", 1, 1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := FindPatternInScope(content, tt.pattern, tt.context, tt.startOffset)
			if err != nil {
				t.Errorf("FindPatternInScope() error = %v", err)
				return
			}
			if len(matches) != tt.wantMatches {
				t.Errorf("FindPatternInScope() got %d matches, want %d", len(matches), tt.wantMatches)
				return
			}
			if tt.wantMatches > 0 && len(matches[0].Lines) != tt.wantLines {
				t.Errorf("FindPatternInScope() first match got %d lines, want %d", len(matches[0].Lines), tt.wantLines)
			}
		})
	}
}
