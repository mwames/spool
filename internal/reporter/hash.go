package reporter

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/spoolid"
)

// SpecHash returns the lowercase hex SHA-256 of the canonical form of the
// given normalized AC and invariant IDs (SPEC-5-2), and the IDs the index
// doesn't define, in the order given. The hash is meaningful only when
// unknown is empty.
func SpecHash(idx *index.Index, ids []string) (hash string, unknown []string) {
	type item struct {
		id, title, desc string
		req             *index.IndexedReq
	}
	var items []item
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if ac := idx.AC(id); ac != nil {
			items = append(items, item{id, ac.Title, ac.Description, idx.Requirement(ac.ReqID)})
		} else if inv := idx.Invariant(id); inv != nil {
			items = append(items, item{id, inv.Title, inv.Description, idx.Requirement(inv.ReqID)})
		} else {
			unknown = append(unknown, id)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })

	var b strings.Builder
	field := func(s string) {
		b.WriteString(strconv.Itoa(len(s)))
		b.WriteByte(':')
		b.WriteString(s)
		b.WriteByte('\n')
	}
	text := func(s string) { field(strings.Join(strings.Fields(s), " ")) }
	normID := func(s string) string {
		if n, err := spoolid.NormalizeID(s); err == nil {
			return n
		}
		return s
	}
	for _, it := range items {
		field(normID(it.id))
		text(it.title)
		text(it.desc)
		r := it.req
		if r == nil {
			continue
		}
		field(normID(r.ID))
		text(r.Title)
		text(r.Description)
		field(strconv.Itoa(len(r.Interface)))
		for _, p := range r.Interface {
			text(p)
		}
		field(strconv.Itoa(len(r.OutOfScope)))
		for _, s := range r.OutOfScope {
			text(s)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), unknown
}
