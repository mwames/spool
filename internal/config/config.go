// Package config handles loading and defaulting of .spool.yaml configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Severity is a diagnostic severity level.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityOff     Severity = "off"
)

// SeverityConfig controls diagnostic severity levels.
type SeverityConfig struct {
	UntestedAC         Severity `yaml:"untested_ac"`
	OrphanedAnnotation Severity `yaml:"orphaned_annotation"`
	MissingAC          Severity `yaml:"missing_ac"`
}

// Config is the parsed representation of .spool.yaml.
type Config struct {
	ReqsDir      string         `yaml:"reqs_dir"`
	TestPatterns []string       `yaml:"test_patterns"`
	Severity     SeverityConfig `yaml:"severity"`
}

// Load reads .spool.yaml from projectRoot and returns the parsed Config.
// If .spool.yaml does not exist, defaults are returned with no error.
func Load(projectRoot string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".spool.yaml"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			applyDefaults(cfg, projectRoot)
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing .spool.yaml: %w", err)
	}

	applyDefaults(cfg, projectRoot)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyDefaults(cfg *Config, projectRoot string) {
	if cfg.ReqsDir == "" {
		cfg.ReqsDir = ".spool"
	}
	if !filepath.IsAbs(cfg.ReqsDir) {
		cfg.ReqsDir = filepath.Join(projectRoot, cfg.ReqsDir)
	}

	if cfg.Severity.UntestedAC == "" {
		cfg.Severity.UntestedAC = SeverityWarning
	}
	if cfg.Severity.OrphanedAnnotation == "" {
		cfg.Severity.OrphanedAnnotation = SeverityError
	}
	if cfg.Severity.MissingAC == "" {
		cfg.Severity.MissingAC = SeverityInfo
	}
}

func validate(cfg *Config) error {
	for _, p := range cfg.TestPatterns {
		if _, err := filepath.Match(p, ""); err != nil {
			return fmt.Errorf("test_patterns: invalid glob %q: %w", p, err)
		}
	}

	checks := []struct {
		field string
		value Severity
	}{
		{"untested_ac", cfg.Severity.UntestedAC},
		{"orphaned_annotation", cfg.Severity.OrphanedAnnotation},
		{"missing_ac", cfg.Severity.MissingAC},
	}
	for _, c := range checks {
		if !validSeverity(c.value) {
			return fmt.Errorf("severity.%s: invalid value %q (must be error, warning, info, or off)", c.field, c.value)
		}
	}

	return nil
}

func validSeverity(s Severity) bool {
	switch s {
	case SeverityError, SeverityWarning, SeverityInfo, SeverityOff:
		return true
	}
	return false
}
