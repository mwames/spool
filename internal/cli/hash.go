package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mwames/Spool/internal/config"
	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/parser"
	"github.com/mwames/Spool/internal/reporter"
)

// Hash runs `spool hash <ID>,<ID>,...` in the project at root, with args
// being the arguments after "hash", and returns the exit status: 0 with the
// hash on stdout, 1 when an ID is unknown, 2 for a usage error (SPEC-5).
func Hash(args []string, root string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "usage error: spool hash needs exactly one comma-separated ID list\n")
		return 2
	}
	ids, err := ParseIDs(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "usage error: %v\n", err)
		return 2
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 1
	}
	parseResult, err := parser.ParseDir(cfg.ReqsDir)
	if err != nil {
		fmt.Fprintf(stderr, "parser error: %v\n", err)
		return 1
	}
	for _, e := range parseResult.Errors {
		fmt.Fprintf(stderr, "parse warning: %v\n", e)
	}
	idx := index.Build(parseResult.Files, nil)

	hash, unknown := reporter.SpecHash(idx, ids)
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "unknown IDs: %s\n", strings.Join(unknown, ", "))
		return 1
	}
	fmt.Fprintln(stdout, hash)
	return 0
}
