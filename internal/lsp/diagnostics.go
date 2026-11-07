package lsp

import (
	"context"
	"fmt"

	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/lint"
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

	for _, iv := range idx.UntestedInvariants() {
		fileURI := uri.File(iv.SourceFile)
		diags[fileURI] = append(diags[fileURI], protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(iv.Line - 1)},
				End:   protocol.Position{Line: uint32(iv.Line - 1), Character: 1000},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "spool",
			Message:  fmt.Sprintf("Untested: %s (%s)", iv.Title, iv.ID),
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

	for _, feat := range idx.Features() {
		for _, req := range feat.Requirements {
			if req.Status != "active" {
				continue
			}
			for _, ac := range req.ACs {
				if lint.GivenWhenThen(ac.Description) {
					continue
				}
				fileURI := uri.File(ac.SourceFile)
				diags[fileURI] = append(diags[fileURI], lineDiagnostic(ac.Line,
					fmt.Sprintf("Not written as Given/When/Then: %s (%s)", ac.Title, ac.ID)))
			}
		}
	}

	return diags
}

// lineDiagnostic is a warning across the 1-based line.
func lineDiagnostic(line int, message string) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line - 1)},
			End:   protocol.Position{Line: uint32(line - 1), Character: 1000},
		},
		Severity: protocol.DiagnosticSeverityWarning,
		Source:   "spool",
		Message:  message,
	}
}

// interfaceDiagnostics collects a warning on the ID line of each active
// requirement with a missing interface path, naming the path (SPEC-1-5).
// ACs not written as Given/When/Then are warned about by buildDiagnostics
// (SPEC-3-4).
func interfaceDiagnostics(idx *index.Index, root string) map[protocol.DocumentURI][]protocol.Diagnostic {
	diags := make(map[protocol.DocumentURI][]protocol.Diagnostic)
	for _, m := range lint.MissingInterfaces(idx, root) {
		req := idx.Requirement(m.ID)
		if req == nil {
			continue
		}
		fileURI := uri.File(req.SourceFile)
		diags[fileURI] = append(diags[fileURI], lineDiagnostic(req.Line,
			fmt.Sprintf("Interface path does not exist: %s (%s)", m.Path, m.ID)))
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
	for fileURI, fileDiags := range interfaceDiagnostics(idx, s.projectRoot) {
		diags[fileURI] = append(diags[fileURI], fileDiags...)
	}

	// Publish diagnostics for files that have warnings.
	for fileURI, fileDiags := range diags {
		_ = s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
			URI:         fileURI,
			Diagnostics: fileDiags,
		})
	}
}
