package reporter

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/mwames/Spool/internal/parser"
)

// checkCanonicalLengths asserts that each line of a hand-written canonical
// form is "<n>:<field>" with n the field's length in bytes, so that a
// miscounted fixture fails as a fixture rather than as the code under test.
func checkCanonicalLengths(t *testing.T, canonical string) {
	t.Helper()
	if !strings.HasSuffix(canonical, "\n") {
		t.Fatalf("fixture: canonical form doesn't end in a newline")
	}
	for _, line := range strings.Split(strings.TrimSuffix(canonical, "\n"), "\n") {
		n, field, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("fixture: line %q has no colon", line)
		}
		if want, err := strconv.Atoi(n); err != nil || want != len(field) {
			t.Fatalf("fixture: line %q: length %s, field is %d bytes", line, n, len(field))
		}
	}
}

// SPEC-5-2
func TestSpecHash_CanonicalForm(t *testing.T) {
	r := req("AUTH-1", "active",
		parser.AcceptanceCriterion{ID: "AUTH-1-2", Title: "Pass\nword", Description: "Given a password,\nwhen   it logs in,\tthen it holds.\n"},
		parser.AcceptanceCriterion{ID: "AUTH-1-10", Title: "Left out", Description: "Given it is unlisted, when hashed, then it is left out."},
	)
	r.Title = "  Log\tin "
	r.Description = "Users\n  log in.\n"
	r.Interface = parser.StringList{"api/z.go", "api/auth.go"}
	r.OutOfScope = parser.StringList{"SSO", "  Two  factor "}
	r = withInvariants(r, parser.Invariant{ID: "AUTH-1-INV-1", Title: "Ünïcode ✓", Description: ""})
	other := req("AUTH-2", "draft", parser.AcceptanceCriterion{ID: "AUTH-2-1", Title: "Bare", Description: "Given none, when hashed, then counts are zero."})
	idx := buildIndex([]*parser.ReqFile{reqFile("AUTH", r, other)}, nil)

	// Ordered by ID in byte order: "AUTH-1-2" < "AUTH-1-INV-1" < "AUTH-2-1".
	canonical := "" +
		"8:AUTH-1-2\n" +
		"9:Pass word\n" +
		"49:Given a password, when it logs in, then it holds.\n" +
		"6:AUTH-1\n" +
		"6:Log in\n" +
		"13:Users log in.\n" +
		"1:2\n" +
		"8:api/z.go\n" +
		"11:api/auth.go\n" +
		"1:2\n" +
		"3:SSO\n" +
		"10:Two factor\n" +
		"12:AUTH-1-INV-1\n" +
		"13:Ünïcode ✓\n" +
		"0:\n" +
		"6:AUTH-1\n" +
		"6:Log in\n" +
		"13:Users log in.\n" +
		"1:2\n" +
		"8:api/z.go\n" +
		"11:api/auth.go\n" +
		"1:2\n" +
		"3:SSO\n" +
		"10:Two factor\n" +
		"8:AUTH-2-1\n" +
		"4:Bare\n" +
		"46:Given none, when hashed, then counts are zero.\n" +
		"6:AUTH-2\n" +
		"6:AUTH-2\n" +
		"0:\n" +
		"1:0\n" +
		"1:0\n"
	checkCanonicalLengths(t, canonical)
	sum := sha256.Sum256([]byte(canonical))
	want := hex.EncodeToString(sum[:])

	got, unknown := SpecHash(idx, []string{"AUTH-2-1", "AUTH-1-INV-1", "AUTH-1-2"})
	if len(unknown) != 0 {
		t.Errorf("unknown = %v, want none", unknown)
	}
	if got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

// SPEC-5-2
func TestSpecHash_UnicodeWhitespace(t *testing.T) {
	// Every character here other than ASCII space with the Unicode
	// White_Space property collapses and is trimmed like a space; the
	// zero-width space, word joiner and zero-width no-break space don't have
	// it and are kept as written.
	r := req("UNI-1", "active", parser.AcceptanceCriterion{
		ID:          "UNI-1-1",
		Title:       "\u00a0Non\u00a0\u00a0breaking\u3000",
		Description: "Given\u2028an\u3000ideographic\u2029space,\u0085when\u1680hashed,\u2003\u202fthen\u205fit\v\f\rcollapses.\u00a0",
	})
	r.Title = "\u3000Zero\u200bwidth\u2060joined\ufeff "
	r.Description = "\u200b\u2028Kept\u200b\u00a0\u200b"
	r.OutOfScope = parser.StringList{"\u2028\u2029Line\u2028separated\u2029"}
	r = withInvariants(r, parser.Invariant{ID: "UNI-1-INV-1", Title: "Inv\u200b", Description: "\u3000\u00a0\u2028"})
	idx := buildIndex([]*parser.ReqFile{reqFile("UNI", r)}, nil)

	canonical := "" +
		"7:UNI-1-1\n" +
		"12:Non breaking\n" +
		"59:Given an ideographic space, when hashed, then it collapses.\n" +
		"5:UNI-1\n" +
		"24:Zero\u200bwidth\u2060joined\ufeff\n" +
		"15:\u200b Kept\u200b \u200b\n" +
		"1:0\n" +
		"1:1\n" +
		"14:Line separated\n" +
		"11:UNI-1-INV-1\n" +
		"6:Inv\u200b\n" +
		"0:\n" +
		"5:UNI-1\n" +
		"24:Zero\u200bwidth\u2060joined\ufeff\n" +
		"15:\u200b Kept\u200b \u200b\n" +
		"1:0\n" +
		"1:1\n" +
		"14:Line separated\n"
	checkCanonicalLengths(t, canonical)
	sum := sha256.Sum256([]byte(canonical))
	want := hex.EncodeToString(sum[:])

	got, unknown := SpecHash(idx, []string{"UNI-1-INV-1", "UNI-1-1"})
	if len(unknown) != 0 {
		t.Errorf("unknown = %v, want none", unknown)
	}
	if got != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}
