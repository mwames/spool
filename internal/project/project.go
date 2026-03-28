// Package project provides shared helpers for discovering test files and interpreters.
package project

import (
	"os"
	"path/filepath"
	"strings"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/scanner/interpreters"
)

// DefaultInterpreters returns the standard set of test file interpreters.
func DefaultInterpreters() []spool.Interpreter {
	return []spool.Interpreter{
		interpreters.Playwright{},
		interpreters.Go{},
		interpreters.JUnit{},
		interpreters.Jest{},
	}
}

// FindTestFiles walks root and returns paths matching any of the given glob patterns.
func FindTestFiles(root string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, pattern := range patterns {
			if MatchPattern(rel, pattern) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	return files, err
}

// MatchPattern matches a file path against a glob pattern.
// Supports ** as a recursive directory wildcard.
func MatchPattern(path, pattern string) bool {
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		suffix := strings.TrimPrefix(parts[1], "/")
		if suffix == "" {
			return true
		}
		matched, _ := filepath.Match(suffix, filepath.Base(path))
		return matched
	}
	matched, _ := filepath.Match(pattern, path)
	return matched
}
