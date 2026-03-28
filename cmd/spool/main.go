package main

import (
	"fmt"
	"os"

	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	spoolsp "github.com/mwames/Spool/internal/lsp"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		spoolsp.Run()
		return
	}

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
	testFiles, err := project.FindTestFiles(root, cfg.TestPatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding test files: %v\n", err)
		os.Exit(1)
	}

	// Scan test files.
	interps := project.DefaultInterpreters()
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
