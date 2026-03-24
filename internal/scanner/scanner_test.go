package scanner

import (
	"os"
	"path/filepath"
	"testing"

	spool "github.com/mwames/Spool"
)

// --- detectAnnotations tests ---

// SCANNER-1-1: Annotation Detected
func TestDetectAnnotations_SlashSlashComment(t *testing.T) {
	src := []byte("// AUTH-1-1\n")
	anns := detectAnnotations(src)
	if len(anns) != 1 {
		t.Fatalf("got %d annotations, want 1", len(anns))
	}
	if anns[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want %q", anns[0].ID, "AUTH-1-1")
	}
	if anns[0].Line != 1 {
		t.Errorf("Line = %d, want 1", anns[0].Line)
	}
}

func TestDetectAnnotations_HashComment(t *testing.T) {
	src := []byte("# AUTH-1-1\n")
	anns := detectAnnotations(src)
	if len(anns) != 1 {
		t.Fatalf("got %d annotations, want 1", len(anns))
	}
	if anns[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want %q", anns[0].ID, "AUTH-1-1")
	}
}

// SCANNER-1-2: Non-Canonical Normalized
func TestDetectAnnotations_NonCanonicalNormalized(t *testing.T) {
	src := []byte("// auth-01-02\n")
	anns := detectAnnotations(src)
	if len(anns) != 1 {
		t.Fatalf("got %d annotations, want 1", len(anns))
	}
	if anns[0].ID != "AUTH-1-2" {
		t.Errorf("ID = %q, want %q", anns[0].ID, "AUTH-1-2")
	}
}

// SCANNER-1-3: Extra Content Not Annotation
func TestDetectAnnotations_ExtraContent(t *testing.T) {
	src := []byte("// AUTH-1-1 some extra text\n")
	anns := detectAnnotations(src)
	if len(anns) != 0 {
		t.Errorf("got %d annotations, want 0", len(anns))
	}
}

// SCANNER-1-4: No Annotations
func TestDetectAnnotations_NoAnnotations(t *testing.T) {
	src := []byte("func TestFoo(t *testing.T) {}\n")
	anns := detectAnnotations(src)
	if len(anns) != 0 {
		t.Errorf("got %d annotations, want 0", len(anns))
	}
}

func TestDetectAnnotations_LineNumbers(t *testing.T) {
	src := []byte("package foo\n\n// AUTH-1-1\n\n// AUTH-2-1\n")
	anns := detectAnnotations(src)
	if len(anns) != 2 {
		t.Fatalf("got %d annotations, want 2", len(anns))
	}
	if anns[0].Line != 3 {
		t.Errorf("anns[0].Line = %d, want 3", anns[0].Line)
	}
	if anns[1].Line != 5 {
		t.Errorf("anns[1].Line = %d, want 5", anns[1].Line)
	}
}

// SCANNER-1-1: Annotation Detected
func TestDetectAnnotations_WithTitle(t *testing.T) {
	src := []byte("// AUTH-1-1: Valid Credentials\n")
	anns := detectAnnotations(src)
	if len(anns) != 1 {
		t.Fatalf("got %d annotations, want 1", len(anns))
	}
	if anns[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want %q", anns[0].ID, "AUTH-1-1")
	}
}

func TestDetectAnnotations_IndentedComment(t *testing.T) {
	src := []byte("  // AUTH-1-1\n")
	anns := detectAnnotations(src)
	if len(anns) != 1 {
		t.Fatalf("got %d annotations, want 1", len(anns))
	}
	if anns[0].ID != "AUTH-1-1" {
		t.Errorf("ID = %q, want %q", anns[0].ID, "AUTH-1-1")
	}
}

// --- associate tests ---

