package spoolid

import "testing"

// INVARIANT-1-1: Invariant ID Format
func TestIsValid_InvariantID(t *testing.T) {
	cases := []string{"SLICE-6-INV-1", "slice-06-inv-01", "Slice-6-Inv-1", "AUTH-12-INV-30"}
	for _, id := range cases {
		if !IsValid(id) {
			t.Errorf("IsValid(%q) = false, want true", id)
		}
	}
}

// INVARIANT-1-1: Invariant ID Format
func TestNormalizeID_InvariantID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"SLICE-6-INV-1", "SLICE-6-INV-1"},
		{"slice-06-inv-01", "SLICE-6-INV-1"},
		{"Slice-6-Inv-1", "SLICE-6-INV-1"},
		{"AUTH-012-INV-030", "AUTH-12-INV-30"},
	}
	for _, c := range cases {
		got, err := NormalizeID(c.in)
		if err != nil {
			t.Errorf("NormalizeID(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// INVARIANT-1-1: Invariant ID Format
func TestInvariantID_Malformed(t *testing.T) {
	cases := []string{"SLICE-6-INV", "SLICE-INV-1", "SLICE-6-I1", "SLICE-6-INV-1-1"}
	for _, id := range cases {
		if IsValid(id) {
			t.Errorf("IsValid(%q) = true, want false", id)
		}
		if _, err := NormalizeID(id); err == nil {
			t.Errorf("NormalizeID(%q) expected error", id)
		}
	}
}
