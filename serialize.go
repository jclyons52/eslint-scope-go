package eslintscope

// This file serializes a ScopeManager into a canonical JSON-comparable shape
// that mirrors the JS oracle driver's serializer, so the Go port can be
// diffed against real eslint-scope. Node identity uses the $id field that the
// oracle annotates onto each AST node.

func getID(n Node) int {
	if n == nil {
		return 0
	}
	switch v := n["$id"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func nid(n Node) any {
	if n == nil {
		return nil
	}
	return getID(n)
}

func intOrNil(i *int) any {
	if i == nil {
		return nil
	}
	return *i
}

func strOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func defSer(d *Definition) map[string]any {
	rest := any(nil)
	if d.Type == VarParameter {
		rest = d.Rest
	}
	return map[string]any{
		"type":   d.Type,
		"name":   nid(d.Name),
		"node":   nid(d.Node),
		"parent": nid(d.Parent),
		"index":  intOrNil(d.Index),
		"kind":   strOrNil(d.Kind),
		"rest":   rest,
	}
}

// SerializeSM renders the scope manager into the shared comparison schema.
func SerializeSM(sm *ScopeManager) map[string]any {
	scopeIdx := map[*Scope]int{}
	for i, s := range sm.Scopes {
		scopeIdx[s] = i
	}
	varIdx := map[*Variable]int{}
	for _, s := range sm.Scopes {
		for _, v := range s.Variables {
			if _, ok := varIdx[v]; !ok {
				varIdx[v] = len(varIdx)
			}
		}
		if s.Type == ScopeGlobal {
			for _, v := range s.implicit.variables {
				if _, ok := varIdx[v]; !ok {
					varIdx[v] = len(varIdx)
				}
			}
		}
	}
	refIdx := map[*Reference]int{}
	for _, s := range sm.Scopes {
		for _, r := range s.References {
			refIdx[r] = len(refIdx)
		}
	}

	varSer := func(v *Variable, si int) map[string]any {
		refs := make([]any, 0, len(v.References))
		for _, r := range v.References {
			refs = append(refs, refIdx[r])
		}
		defs := make([]any, 0, len(v.Defs))
		for _, d := range v.Defs {
			defs = append(defs, defSer(d))
		}
		ids := make([]any, 0, len(v.Identifiers))
		for _, idn := range v.Identifiers {
			ids = append(ids, getID(idn))
		}
		return map[string]any{
			"id":          varIdx[v],
			"name":        v.Name,
			"stack":       v.Stack,
			"tainted":     v.Tainted,
			"scope":       si,
			"identifiers": ids,
			"defs":        defs,
			"references":  refs,
		}
	}

	scopes := make([]any, 0, len(sm.Scopes))
	for si, s := range sm.Scopes {
		childScopes := make([]any, 0, len(s.ChildScopes))
		for _, c := range s.ChildScopes {
			childScopes = append(childScopes, scopeIdx[c])
		}
		refIds := make([]any, 0, len(s.References))
		for _, r := range s.References {
			refIds = append(refIds, refIdx[r])
		}
		through := make([]any, 0, len(s.Through))
		for _, r := range s.Through {
			through = append(through, refIdx[r])
		}
		vars := make([]any, 0, len(s.Variables))
		for _, v := range s.Variables {
			vars = append(vars, varSer(v, si))
		}
		obj := map[string]any{
			"index":                   si,
			"type":                    s.Type,
			"blockId":                 nid(s.Block),
			"blockType":               nodeType(s.Block),
			"dynamic":                 s.Dynamic,
			"isStrict":                s.IsStrict,
			"functionExpressionScope": s.FunctionExpressionScope,
			"directCallToEvalScope":   s.directCallToEvalScope,
			"thisFound":               s.thisFound,
			"upper":                   indexOrNil(scopeIdx, s.Upper),
			"variableScope":           scopeIdx[s.variableScope],
			"childScopes":             childScopes,
			"references":              refIds,
			"through":                 through,
			"isArgumentsMaterialized": s.IsArgumentsMaterialized(),
			"isThisMaterialized":      s.IsThisMaterialized(),
			"variables":               vars,
		}
		if s.Type == ScopeGlobal {
			im := make([]any, 0, len(s.implicit.variables))
			for _, v := range s.implicit.variables {
				im = append(im, varSer(v, si))
			}
			left := make([]any, 0, len(s.implicit.left))
			for _, r := range s.implicit.left {
				left = append(left, refIdx[r])
			}
			obj["implicitVariables"] = im
			obj["implicitLeft"] = left
		}
		scopes = append(scopes, obj)
	}

	nodeToScope := map[string]any{}
	for _, s := range sm.Scopes {
		if s.Block == nil {
			continue
		}
		id := getID(s.Block)
		key := itoa(id)
		arr, _ := nodeToScope[key].([]any)
		nodeToScope[key] = append(arr, scopeIdx[s])
	}

	declared := map[string]any{}
	for _, s := range sm.Scopes {
		for _, v := range s.Variables {
			addDeclaredSer(declared, varIdx[v], v.Defs)
		}
	}
	for _, d := range sm.GlobalScope.implicit.variables {
		addDeclaredSer(declared, varIdx[d], d.Defs)
	}

	return map[string]any{
		"scopes":            scopes,
		"globalScope":       indexOrNil(scopeIdx, sm.GlobalScope),
		"nodeToScope":       nodeToScope,
		"declaredVariables": declared,
	}
}

func addDeclaredSer(declared map[string]any, varID int, defs []*Definition) {
	for _, d := range defs {
		for _, n := range []Node{d.Node, d.Parent} {
			if n == nil {
				continue
			}
			key := itoa(getID(n))
			arr, _ := declared[key].([]any)
			dup := false
			for _, x := range arr {
				if f, ok := x.(float64); ok && int(f) == varID {
					dup = true
					break
				}
				if i, ok := x.(int); ok && i == varID {
					dup = true
					break
				}
			}
			if !dup {
				declared[key] = append(arr, varID)
			}
		}
	}
}

func indexOrNil(m map[*Scope]int, s *Scope) any {
	if s == nil {
		return nil
	}
	return m[s]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
