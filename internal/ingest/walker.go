package ingest

import (
	"context"
	"graphdb/internal/storage"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type Walker struct {
	WorkerPool *WorkerPool
}

func NewWalker(workers int, emitter storage.Emitter) *Walker {
	return &Walker{
		WorkerPool: NewWorkerPool(workers, emitter),
	}
}

func (w *Walker) Run(ctx context.Context, dirPath string) error {
	w.WorkerPool.Start()
	defer w.WorkerPool.Stop()

	return w.Walk(ctx, dirPath, func(path string, d fs.DirEntry) error {
		if !d.IsDir() {
			w.WorkerPool.Submit(dirPath, path)
		}
		return nil
	})
}

func (w *Walker) Count(ctx context.Context, dirPath string) (int64, error) {
	var count int64
	err := w.Walk(ctx, dirPath, func(path string, d fs.DirEntry) error {
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

func (w *Walker) Walk(ctx context.Context, rootPath string, visitor func(path string, d fs.DirEntry) error) error {
	return w.walkRecursiveWithPatterns(ctx, rootPath, rootPath, []string{}, visitor)
}

func (w *Walker) walkRecursiveWithPatterns(ctx context.Context, rootPath string, currentPath string, patterns []string, visitor func(path string, d fs.DirEntry) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check for local .gitignore and extend patterns if found
	gitignorePath := filepath.Join(currentPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		content, err := os.ReadFile(gitignorePath)
		if err == nil {
			// Parse and rewrite patterns
			lines := strings.Split(string(content), "\n")
			relDir, _ := filepath.Rel(rootPath, currentPath)
			relDir = filepath.ToSlash(relDir)
			
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}

				// Handle negation
				isNegated := strings.HasPrefix(trimmed, "!")
				if isNegated {
					trimmed = trimmed[1:]
				}

				// Handle root-relative anchor
				if strings.HasPrefix(trimmed, "/") {
					trimmed = trimmed[1:]
				}

				// Prepend current directory relative to root
				if relDir != "." {
					trimmed = filepath.ToSlash(filepath.Join(relDir, trimmed))
				}

				// Restore negation
				if isNegated {
					trimmed = "!" + trimmed
				}

				patterns = append(patterns, trimmed)
			}
		}
	}

	// Build a single matcher for the current context (if any patterns exist)
	var matcher gitignore.IgnoreMatcher
	if len(patterns) > 0 {
		matcher = gitignore.NewGitIgnoreFromReader(rootPath, strings.NewReader(strings.Join(patterns, "\n")))
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return err
	}

	for _, d := range entries {
		fullPath := filepath.Join(currentPath, d.Name())
		isDir := d.IsDir()

		// Always ignore the internal .git directory to prevent indexing VCS metadata
		if isDir && d.Name() == ".git" {
			continue
		}

		// Dynamically ignore nested git repositories or worktrees (which contain their own .git entry)
		if isDir {
			if _, err := os.Stat(filepath.Join(fullPath, ".git")); err == nil {
				continue
			}
		}

		// Check ignored status
		if matcher != nil {
			if matcher.Match(fullPath, isDir) {
				continue
			}
		}

		// Visit the file or directory
		err = visitor(fullPath, d)
		if err == filepath.SkipDir {
			continue
		}
		if err != nil {
			return err
		}

		// Recurse if it's a directory
		if isDir {
			// Pass a copy of patterns to avoid side effects (though append returns new slice if capacity exceeded, 
			// it's safer to rely on slice value semantics in recursion)
			// Actually, 'patterns' is a new slice in this scope (value passed), but the underlying array might be shared.
			// However, since we appended above, 'patterns' in this scope points to a new array or the same. 
			// In the recursive call, we want the *current* accumulated patterns.
			// The next level will append to it. 
			// We should pass 'patterns' as is. 
			// Wait, if next level appends, does it affect this level's future usage?
			// No, because we don't use 'patterns' after the loop.
			// But for siblings in the loop? 
			// We iterate 'entries'. For each directory 'd', we call recurse.
			// 'patterns' variable here includes patterns from 'currentPath/.gitignore'.
			// This is correct for all children of 'currentPath'.
			
			err = w.walkRecursiveWithPatterns(ctx, rootPath, fullPath, patterns, visitor)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
