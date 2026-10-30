package interpreters

import (
	"regexp"
	"strings"

	spool "github.com/mwames/Spool"
)

var (
	rustAttribute = regexp.MustCompile(`^\s*#\[\s*([\w:]+)`)
	rustFnDecl    = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?fn\s+(\w+)`)
)

// Rust extracts test functions from Rust source files. A function is a test
// when the attribute lines directly above it include a test marker.
type Rust struct{}

func (Rust) Supports(filename string) bool {
	return strings.HasSuffix(filename, ".rs")
}

func (Rust) ExtractTestFunctions(_ string, contents []byte) ([]spool.TestFunction, error) {
	var funcs []spool.TestFunction
	inBlock, marked := false, false
	for i, line := range strings.Split(string(contents), "\n") {
		if m := rustAttribute.FindStringSubmatch(line); m != nil {
			inBlock = true
			marked = marked || isRustTestMarker(m[1])
			continue
		}
		if inBlock && marked {
			if m := rustFnDecl.FindStringSubmatch(line); m != nil {
				funcs = append(funcs, spool.TestFunction{Name: m[1], Line: i + 1})
			}
		}
		inBlock, marked = false, false
	}
	return funcs, nil
}

// isRustTestMarker reports whether an attribute path marks a test: its last
// segment ends in "test" (test, tokio::test, rstest) or is test_case.
func isRustTestMarker(path string) bool {
	last := path[strings.LastIndex(path, ":")+1:]
	return strings.HasSuffix(last, "test") || last == "test_case"
}
