package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, ".spool.yaml"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

// CONFIG-1-2: Missing Config Defaults
func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ReqsDir != filepath.Join(dir, ".spool") {
		t.Errorf("ReqsDir = %q, want %q", cfg.ReqsDir, filepath.Join(dir, ".spool"))
	}
	if len(cfg.TestPatterns) != 0 {
		t.Errorf("TestPatterns = %v, want empty", cfg.TestPatterns)
	}
	if cfg.Severity.UntestedAC != SeverityWarning {
		t.Errorf("UntestedAC = %q, want %q", cfg.Severity.UntestedAC, SeverityWarning)
	}
	if cfg.Severity.OrphanedAnnotation != SeverityError {
		t.Errorf("OrphanedAnnotation = %q, want %q", cfg.Severity.OrphanedAnnotation, SeverityError)
	}
	if cfg.Severity.MissingAC != SeverityInfo {
		t.Errorf("MissingAC = %q, want %q", cfg.Severity.MissingAC, SeverityInfo)
	}
}

// CONFIG-1-1: Valid Config Parsing
// CONFIG-3-1: Configured Test Patterns
// CONFIG-4-1: Configured Severity Values
func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
reqs_dir: requirements
test_patterns:
  - "**/*_test.go"
  - "**/*.test.ts"
severity:
  untested_ac: error
  orphaned_annotation: warning
  missing_ac: "off"
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ReqsDir != filepath.Join(dir, "requirements") {
		t.Errorf("ReqsDir = %q, want %q", cfg.ReqsDir, filepath.Join(dir, "requirements"))
	}
	if len(cfg.TestPatterns) != 2 || cfg.TestPatterns[0] != "**/*_test.go" || cfg.TestPatterns[1] != "**/*.test.ts" {
		t.Errorf("TestPatterns = %v, want [**/*_test.go **/*.test.ts]", cfg.TestPatterns)
	}
	if cfg.Severity.UntestedAC != SeverityError {
		t.Errorf("UntestedAC = %q, want %q", cfg.Severity.UntestedAC, SeverityError)
	}
	if cfg.Severity.OrphanedAnnotation != SeverityWarning {
		t.Errorf("OrphanedAnnotation = %q, want %q", cfg.Severity.OrphanedAnnotation, SeverityWarning)
	}
	if cfg.Severity.MissingAC != SeverityOff {
		t.Errorf("MissingAC = %q, want %q", cfg.Severity.MissingAC, SeverityOff)
	}
}

// CONFIG-1-3: Invalid YAML Error
func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "reqs_dir: [invalid\n")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !contains(err.Error(), ".spool.yaml") {
		t.Errorf("error %q should mention .spool.yaml", err)
	}
}

// CONFIG-1-4: Unknown Fields Ignored
func TestLoad_UnknownFields(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
reqs_dir: docs
some_future_field: true
another_thing: 42
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReqsDir != filepath.Join(dir, "docs") {
		t.Errorf("ReqsDir = %q, want %q", cfg.ReqsDir, filepath.Join(dir, "docs"))
	}
}

// CONFIG-2-1: Configured Reqs Directory
// CONFIG-2-3: Relative Path Resolution
func TestLoad_ReqsDirRelative(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "reqs_dir: specs/requirements\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReqsDir != filepath.Join(dir, "specs/requirements") {
		t.Errorf("ReqsDir = %q, want %q", cfg.ReqsDir, filepath.Join(dir, "specs/requirements"))
	}
}

// CONFIG-2-3: Relative Path Resolution
func TestLoad_ReqsDirAbsolute(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "reqs_dir: /absolute/path\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReqsDir != "/absolute/path" {
		t.Errorf("ReqsDir = %q, want /absolute/path", cfg.ReqsDir)
	}
}

// CONFIG-2-2: Default Reqs Directory
func TestLoad_ReqsDirDefault(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "test_patterns: [\"**/*.test.go\"]\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReqsDir != filepath.Join(dir, ".spool") {
		t.Errorf("ReqsDir = %q, want %q", cfg.ReqsDir, filepath.Join(dir, ".spool"))
	}
}

// CONFIG-3-2: Default Test Patterns
func TestLoad_NoTestPatterns(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "reqs_dir: reqs\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.TestPatterns) != 0 {
		t.Errorf("TestPatterns = %v, want empty", cfg.TestPatterns)
	}
}

// CONFIG-3-3: Invalid Glob Error
func TestLoad_InvalidGlob(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
test_patterns:
  - "**/*_test.go"
  - "[invalid"
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid glob")
	}
	if !contains(err.Error(), "[invalid") {
		t.Errorf("error %q should mention the bad pattern", err)
	}
}

// CONFIG-4-2: Default Severity Values
func TestLoad_SeverityPartialOverride(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
severity:
  untested_ac: "off"
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Severity.UntestedAC != SeverityOff {
		t.Errorf("UntestedAC = %q, want %q", cfg.Severity.UntestedAC, SeverityOff)
	}
	if cfg.Severity.OrphanedAnnotation != SeverityError {
		t.Errorf("OrphanedAnnotation = %q, want %q", cfg.Severity.OrphanedAnnotation, SeverityError)
	}
	if cfg.Severity.MissingAC != SeverityInfo {
		t.Errorf("MissingAC = %q, want %q", cfg.Severity.MissingAC, SeverityInfo)
	}
}

// CONFIG-4-3: Invalid Severity Error
func TestLoad_InvalidSeverity(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		field string
		value string
	}{
		{"untested_ac", "severity:\n  untested_ac: fatal", "untested_ac", "fatal"},
		{"orphaned_annotation", "severity:\n  orphaned_annotation: none", "orphaned_annotation", "none"},
		{"missing_ac", "severity:\n  missing_ac: WARN", "missing_ac", "WARN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, tt.yaml)

			_, err := Load(dir)
			if err == nil {
				t.Fatal("expected error for invalid severity")
			}
			if !contains(err.Error(), tt.field) {
				t.Errorf("error %q should mention field %q", err, tt.field)
			}
			if !contains(err.Error(), tt.value) {
				t.Errorf("error %q should mention value %q", err, tt.value)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
