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

	var itemID, title, description, reqID string
	if ac := idx.AC(id); ac != nil {
		itemID, title, description, reqID = ac.ID, ac.Title, ac.Description, ac.ReqID
	} else if iv := idx.Invariant(id); iv != nil {
		itemID, title, description, reqID = iv.ID, iv.Title, iv.Description, iv.ReqID
	} else {
		return nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "**%s: %s**\n\n", itemID, title)

	if desc := strings.TrimSpace(description); desc != "" {
		fmt.Fprintf(&md, "%s\n\n", desc)
	}

	if req := idx.Requirement(reqID); req != nil {
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

	// Try as AC or invariant first.
	var itemID, itemTitle string
	var tests []index.LinkedTest
	found := false
	if ac := idx.AC(id); ac != nil {
		itemID, itemTitle, tests, found = ac.ID, ac.Title, ac.Tests, true
	} else if iv := idx.Invariant(id); iv != nil {
		itemID, itemTitle, tests, found = iv.ID, iv.Title, iv.Tests, true
	}
	if found {
		var md strings.Builder
		fmt.Fprintf(&md, "**%s: %s**\n\n", itemID, itemTitle)

		if len(tests) > 0 {
			fmt.Fprintf(&md, "Covered by %d test(s):\n", len(tests))
			for _, t := range tests {
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
		for _, iv := range req.Invariants {
			if len(iv.Tests) > 0 {
				fmt.Fprintf(&md, "- %s: %d test(s)\n", iv.ID, len(iv.Tests))
			} else {
				fmt.Fprintf(&md, "- %s: untested\n", iv.ID)
			}
		}

		if len(req.OutOfScope) > 0 {
			md.WriteString("\nOut of scope:\n")
			for _, item := range req.OutOfScope {
				fmt.Fprintf(&md, "- %s\n", item)
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
