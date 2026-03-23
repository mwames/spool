package spoolid

import "testing"

func TestNormalizeID_Canonical(t *testing.T) {
	got, err := NormalizeID("CONFIG-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CONFIG-1" {
		t.Errorf("got %q, want %q", got, "CONFIG-1")
	}
}

func TestNormalizeID_CanonicalAC(t *testing.T) {
	got, err := NormalizeID("CONFIG-1-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CONFIG-1-1" {
		t.Errorf("got %q, want %q", got, "CONFIG-1-1")
	}
}

func TestNormalizeID_Lowercase(t *testing.T) {
	got, err := NormalizeID("config-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CONFIG-1" {
		t.Errorf("got %q, want %q", got, "CONFIG-1")
	}
}

func TestNormalizeID_LeadingZeros(t *testing.T) {
	got, err := NormalizeID("PARSER-02-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "PARSER-2-1" {
		t.Errorf("got %q, want %q", got, "PARSER-2-1")
	}
}

func TestNormalizeID_LowercaseAndZeros(t *testing.T) {
	got, err := NormalizeID("config-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CONFIG-1" {
		t.Errorf("got %q, want %q", got, "CONFIG-1")
	}
}

func TestNormalizeID_Invalid(t *testing.T) {
	cases := []string{"not an id", "CONFIG", "123", "CONFIG-", "-1-1", ""}
	for _, id := range cases {
		_, err := NormalizeID(id)
		if err == nil {
			t.Errorf("NormalizeID(%q) expected error", id)
		}
	}
}

func TestIsValid_True(t *testing.T) {
	cases := []string{"CONFIG-1", "config-1", "PARSER-02-01", "AUTH-1-1"}
	for _, id := range cases {
		if !IsValid(id) {
			t.Errorf("IsValid(%q) = false, want true", id)
		}
	}
}

func TestIsValid_False(t *testing.T) {
	cases := []string{"not an id", "CONFIG", "123-1", "", "CONFIG-1-1-1"}
	for _, id := range cases {
		if IsValid(id) {
			t.Errorf("IsValid(%q) = true, want false", id)
		}
	}
}
