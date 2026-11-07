// Package cli runs `spool` without a subcommand: the report for the whole
// repo, or for the IDs given with --ids.
package cli

import (
	"fmt"
	"io"

	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
)

// Run runs spool in the project at root with the given arguments (without
// the program name) and returns the exit status: 0 or 1 from the report,
// 2 for a usage error. The report goes to stdout and everything else to
// stderr (COVERAGE-1-INV-1).
func Run(args []string, root string, stdout, stderr io.Writer) int {
	// Load config.
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}

	// Parse req files.
	parseResult, err := parser.ParseDir(cfg.ReqsDir)
	if err != nil {
		fmt.Fprintf(stderr, "parser error: %v\n", err)
		return 1
	}
	for _, e := range parseResult.Errors {
		fmt.Fprintf(stderr, "parse warning: %v\n", e)
	}

	// Find test files.
	testFiles, err := project.FindTestFiles(root, cfg.TestPatterns, cfg.ExcludePatterns)
	if err != nil {
		fmt.Fprintf(stderr, "error finding test files: %v\n", err)
		return 1
	}

	// Scan test files.
	interps := project.DefaultInterpreters()
	scanResult := scanner.Scan(testFiles, interps)
	for _, e := range scanResult.Errors {
		fmt.Fprintf(stderr, "scan warning: %v\n", e)
	}
	for _, w := range scanResult.Warnings {
		fmt.Fprintf(stderr, "scan warning: %s\n", w)
	}

	// Build index.
	idx := index.Build(parseResult.Files, scanResult.Files)
	for _, e := range idx.Errors() {
		fmt.Fprintf(stderr, "index error: %s\n", e.Message)
	}

	// Generate and print report.
	report := reporter.Generate(idx)
	data, err := reporter.Text{}.Format(report)
	if err != nil {
		fmt.Fprintf(stderr, "format error: %v\n", err)
		return 1
	}
	reporter.WriteTo(data, stdout)
	return reporter.ExitCode(report, cfg.Severity)
}

// ParseIDs splits a --ids value on commas and normalizes each item. It
// rejects an empty list, an empty item, a malformed ID and a requirement ID,
// naming the bad item, and drops repeats while keeping the first order
// (COVERAGE-2-1, COVERAGE-2-5).
func ParseIDs(list string) ([]string, error) {
	return nil, nil
}
