// Package scanner detects test annotations and associates them with test functions.
package scanner

import (
	"fmt"
	"os"
	"strings"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/spoolid"
)

// Annotation is a detected Spool ID annotation in a source file.
type Annotation struct {
	ID   string
	Line int
}

// TestMapping associates a test function with its Spool annotations.
type TestMapping struct {
	Function    spool.TestFunction
	Annotations []Annotation
}

// FileResult holds the scan results for a single file.
type FileResult struct {
	Path     string
	Mappings []TestMapping
	Orphaned []Annotation
}

// FileError records a scan failure for a specific file.
type FileError struct {
	Path string
	Err  error
}

func (e FileError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

// ScanResult holds the results of scanning multiple files.
type ScanResult struct {
	Files    []FileResult
	Errors   []FileError
	Warnings []string
}

// Scan reads each file, detects annotations, extracts test functions via
// the first matching interpreter, and associates annotations with functions.
func Scan(files []string, interpreters []spool.Interpreter) *ScanResult {
	result := &ScanResult{}

	for _, path := range files {
		interp := findInterpreter(path, interpreters)
		if interp == nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("no interpreter for %s", path))
			continue
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, FileError{Path: path, Err: err})
			continue
		}

		funcs, err := interp.ExtractTestFunctions(path, contents)
		if err != nil {
			result.Errors = append(result.Errors, FileError{Path: path, Err: err})
			continue
		}

		anns := detectAnnotations(contents)
		mappings, orphaned := associate(anns, funcs)

		result.Files = append(result.Files, FileResult{
			Path:     path,
			Mappings: mappings,
			Orphaned: orphaned,
		})
	}

	return result
}

// commentPrefixes are the comment styles the scanner recognizes.
var commentPrefixes = []string{"//", "#"}

// detectAnnotations finds Spool ID annotations in source file contents.
func detectAnnotations(contents []byte) []Annotation {
	var anns []Annotation
	for i, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				body := strings.TrimSpace(trimmed[len(prefix):])
				candidate := body
				if idx := strings.Index(body, ":"); idx > 0 {
					candidate = strings.TrimSpace(body[:idx])
				}
				if spoolid.IsValid(candidate) {
					normalized, _ := spoolid.NormalizeID(candidate)
					anns = append(anns, Annotation{ID: normalized, Line: i + 1})
				}
				break
			}
		}
	}
	return anns
}

// associate pairs annotations with the test function that follows them.
// Annotations with no following function are returned as orphaned.
func associate(anns []Annotation, funcs []spool.TestFunction) ([]TestMapping, []Annotation) {
	var mappings []TestMapping
	var pending []Annotation
	ai := 0

	for _, fn := range funcs {
		// Collect all annotations before this function.
		for ai < len(anns) && anns[ai].Line < fn.Line {
			pending = append(pending, anns[ai])
			ai++
		}
		// Attach pending annotations to this function.
		mappings = append(mappings, TestMapping{
			Function:    fn,
			Annotations: pending,
		})
		pending = nil
	}

	// Any remaining annotations are orphaned.
	orphaned := append(pending, anns[ai:]...)
	return mappings, orphaned
}

// findInterpreter returns the first interpreter that supports the given filename.
func findInterpreter(filename string, interpreters []spool.Interpreter) spool.Interpreter {
	for _, interp := range interpreters {
		if interp.Supports(filename) {
			return interp
		}
	}
	return nil
}
