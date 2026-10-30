package interpreters

import "testing"

func extractRust(t *testing.T, src string) []string {
	t.Helper()
	funcs, err := Rust{}.ExtractTestFunctions("lib.rs", []byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var names []string
	for _, f := range funcs {
		names = append(names, f.Name)
	}
	return names
}

func assertNames(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			return
		}
	}
}

// SCANNER-7-1: Rust Test Extraction
func TestRustInterpreter_Supports(t *testing.T) {
	r := Rust{}
	if !r.Supports("src/lib.rs") {
		t.Error("should support .rs")
	}
	if r.Supports("foo_test.go") {
		t.Error("should not support .go")
	}
}

// SCANNER-7-1: Rust Test Extraction
func TestRustInterpreter_TestAttribute(t *testing.T) {
	src := []byte(`use super::*;

#[test]
fn adds_two() {
    assert_eq!(add(2, 2), 4);
}
`)
	funcs, err := Rust{}.ExtractTestFunctions("lib.rs", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "adds_two" {
		t.Errorf("Name = %q, want %q", funcs[0].Name, "adds_two")
	}
	if funcs[0].Line != 4 {
		t.Errorf("Line = %d, want 4", funcs[0].Line)
	}
}

// SCANNER-7-2: Test Marker Attributes
func TestRustInterpreter_TestMarkers(t *testing.T) {
	got := extractRust(t, `#[tokio::test]
async fn tokio_plain() {}

#[tokio::test(flavor = "multi_thread")]
async fn tokio_args() {}

#[rstest]
fn rstest_case() {}

#[wasm_bindgen_test]
fn wasm() {}

#[test_case(1 ; "one")]
fn test_case_attr(x: u32) {}
`)
	assertNames(t, got, "tokio_plain", "tokio_args", "rstest_case", "wasm", "test_case_attr")
}

// SCANNER-7-3: Non-Marker Attributes Ignored
func TestRustInterpreter_NonMarkerAttributes(t *testing.T) {
	got := extractRust(t, `#[cfg(test)]
fn cfg_helper() {}

#[allow(dead_code)]
fn unused() {}

#[inline]
fn fast() {}

#[derive(Debug)]
struct Point;
`)
	assertNames(t, got)
}

// SCANNER-7-4: Multiple Attributes
func TestRustInterpreter_MultipleAttributes(t *testing.T) {
	got := extractRust(t, `#[test]
#[should_panic(expected = "boom")]
fn marker_first() {}

#[serial]
#[ignore]
#[tokio::test]
async fn marker_last() {}

#[rstest]
#[case(1)]
#[case(2)]
fn cases(#[case] x: u32) {}
`)
	assertNames(t, got, "marker_first", "marker_last", "cases")
}

// SCANNER-7-5: Function Qualifiers
func TestRustInterpreter_Qualifiers(t *testing.T) {
	got := extractRust(t, `#[test]
pub fn public() {}

#[tokio::test]
async fn asynchronous() {}

#[tokio::test]
pub async fn both() {}

#[test]
pub(crate) fn crate_visible() {}
`)
	assertNames(t, got, "public", "asynchronous", "both", "crate_visible")
}

// SCANNER-7-6: Nested Module Tests
func TestRustInterpreter_NestedModule(t *testing.T) {
	src := []byte(`pub fn add(a: u32, b: u32) -> u32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn adds() {
        assert_eq!(add(1, 2), 3);
    }
}
`)
	funcs, err := Rust{}.ExtractTestFunctions("lib.rs", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(funcs))
	}
	if funcs[0].Name != "adds" || funcs[0].Line != 10 {
		t.Errorf("got %+v, want adds at line 10", funcs[0])
	}
}

// SCANNER-7-7: Unattributed Functions Ignored
func TestRustInterpreter_UnattributedFunctions(t *testing.T) {
	got := extractRust(t, `fn helper() {}

pub fn test_looking_name() {}

#[test]

fn separated_by_blank_line() {}
`)
	assertNames(t, got)
}
