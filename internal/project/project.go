// Package project provides shared helpers for discovering test files and interpreters.
package project

import (
	"os"
	"path"
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
		interpreters.Rust{},
	}
}

// FindTestFiles walks root and returns paths matching any of the given glob
// patterns. Directories and files matching an exclude pattern are skipped, as
// is .git.
func FindTestFiles(root string, patterns, excludes []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if d.Name() == ".git" || (rel != "." && matchAny(rel, excludes)) {
				return filepath.SkipDir
			}
			return nil
		}
		if matchAny(rel, patterns) && !matchAny(rel, excludes) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func matchAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if MatchPattern(path, pattern) {
			return true
		}
	}
	return false
}

// MatchPattern matches a path relative to the project root against a glob
// pattern. Each path segment is matched with path.Match, and a ** segment
// matches any number of directories, including none.
func MatchPattern(relPath, pattern string) bool {
	return matchSegments(strings.Split(filepath.ToSlash(relPath), "/"), strings.Split(pattern, "/"))
}

func matchSegments(segs, pats []string) bool {
	if len(pats) == 0 {
		return len(segs) == 0
	}
	if pats[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(segs[i:], pats[1:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, _ := path.Match(pats[0], segs[0]); !ok {
		return false
	}
	return matchSegments(segs[1:], pats[1:])
}
