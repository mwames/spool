package reporter

import (
	"fmt"
	"strings"

	spool "github.com/mwames/Spool"
)

// Markdown formats a report as markdown.
type Markdown struct{}

func (Markdown) Format(r *spool.Report) ([]byte, error) {
	var b strings.Builder

	b.WriteString("# Spool Traceability Report\n\n")

	// Summary table
	if len(r.Features) > 0 {
		b.WriteString("| Feature | Reqs | ACs | Linked | Untested | Excluded |\n")
		b.WriteString("| ------- | ---- | --- | ------ | -------- | -------- |\n")
		for _, f := range r.Features {
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d |\n", f.Name, f.Requirements, f.ACs, f.LinkedACs, f.UntestedACs, f.ExcludedRequirements))
		}
		b.WriteString(fmt.Sprintf("\n**Totals:** %d requirements, %d ACs, %d linked, %d untested, %d orphaned\n", r.TotalRequirements, r.TotalACs, r.LinkedACs, r.UntestedACs, r.OrphanedCount))
	}

	// Untested ACs
	if len(r.Untested) > 0 {
		b.WriteString("\n## Untested Acceptance Criteria\n\n")
		for _, u := range r.Untested {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", u.ID, u.Description))
		}
	}

	// Orphaned annotations
	if len(r.Orphaned) > 0 {
		b.WriteString("\n## Orphaned Annotations\n\n")
		for _, o := range r.Orphaned {
			b.WriteString(fmt.Sprintf("- **%s** at `%s:%d`\n", o.ID, o.File, o.Line))
		}
	}

	return []byte(b.String()), nil
}
