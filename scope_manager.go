package eslintscope

// Options tailors the scope analysis.
type Options struct {
	Optimistic       bool
	Directive        bool
	IgnoreEval       bool
	NodejsScope      bool
	ImpliedStrict    bool
	SourceType       string // one of 'script', 'module', 'commonjs'
	ECMAVersion      int
	ChildVisitorKeys map[string][]string
	Fallback         string
}

// ScopeManager holds the result of an analysis: the nested scopes, the
// variables they define, and the references resolved against them.
type ScopeManager struct {
	Scopes            []*Scope
	GlobalScope       *Scope
	nodeToScope       map[uintptr][]*Scope
	currentScope      *Scope
	options           *Options
	declaredVariables map[uintptr][]*Variable
}

func newScopeManager(options *Options) *ScopeManager {
	return &ScopeManager{
		Scopes:            []*Scope{},
		nodeToScope:       map[uintptr][]*Scope{},
		options:           options,
		declaredVariables: map[uintptr][]*Variable{},
	}
}

func (m *ScopeManager) useDirective() bool { return m.options.Directive }
func (m *ScopeManager) isOptimistic() bool { return m.options.Optimistic }
func (m *ScopeManager) ignoreEval() bool   { return m.options.IgnoreEval }

// IsGlobalReturn reports whether the script is under a node/commonjs context.
func (m *ScopeManager) IsGlobalReturn() bool {
	return m.options.NodejsScope || m.options.SourceType == "commonjs"
}

// IsModule reports whether the source type is module.
func (m *ScopeManager) IsModule() bool { return m.options.SourceType == "module" }

// IsImpliedStrict reports whether implied strict mode is in effect.
func (m *ScopeManager) IsImpliedStrict() bool { return m.options.ImpliedStrict }

// IsStrictModeSupported reports whether ecmaVersion supports strict mode.
func (m *ScopeManager) IsStrictModeSupported() bool { return m.options.ECMAVersion >= 5 }

func (m *ScopeManager) isES6() bool { return m.options.ECMAVersion >= 6 }

// GetDeclaredVariables returns variables declared by the node (empty if none).
func (m *ScopeManager) GetDeclaredVariables(node Node) []*Variable {
	if v, ok := m.declaredVariables[nodeID(node)]; ok {
		return v
	}
	return []*Variable{}
}

// Acquire returns the scope for a node per the described heuristic.
func (m *ScopeManager) Acquire(node Node, inner bool) *Scope {
	scopes := m.nodeToScope[nodeID(node)]
	if len(scopes) == 0 {
		return nil
	}
	if len(scopes) == 1 {
		return scopes[0]
	}
	if inner {
		for i := len(scopes) - 1; i >= 0; i-- {
			if scopes[i].Type != ScopeFunction || !scopes[i].FunctionExpressionScope {
				return scopes[i]
			}
		}
	} else {
		for _, scope := range scopes {
			if scope.Type != ScopeFunction || !scope.FunctionExpressionScope {
				return scope
			}
		}
	}
	return nil
}

// AcquireAll returns all scopes for a node (nil if none).
func (m *ScopeManager) AcquireAll(node Node) []*Scope {
	return m.nodeToScope[nodeID(node)]
}

// Release returns the upper scope for the node.
func (m *ScopeManager) Release(node Node, inner bool) *Scope {
	scopes := m.nodeToScope[nodeID(node)]
	if len(scopes) == 0 {
		return nil
	}
	upper := scopes[0].Upper
	if upper == nil {
		return nil
	}
	return m.Acquire(upper.Block, inner)
}

func (m *ScopeManager) nestScope(s *Scope) *Scope {
	if s.Type == ScopeGlobal {
		m.GlobalScope = s
	}
	m.currentScope = s
	return s
}

func (m *ScopeManager) nestGlobalScope(node Node) *Scope {
	return m.nestScope(NewGlobalScope(m, node))
}
func (m *ScopeManager) nestBlockScope(node Node) *Scope {
	return m.nestScope(newBlockScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestFunctionScope(node Node, isMethodDefinition bool) *Scope {
	return m.nestScope(newFunctionScope(m, m.currentScope, node, isMethodDefinition))
}
func (m *ScopeManager) nestForScope(node Node) *Scope {
	return m.nestScope(newForScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestCatchScope(node Node) *Scope {
	return m.nestScope(newCatchScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestWithScope(node Node) *Scope {
	return m.nestScope(newWithScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestClassScope(node Node) *Scope {
	return m.nestScope(newClassScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestClassFieldInitializerScope(node Node) *Scope {
	return m.nestScope(newClassFieldInitializerScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestClassStaticBlockScope(node Node) *Scope {
	return m.nestScope(newClassStaticBlockScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestSwitchScope(node Node) *Scope {
	return m.nestScope(newSwitchScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestModuleScope(node Node) *Scope {
	return m.nestScope(newModuleScope(m, m.currentScope, node))
}
func (m *ScopeManager) nestFunctionExpressionNameScope(node Node) *Scope {
	return m.nestScope(newFunctionExpressionNameScope(m, m.currentScope, node))
}
