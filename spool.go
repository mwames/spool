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

	// Invariants are counted and listed separately from ACs (INVARIANT-1).
	TotalInvariants       int
	LinkedInvariants      int
	UntestedInvariants    int
	UntestedInvariantList []UntestedInvariant

	// Requirements lists every active requirement, ordered as Features,
	// then as in the .req file (SPEC-2-2).
	Requirements []RequirementEntry

	// Lint findings. They never change the coverage counts above
	// (SPEC-3-INV-1).
	MissingInterfaces []MissingInterface // SPEC-1-3
	NotGivenWhenThen  []FormFinding      // SPEC-3-2
}

// RequirementEntry is an active requirement with its interface paths and
// out_of_scope items, as written (SPEC-2-2).
type RequirementEntry struct {
	ID         string
	Feature    string
	Title      string
	Interface  []string
	OutOfScope []string
}

// MissingInterface is an interface path of an active requirement that
// exists as neither a file nor a directory under the project root.
type MissingInterface struct {
	ID   string // the requirement's ID
	Path string // as written
}

// FormFinding is an AC of an active requirement that isn't written as
// Given/When/Then.
type FormFinding struct {
	ID      string
	Feature string
	Title   string
}

// FeatureReport captures per-feature traceability totals.
type FeatureReport struct {
	Name                 string
	Requirements         int
	ACs                  int
	LinkedACs            int
	UntestedACs          int
	ExcludedRequirements int
	Invariants           int
	LinkedInvariants     int
	UntestedInvariants   int
}

// UntestedAC identifies an acceptance criterion with no linked tests.
type UntestedAC struct {
	ID          string
	Description string
	Feature     string
}

// UntestedInvariant identifies an invariant with no linked tests.
type UntestedInvariant struct {
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

// CoverageReport lists, for each ID given to `spool --ids`, whether it has a
// linked test (COVERAGE-2).
type CoverageReport struct {
	IDs     []CoverageEntry
	Covered int
	Total   int
}

// CoverageEntry is one listed ID. Kind is "ac", "invariant" or "unknown".
type CoverageEntry struct {
	ID      string
	Kind    string
	Covered bool
	Tests   []CoverageTest
}

// CoverageTest is a test linked to a listed ID. File is relative to the
// project root.
type CoverageTest struct {
	File     string
	Line     int
	Function string
}
