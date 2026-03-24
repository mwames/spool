package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
	"github.com/mwames/Spool/internal/scanner/interpreters"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Load config.
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Parse req files.
	parseResult, err := parser.ParseDir(cfg.ReqsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parser error: %v\n", err)
		os.Exit(1)
	}
	for _, e := range parseResult.Errors {
		fmt.Fprintf(os.Stderr, "parse warning: %v\n", e)
	}

	// Find test files.
	testFiles, err := findTestFiles(root, cfg.TestPatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding test files: %v\n", err)
		os.Exit(1)
	}

	// Scan test files.
	interps := defaultInterpreters()
	scanResult := scanner.Scan(testFiles, interps)
	for _, e := range scanResult.Errors {
		fmt.Fprintf(os.Stderr, "scan warning: %v\n", e)
	}
	for _, w := range scanResult.Warnings {
		fmt.Fprintf(os.Stderr, "scan warning: %s\n", w)
	}

	// Build index.
	idx := index.Build(parseResult.Files, scanResult.Files)
	for _, e := range idx.Errors() {
		fmt.Fprintf(os.Stderr, "index error: %s\n", e.Message)
	}

	// Generate and print report.
	report := reporter.Generate(idx)
	data, err := reporter.Text{}.Format(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "format error: %v\n", err)
		os.Exit(1)
	}
	reporter.Write(data, "")
}

func defaultInterpreters() []spool.Interpreter {
	return []spool.Interpreter{
		interpreters.Playwright{},
		interpreters.Go{},
		interpreters.JUnit{},
		interpreters.Jest{},
	}
}

func findTestFiles(root string, patterns []string) ([]string, error) {
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
			if matchPattern(rel, pattern) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	return files, err
}

// matchPattern matches a file path against a glob pattern.
// Supports ** as a recursive directory wildcard.
func matchPattern(path, pattern string) bool {
	// Simple ** support: split pattern on ** and match segments.
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
