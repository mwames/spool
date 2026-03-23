// Package spoolid provides Spool ID validation and normalization.
package spoolid

import (
	"fmt"
	"regexp"
	"strings"
)

// idPattern matches valid Spool IDs: PREFIX-N or PREFIX-N-N
var idPattern = regexp.MustCompile(`^[A-Za-z]+-\d+(-\d+)?$`)

// IsValid reports whether id is a valid Spool ID.
func IsValid(id string) bool {
	return idPattern.MatchString(id)
}

// NormalizeID normalizes a Spool ID to canonical form:
// uppercase prefix, no leading zeros in number segments.
func NormalizeID(id string) (string, error) {
	if !IsValid(id) {
		return "", fmt.Errorf("malformed ID %q", id)
	}

	parts := strings.Split(id, "-")
	parts[0] = strings.ToUpper(parts[0])
	for i := 1; i < len(parts); i++ {
		parts[i] = stripLeadingZeros(parts[i])
	}
	return strings.Join(parts, "-"), nil
}

func stripLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}
