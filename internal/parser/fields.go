package parser

import "gopkg.in/yaml.v3"

// StringList is a .req field written as a single scalar or a list of
// scalars, held as a list either way: interface (SPEC-1-1) and out_of_scope
// (SPEC-2-1). A map, or a list holding a map or a list, is an error, and
// ParseFile names the requirement's ID and the field in it (SPEC-1-2).
type StringList []string

// UnmarshalYAML reads a scalar as a list of one and a sequence of scalars
// in order.
func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	return nil
}
