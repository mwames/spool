package reporter

import (
	"encoding/json"

	spool "github.com/mwames/Spool"
)

// JSON formats a report as JSON with camelCase field names.
type JSON struct{}

type jsonReport struct {
	TotalRequirements int                 `json:"totalRequirements"`
	TotalACs          int                 `json:"totalACs"`
	LinkedACs         int                 `json:"linkedACs"`
	UntestedACs       int                 `json:"untestedACs"`
	OrphanedCount     int                 `json:"orphanedCount"`
	Features          []jsonFeatureReport `json:"features"`
	Untested          []jsonUntestedAC    `json:"untested"`
	Orphaned          []jsonOrphanedRef   `json:"orphaned"`

	TotalInvariants       int                     `json:"totalInvariants"`
	LinkedInvariants      int                     `json:"linkedInvariants"`
	UntestedInvariants    int                     `json:"untestedInvariants"`
	UntestedInvariantList []jsonUntestedInvariant `json:"untestedInvariantList"`
}

type jsonFeatureReport struct {
	Name                 string `json:"name"`
	Requirements         int    `json:"requirements"`
	ACs                  int    `json:"acs"`
	LinkedACs            int    `json:"linkedACs"`
	UntestedACs          int    `json:"untestedACs"`
	ExcludedRequirements int    `json:"excludedRequirements"`
	Invariants           int    `json:"invariants"`
	LinkedInvariants     int    `json:"linkedInvariants"`
	UntestedInvariants   int    `json:"untestedInvariants"`
}

type jsonUntestedAC struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Feature     string `json:"feature"`
}

type jsonUntestedInvariant struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Feature     string `json:"feature"`
}

type jsonOrphanedRef struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func (JSON) Format(r *spool.Report) ([]byte, error) {
	jr := jsonReport{
		TotalRequirements: r.TotalRequirements,
		TotalACs:          r.TotalACs,
		LinkedACs:         r.LinkedACs,
		UntestedACs:       r.UntestedACs,
		OrphanedCount:     r.OrphanedCount,

		TotalInvariants:    r.TotalInvariants,
		LinkedInvariants:   r.LinkedInvariants,
		UntestedInvariants: r.UntestedInvariants,
	}

	for _, f := range r.Features {
		jr.Features = append(jr.Features, jsonFeatureReport{
			Name:                 f.Name,
			Requirements:         f.Requirements,
			ACs:                  f.ACs,
			LinkedACs:            f.LinkedACs,
			UntestedACs:          f.UntestedACs,
			ExcludedRequirements: f.ExcludedRequirements,
			Invariants:           f.Invariants,
			LinkedInvariants:     f.LinkedInvariants,
			UntestedInvariants:   f.UntestedInvariants,
		})
	}

	for _, u := range r.Untested {
		jr.Untested = append(jr.Untested, jsonUntestedAC{
			ID:          u.ID,
			Description: u.Description,
			Feature:     u.Feature,
		})
	}

	for _, u := range r.UntestedInvariantList {
		jr.UntestedInvariantList = append(jr.UntestedInvariantList, jsonUntestedInvariant{
			ID:          u.ID,
			Description: u.Description,
			Feature:     u.Feature,
		})
	}

	for _, o := range r.Orphaned {
		jr.Orphaned = append(jr.Orphaned, jsonOrphanedRef{
			ID:   o.ID,
			File: o.File,
			Line: o.Line,
		})
	}

	return json.MarshalIndent(jr, "", "  ")
}