// SCANNER-2-1: Immediate Association
func TestAssociate_ImmediatelyBefore(t *testing.T) {
	anns := []Annotation{{ID: "AUTH-1-1", Line: 3}}
	funcs := []spool.TestFunction{{Name: "TestLogin", Line: 4}}

	mappings, orphaned := associate(anns, funcs)
	if len(orphaned) != 0 {
		t.Errorf("got %d orphaned, want 0", len(orphaned))
	}
	if len(mappings) != 1 {
		t.Fatalf("got %d mappings, want 1", len(mappings))
	}
	if mappings[0].Function.Name != "TestLogin" {
		t.Errorf("Function = %q, want %q", mappings[0].Function.Name, "TestLogin")
	}
	if len(mappings[0].Annotations) != 1 || mappings[0].Annotations[0].ID != "AUTH-1-1" {
		t.Errorf("Annotations = %v", mappings[0].Annotations)
	}
}

// SCANNER-2-2: Separated by Blank Lines
func TestAssociate_SeparatedByBlankLines(t *testing.T) {
	anns := []Annotation{{ID: "AUTH-1-1", Line: 3}}
	funcs := []spool.TestFunction{{Name: "TestLogin", Line: 10}}

	mappings, orphaned := associate(anns, funcs)
	if len(orphaned) != 0 {
		t.Errorf("got %d orphaned, want 0", len(orphaned))
	}
	if len(mappings) != 1 || len(mappings[0].Annotations) != 1 {
		t.Errorf("mappings = %v", mappings)
	}
}

// SCANNER-2-3: Stacked Annotations
func TestAssociate_StackedAnnotations(t *testing.T) {
	anns := []Annotation{
		{ID: "AUTH-1-1", Line: 3},
		{ID: "AUTH-1-2", Line: 4},
	}
	funcs := []spool.TestFunction{{Name: "TestLogin", Line: 5}}

	mappings, orphaned := associate(anns, funcs)
	if len(orphaned) != 0 {
		t.Errorf("got %d orphaned, want 0", len(orphaned))
	}
	if len(mappings) != 1 {
		t.Fatalf("got %d mappings, want 1", len(mappings))
	}
	if len(mappings[0].Annotations) != 2 {
		t.Errorf("got %d annotations, want 2", len(mappings[0].Annotations))
	}
}

// SCANNER-2-4: Same ID Multiple Functions
func TestAssociate_SameIDMultipleFunctions(t *testing.T) {
	anns := []Annotation{
		{ID: "AUTH-1-1", Line: 3},
		{ID: "AUTH-1-1", Line: 10},
	}
	funcs := []spool.TestFunction{
		{Name: "TestLoginBasic", Line: 4},
		{Name: "TestLoginOAuth", Line: 11},
	}

	mappings, orphaned := associate(anns, funcs)
	if len(orphaned) != 0 {
		t.Errorf("got %d orphaned, want 0", len(orphaned))
	}
	if len(mappings) != 2 {
		t.Fatalf("got %d mappings, want 2", len(mappings))
	}
	if mappings[0].Annotations[0].ID != "AUTH-1-1" || mappings[1].Annotations[0].ID != "AUTH-1-1" {
		t.Errorf("expected AUTH-1-1 on both mappings")
	}
}

// SCANNER-2-5: Orphaned Annotation
func TestAssociate_OrphanedAnnotation(t *testing.T) {
	anns := []Annotation{{ID: "AUTH-1-1", Line: 3}}
	funcs := []spool.TestFunction{} // no functions

	mappings, orphaned := associate(anns, funcs)
	if len(mappings) != 0 {
		t.Errorf("got %d mappings, want 0", len(mappings))
	}
	if len(orphaned) != 1 {
		t.Fatalf("got %d orphaned, want 1", len(orphaned))
	}
	if orphaned[0].ID != "AUTH-1-1" {
		t.Errorf("orphaned ID = %q, want %q", orphaned[0].ID, "AUTH-1-1")
	}
}

// --- Interpreter selection tests ---

type mockInterpreter struct {
	supports bool
	funcs    []spool.TestFunction
	err      error
}

func (m mockInterpreter) Supports(_ string) bool { return m.supports }
func (m mockInterpreter) ExtractTestFunctions(_ string, _ []byte) ([]spool.TestFunction, error) {
	return m.funcs, m.err
}

