package eslintscope

import (
	"reflect"
)

// Node is an ESTree AST node carried as a generic map, matching the shape that
// espree/eslint-scope operate on (the same convention used by estraverse-go).
type Node = map[string]any

// nodeID returns a stable identity for a Node map for the lifetime of that
// map value (mirrors JS object identity used by eslint-scope's WeakMaps).
func nodeID(n Node) uintptr {
	if n == nil {
		return 0
	}
	return reflect.ValueOf(n).Pointer()
}

// nodeType returns the "type" field of a node, or "" if nil/absent.
func nodeType(n Node) string {
	if n == nil {
		return ""
	}
	t, _ := n["type"].(string)
	return t
}

// isNode reports whether v is a non-nil node carrying a string "type".
func isNode(v any) bool {
	if v == nil {
		return false
	}
	m, ok := v.(Node)
	if !ok {
		return false
	}
	_, ok = m["type"].(string)
	return ok
}

// isProperty reports whether nodeType/key form an Object properties list
// (matching esrecurse's isProperty).
func isProperty(nodeType, key string) bool {
	return (nodeType == "ObjectExpression" || nodeType == "ObjectPattern") && key == "properties"
}

// getString reads a string field.
func getString(n Node, key string) string {
	if n == nil {
		return ""
	}
	s, _ := n[key].(string)
	return s
}

// getNode reads a child node field (nil if absent or not a node).
func getNode(n Node, key string) Node {
	if n == nil {
		return nil
	}
	c, ok := n[key]
	if !ok || c == nil {
		return nil
	}
	if m, ok := c.(Node); ok {
		return m
	}
	if m, ok := c.(map[string]any); ok {
		return Node(m)
	}
	return nil
}

// getNodes reads a child node-array field (nil elements preserved as nils per
// the ESTree spec, e.g. ArrayExpression.elements may hold nulls).
func getNodes(n Node, key string) []Node {
	if n == nil {
		return nil
	}
	c, ok := n[key]
	if !ok || c == nil {
		return nil
	}
	arr, ok := c.([]any)
	if !ok {
		return nil
	}
	out := make([]Node, 0, len(arr))
	for _, el := range arr {
		if el == nil {
			out = append(out, nil)
			continue
		}
		if m, ok := el.(Node); ok {
			out = append(out, m)
			continue
		}
		if m, ok := el.(map[string]any); ok {
			out = append(out, Node(m))
		}
	}
	return out
}

// getBools reads a boolean field.
func getBool(n Node, key string) bool {
	if n == nil {
		return false
	}
	b, _ := n[key].(bool)
	return b
}

// rangeStart returns node.range[0].
func rangeStart(n Node) int {
	if n == nil {
		return 0
	}
	r, ok := n["range"].([]any)
	if !ok || len(r) == 0 {
		return 0
	}
	f, _ := r[0].(float64)
	return int(f)
}
