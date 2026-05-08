// Package api is the public Go-library entry point to Spool. It exposes
// LoadProject and a queryable Project type that wrap the internal pipeline.
package api

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/reporter"
	"github.com/mwames/Spool/internal/scanner"
)

// ProjectErrorKind categorizes failures returned by LoadProject.
type ProjectErrorKind string

const (
	// ErrInvalidProject means the project path could not be resolved to a directory.
	ErrInvalidProject ProjectErrorKind = "invalid_project"
	// ErrMissingConfig means the project directory does not contain a .spool.yaml.
	ErrMissingConfig ProjectErrorKind = "missing_config"
	// ErrInvalidConfig means .spool.yaml exists but could not be parsed or validated.
	ErrInvalidConfig ProjectErrorKind = "invalid_config"
)

// ProjectError is a categorized failure returned by LoadProject.
type ProjectError struct {
	Kind    ProjectErrorKind
	Path    string
	Message string
}

func (e *ProjectError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Path)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// WarningKind categorizes a non-fatal diagnostic from project loading.
type WarningKind string

const (
	WarningParse WarningKind = "parse"
	WarningScan  WarningKind = "scan"
)

// Warning is a non-fatal diagnostic surfaced alongside a successful project load.
type Warning struct {
	Kind    WarningKind
	Message string
}

// ValidationError is a structural error detected during index construction.
type ValidationError struct {
	Message string
}

// TestRef identifies a test function linked to an acceptance criterion.
type TestRef struct {
	Function string
	File     string
	Line     int
}

// AcceptanceCriterionView is a read-only view of an acceptance criterion enriched with linked tests.
type AcceptanceCriterionView struct {
	ID               string
	Title            string
	Description      string
	RequirementID    string
	RequirementTitle string
	Feature          string
	SourceFile       string
	Tests            []TestRef
}

// RequirementView is a read-only view of a requirement and its acceptance criteria.
type RequirementView struct {
	ID                 string
	Title              string
	Description        string
	Status             string
	Feature            string
	SourceFile         string
	AcceptanceCriteria []AcceptanceCriterionView
}

// FeatureView is a read-only summary of a feature.
type FeatureView struct {
	Name             string
	RequirementCount int
	ACCount          int
}

// RequirementFilter narrows the result set returned by Project.Requirements.
// Empty fields disable that filter.
type RequirementFilter struct {
	Feature string
	Status  string
}

// Project is a loaded Spool project, queryable through view methods.
// It is constructed by LoadProject and is safe for concurrent reads.
type Project struct {
	idx      *index.Index
	report   *spool.Report
	warnings []Warning
}

// LoadProject runs the full Spool pipeline (config, parse, scan, index) against
// the given project root. The path must be absolute. Per-file parse and scan
// failures are returned as warnings on the resulting Project; failures that
// prevent the pipeline from running (missing or invalid .spool.yaml, missing
// project directory) are returned as a *ProjectError.
func LoadProject(projectRoot string) (*Project, error) {
	if !filepath.IsAbs(projectRoot) {
		return nil, &ProjectError{
			Kind:    ErrInvalidProject,
			Path:    projectRoot,
			Message: "project path must be absolute",
		}
	}

	info, err := os.Stat(projectRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &ProjectError{
				Kind:    ErrInvalidProject,
				Path:    projectRoot,
				Message: "project directory does not exist",
			}
		}
		return nil, &ProjectError{
			Kind:    ErrInvalidProject,
			Path:    projectRoot,
			Message: err.Error(),
		}
	}
	if !info.IsDir() {
		return nil, &ProjectError{
			Kind:    ErrInvalidProject,
			Path:    projectRoot,
			Message: "project path is not a directory",
		}
	}

	configPath := filepath.Join(projectRoot, ".spool.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &ProjectError{
				Kind:    ErrMissingConfig,
				Path:    projectRoot,
				Message: ".spool.yaml not found in project directory",
			}
		}
		return nil, &ProjectError{
			Kind:    ErrInvalidConfig,
			Path:    configPath,
			Message: err.Error(),
		}
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		return nil, &ProjectError{
			Kind:    ErrInvalidConfig,
			Path:    configPath,
			Message: err.Error(),
		}
	}

	var warnings []Warning

	parseResult := &parser.ParseResult{}
	if _, statErr := os.Stat(cfg.ReqsDir); statErr == nil {
		parseResult, err = parser.ParseDir(cfg.ReqsDir)
		if err != nil {
			return nil, &ProjectError{
				Kind:    ErrInvalidConfig,
				Path:    cfg.ReqsDir,
				Message: err.Error(),
			}
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return nil, &ProjectError{
			Kind:    ErrInvalidConfig,
			Path:    cfg.ReqsDir,
			Message: statErr.Error(),
		}
	}
	// If the reqs directory does not exist, parseResult stays empty —
	// an empty Spool project is valid and yields empty results.
	for _, e := range parseResult.Errors {
		warnings = append(warnings, Warning{Kind: WarningParse, Message: e.Error()})
	}

	testFiles, err := project.FindTestFiles(projectRoot, cfg.TestPatterns)
	if err != nil {
		return nil, &ProjectError{
			Kind:    ErrInvalidConfig,
			Path:    projectRoot,
			Message: err.Error(),
		}
	}

	scanResult := scanner.Scan(testFiles, project.DefaultInterpreters())
	for _, e := range scanResult.Errors {
		warnings = append(warnings, Warning{Kind: WarningScan, Message: e.Error()})
	}
	for _, w := range scanResult.Warnings {
		warnings = append(warnings, Warning{Kind: WarningScan, Message: w})
	}

	idx := index.Build(parseResult.Files, scanResult.Files)
	report := reporter.Generate(idx)

	return &Project{
		idx:      idx,
		report:   report,
		warnings: warnings,
	}, nil
}

