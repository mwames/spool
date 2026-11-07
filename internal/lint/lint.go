// Package lint checks the requirements themselves rather than their
// coverage: interface paths that don't exist (SPEC-1) and ACs not written
// as Given/When/Then (SPEC-3).
package lint

import (
	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/index"
)

// GivenWhenThen reports whether an AC description, ignoring leading
// whitespace, starts with the word "given" and has the words "when" and
// "then" after it in that order: whole words, any case, across line breaks
// (SPEC-3-1).
func GivenWhenThen(description string) bool {
	return false
}

// MissingInterfaces returns one finding per interface path of an active
// requirement that exists as neither a file nor a directory under root
// (SPEC-1-3).
func MissingInterfaces(idx *index.Index, root string) []spool.MissingInterface {
	return nil
}

// NotGivenWhenThen returns one finding per AC of an active requirement whose
// description fails GivenWhenThen (SPEC-3-2).
func NotGivenWhenThen(idx *index.Index) []spool.FormFinding {
	return nil
}
