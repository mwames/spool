package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringList is a .req field written as a single scalar or a list of
// scalars, held as a list either way: interface (SPEC-1-1) and out_of_scope
// (SPEC-2-1). A map, or a list holding a map or a list, is an error, and
// ParseFile names the requirement's ID and the field in it (SPEC-1-2).
type StringList []string

// UnmarshalYAML reads a scalar as a list of one and a sequence of scalars
// in order.
func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag == "!!null" {
			*l = nil
			return nil
		}
		*l = StringList{node.Value}
		return nil
	case yaml.SequenceNode:
		items := make(StringList, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: list items must be strings", item.Line)
			}
			items = append(items, item.Value)
		}
		*l = items
		return nil
	}
	return fmt.Errorf("line %d: must be a string or a list of strings", node.Line)
}
