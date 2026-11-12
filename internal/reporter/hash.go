package reporter

import "github.com/mwames/Spool/internal/index"

// SpecHash returns the lowercase hex SHA-256 of the canonical form of the
// given normalized AC and invariant IDs (SPEC-5-2), and the IDs the index
// doesn't define, in the order given. The hash is meaningful only when
// unknown is empty.
func SpecHash(idx *index.Index, ids []string) (hash string, unknown []string) {
	return "", nil
}
