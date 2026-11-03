package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const demoReq = `feature: DEMO

requirements:
  - id: DEMO-1
    title: Demo
    description: A demo requirement.
    status: active
    deciders:
      - Matt Ames
    consulted: []
    date: 2026-11-03
    rationale: Fixture.
    superseded_by:
    acceptance_criteria:
      - id: DEMO-1-1
        title: Demo AC
        description: >
          Given a demo, when it runs, then it passes.
`

// demoTest links DEMO-1-1. It is one line so that Spool does not read the
// annotation inside the string as an annotation in this file.
const demoTest = "package demo\n\nimport \"testing\"\n\n// DEMO-1-1\nfunc TestDemo(t *testing.T) {}\n"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func demoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".spool.yaml"), "test_patterns:\n  - \"**/*_test.go\"\n")
	writeFile(t, filepath.Join(dir, ".spool", "demo.req"), demoReq)
	return dir
}

// call sends one message through the server's handler and returns the
// result it replied with.
func call(t *testing.T, srv *Server, msg jsonrpc2.Request) json.RawMessage {
	t.Helper()
	var result json.RawMessage
	var replyErr error
	reply := func(_ context.Context, r interface{}, err error) error {
		replyErr = err
		if r != nil {
			b, mErr := json.Marshal(r)
			if mErr != nil {
				t.Fatal(mErr)
			}
			result = b
		}
		return nil
	}
	if err := newHandler(srv, nil)(context.Background(), reply, msg); err != nil {
		t.Fatalf("%s: handler error: %v", msg.Method(), err)
	}
	if replyErr != nil {
		t.Fatalf("%s: replied with error: %v", msg.Method(), replyErr)
	}
	return result
}

func initialize(t *testing.T, srv *Server, root string) json.RawMessage {
	t.Helper()
	req, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), "initialize", protocol.InitializeParams{
		RootURI: protocol.DocumentURI(uri.File(root)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return call(t, srv, req)
}

// LSP-3-1
func TestHandleInitialize_ReturnsCapabilities(t *testing.T) {
	dir := demoProject(t)
	srv := NewServer("")

	raw := initialize(t, srv, dir)

	var result protocol.InitializeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if def, ok := result.Capabilities.DefinitionProvider.(bool); !ok || !def {
		t.Errorf("DefinitionProvider = %v, want true", result.Capabilities.DefinitionProvider)
	}
	if srv.projectRoot != dir {
		t.Errorf("projectRoot = %q, want %q", srv.projectRoot, dir)
	}
	if srv.idx == nil || srv.idx.AC("DEMO-1-1") == nil {
		t.Error("initialize did not index the project at rootUri")
	}
}

// LSP-3-2
func TestHandleDidSave_RebuildsIndexFromDisk(t *testing.T) {
	dir := demoProject(t)
	srv := NewServer("")
	initialize(t, srv, dir)

	if ac := srv.idx.AC("DEMO-1-1"); ac == nil || len(ac.Tests) != 0 {
		t.Fatalf("before save: DEMO-1-1 = %+v, want indexed and untested", ac)
	}

	writeFile(t, filepath.Join(dir, "demo_test.go"), demoTest)
	save, err := jsonrpc2.NewNotification("textDocument/didSave", protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri.File(filepath.Join(dir, "demo_test.go")))},
	})
	if err != nil {
		t.Fatal(err)
	}
	call(t, srv, save)

	ac := srv.idx.AC("DEMO-1-1")
	if ac == nil || len(ac.Tests) != 1 || ac.Tests[0].Function.Name != "TestDemo" {
		t.Errorf("after save: DEMO-1-1 = %+v, want linked to TestDemo", ac)
	}
}
