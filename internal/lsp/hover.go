package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwames/Spool/internal/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func handleHover(ctx context.Context, srv *Server, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, err.Error()))
	}

	filePath := uri.URI(params.TextDocument.URI).Filename()
	line := int(params.Position.Line) // 0-based

	srv.mu.RLock()
	idx := srv.idx
	srv.mu.RUnlock()

	if idx == nil {
		return reply(ctx, nil, nil)
	}

	hover := buildHover(idx, filePath, line)
	if hover == nil {
		return reply(ctx, nil, nil)
	}
	return reply(ctx, hover, nil)
}

// buildHover returns hover info for the given file and 0-based line, or nil.
func buildHover(idx *index.Index, filePath string, line int) *protocol.Hover {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	if line >= len(lines) {
		return nil
	}

	if strings.HasSuffix(filePath, ".req") {
		return buildReqHover(idx, lines[line])
	}
	return buildTestHover(idx, lines[line])
}

// buildTestHover returns hover info for a Spool ID annotation in a test file.
func buildTestHover(idx *index.Index, lineText string) *protocol.Hover {
	id := extractAnnotationID(lineText)
	if id == "" {
		return nil
	}

	ac := idx.AC(id)
	if ac == nil {
		return nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "**%s: %s**\n\n", ac.ID, ac.Title)

	if desc := strings.TrimSpace(ac.Description); desc != "" {
		fmt.Fprintf(&md, "%s\n\n", desc)
	}

	if req := idx.Requirement(ac.ReqID); req != nil {
		fmt.Fprintf(&md, "*Requirement: %s — %s*", req.ID, req.Title)
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: md.String(),
		},
	}
}

// buildReqHover returns hover info for a Spool ID in a .req file.
func buildReqHover(idx *index.Index, lineText string) *protocol.Hover {
	id := extractReqID(lineText)
	if id == "" {
		return nil
	}

	// Try as AC first.
	if ac := idx.AC(id); ac != nil {
		var md strings.Builder
		fmt.Fprintf(&md, "**%s: %s**\n\n", ac.ID, ac.Title)

		if len(ac.Tests) > 0 {
			fmt.Fprintf(&md, "Covered by %d test(s):\n", len(ac.Tests))
			for _, t := range ac.Tests {
				fmt.Fprintf(&md, "- `%s` (%s:%d)\n", t.Function.Name, filepath.Base(t.File), t.Function.Line)
			}
		} else {
			md.WriteString("No covering tests")
		}

		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: md.String(),
			},
		}
	}

	// Try as requirement.
	if req := idx.Requirement(id); req != nil {
		var md strings.Builder
		fmt.Fprintf(&md, "**%s: %s**\n\n", req.ID, req.Title)

		for _, iac := range req.ACs {
			if len(iac.Tests) > 0 {
				fmt.Fprintf(&md, "- %s: %d test(s)\n", iac.ID, len(iac.Tests))
			} else {
				fmt.Fprintf(&md, "- %s: untested\n", iac.ID)
			}
		}

		return &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: md.String(),
			},
		}
	}

	return nil
}
