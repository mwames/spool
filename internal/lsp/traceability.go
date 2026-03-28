package lsp

import (
	"context"

	"github.com/mwames/Spool/internal/index"
	"go.lsp.dev/jsonrpc2"
)

// TraceabilityTree is the response for the spool/traceability custom request.
type TraceabilityTree struct {
	Features []TraceabilityFeature `json:"features"`
}

// TraceabilityFeature represents a feature in the tree.
type TraceabilityFeature struct {
	Name         string                    `json:"name"`
	Requirements []TraceabilityRequirement `json:"requirements"`
}

// TraceabilityRequirement represents a requirement in the tree.
type TraceabilityRequirement struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Status string           `json:"status"`
	File   string           `json:"file"`
	Line   int              `json:"line"`
	ACs    []TraceabilityAC `json:"acs"`
}

// TraceabilityAC represents an acceptance criterion in the tree.
type TraceabilityAC struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	TestCount int    `json:"testCount"`
}

// buildTraceabilityTree constructs the full tree from the index.
func buildTraceabilityTree(idx *index.Index) TraceabilityTree {
	features := idx.Features()
	tree := TraceabilityTree{
		Features: make([]TraceabilityFeature, 0, len(features)),
	}

	for _, f := range features {
		tf := TraceabilityFeature{
			Name:         f.Name,
			Requirements: make([]TraceabilityRequirement, 0, len(f.Requirements)),
		}
		for _, r := range f.Requirements {
			tr := TraceabilityRequirement{
				ID:     r.ID,
				Title:  r.Title,
				Status: r.Status,
				File:   r.SourceFile,
				Line:   r.Line,
				ACs:    make([]TraceabilityAC, 0, len(r.ACs)),
			}
			for _, ac := range r.ACs {
				tr.ACs = append(tr.ACs, TraceabilityAC{
					ID:        ac.ID,
					Title:     ac.Title,
					File:      ac.SourceFile,
					Line:      ac.Line,
					TestCount: len(ac.Tests),
				})
			}
			tf.Requirements = append(tf.Requirements, tr)
		}
		tree.Features = append(tree.Features, tf)
	}

	return tree
}

func handleTraceability(ctx context.Context, srv *Server, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	srv.mu.RLock()
	idx := srv.idx
	srv.mu.RUnlock()

	if idx == nil {
		return reply(ctx, TraceabilityTree{Features: []TraceabilityFeature{}}, nil)
	}

	tree := buildTraceabilityTree(idx)
	return reply(ctx, tree, nil)
}
