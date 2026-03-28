package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// newHandler returns a jsonrpc2.Handler that dispatches LSP methods.
func newHandler(srv *Server, conn jsonrpc2.Conn) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case "initialize":
			return handleInitialize(ctx, srv, reply, req)
		case "initialized":
			return reply(ctx, nil, nil)
		case "shutdown":
			return reply(ctx, nil, nil)
		case "exit":
			return reply(ctx, nil, nil)
		case "textDocument/didOpen":
			_ = srv.reindex(ctx)
			return reply(ctx, nil, nil)
		case "textDocument/didClose":
			return reply(ctx, nil, nil)
		case "textDocument/didSave":
			return handleDidSave(ctx, srv, reply, req)
		case "textDocument/definition":
			return handleDefinition(ctx, srv, reply, req)
		case "textDocument/documentLink":
			return handleDocumentLink(ctx, srv, reply, req)
		case "textDocument/codeLens":
			return handleCodeLens(ctx, srv, reply, req)
		case "textDocument/hover":
			return handleHover(ctx, srv, reply, req)
		case "spool/traceability":
			return handleTraceability(ctx, srv, reply, req)
		default:
			return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "method not supported"))
		}
	}
}

func handleInitialize(ctx context.Context, srv *Server, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, err.Error()))
	}

	if params.RootURI != "" {
		srv.projectRoot = uri.URI(params.RootURI).Filename()
	}

	_ = srv.reindex(ctx)

	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Save: &protocol.SaveOptions{
					IncludeText: false,
				},
			},
			DefinitionProvider:   true,
			DocumentLinkProvider: &protocol.DocumentLinkOptions{},
			CodeLensProvider:     &protocol.CodeLensOptions{},
			HoverProvider:        true,
		},
	}
	return reply(ctx, result, nil)
}

func handleDidSave(ctx context.Context, srv *Server, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	_ = srv.reindex(ctx)
	return reply(ctx, nil, nil)
}

func handleDocumentLink(ctx context.Context, srv *Server, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentLinkParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, err.Error()))
	}

	filePath := uri.URI(params.TextDocument.URI).Filename()

	srv.mu.RLock()
	idx := srv.idx
	srv.mu.RUnlock()

	if idx == nil {
		return reply(ctx, nil, nil)
	}

	links := collectLinks(idx, filePath)
	if len(links) == 0 {
		return reply(ctx, []protocol.DocumentLink{}, nil)
	}
	return reply(ctx, links, nil)
}

func handleDefinition(ctx context.Context, srv *Server, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, err.Error()))
	}

	filePath := uri.URI(params.TextDocument.URI).Filename()
	line := int(params.Position.Line) // 0-based from LSP

	srv.mu.RLock()
	idx := srv.idx
	srv.mu.RUnlock()

	if idx == nil {
		return reply(ctx, nil, nil)
	}

	locations := resolve(idx, filePath, line)
	if len(locations) == 0 {
		return reply(ctx, nil, nil)
	}
	return reply(ctx, locations, nil)
}
