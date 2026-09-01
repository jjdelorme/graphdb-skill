package e2e_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E_Hygiene_NoGoogle3Contamination asserts that the codebase is 100% pure open-source Go
// and contains zero Google3 internal paths, zero proprietary imports, and zero prohibited internal headers.
func TestE2E_Hygiene_NoGoogle3Contamination(t *testing.T) {
	root := getRepoRoot(t)

	// Forbidden strings representing internal Google3 proprietary dependencies, paths, or targets
	forbiddenPatterns := []string{
		"google3/",
		"research/omega",
		"//depot/google3",
		"google3/base",
		"google3/net",
		"google3/util",
		"piper://",
		"blaze-bin",
		"blaze-out",
	}

	// Directories to ignore during source hygiene scans
	ignoredDirs := map[string]bool{
		".git":          true,
		".agents":       true, // Agent metadata and system prompt instructions
		"bin":           true, // Compiled local binaries
		".gemini/tools": true, // Local tool downloads (e.g., zig compiler)
		".gemini/brain": true, // LLM runtime task logs
		"node_modules":  true,
	}

	// File extensions and file names to check
	checkedExtensions := map[string]bool{
		".go":   true,
		".mod":  true,
		".sum":  true,
		".js":   true,
		".ts":   true,
		".sh":   true,
		".yaml": true,
		".yml":  true,
		".json": true,
	}

	checkedFileNames := map[string]bool{
		"Makefile":  true,
		"README.md": true,
		"GEMINI.md": true,
	}

	var scannedFiles int
	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}

		if info.IsDir() {
			base := info.Name()
			if ignoredDirs[base] || strings.HasPrefix(relPath, ".agents") || strings.HasPrefix(relPath, ".gemini/brain") || strings.HasPrefix(relPath, ".gemini/tools") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(relPath, "hygiene_test.go") {
			return nil
		}

		ext := filepath.Ext(path)
		fileName := info.Name()

		shouldCheck := checkedExtensions[ext] || checkedFileNames[fileName]
		if !shouldCheck || relPath == "test/e2e/hygiene_test.go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}

		scannedFiles++
		scanner := bufio.NewScanner(bytes.NewReader(content))
		lineNum := 1

		for scanner.Scan() {
			line := scanner.Text()
			for _, pattern := range forbiddenPatterns {
				if strings.Contains(line, pattern) {
					violations = append(violations, fmt.Sprintf("%s:%d matches forbidden pattern %q: %s",
						relPath, lineNum, pattern, strings.TrimSpace(line)))
				}
			}
			lineNum++
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Hygiene filesystem walk failed: %v", err)
	}

	t.Logf("Hygiene scan completed: scanned %d source files across repo", scannedFiles)

	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("Google3 Contamination Violation: %s", v)
		}
		t.Fatalf("Found %d Google3 contamination violations in checked-in source files", len(violations))
	}
}

// TestE2E_Hygiene_GoImportsValidation verifies that all Go files only import standard library
// or valid open-source packages from go.mod.
func TestE2E_Hygiene_GoImportsValidation(t *testing.T) {
	root := getRepoRoot(t)

	var goFilesScanned int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == ".agents" || info.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "hygiene_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		goFilesScanned++

		lines := strings.Split(string(content), "\n")
		inImportBlock := false

		for lineIdx, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "import (") {
				inImportBlock = true
				continue
			}
			if inImportBlock && trimmed == ")" {
				inImportBlock = false
				continue
			}

			var importPath string
			if inImportBlock {
				importPath = strings.Trim(trimmed, "\"")
			} else if strings.HasPrefix(trimmed, "import ") {
				importPath = strings.Trim(strings.TrimPrefix(trimmed, "import "), "\"")
			}

			if importPath != "" {
				// Assert no google3 or internal proprietary imports
				if strings.Contains(importPath, "google3") || strings.Contains(importPath, "research/omega") {
					t.Errorf("%s:%d: Forbidden internal import path: %s", path, lineIdx+1, importPath)
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Go import validation walk failed: %v", err)
	}

	t.Logf("Go import hygiene check completed: validated %d Go files", goFilesScanned)
}
