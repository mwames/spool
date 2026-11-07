package reporter

import (
	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
)

// Coverage reports the given normalized AC and invariant IDs, in order, with
// their linked tests. Test files are made relative to root (COVERAGE-2).
func Coverage(idx *index.Index, root string, ids []string) *spool.CoverageReport {
	return &spool.CoverageReport{}
}

// CoverageText formats a coverage report as text: each ID and its state on
// one line, its tests indented below as "<file>:<line> <function>", and a
// last line "<n>/<m> covered" (COVERAGE-2-1).
func CoverageText(r *spool.CoverageReport) []byte {
	return nil
}

// CoverageJSON formats a coverage report as JSON (COVERAGE-2-6).
func CoverageJSON(r *spool.CoverageReport) ([]byte, error) {
	return nil, nil
}

// CoverageExitCode returns 0 when every listed ID is covered and 1 otherwise,
// whatever the configured severities (COVERAGE-2-2, COVERAGE-2-3).
func CoverageExitCode(r *spool.CoverageReport) int {
	return 0
}
