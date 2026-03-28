package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/spoolid"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func handleCodeLens(ctx context.Context, srv *Server, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeLensParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, err.Error()))
	}

	filePath := uri.URI(params.TextDocument.URI).Filename()

	srv.mu.RLock()
	idx := srv.idx
	srv.mu.RUnlock()

	if idx == nil {
		return reply(ctx, []protocol.CodeLens{}, nil)
	}

	lenses := collectCodeLenses(idx, filePath)
	return reply(ctx, lenses, nil)
}

// collectCodeLenses returns code lenses for all Spool IDs in the given file.
func collectCodeLenses(idx *index.Index, filePath string) []protocol.CodeLens {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")

	if strings.HasSuffix(filePath, ".req") {
		return collectReqCodeLenses(idx, lines)
	}
	return collectTestCodeLenses(idx, filePath, lines)
}

func collectReqCodeLenses(idx *index.Index, lines []string) []protocol.CodeLens {
	var lenses []protocol.CodeLens
	for i, line := range lines {
		m := reqIDPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		candidate := m[1]
		if !spoolid.IsValid(candidate) {
			continue
		}
		normalized, _ := spoolid.NormalizeID(candidate)

		ac := idx.AC(normalized)
		if ac == nil {
			continue
		}

		r := protocol.Range{
			Start: protocol.Position{Line: uint32(i)},
			End:   protocol.Position{Line: uint32(i)},
		}

		if len(ac.Tests) > 0 {
			// Build location arguments for the command.
			var args []interface{}
			for _, t := range ac.Tests {
				args = append(args, map[string]interface{}{
					"uri":  string(uri.File(t.File)),
					"line": t.Function.Line - 1, // 0-based for LSP
				})
			}

			title := fmt.Sprintf("%d test(s)", len(ac.Tests))
			lenses = append(lenses, protocol.CodeLens{
				Range: r,
				Command: &protocol.Command{
					Title:     title,
					Command:   "spool.goToTests",
					Arguments: args,
				},
			})
		} else {
			lenses = append(lenses, protocol.CodeLens{
				Range: r,
				Command: &protocol.Command{
					Title:   "untested",
					Command: "",
				},
			})
		}
	}
	return lenses
}

func collectTestCodeLenses(idx *index.Index, filePath string, lines []string) []protocol.CodeLens {
	var lenses []protocol.CodeLens
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		var body string
		found := false
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				body = strings.TrimSpace(trimmed[len(prefix):])
				found = true
				break
			}
		}
		if !found {
			continue
		}

		candidate := body
		if ci := strings.Index(body, ":"); ci > 0 {
			candidate = strings.TrimSpace(body[:ci])
		}
		if !spoolid.IsValid(candidate) {
			continue
		}
		normalized, _ := spoolid.NormalizeID(candidate)

		ac := idx.AC(normalized)
		if ac == nil {
			continue
		}

		title := fmt.Sprintf("covers: %s (%s)", ac.Title, ac.ID)
		lenses = append(lenses, protocol.CodeLens{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(i)},
				End:   protocol.Position{Line: uint32(i)},
			},
			Command: &protocol.Command{
				Title:   title,
				Command: "spool.goToAC",
				Arguments: []interface{}{
					map[string]interface{}{
						"uri":  string(uri.File(ac.SourceFile)),
						"line": ac.Line - 1, // 0-based for LSP
					},
				},
			},
		})
	}
	return lenses
}

// testLocArg is used to marshal location arguments for code lens commands.
type testLocArg struct {
	URI      string `json:"uri"`
	Line     int    `json:"line"`
	FuncName string `json:"funcName"`
	FileName string `json:"fileName"`
}

// buildTestLocArgs creates serializable location arguments for a set of linked tests.
func buildTestLocArgs(tests []index.LinkedTest) []interface{} {
	var args []interface{}
	for _, t := range tests {
		args = append(args, testLocArg{
			URI:      string(uri.File(t.File)),
			Line:     t.Function.Line - 1,
			FuncName: t.Function.Name,
			FileName: filepath.Base(t.File),
		})
	}
	return args
}
