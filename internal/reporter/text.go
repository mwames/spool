package reporter

import (
	"fmt"
	"strings"

	spool "github.com/mwames/Spool"
)

// Text formats a report as plain text suitable for terminal output.
type Text struct{}

func (Text) Format(r *spool.Report) ([]byte, error) {
	var b strings.Builder

	b.WriteString("Spool Traceability Report\n")
	b.WriteString("========================\n\n")

	// Summary table
	if len(r.Features) > 0 {
		b.WriteString(fmt.Sprintf("%-20s %6s %6s %8s %10s %10s\n", "Feature", "Reqs", "ACs", "Linked", "Untested", "Excluded"))
		b.WriteString(fmt.Sprintf("%-20s %6s %6s %8s %10s %10s\n", "-------", "----", "---", "------", "--------", "--------"))
		for _, f := range r.Features {
			b.WriteString(fmt.Sprintf("%-20s %6d %6d %8d %10d %10d\n", f.Name, f.Requirements, f.ACs, f.LinkedACs, f.UntestedACs, f.ExcludedRequirements))
		}
		b.WriteString(fmt.Sprintf("\nTotals: %d requirements, %d ACs, %d linked, %d untested, %d orphaned\n", r.TotalRequirements, r.TotalACs, r.LinkedACs, r.UntestedACs, r.OrphanedCount))
	}

	// Invariant summary
	if r.TotalInvariants > 0 {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%-20s %10s %8s %10s\n", "Feature", "Invariants", "Linked", "Untested"))
		b.WriteString(fmt.Sprintf("%-20s %10s %8s %10s\n", "-------", "----------", "------", "--------"))
		for _, f := range r.Features {
			b.WriteString(fmt.Sprintf("%-20s %10d %8d %10d\n", f.Name, f.Invariants, f.LinkedInvariants, f.UntestedInvariants))
		}
		b.WriteString(fmt.Sprintf("\nInvariant totals: %d invariants, %d linked, %d untested\n", r.TotalInvariants, r.LinkedInvariants, r.UntestedInvariants))
	}

	// Untested ACs
	if len(r.Untested) > 0 {
		b.WriteString("\nUntested Acceptance Criteria\n")
		b.WriteString("----------------------------\n")
		for _, u := range r.Untested {
			b.WriteString(fmt.Sprintf("  %s: %s\n", u.ID, u.Description))
		}
	}

	// Untested invariants
	if len(r.UntestedInvariantList) > 0 {
		b.WriteString("\nUntested Invariants\n")
		b.WriteString("-------------------\n")
		for _, u := range r.UntestedInvariantList {
			b.WriteString(fmt.Sprintf("  %s: %s\n", u.ID, u.Description))
		}
	}

	// Orphaned annotations
	if len(r.Orphaned) > 0 {
		b.WriteString("\nOrphaned Annotations\n")
		b.WriteString("--------------------\n")
		for _, o := range r.Orphaned {
			b.WriteString(fmt.Sprintf("  %s at %s:%d\n", o.ID, o.File, o.Line))
		}
	}

	// Lint findings
	if len(r.MissingInterfaces) > 0 {
		b.WriteString("\nMissing Interfaces\n")
		b.WriteString("------------------\n")
		for _, m := range r.MissingInterfaces {
			b.WriteString(fmt.Sprintf("  %s: %s\n", m.ID, m.Path))
		}
	}
	if len(r.NotGivenWhenThen) > 0 {
		b.WriteString("\nNot Given/When/Then\n")
		b.WriteString("-------------------\n")
		for _, f := range r.NotGivenWhenThen {
			b.WriteString(fmt.Sprintf("  %s: %s\n", f.ID, f.Title))
		}
	}

	// No issues
	if len(r.MissingInterfaces) == 0 && len(r.NotGivenWhenThen) == 0 &&
		len(r.Untested) == 0 && len(r.UntestedInvariantList) == 0 && len(r.Orphaned) == 0 && r.TotalACs > 0 {
		b.WriteString("\nFull traceability — no issues found.\n")
	}

	return []byte(b.String()), nil
}
