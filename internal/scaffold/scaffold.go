// Package scaffold implements spool init: it detects the project types in a
// directory and writes .spool.yaml, an example requirement, and SPOOL.md.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/example.req.tmpl templates/SPOOL.md
var templates embed.FS

// ErrAlreadyInitialized is returned when the directory already has a .spool.yaml.
var ErrAlreadyInitialized = errors.New("spool is already initialized here: .spool.yaml exists")

// ProjectType describes a kind of project and the patterns that suit it.
type ProjectType struct {
	Name            string
	Manifests       []string
	TestPatterns    []string
	ExcludePatterns []string
}

// ProjectTypes are checked in this order, which fixes the order of the
// patterns written for mixed projects.
var ProjectTypes = []ProjectType{
	{"go", []string{"go.mod"}, []string{"**/*_test.go"}, []string{"**/vendor"}},
	{"rust", []string{"Cargo.toml"}, []string{"**/*.rs"}, []string{"**/target"}},
	{"javascript", []string{"package.json"}, []string{"**/*.test.*", "**/*.spec.*"}, []string{"**/node_modules", "**/dist"}},
	{"java", []string{"pom.xml", "build.gradle", "build.gradle.kts"}, []string{"**/src/test/**/*.java"}, []string{"**/target", "**/build"}},
}

// detectSkipDirs hold third-party or generated code whose manifests must not
// be mistaken for the project's own.
var detectSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true, "dist": true, "build": true,
}

// Detection is the result of scanning a directory for project manifests.
type Detection struct {
	Types           []string
	TestPatterns    []string
	ExcludePatterns []string
}

// Detect walks root for manifest files and combines the patterns of every
// project type found, each pattern once.
func Detect(root string) (Detection, error) {
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && detectSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		found[d.Name()] = true
		return nil
	})
	if err != nil {
		return Detection{}, err
	}

	det := Detection{TestPatterns: []string{}, ExcludePatterns: []string{}}
	for _, pt := range ProjectTypes {
		for _, m := range pt.Manifests {
			if found[m] {
				det.Types = append(det.Types, pt.Name)
				det.TestPatterns = appendUnique(det.TestPatterns, pt.TestPatterns...)
				det.ExcludePatterns = appendUnique(det.ExcludePatterns, pt.ExcludePatterns...)
				break
			}
		}
	}
	return det, nil
}

func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		dup := false
		for _, existing := range list {
			if existing == item {
				dup = true
				break
			}
		}
		if !dup {
			list = append(list, item)
		}
	}
	return list
}

// Init scaffolds Spool in root, reporting each created or skipped path to out.
// Existing files are never overwritten.
func Init(root string, out io.Writer) error {
	configPath := filepath.Join(root, ".spool.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return ErrAlreadyInitialized
	}

	det, err := Detect(root)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(renderConfig(det)), 0644); err != nil {
		return err
	}
	fmt.Fprintln(out, "created .spool.yaml")
	if len(det.Types) == 0 {
		fmt.Fprintln(out, "no project type detected: edit test_patterns in .spool.yaml")
	} else {
		fmt.Fprintf(out, "detected: %s\n", strings.Join(det.Types, ", "))
	}

	reqsDir := filepath.Join(root, ".spool")
	if err := os.MkdirAll(reqsDir, 0755); err != nil {
		return err
	}
	existing, _ := filepath.Glob(filepath.Join(reqsDir, "*.req"))
	if len(existing) > 0 {
		fmt.Fprintln(out, "skipped .spool/example.req (.spool/ already has requirements)")
	} else {
		example, _ := templates.ReadFile("templates/example.req.tmpl")
		dated := strings.ReplaceAll(string(example), "{{DATE}}", time.Now().Format("2006-01-02"))
		if err := os.WriteFile(filepath.Join(reqsDir, "example.req"), []byte(dated), 0644); err != nil {
			return err
		}
		fmt.Fprintln(out, "created .spool/example.req")
	}

	guidePath := filepath.Join(root, "SPOOL.md")
	if _, err := os.Stat(guidePath); err == nil {
		fmt.Fprintln(out, "skipped SPOOL.md (already exists)")
	} else {
		guide, _ := templates.ReadFile("templates/SPOOL.md")
		if err := os.WriteFile(guidePath, guide, 0644); err != nil {
			return err
		}
		fmt.Fprintln(out, "created SPOOL.md")
	}
	return nil
}

func renderConfig(det Detection) string {
	var b strings.Builder
	writeList(&b, "test_patterns", det.TestPatterns)
	writeList(&b, "exclude_patterns", det.ExcludePatterns)
	return b.String()
}

func writeList(b *strings.Builder, key string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "%s: []\n", key)
		return
	}
	fmt.Fprintf(b, "%s:\n", key)
	for _, item := range items {
		fmt.Fprintf(b, "  - %q\n", item)
	}
}
