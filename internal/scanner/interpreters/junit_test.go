package interpreters

import "testing"

func TestJUnitInterpreter_Supports(t *testing.T) {
	j := JUnit{}
	if !j.Supports("FooTest.java") {
		t.Error("should support .java")
	}
	if j.Supports("foo_test.go") {
		t.Error("should not support .go")
	}
}

func TestJUnitInterpreter_TestAnnotation(t *testing.T) {
	src := []byte(`import org.junit.jupiter.api.Test;

public class FooTest {
    @Test
    void shouldAdd() {
        // test
    }
}
`)
	funcs, err := JUnit{}.ExtractTestFunctions("FooTest.java", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "shouldAdd" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "shouldAdd")
	}
	if funcs[0].Line != 5 {
		t.Errorf("Line = %d, want 5", funcs[0].Line)
	}
}

func TestJUnitInterpreter_MultipleMethods(t *testing.T) {
	src := []byte(`public class FooTest {
    @Test
    void testA() {}

    @Test
    public void testB() {}
}
`)
	funcs, err := JUnit{}.ExtractTestFunctions("FooTest.java", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 2 {
		t.Fatalf("got %d funcs, want 2", len(funcs))
	}
	if funcs[0].Name != "testA" || funcs[1].Name != "testB" {
		t.Errorf("got %v", funcs)
	}
}

func TestJUnitInterpreter_NoAnnotation(t *testing.T) {
	src := []byte(`public class Foo {
    public void helper() {}
}
`)
	funcs, err := JUnit{}.ExtractTestFunctions("Foo.java", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("got %d funcs, want 0", len(funcs))
	}
}

func TestJUnitInterpreter_ParameterizedTest(t *testing.T) {
	src := []byte(`public class FooTest {
    @ParameterizedTest
    void testParam(int val) {}
}
`)
	funcs, err := JUnit{}.ExtractTestFunctions("FooTest.java", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "testParam" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "testParam")
	}
}
