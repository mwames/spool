// Package parser handles parsing of .req YAML files and directory traversal.
package parser

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mwames/Spool/internal/spoolid"
	"gopkg.in/yaml.v3"
)

// ReqFile is a parsed .req file.
type ReqFile struct {
	Feature      string        `yaml:"feature"`
	Requirements []Requirement `yaml:"requirements"`
	SourceFile   string        `yaml:"-"`
	Warnings     []string      `yaml:"-"`
}

// Requirement is a single requirement within a .req file.
type Requirement struct {
	ID                 string                `yaml:"id"`
	Title              string                `yaml:"title"`
	Description        string                `yaml:"description"`
	Status             string                `yaml:"status"` // folded once parsed (SPEC-4-4)
	Deciders           []string              `yaml:"deciders"`
	Consulted          []string              `yaml:"consulted"`
	Date               string                `yaml:"date"`
	Rationale          string                `yaml:"rationale"`
	SupersededBy       string                `yaml:"superseded_by"`
	Interface          StringList            `yaml:"interface"`    // paths relative to the project root (SPEC-1)
	OutOfScope         StringList            `yaml:"out_of_scope"` // (SPEC-2)
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria"`
	Invariants         []Invariant           `yaml:"invariants"`
	Line               int                   `yaml:"-"` // 1-based line number in the .req file
}

// AcceptanceCriterion is a single AC within a requirement.
type AcceptanceCriterion struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"` // its own, else its requirement's as written (SPEC-4-1)
	Line        int    `yaml:"-"`      // 1-based line number in the .req file
}

// Invariant is a property a requirement must keep on every run, with an ID of
// the form <REQ>-<n>-INV-<n> (INVARIANT-1).
type Invariant struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"` // its own, else its requirement's as written (SPEC-4-1)
	Line        int    `yaml:"-"`      // 1-based line number in the .req file
}

// FileError records a parse failure for a specific file.
type FileError struct {
	Path string
	Err  error
}

func (e FileError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

// ParseResult holds the results of parsing a directory of .req files.
type ParseResult struct {
	Files  []*ReqFile
	Errors []FileError
}

// ParseFile parses a single .req file. Returns an error for fatal issues
// (unreadable file, invalid YAML). Malformed IDs produce warnings on ReqFile.
func ParseFile(path string) (*ReqFile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	if err := checkStringLists(data); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(absPath), err)
	}

	rf := &ReqFile{}
	if err := yaml.Unmarshal(data, rf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(absPath), err)
	}

	if err := resolveStatuses(rf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(absPath), err)
	}

	rf.SourceFile = absPath
	extractLineNumbers(data, rf)
	normalizeIDs(rf)

	return rf, nil
}

// checkStringLists validates the interface and out_of_scope fields of each
// requirement, naming the requirement's ID and the field in the error
// (SPEC-1-2, SPEC-2-1). A file that isn't valid YAML is left to the
// caller's own parse.
func checkStringLists(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		reqs := root.Content[i+1]
		if root.Content[i].Value != "requirements" || reqs.Kind != yaml.SequenceNode {
			continue
		}
		for _, req := range reqs.Content {
			if req.Kind != yaml.MappingNode {
				continue
			}
			id := ""
			for j := 0; j+1 < len(req.Content); j += 2 {
				if req.Content[j].Value == "id" {
					id = req.Content[j+1].Value
				}
			}
			for j := 0; j+1 < len(req.Content); j += 2 {
				field := req.Content[j].Value
				if field != "interface" && field != "out_of_scope" {
					continue
				}
				var l StringList
				err := req.Content[j+1].Decode(&l)
				if err == nil && field == "interface" {
					for _, p := range l {
						if p == "" {
							err = fmt.Errorf("line %d: path must not be empty", req.Content[j+1].Line)
						}
					}
				}
				if err != nil {
					return fmt.Errorf("requirement %s: %s: %v", id, field, err)
				}
			}
		}
	}
	return nil
}

// statusRank orders item statuses for folding: deprecated < active < draft
// (SPEC-4-4).
var statusRank = map[string]int{"deprecated": 0, "active": 1, "draft": 2}

