# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Spool?

Spool is a requirements traceability tool written in Go. It links structured requirements (defined in `.req` YAML files) to test annotations in source code, then generates coverage reports showing which acceptance criteria have tests and which don't.

## Build & Test Commands

```bash
go build ./...                          # Build all packages
go run ./cmd/spool                      # Run the CLI (from a project with .spool.yaml)
go test ./...                           # Run all tests
go test ./internal/parser               # Run tests for a single package
go test ./internal/parser -run TestName # Run a single test
```

No external tools (linters, formatters, task runners) are configured. The only dependency is `gopkg.in/yaml.v3`.

## Architecture

The pipeline runs as a linear chain in `cmd/spool/main.go`:

**config** → **parser** → **scanner** → **index** → **reporter**

1. **config** (`internal/config/`) — Loads `.spool.yaml` from the project root. Defaults: reqs dir is `.spool/`, severity levels are warning/error/info.
2. **parser** (`internal/parser/`) — Walks the reqs directory and parses `.req` files (YAML). Normalizes all Spool IDs via the `spoolid` package.
3. **scanner** (`internal/scanner/`) — Reads test files matched by glob patterns, detects Spool ID annotations in comments (`//` or `#` prefixed), extracts test functions via **interpreters**, and associates annotations with the test function that follows them.
4. **index** (`internal/index/`) — Builds a traceability graph joining requirements/ACs to test annotations. Detects duplicates, prefix mismatches, untested ACs, and orphaned annotations.
5. **reporter** (`internal/reporter/`) — Generates a `Report` from the index and formats it as text, JSON, or markdown.

### Key types

- Core interfaces and report types live in the **root package** (`spool.go`): `Interpreter`, `TestFunction`, `Formatter`, `Report`.
- **Spool IDs** (`internal/spoolid/`) follow the pattern `PREFIX-N` (requirement) or `PREFIX-N-N` (acceptance criterion). The prefix must match the feature name. IDs are normalized to uppercase with no leading zeros.
- **Interpreters** (`internal/scanner/interpreters/`) implement the `spool.Interpreter` interface for different test frameworks: Go, JUnit, Jest, Playwright. Each detects whether it supports a file and extracts test function names/lines.

### Req file format

`.req` files are YAML with structure: `feature`, `requirements[]` each having `id`, `title`, `description`, `status`, `acceptance_criteria[]`. Requirements with `status: active` are included in traceability; other statuses are excluded.

### Test annotation convention

Tests link to ACs by placing a comment with the AC's Spool ID above the test function:

```go
// CONFIG-1-1
func TestValidConfigParsing(t *testing.T) { ... }
```

## Self-hosting

Spool uses itself for requirements traceability. The `.spool/` directory contains `.req` files defining Spool's own requirements, and the test files contain Spool ID annotations linking back to those requirements.
