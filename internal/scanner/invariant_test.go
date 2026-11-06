package scanner

import (
	"os"
	"path/filepath"
	"testing"

	spool "github.com/mwames/Spool"
)

// INVARIANT-1-3: Invariant Annotations
func TestScan_InvariantAnnotation(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"slash", "// SLICE-6-INV-1"},
		{"hash", "# SLICE-6-INV-1"},
		{"slash with title", "// SLICE-6-INV-1: No Lost Jobs"},
		{"hash with title", "# SLICE-6-INV-1: No Lost Jobs"},
		{"non-canonical", "// slice-06-inv-01: No Lost Jobs"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "jobs_test.xyz")
			os.WriteFile(path, []byte(tt.line+"\ntest line\n"), 0644)

			interp := mockInterpreter{
				supports: true,
				funcs:    []spool.TestFunction{{Name: "TestNoLostJobs", Line: 2}},
			}

			result := Scan([]string{path}, []spool.Interpreter{interp})
			if len(result.Files) != 1 || len(result.Files[0].Mappings) != 1 {
				t.Fatalf("got %+v, want one file with one mapping", result.Files)
			}
			m := result.Files[0].Mappings[0]
			if m.Function.Name != "TestNoLostJobs" {
				t.Errorf("Function = %q, want %q", m.Function.Name, "TestNoLostJobs")
			}
			if len(m.Annotations) != 1 || m.Annotations[0].ID != "SLICE-6-INV-1" {
				t.Errorf("Annotations = %v, want SLICE-6-INV-1", m.Annotations)
			}
		})
	}
}
