package lsp

import (
	"context"
	"fmt"

	"github.com/mwames/Spool/internal/index"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// buildDiagnostics collects warning diagnostics grouped by file URI.
func buildDiagnostics(idx *index.Index) map[protocol.DocumentURI][]protocol.Diagnostic {
	diags := make(map[protocol.DocumentURI][]protocol.Diagnostic)

	for _, ac := range idx.UntestedACs() {
		fileURI := uri.File(ac.SourceFile)
		diags[fileURI] = append(diags[fileURI], protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(ac.Line - 1)},
				End:   protocol.Position{Line: uint32(ac.Line - 1), Character: 1000},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "spool",
			Message:  fmt.Sprintf("Untested: %s (%s)", ac.Title, ac.ID),
		})
	}

	for _, ann := range idx.OrphanedAnnotations() {
		fileURI := uri.File(ann.File)
		diags[fileURI] = append(diags[fileURI], protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(ann.Line - 1)},
				End:   protocol.Position{Line: uint32(ann.Line - 1), Character: 1000},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "spool",
			Message:  fmt.Sprintf("No matching AC: %s", ann.ID),
		})
	}

	return diags
}

// publishDiagnostics sends diagnostics for all affected files to the client.
func (s *Server) publishDiagnostics(ctx context.Context) {
	if s.conn == nil {
		return
	}

	s.mu.RLock()
	idx := s.idx
	s.mu.RUnlock()

	if idx == nil {
		return
	}

	diags := buildDiagnostics(idx)

	// Publish diagnostics for files that have warnings.
	for fileURI, fileDiags := range diags {
		_ = s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			URI:         fileURI,
			Diagnostics: fileDiags,
		})
	}
}
