package project

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func assertMatches(t *testing.T, pattern string, match, noMatch []string) {
	t.Helper()
	for _, p := range match {
		if !MatchPattern(p, pattern) {
			t.Errorf("MatchPattern(%q, %q) = false, want true", p, pattern)
		}
	}
	for _, p := range noMatch {
		if MatchPattern(p, pattern) {
			t.Errorf("MatchPattern(%q, %q) = true, want false", p, pattern)
		}
	}
}

// CONFIG-5-1: Plain Pattern Matches Whole Path
func TestMatchPattern_PlainPattern(t *testing.T) {
	assertMatches(t, "tests/*.rs",
		[]string{"tests/a.rs"},
		[]string{"tests/sub/a.rs", "src/a.rs"})
}

// CONFIG-5-2: Prefix Before Double Star Respected
func TestMatchPattern_PrefixBeforeDoubleStar(t *testing.T) {
	assertMatches(t, "tests/**/*.rs",
		[]string{"tests/a.rs", "tests/sub/deep/a.rs"},
		[]string{"src/a.rs", "src/tests/a.rs"})
}

// CONFIG-5-3: Leading Double Star Matches Any Depth
func TestMatchPattern_LeadingDoubleStar(t *testing.T) {
	assertMatches(t, "**/*_test.go",
		[]string{"a_test.go", "pkg/sub/a_test.go"},
		[]string{"pkg/a.go"})
}

// CONFIG-5-4: Multiple Double Stars
func TestMatchPattern_MultipleDoubleStars(t *testing.T) {
	assertMatches(t, "**/tests/**/*.rs",
		[]string{"tests/a.rs", "crates/x/tests/sub/a.rs"},
		[]string{"crates/x/src/a.rs"})
}

func writeTree(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func findRel(t *testing.T, root string, patterns, excludes []string) []string {
	t.Helper()
	files, err := FindTestFiles(root, patterns, excludes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var rel []string
	for _, f := range files {
		r, _ := filepath.Rel(root, f)
		rel = append(rel, filepath.ToSlash(r))
	}
	sort.Strings(rel)
	return rel
}

func assertFiles(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// CONFIG-6-4: Excluded Directory Not Walked
func TestFindTestFiles_ExcludedDirectory(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "src/lib.rs", "target/debug/build/out.rs", "crates/a/target/gen.rs")

	got := findRel(t, root, []string{"**/*.rs"}, []string{"target"})
	assertFiles(t, got, "crates/a/target/gen.rs", "src/lib.rs")

	got = findRel(t, root, []string{"**/*.rs"}, []string{"**/target"})
	assertFiles(t, got, "src/lib.rs")
}

// CONFIG-6-5: Excluded File Not Scanned
func TestFindTestFiles_ExcludedFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "a_test.go", "gen/b_test.go")

	got := findRel(t, root, []string{"**/*_test.go"}, []string{"gen/b_test.go"})
	assertFiles(t, got, "a_test.go")
}

// CONFIG-6-6: Git Directory Always Skipped
func TestFindTestFiles_GitAlwaysSkipped(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "a_test.go", ".git/hooks/b_test.go")

	got := findRel(t, root, []string{"**/*_test.go"}, nil)
	assertFiles(t, got, "a_test.go")
}
