package interpreters

import "testing"

func TestJestInterpreter_Supports(t *testing.T) {
	j := Jest{}
	for _, f := range []string{"foo.test.js", "foo.test.ts", "foo.spec.js", "foo.spec.ts"} {
		if !j.Supports(f) {
			t.Errorf("should support %q", f)
		}
	}
	for _, f := range []string{"foo.go", "foo.java", "foo.js"} {
		if j.Supports(f) {
			t.Errorf("should not support %q", f)
		}
	}
}

func TestJestInterpreter_ItCall(t *testing.T) {
	src := []byte(`describe('math', () => {
  it('adds numbers', () => {
    expect(1+1).toBe(2);
  });
});
`)
	funcs, err := Jest{}.ExtractTestFunctions("math.test.js", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "adds numbers" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "adds numbers")
	}
	if funcs[0].Line != 2 {
		t.Errorf("Line = %d, want 2", funcs[0].Line)
	}
}

func TestJestInterpreter_TestCall(t *testing.T) {
	src := []byte(`test('subtracts numbers', () => {
  expect(2-1).toBe(1);
});
`)
	funcs, err := Jest{}.ExtractTestFunctions("math.test.js", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "subtracts numbers" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "subtracts numbers")
	}
}

func TestJestInterpreter_DoubleQuotes(t *testing.T) {
	src := []byte("it(\"works with doubles\", () => {});\n")
	funcs, err := Jest{}.ExtractTestFunctions("foo.test.js", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 || funcs[0].Name != "works with doubles" {
		t.Errorf("got %v", funcs)
	}
}

func TestJestInterpreter_BacktickQuotes(t *testing.T) {
	src := []byte("test(`works with backticks`, () => {});\n")
	funcs, err := Jest{}.ExtractTestFunctions("foo.test.js", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 || funcs[0].Name != "works with backticks" {
		t.Errorf("got %v", funcs)
	}
}

func TestJestInterpreter_Multiple(t *testing.T) {
	src := []byte(`it('first', () => {});
test('second', () => {});
it('third', () => {});
`)
	funcs, err := Jest{}.ExtractTestFunctions("foo.test.ts", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 3 {
		t.Fatalf("got %d funcs, want 3", len(funcs))
	}
}
