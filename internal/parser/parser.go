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
	ID                 string               `yaml:"id"`
	Title              string               `yaml:"title"`
	Description        string               `yaml:"description"`
	Status             string               `yaml:"status"`
	Deciders           []string             `yaml:"deciders"`
	Consulted          []string             `yaml:"consulted"`
	Date               string               `yaml:"date"`
	Rationale          string               `yaml:"rationale"`
	SupersededBy       string               `yaml:"superseded_by"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria"`
}

// AcceptanceCriterion is a single AC within a requirement.
type AcceptanceCriterion struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
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

	rf := &ReqFile{}
	if err := yaml.Unmarshal(data, rf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(absPath), err)
	}

	rf.SourceFile = absPath
	normalizeIDs(rf)

	return rf, nil
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
	}
}
