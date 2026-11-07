// Package lint checks the requirements themselves rather than their
// coverage: interface paths that don't exist (SPEC-1) and ACs not written
// as Given/When/Then (SPEC-3).
package lint

import (
	"os"
	"path/filepath"
	"regexp"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
)

// GivenWhenThen reports whether an AC description, ignoring leading
// whitespace, starts with the word "given" and has the words "when" and
// "then" after it in that order: whole words, any case, across line breaks
// (SPEC-3-1).
func GivenWhenThen(description string) bool {
	loc := givenRe.FindStringIndex(description)
	if loc == nil {
		return false
	}
	rest := description[loc[1]:]
	w := whenRe.FindStringIndex(rest)
	if w == nil {
		return false
	}
	return thenRe.MatchString(rest[w[1]:])
}

var (
	givenRe = regexp.MustCompile(`(?i)^\s*given\b`)
	whenRe  = regexp.MustCompile(`(?i)\bwhen\b`)
	thenRe  = regexp.MustCompile(`(?i)\bthen\b`)
)

// MissingInterfaces returns one finding per interface path of an active
// requirement that exists as neither a file nor a directory under root
// (SPEC-1-3).
func MissingInterfaces(idx *index.Index, root string) []spool.MissingInterface {
	var out []spool.MissingInterface
	for _, feat := range idx.Features() {
		for _, req := range feat.Requirements {
			if req.Status != "active" {
				continue
			}
			for _, p := range req.Interface {
				if _, err := os.Stat(filepath.Join(root, p)); err != nil {
					out = append(out, spool.MissingInterface{ID: req.ID, Path: p})
				}
			}
		}
	}
	return out
}

// NotGivenWhenThen returns one finding per AC of an active requirement whose
// description fails GivenWhenThen (SPEC-3-2).
func NotGivenWhenThen(idx *index.Index) []spool.FormFinding {
	var out []spool.FormFinding
	for _, feat := range idx.Features() {
		for _, req := range feat.Requirements {
			if req.Status != "active" {
				continue
			}
			for _, ac := range req.ACs {
				if !GivenWhenThen(ac.Description) {
					out = append(out, spool.FormFinding{ID: ac.ID, Feature: ac.Feature, Title: ac.Title})
				}
			}
		}
	}
	return out
}
