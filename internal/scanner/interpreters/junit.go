package interpreters

import (
	"regexp"
	"strings"

	spool "github.com/mwames/Spool"
)

var (
	junitTestAnnotation = regexp.MustCompile(`^\s*@(Test|ParameterizedTest)\b`)
	junitMethodSig      = regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+)?(?:\w+\s+)+(\w+)\s*\(`)
)

// JUnit extracts test functions from JUnit test files.
type JUnit struct{}

func (JUnit) Supports(filename string) bool {
	return strings.HasSuffix(filename, ".java")
}

func (JUnit) ExtractTestFunctions(_ string, contents []byte) ([]spool.TestFunction, error) {
	var funcs []spool.TestFunction
	lines := strings.Split(string(contents), "\n")
	pendingTest := false
	for i, line := range lines {
		if junitTestAnnotation.MatchString(line) {
			pendingTest = true
			continue
		}
		if pendingTest {
			if m := junitMethodSig.FindStringSubmatch(line); m != nil {
				funcs = append(funcs, spool.TestFunction{Name: m[1], Line: i + 1})
				pendingTest = false
			}
		}
	}
	return funcs, nil
}
