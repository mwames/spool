// Package cli runs `spool` without a subcommand: the report for the whole
// repo, or for the IDs given with --ids.
package cli

import (
	"fmt"
	"io"
	"strings"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/lint"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
	"github.com/mwames/Spool/internal/spoolid"
)

// Run runs spool in the project at root with the given arguments (without
// the program name) and returns the exit status: 0 or 1 from the report,
// 2 for a usage error. The report goes to stdout and everything else to
// stderr (COVERAGE-1-INV-1).
func Run(args []string, root string, stdout, stderr io.Writer) int {
	format, ids, haveIDs, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "usage error: %v\n", err)
		return 2
	}

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

	if haveIDs {
		cov := reporter.Coverage(idx, root, ids)
		var data []byte
		if format == "json" {
			data, err = reporter.CoverageJSON(cov)
			if err != nil {
				fmt.Fprintf(stderr, "format error: %v\n", err)
				return 1
			}
		} else {
			data = reporter.CoverageText(cov)
		}
		reporter.WriteTo(data, stdout)
		return reporter.CoverageExitCode(cov)
	}

	// Generate and print report.
	report := reporter.Generate(idx)
	report.MissingInterfaces = lint.MissingInterfaces(idx, root)
	report.NotGivenWhenThen = lint.NotGivenWhenThen(idx)
	var f spool.Formatter = reporter.Text{}
	switch format {
	case "json":
		f = reporter.JSON{}
	case "markdown":
		f = reporter.Markdown{}
	}
	data, err := f.Format(report)
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
	if list == "" {
		return nil, fmt.Errorf("empty ID list")
	}
	var ids []string
	seen := map[string]bool{}
	for _, item := range strings.Split(list, ",") {
		if item == "" {
			return nil, fmt.Errorf("empty item in ID list %q", list)
		}
		id, err := spoolid.NormalizeID(item)
		if err != nil {
			return nil, fmt.Errorf("bad ID %q: not an AC or invariant ID", item)
		}
		if strings.Count(id, "-") < 2 {
			return nil, fmt.Errorf("bad ID %q: a requirement ID, not an AC or invariant ID", item)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// parseArgs reads the --format and --ids flags, each as "--flag value" or
// "--flag=value", in any order (COVERAGE-1, COVERAGE-2).
func parseArgs(args []string) (format string, ids []string, haveIDs bool, err error) {
	format = "text"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return "", nil, false, fmt.Errorf("unexpected argument %q", arg)
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--format" && name != "--ids" {
			return "", nil, false, fmt.Errorf("unknown flag %q", name)
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", nil, false, fmt.Errorf("flag %s needs a value", name)
			}
			i++
			value = args[i]
		}
		if name == "--format" {
			if value != "text" && value != "json" && value != "markdown" {
				return "", nil, false, fmt.Errorf("unknown format %q", value)
			}
			format = value
			continue
		}
		if ids, err = ParseIDs(value); err != nil {
			return "", nil, false, err
		}
		haveIDs = true
	}
	if haveIDs && format == "markdown" {
		return "", nil, false, fmt.Errorf("format markdown is not available with --ids")
	}
	return format, ids, haveIDs, nil
}
