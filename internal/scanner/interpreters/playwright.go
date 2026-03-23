package interpreters

import (
	"regexp"
	"strings"

	spool "github.com/mwames/Spool"
)

var (
	playwrightImport   = regexp.MustCompile(`@playwright/test`)
	playwrightTestCall = regexp.MustCompile("^\\s*test\\(\\s*(?:'([^']*)'|\"([^\"]*)\"|`([^`]*)`)")
)

// Playwright extracts test functions from Playwright test files.
type Playwright struct{}

func (Playwright) Supports(filename string) bool {
	return hasJSSuffix(filename, ".spec")
}

func (Playwright) ExtractTestFunctions(_ string, contents []byte) ([]spool.TestFunction, error) {
	src := string(contents)
	if !playwrightImport.MatchString(src) {
		return nil, nil
	}

	var funcs []spool.TestFunction
	for i, line := range strings.Split(src, "\n") {
		if m := playwrightTestCall.FindStringSubmatch(line); m != nil {
			name := firstNonEmpty(m[1], m[2], m[3])
			funcs = append(funcs, spool.TestFunction{Name: name, Line: i + 1})
		}
	}
	return funcs, nil
}
