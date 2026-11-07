package reporter

import (
	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
)

// ExitCode returns 1 when the report contains a finding whose configured
// severity is error, and 0 otherwise.
func ExitCode(r *spool.Report, sev config.SeverityConfig) int {
	if r.UntestedACs > 0 && sev.UntestedAC == config.SeverityError {
		return 1
	}
	if r.OrphanedCount > 0 && sev.OrphanedAnnotation == config.SeverityError {
		return 1
	}
	if r.UntestedInvariants > 0 && sev.UntestedInvariant == config.SeverityError {
		return 1
	}
	if len(r.MissingInterfaces) > 0 && sev.MissingInterface.Or(config.SeverityError) == config.SeverityError {
		return 1
	}
	if len(r.NotGivenWhenThen) > 0 && sev.GivenWhenThen.Or(config.SeverityError) == config.SeverityError {
		return 1
	}
	return 0
}
