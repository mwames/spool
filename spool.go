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

// Report captures the full traceability summary across all features.
type Report struct {
	TotalRequirements int
	TotalACs          int
	LinkedACs         int
	UntestedACs       int
	OrphanedCount     int
	Features          []FeatureReport
	Untested          []UntestedAC
	Orphaned          []OrphanedRef
}

// FeatureReport captures per-feature traceability totals.
type FeatureReport struct {
	Name                 string
	Requirements         int
	ACs                  int
	LinkedACs            int
	UntestedACs          int
	ExcludedRequirements int
}

// UntestedAC identifies an acceptance criterion with no linked tests.
type UntestedAC struct {
	ID          string
	Description string
	Feature     string
}

// OrphanedRef identifies a test annotation referencing an unknown ID.
type OrphanedRef struct {
	ID   string
	File string
	Line int
}

// Formatter formats a Report into a specific output format.
type Formatter interface {
	Format(report *Report) ([]byte, error)
}
