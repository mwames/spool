package interpreters

import (
	"regexp"
	"strings"

	spool "github.com/mwames/Spool"
)

var goTestFunc = regexp.MustCompile(`^func\s+(Test[A-Z]\w*)\s*\(\s*\w+\s+\*testing\.T\s*\)`)

// Go extracts test functions from Go test files.
type Go struct{}

func (Go) Supports(filename string) bool {
	return strings.HasSuffix(filename, "_test.go")
}

func (Go) ExtractTestFunctions(_ string, contents []byte) ([]spool.TestFunction, error) {
	var funcs []spool.TestFunction
	for i, line := range strings.Split(string(contents), "\n") {
		if m := goTestFunc.FindStringSubmatch(line); m != nil {
			funcs = append(funcs, spool.TestFunction{Name: m[1], Line: i + 1})
		}
	}
	return funcs, nil
}
