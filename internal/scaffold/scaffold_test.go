package scaffold

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/parser"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func runInit(t *testing.T, root string) string {
	t.Helper()
	var out bytes.Buffer
	if err := Init(root, &out); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return out.String()
}

func loadConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func assertList(t *testing.T, field string, got []string, want ...string) {
	t.Helper()
	if len(want) == 0 {
		want = []string{}
	}
	if got == nil {
		got = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

// INIT-1-1: Fresh Init
func TestInit_Fresh(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"go.mod": "module x\n"})

	out := runInit(t, root)
	for _, p := range []string{".spool.yaml", ".spool/example.req", "SPOOL.md"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("%s not created: %v", p, err)
		}
		if !strings.Contains(out, "created "+p) {
			t.Errorf("output %q does not report created %s", out, p)
		}
	}
}

// INIT-1-2: Already Initialized
func TestInit_AlreadyInitialized(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{".spool.yaml": "test_patterns: []\n"})

	var out bytes.Buffer
	err := Init(root, &out)
	if !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("err = %v, want ErrAlreadyInitialized", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only .spool.yaml", len(entries))
	}
	data, _ := os.ReadFile(filepath.Join(root, ".spool.yaml"))
	if string(data) != "test_patterns: []\n" {
		t.Errorf(".spool.yaml changed to %q", data)
	}
}

// INIT-1-3: Existing Files Preserved
func TestInit_ExistingFilesPreserved(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"SPOOL.md":        "my guide\n",
		".spool/auth.req": "feature: AUTH\n",
	})

	out := runInit(t, root)
	if _, err := os.Stat(filepath.Join(root, ".spool.yaml")); err != nil {
		t.Errorf(".spool.yaml not created: %v", err)
	}
	if guide, _ := os.ReadFile(filepath.Join(root, "SPOOL.md")); string(guide) != "my guide\n" {
		t.Errorf("SPOOL.md changed to %q", guide)
	}
	if req, _ := os.ReadFile(filepath.Join(root, ".spool/auth.req")); string(req) != "feature: AUTH\n" {
		t.Errorf("auth.req changed to %q", req)
	}
	if _, err := os.Stat(filepath.Join(root, ".spool/example.req")); err == nil {
		t.Error("example.req added beside existing requirements")
	}
	for _, p := range []string{"skipped SPOOL.md", "skipped .spool/example.req"} {
		if !strings.Contains(out, p) {
			t.Errorf("output %q does not report %s", out, p)
		}
	}
}

// INIT-1-4: Generated Config Loads
func TestInit_GeneratedConfigLoads(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"Cargo.toml": "", "web/package.json": "{}"})

	runInit(t, root)
	det, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig(t, root)
	assertList(t, "TestPatterns", cfg.TestPatterns, det.TestPatterns...)
	assertList(t, "ExcludePatterns", cfg.ExcludePatterns, det.ExcludePatterns...)
}

// INIT-1-5: Generated Example Parses
func TestInit_GeneratedExampleParses(t *testing.T) {
	root := t.TempDir()
	runInit(t, root)

	result, err := parser.ParseDir(filepath.Join(root, ".spool"))
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Errorf("parse errors: %v", result.Errors)
	}
	if len(result.Files) != 1 {
		t.Errorf("got %d parsed files, want 1", len(result.Files))
	}
}

func detectIn(t *testing.T, files ...string) Detection {
	t.Helper()
	root := t.TempDir()
	m := map[string]string{}
	for _, f := range files {
		m[f] = ""
	}
	writeFiles(t, root, m)
	det, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	return det
}

// INIT-2-1: Go Project
func TestDetect_Go(t *testing.T) {
	det := detectIn(t, "go.mod")
	assertList(t, "TestPatterns", det.TestPatterns, "**/*_test.go")
	assertList(t, "ExcludePatterns", det.ExcludePatterns, "**/vendor")
}

// INIT-2-2: Rust Project
func TestDetect_Rust(t *testing.T) {
	det := detectIn(t, "Cargo.toml")
	assertList(t, "TestPatterns", det.TestPatterns, "**/*.rs")
	assertList(t, "ExcludePatterns", det.ExcludePatterns, "**/target")
}

// INIT-2-3: JavaScript Project
func TestDetect_JavaScript(t *testing.T) {
	det := detectIn(t, "package.json")
	assertList(t, "TestPatterns", det.TestPatterns, "**/*.test.*", "**/*.spec.*")
	assertList(t, "ExcludePatterns", det.ExcludePatterns, "**/node_modules", "**/dist")
}

// INIT-2-4: Java Project
func TestDetect_Java(t *testing.T) {
	for _, manifest := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		det := detectIn(t, manifest)
		assertList(t, manifest+" TestPatterns", det.TestPatterns, "**/src/test/**/*.java")
		assertList(t, manifest+" ExcludePatterns", det.ExcludePatterns, "**/target", "**/build")
	}
}

// INIT-2-5: Mixed Project
func TestDetect_Mixed(t *testing.T) {
	det := detectIn(t, "pom.xml", "Cargo.toml", "go.mod")
	assertList(t, "Types", det.Types, "go", "rust", "java")
	assertList(t, "TestPatterns", det.TestPatterns, "**/*_test.go", "**/*.rs", "**/src/test/**/*.java")
	assertList(t, "ExcludePatterns", det.ExcludePatterns, "**/vendor", "**/target", "**/build")
}

// INIT-2-6: Nested Manifest Detected
func TestDetect_NestedManifest(t *testing.T) {
	det := detectIn(t, "crates/core/Cargo.toml")
	assertList(t, "Types", det.Types, "rust")
}

// INIT-2-7: Dependency Manifests Ignored
func TestDetect_DependencyManifestsIgnored(t *testing.T) {
	det := detectIn(t,
		"node_modules/a/package.json",
		"vendor/b/go.mod",
		"target/package/Cargo.toml",
		"dist/package.json",
		"build/pom.xml",
		".git/modules/x/Cargo.toml",
	)
	assertList(t, "Types", det.Types)
}

// INIT-2-8: Unknown Project
func TestInit_UnknownProject(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"README.md": "hi\n"})

	out := runInit(t, root)
	cfg := loadConfig(t, root)
	assertList(t, "TestPatterns", cfg.TestPatterns)
	if !strings.Contains(out, "no project type detected") || !strings.Contains(out, "test_patterns") {
		t.Errorf("output %q does not say to edit test_patterns", out)
	}
}