// SCANNER-3-1: Matching Interpreter
func TestFindInterpreter_Match(t *testing.T) {
	interps := []spool.Interpreter{
		mockInterpreter{supports: false},
		mockInterpreter{supports: true},
	}
	got := findInterpreter("foo_test.go", interps)
	if got == nil {
		t.Fatal("expected non-nil interpreter")
	}
}

// SCANNER-3-2: No Match
func TestFindInterpreter_NoMatch(t *testing.T) {
	interps := []spool.Interpreter{
		mockInterpreter{supports: false},
	}
	got := findInterpreter("foo.rb", interps)
	if got != nil {
		t.Error("expected nil for no match")
	}
}

// SCANNER-3-3: First Match Wins
func TestFindInterpreter_FirstMatchWins(t *testing.T) {
	first := mockInterpreter{supports: true, funcs: []spool.TestFunction{{Name: "first"}}}
	second := mockInterpreter{supports: true, funcs: []spool.TestFunction{{Name: "second"}}}
	interps := []spool.Interpreter{first, second}

	got := findInterpreter("foo.spec.ts", interps)
	// Verify it's the first one by extracting
	funcs, _ := got.ExtractTestFunctions("", nil)
	if len(funcs) != 1 || funcs[0].Name != "first" {
		t.Errorf("expected first interpreter, got %v", funcs)
	}
}

// SCANNER-5-1: Third-Party Interpreter Registered
func TestScan_ThirdPartyInterpreter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom_test.xyz")
	os.WriteFile(path, []byte("// AUTH-1-1\ntest line\n"), 0644)

	custom := mockInterpreter{
		supports: true,
		funcs:    []spool.TestFunction{{Name: "customTest", Line: 2}},
	}

	result := Scan([]string{path}, []spool.Interpreter{custom})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(result.Files))
	}
	if len(result.Files[0].Mappings) != 1 {
		t.Fatalf("got %d mappings, want 1", len(result.Files[0].Mappings))
	}
	if result.Files[0].Mappings[0].Function.Name != "customTest" {
		t.Errorf("Function = %q, want %q", result.Files[0].Mappings[0].Function.Name, "customTest")
	}
}

// --- Scan orchestration tests ---

// SCANNER-6-1: Unreadable File
func TestScan_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good_test.go")
	os.WriteFile(good, []byte("// AUTH-1-1\nfunc TestFoo(t *testing.T) {}\n"), 0644)
	bad := filepath.Join(dir, "missing_test.go")

	interp := mockInterpreter{
		supports: true,
		funcs:    []spool.TestFunction{{Name: "TestFoo", Line: 2}},
	}

	result := Scan([]string{good, bad}, []spool.Interpreter{interp})
	if len(result.Files) != 1 {
		t.Errorf("got %d files, want 1", len(result.Files))
	}
	if len(result.Errors) != 1 {
		t.Errorf("got %d errors, want 1", len(result.Errors))
	}
}

// SCANNER-6-2: Unparseable File
func TestScan_UnparseableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad_test.go")
	os.WriteFile(path, []byte("some content\n"), 0644)

	failing := mockInterpreter{
		supports: true,
		err:      os.ErrInvalid,
	}

	result := Scan([]string{path}, []spool.Interpreter{failing})
	if len(result.Files) != 0 {
		t.Errorf("got %d files, want 0", len(result.Files))
	}
	if len(result.Errors) != 1 {
		t.Errorf("got %d errors, want 1", len(result.Errors))
	}
}

// SCANNER-3-2: No Interpreter Warning
func TestScan_NoInterpreterWarning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rb")
	os.WriteFile(path, []byte("# AUTH-1-1\n"), 0644)

	result := Scan([]string{path}, []spool.Interpreter{mockInterpreter{supports: false}})
	if len(result.Warnings) != 1 {
		t.Errorf("got %d warnings, want 1", len(result.Warnings))
	}
}

func TestScan_EmptyFileList(t *testing.T) {
	result := Scan(nil, nil)
	if len(result.Files) != 0 || len(result.Errors) != 0 || len(result.Warnings) != 0 {
		t.Errorf("expected empty result")
	}
}
