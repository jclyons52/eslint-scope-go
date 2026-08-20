package eslintscope

import (
	"sort"
)

// visitorBase reproduces the esrecurse.Visitor traversal machinery that
// eslint-scope's Referencer / PatternVisitor / Importer all inherit: a
// childVisitorKeys table (estraverse.VisitorKeys, merged with options) and an
// optional iteration fallback for unknown node types.
type visitorBase struct {
	childKeys map[string][]string
	fallback  func(Node) []string
}

func newVisitorBase(options *Options) visitorBase {
	keys := VisitorKeys
	if len(options.ChildVisitorKeys) > 0 {
		keys = make(map[string][]string, len(VisitorKeys)+len(options.ChildVisitorKeys))
		for k, v := range VisitorKeys {
			keys[k] = v
		}
		for k, v := range options.ChildVisitorKeys {
			keys[k] = v
		}
	}
	vb := visitorBase{childKeys: keys}
	if options.Fallback == "iteration" {
		vb.fallback = iterateKeys
	}
	return vb
}

// iterateKeys reproduces Object.keys for the "iteration" fallback. A Go map
// loses JS insertion order, so keys are emitted in sorted order for
// determinism. (Only reached for node types absent from VisitorKeys.)
func iterateKeys(n Node) []string {
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// readChildren walks the children of node per the visitor keys, dispatching
// each child that is a node (or an ObjectExpression/ObjectPattern properties
// value) through dispatch.
func (vb visitorBase) readChildren(node Node, dispatch func(Node)) {
	if node == nil {
		return
	}
	typ := nodeType(node)
	if typ == "" {
		typ = "Property"
	}
	children := vb.childKeys[typ]
	if children == nil {
		if vb.fallback != nil {
			children = vb.fallback(node)
		} else {
			panic("Unknown node type " + typ + ".")
		}
	}
	for _, key := range children {
		child := node[key]
		if child == nil {
			continue
		}
		switch c := child.(type) {
		case []any:
			for _, el := range c {
				if el == nil {
					continue
				}
				if isNode(el) || isProperty(typ, key) {
					dispatch(el.(Node))
				}
			}
		case Node:
			if isNode(c) {
				dispatch(c)
			}
		}
	}
}
