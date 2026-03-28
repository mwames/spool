// Package lsp implements a Language Server Protocol server for Spool.
package lsp

import (
	"context"
	"fmt"
	"os"
	"sync"

	spool "github.com/mwames/Spool"
	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/project"
	"github.com/mwames/Spool/internal/scanner"
	"go.lsp.dev/jsonrpc2"
)

// Server holds the state for the Spool LSP server.
type Server struct {
	mu           sync.RWMutex
	conn         jsonrpc2.Conn
	projectRoot  string
	cfg          *config.Config
	idx          *index.Index
	scanResults  []scanner.FileResult
	interpreters []spool.Interpreter
}

// NewServer creates a new Server for the given project root.
func NewServer(root string) *Server {
	return &Server{
		projectRoot:  root,
		interpreters: project.DefaultInterpreters(),
	}
}

// reindex rebuilds the traceability index from disk and publishes diagnostics.
func (s *Server) reindex(ctx context.Context) error {
	cfg, err := config.Load(s.projectRoot)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	parseResult, err := parser.ParseDir(cfg.ReqsDir)
	if err != nil {
		return fmt.Errorf("parser: %w", err)
	}

	testFiles, err := project.FindTestFiles(s.projectRoot, cfg.TestPatterns)
	if err != nil {
		return fmt.Errorf("find test files: %w", err)
	}

	scanResult := scanner.Scan(testFiles, s.interpreters)
	idx := index.Build(parseResult.Files, scanResult.Files)

	s.mu.Lock()
	s.cfg = cfg
	s.idx = idx
	s.scanResults = scanResult.Files
	s.mu.Unlock()

	s.publishDiagnostics(ctx)
	return nil
}

// Run starts the LSP server on stdin/stdout.
func Run() {
	ctx := context.Background()

	stream := jsonrpc2.NewStream(stdrwc{})
	srv := NewServer("")

	conn := jsonrpc2.NewConn(stream)
	srv.conn = conn
	conn.Go(ctx, newHandler(srv, conn))

	<-conn.Done()
	if err := conn.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "lsp: %v\n", err)
		os.Exit(1)
	}
}

// stdrwc wraps stdin/stdout as an io.ReadWriteCloser.
type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdrwc) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdrwc) Close() error                { return nil }
