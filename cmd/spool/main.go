package main

import (
	"fmt"
	"os"

	"github.com/mwames/Spool/internal/cli"
	spoolsp "github.com/mwames/Spool/internal/lsp"
	"github.com/mwames/Spool/internal/scaffold"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		spoolsp.Run()
		return
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := scaffold.Init(root, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	os.Exit(cli.Run(os.Args[1:], root, os.Stdout, os.Stderr))
}