// resolveStatuses checks every status, gives each item without one its
// requirement's as written, and folds each requirement's status from its
// items' (SPEC-4-1 to SPEC-4-4).
func resolveStatuses(rf *ReqFile) error {
	for i := range rf.Requirements {
		r := &rf.Requirements[i]
		if r.Status != "" && r.Status != "superseded" {
			if _, ok := statusRank[r.Status]; !ok {
				return fmt.Errorf("requirement %s: unknown status %q", r.ID, r.Status)
			}
		}
		for _, a := range r.AcceptanceCriteria {
			if _, ok := statusRank[a.Status]; a.Status != "" && !ok {
				return fmt.Errorf("acceptance criterion %s: unknown status %q", a.ID, a.Status)
			}
		}
		for _, v := range r.Invariants {
			if _, ok := statusRank[v.Status]; v.Status != "" && !ok {
				return fmt.Errorf("invariant %s: unknown status %q", v.ID, v.Status)
			}
		}

		written := r.Status
		items := len(r.AcceptanceCriteria) + len(r.Invariants)
		folded := ""
		fold := func(status string) {
			if status == "" {
				status = written
			}
			if folded == "" || statusRank[status] > statusRank[folded] {
				folded = status
			}
		}
		for j := range r.AcceptanceCriteria {
			a := &r.AcceptanceCriteria[j]
			if a.Status == "" {
				if written == "" {
					return fmt.Errorf("requirement %s: status is missing", r.ID)
				}
				a.Status = written
			}
			fold(a.Status)
		}
		for j := range r.Invariants {
			v := &r.Invariants[j]
			if v.Status == "" {
				if written == "" {
					return fmt.Errorf("requirement %s: status is missing", r.ID)
				}
				v.Status = written
			}
			fold(v.Status)
		}
		if written == "" && items == 0 {
			return fmt.Errorf("requirement %s: status is missing", r.ID)
		}
		if written != "superseded" && items > 0 {
			r.Status = folded
		}
	}
	return nil
}

// extractLineNumbers uses yaml.Node to populate Line fields on requirements and ACs.
func extractLineNumbers(data []byte, rf *ReqFile) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}

	// Find the "requirements" key in the root mapping.
	var reqsSeq *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "requirements" {
			reqsSeq = root.Content[i+1]
			break
		}
	}
	if reqsSeq == nil || reqsSeq.Kind != yaml.SequenceNode {
		return
	}

	for i, reqNode := range reqsSeq.Content {
		if i >= len(rf.Requirements) {
			break
		}
		if reqNode.Kind != yaml.MappingNode {
			continue
		}
		rf.Requirements[i].Line = reqNode.Line

		// Find "acceptance_criteria" within this requirement mapping.
		for j := 0; j+1 < len(reqNode.Content); j += 2 {
			if reqNode.Content[j].Value == "acceptance_criteria" {
				acsSeq := reqNode.Content[j+1]
				if acsSeq.Kind != yaml.SequenceNode {
					break
				}
				for k, acNode := range acsSeq.Content {
					if k >= len(rf.Requirements[i].AcceptanceCriteria) {
						break
					}
					if acNode.Kind == yaml.MappingNode {
						rf.Requirements[i].AcceptanceCriteria[k].Line = acNode.Line
					}
				}
				break
			}
		}

		// Find "invariants" within this requirement mapping.
		for j := 0; j+1 < len(reqNode.Content); j += 2 {
			if reqNode.Content[j].Value == "invariants" {
				invSeq := reqNode.Content[j+1]
				if invSeq.Kind != yaml.SequenceNode {
					break
				}
				for k, invNode := range invSeq.Content {
					if k >= len(rf.Requirements[i].Invariants) {
						break
					}
					if invNode.Kind == yaml.MappingNode {
						rf.Requirements[i].Invariants[k].Line = invNode.Line
					}
				}
				break
			}
		}
	}
}

// ParseDir recursively walks dir and parses all .req files.
// Returns an error only if the directory itself cannot be walked.
// Individual file parse failures are collected in ParseResult.Errors.
func ParseDir(dir string) (*ParseResult, error) {
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reqs directory does not exist: %s", dir)
		}
		return nil, err
	}

	result := &ParseResult{}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".req" {
			return nil
		}

		rf, parseErr := ParseFile(path)
		if parseErr != nil {
			result.Errors = append(result.Errors, FileError{Path: path, Err: parseErr})
			return nil
		}
		result.Files = append(result.Files, rf)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func normalizeIDs(rf *ReqFile) {
	for i := range rf.Requirements {
		r := &rf.Requirements[i]
		normalized, err := spoolid.NormalizeID(r.ID)
		if err != nil {
			rf.Warnings = append(rf.Warnings, fmt.Sprintf("requirement %q: %v", r.ID, err))
		} else {
			r.ID = normalized
		}

		for j := range r.AcceptanceCriteria {
			ac := &r.AcceptanceCriteria[j]
			normalized, err := spoolid.NormalizeID(ac.ID)
			if err != nil {
				rf.Warnings = append(rf.Warnings, fmt.Sprintf("acceptance criterion %q: %v", ac.ID, err))
			} else {
				ac.ID = normalized
			}
		}

		for j := range r.Invariants {
			inv := &r.Invariants[j]
			normalized, err := spoolid.NormalizeID(inv.ID)
			if err != nil {
				rf.Warnings = append(rf.Warnings, fmt.Sprintf("invariant %q: %v", inv.ID, err))
			} else {
				inv.ID = normalized
			}
		}
	}
}
