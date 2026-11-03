package reporter

import (
	"testing"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
)

func severities(untested, orphaned config.Severity) config.SeverityConfig {
	return config.SeverityConfig{UntestedAC: untested, OrphanedAnnotation: orphaned}
}

// REPORTER-7-1: Untested Error Fails
func TestExitCode_UntestedError(t *testing.T) {
	r := &spool.Report{UntestedACs: 1}
	if got := ExitCode(r, severities(config.SeverityError, config.SeverityOff)); got != 1 {
		t.Errorf("ExitCode = %d, want 1", got)
	}
}

// REPORTER-7-2: Orphaned Error Fails
func TestExitCode_OrphanedError(t *testing.T) {
	r := &spool.Report{OrphanedCount: 1}
	if got := ExitCode(r, severities(config.SeverityOff, config.SeverityError)); got != 1 {
		t.Errorf("ExitCode = %d, want 1", got)
	}
}

// REPORTER-7-3: Lower Severities Pass
func TestExitCode_LowerSeverities(t *testing.T) {
	r := &spool.Report{UntestedACs: 2, OrphanedCount: 3}
	for _, s := range []config.Severity{config.SeverityWarning, config.SeverityInfo, config.SeverityOff} {
		if got := ExitCode(r, severities(s, s)); got != 0 {
			t.Errorf("severity %s: ExitCode = %d, want 0", s, got)
		}
	}
}

// REPORTER-7-4: Clean Report Passes
func TestExitCode_CleanReport(t *testing.T) {
	r := &spool.Report{TotalACs: 5, LinkedACs: 5}
	if got := ExitCode(r, severities(config.SeverityError, config.SeverityError)); got != 0 {
		t.Errorf("ExitCode = %d, want 0", got)
	}
}
