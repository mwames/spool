package config

import (
	"strings"
	"testing"
)

// INVARIANT-1-6: Untested Invariant Severity
func TestLoad_UntestedInvariantDefault(t *testing.T) {
	t.Run("no config file", func(t *testing.T) {
		cfg, err := Load(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Severity.UntestedInvariant != SeverityError {
			t.Errorf("UntestedInvariant = %q, want %q", cfg.Severity.UntestedInvariant, SeverityError)
		}
	})

	t.Run("not set", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, "severity:\n  untested_ac: warning\n")

		cfg, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Severity.UntestedInvariant != SeverityError {
			t.Errorf("UntestedInvariant = %q, want %q", cfg.Severity.UntestedInvariant, SeverityError)
		}
	})
}

// INVARIANT-1-6: Untested Invariant Severity
func TestLoad_UntestedInvariantConfigured(t *testing.T) {
	for _, s := range []Severity{SeverityError, SeverityWarning, SeverityInfo, SeverityOff} {
		t.Run(string(s), func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, "severity:\n  untested_invariant: \""+string(s)+"\"\n")

			cfg, err := Load(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Severity.UntestedInvariant != s {
				t.Errorf("UntestedInvariant = %q, want %q", cfg.Severity.UntestedInvariant, s)
			}
		})
	}
}

// INVARIANT-1-6: Untested Invariant Severity
func TestLoad_InvalidUntestedInvariantSeverity(t *testing.T) {
	for _, value := range []string{"fatal", "WARN"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, "severity:\n  untested_invariant: "+value+"\n")

			_, err := Load(dir)
			if err == nil {
				t.Fatal("expected error for invalid severity")
			}
			if !strings.Contains(err.Error(), "untested_invariant") {
				t.Errorf("error %q should mention field untested_invariant", err)
			}
			if !strings.Contains(err.Error(), value) {
				t.Errorf("error %q should mention value %q", err, value)
			}
		})
	}
}
