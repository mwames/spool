// Package spool provides the core types for the Spool requirements language server.
package spool

// TestFunction represents a test function extracted from a source file.
type TestFunction struct {
	Name string
	Line int
}

// Interpreter extracts test functions from source files.
type Interpreter interface {
	Supports(filename string) bool
	ExtractTestFunctions(filename string, contents []byte) ([]TestFunction, error)
}