// Report returns the full coverage report for this project.
func (p *Project) Report() *spool.Report {
	return p.report
}

// Warnings returns all non-fatal parse and scan diagnostics collected during loading.
func (p *Project) Warnings() []Warning {
	out := make([]Warning, len(p.warnings))
	copy(out, p.warnings)
	return out
}

// ValidationErrors returns structural errors detected during index construction
// (duplicate IDs, prefix mismatches).
func (p *Project) ValidationErrors() []ValidationError {
	errs := p.idx.Errors()
	out := make([]ValidationError, len(errs))
	for i, e := range errs {
		out[i] = ValidationError{Message: e.Message}
	}
	return out
}

// Requirement returns the requirement with the given ID, or nil if no such
// requirement exists. The ID lookup is case-insensitive on the prefix to match
// Spool's normalization rules.
func (p *Project) Requirement(id string) *RequirementView {
	r := p.idx.Requirement(id)
	if r == nil {
		return nil
	}
	view := RequirementView{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Feature:     r.Feature,
		SourceFile:  r.SourceFile,
	}
	for _, ac := range r.ACs {
		view.AcceptanceCriteria = append(view.AcceptanceCriteria, acView(ac, r.Title))
	}
	return &view
}

// AC returns the acceptance criterion with the given ID, or nil if none exists.
func (p *Project) AC(id string) *AcceptanceCriterionView {
	ac := p.idx.AC(id)
	if ac == nil {
		return nil
	}
	parentTitle := ""
	if parent := p.idx.Requirement(ac.ReqID); parent != nil {
		parentTitle = parent.Title
	}
	view := acView(ac, parentTitle)
	return &view
}

// Requirements returns all requirements, optionally narrowed by feature and status.
// Acceptance criteria are not inlined; use Requirement(id) to retrieve them.
// Results are sorted by ID for deterministic ordering.
func (p *Project) Requirements(filter RequirementFilter) []RequirementView {
	var out []RequirementView
	for _, feat := range p.idx.Features() {
		if filter.Feature != "" && feat.Name != filter.Feature {
			continue
		}
		for _, r := range feat.Requirements {
			if filter.Status != "" && r.Status != filter.Status {
				continue
			}
			out = append(out, RequirementView{
				ID:          r.ID,
				Title:       r.Title,
				Description: r.Description,
				Status:      r.Status,
				Feature:     r.Feature,
				SourceFile:  r.SourceFile,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Features returns all features in the project with their requirement and AC counts,
// sorted alphabetically by name.
func (p *Project) Features() []FeatureView {
	feats := p.idx.Features()
	out := make([]FeatureView, 0, len(feats))
	for _, f := range feats {
		acCount := 0
		for _, r := range f.Requirements {
			acCount += len(r.ACs)
		}
		out = append(out, FeatureView{
			Name:             f.Name,
			RequirementCount: len(f.Requirements),
			ACCount:          acCount,
		})
	}
	return out
}

// FormatReport renders a Report using the named formatter. Accepted formats are
// "json", "text", and "markdown".
func FormatReport(r *spool.Report, format string) ([]byte, error) {
	switch format {
	case "json":
		return reporter.JSON{}.Format(r)
	case "text":
		return reporter.Text{}.Format(r)
	case "markdown":
		return reporter.Markdown{}.Format(r)
	default:
		return nil, fmt.Errorf("unknown format %q (accepted: json, text, markdown)", format)
	}
}

func acView(ac *index.IndexedAC, parentTitle string) AcceptanceCriterionView {
	view := AcceptanceCriterionView{
		ID:               ac.ID,
		Title:            ac.Title,
		Description:      ac.Description,
		RequirementID:    ac.ReqID,
		RequirementTitle: parentTitle,
		Feature:          ac.Feature,
		SourceFile:       ac.SourceFile,
	}
	for _, t := range ac.Tests {
		view.Tests = append(view.Tests, TestRef{
			Function: t.Function.Name,
			File:     t.File,
			Line:     t.Function.Line,
		})
	}
	return view
}
