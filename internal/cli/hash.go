package cli

import "io"

// Hash runs `spool hash <ID>,<ID>,...` in the project at root, with args
// being the arguments after "hash", and returns the exit status: 0 with the
// hash on stdout, 1 when an ID is unknown, 2 for a usage error (SPEC-5).
func Hash(args []string, root string, stdout, stderr io.Writer) int {
	return 2
}
