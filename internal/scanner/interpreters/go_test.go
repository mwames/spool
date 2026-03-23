package interpreters

import "testing"

func TestGoInterpreter_Supports(t *testing.T) {
	g := Go{}
	if !g.Supports("foo_test.go") {
		t.Error("should support *_test.go")
	}
	if g.Supports("foo.go") {
		t.Error("should not support non-test .go files")
	}
	if g.Supports("foo.test.js") {
		t.Error("should not support .js files")
	}
}

func TestGoInterpreter_BasicTestFunc(t *testing.T) {
	src := []byte(`package foo

import "testing"

func TestAdd(t *testing.T) {
	// test body
}
`)
	funcs, err := Go{}.ExtractTestFunctions("foo_test.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "TestAdd" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "TestAdd")
	}
	if funcs[0].Line != 5 {
		t.Errorf("Line = %d, want 5", funcs[0].Line)
	}
}

func TestGoInterpreter_MultipleFuncs(t *testing.T) {
	src := []byte(`package foo

import "testing"

func TestAdd(t *testing.T) {}

func TestSub(t *testing.T) {}
`)
	funcs, err := Go{}.ExtractTestFunctions("foo_test.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 2 {
		t.Fatalf("got %d funcs, want 2", len(funcs))
	}
	if funcs[0].Name != "TestAdd" || funcs[1].Name != "TestSub" {
		t.Errorf("got %v", funcs)
	}
}

func TestGoInterpreter_NonTestIgnored(t *testing.T) {
	src := []byte(`package foo

import "testing"

func helper(t *testing.T) {}

func BenchmarkFoo(b *testing.B) {}

func TestReal(t *testing.T) {}
`)
	funcs, err := Go{}.ExtractTestFunctions("foo_test.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "TestReal" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "TestReal")
	}
}

func TestGoInterpreter_TestMainIgnored(t *testing.T) {
	src := []byte(`package foo

import "testing"

func TestMain(m *testing.M) {}

func TestReal(t *testing.T) {}
`)
	funcs, err := Go{}.ExtractTestFunctions("foo_test.go", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "TestReal" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "TestReal")
	}
}

func TestGoInterpreter_EmptyFile(t *testing.T) {
	funcs, err := Go{}.ExtractTestFunctions("foo_test.go", []byte("package foo\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("got %d funcs, want 0", len(funcs))
	}
}
