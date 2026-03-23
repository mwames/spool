package interpreters

import (
	"regexp"
	"strings"

	spool "github.com/mwames/Spool"
)

var jestTestCall = regexp.MustCompile("^\\s*(?:it|test)\\(\\s*(?:'([^']*)'|\"([^\"]*)\"|`([^`]*)`)")

// Jest extracts test functions from Jest test files.
type Jest struct{}

func (Jest) Supports(filename string) bool {
	return hasJSSuffix(filename, ".test") || hasJSSuffix(filename, ".spec")
}

func (Jest) ExtractTestFunctions(_ string, contents []byte) ([]spool.TestFunction, error) {
	var funcs []spool.TestFunction
	for i, line := range strings.Split(string(contents), "\n") {
		if m := jestTestCall.FindStringSubmatch(line); m != nil {
			name := firstNonEmpty(m[1], m[2], m[3])
			funcs = append(funcs, spool.TestFunction{Name: name, Line: i + 1})
		}
	}
	return funcs, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func hasJSSuffix(filename, mid string) bool {
	return strings.HasSuffix(filename, mid+".js") ||
		strings.HasSuffix(filename, mid+".ts") ||
		strings.HasSuffix(filename, mid+".jsx") ||
		strings.HasSuffix(filename, mid+".tsx")
}
