package eslintscope

// Scope types.
const (
	ScopeGlobal                 = "global"
	ScopeModule                 = "module"
	ScopeFunction               = "function"
	ScopeFunctionExpressionName = "function-expression-name"
	ScopeBlock                  = "block"
	ScopeSwitch                 = "switch"
	ScopeCatch                  = "catch"
	ScopeWith                   = "with"
	ScopeFor                    = "for"
	ScopeClass                  = "class"
	ScopeClassFieldInitializer  = "class-field-initializer"
	ScopeClassStaticBlock       = "class-static-block"
)

// isVariableScopeType reports whether a scope type creates a variableScope.
func isVariableScopeType(t string) bool {
	switch t {
	case ScopeGlobal, ScopeModule, ScopeFunction, ScopeClassFieldInitializer, ScopeClassStaticBlock:
		return true
	}
	return false
}

// implicit holds the global-scope implicit variables state.
type implicit struct {
	set       map[string]*Variable
	variables []*Variable
	left      []*Reference
}

// Scope is a lexical scope. It carries the union of behaviours that in the JS
// source are spread across the Scope subclasses; the constructor and the
// type-dependent methods branch on Type to reproduce each subclass exactly.
type Scope struct {
	m                       *ScopeManager
	Type                    string
	Set                     map[string]*Variable
	taints                  map[string]bool
	Dynamic                 bool
	Block                   Node
	Through                 []*Reference
	Variables               []*Variable
	References              []*Reference
	variableScope           *Scope
	FunctionExpressionScope bool
	directCallToEvalScope   bool
	thisFound               bool
	left                    []*Reference // nil once closed
	Upper                   *Scope
	IsStrict                bool
	ChildScopes             []*Scope
	declaredVariables       map[uintptr][]*Variable
	implicit                implicit
}

func newScope(m *ScopeManager, type_ string, upper *Scope, block Node, isMethodDefinition bool) *Scope {
	s := &Scope{
		m:                 m,
		Type:              type_,
		Set:               map[string]*Variable{},
		taints:            map[string]bool{},
		Block:             block,
		Through:           []*Reference{},
		Variables:         []*Variable{},
		References:        []*Reference{},
		left:              []*Reference{},
		Upper:             upper,
		ChildScopes:       []*Scope{},
		declaredVariables: m.declaredVariables,
	}
	s.Dynamic = type_ == ScopeGlobal || type_ == ScopeWith
	if isVariableScopeType(type_) {
		s.variableScope = s
	} else {
		s.variableScope = upper.variableScope
	}
	if m.IsStrictModeSupported() {
		s.IsStrict = isStrictScope(s, block, isMethodDefinition, m.useDirective())
	} else {
		s.IsStrict = false
	}
	if upper != nil {
		upper.ChildScopes = append(upper.ChildScopes, s)
	}
	registerScope(m, s)
	return s
}

// NewGlobalScope constructs the global scope (type "global").
func NewGlobalScope(m *ScopeManager, block Node) *Scope {
	s := newScope(m, ScopeGlobal, nil, block, false)
	s.implicit = implicit{set: map[string]*Variable{}, variables: []*Variable{}}
	return s
}

// newModuleScope constructs the module scope.
func newModuleScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeModule, upper, block, false)
}

// newFunctionExpressionNameScope defines the named function-expression scope.
func newFunctionExpressionNameScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	s := newScope(m, ScopeFunctionExpressionName, upper, block, false)
	s.Define(getNode(block, "id"), &Definition{Type: VarFunctionName, Name: getNode(block, "id"), Node: block})
	s.FunctionExpressionScope = true
	return s
}

func newCatchScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeCatch, upper, block, false)
}

func newWithScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeWith, upper, block, false)
}

func newBlockScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeBlock, upper, block, false)
}

func newSwitchScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeSwitch, upper, block, false)
}

func newFunctionScope(m *ScopeManager, upper *Scope, block Node, isMethodDefinition bool) *Scope {
	s := newScope(m, ScopeFunction, upper, block, isMethodDefinition)
	if nodeType(block) != SyntaxArrowFunctionExpression {
		s.defineArguments()
	}
	return s
}

func newForScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeFor, upper, block, false)
}

func newClassScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeClass, upper, block, false)
}

func newClassFieldInitializerScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeClassFieldInitializer, upper, block, true)
}

func newClassStaticBlockScope(m *ScopeManager, upper *Scope, block Node) *Scope {
	return newScope(m, ScopeClassStaticBlock, upper, block, true)
}

// registerScope pushes scope onto the manager and links node->scope.
func registerScope(m *ScopeManager, s *Scope) {
	m.Scopes = append(m.Scopes, s)
	id := nodeID(s.Block)
	existing, ok := m.nodeToScope[id]
	if ok {
		m.nodeToScope[id] = append(existing, s)
	} else {
		m.nodeToScope[id] = []*Scope{s}
	}
}

// isStrictScope implements the isStrictScope helper.
func isStrictScope(scope *Scope, block Node, isMethodDefinition, useDirective bool) bool {
	var body Node
	if scope.Upper != nil && scope.Upper.IsStrict {
		return true
	}
	if isMethodDefinition {
		return true
	}
	if scope.Type == ScopeClass || scope.Type == ScopeModule {
		return true
	}
	if scope.Type == ScopeBlock || scope.Type == ScopeSwitch {
		return false
	}
	if scope.Type == ScopeFunction {
		if nodeType(block) == SyntaxArrowFunctionExpression && nodeType(getNode(block, "body")) != SyntaxBlockStatement {
			return false
		}
		if nodeType(block) == SyntaxProgram {
			body = block
		} else {
			body = getNode(block, "body")
		}
		if body == nil {
			return false
		}
	} else if scope.Type == ScopeGlobal {
		body = block
	} else {
		return false
	}

	stmts := getNodes(body, "body")
	if useDirective {
		for _, stmt := range stmts {
			if nodeType(stmt) != SyntaxDirectiveStatement {
				break
			}
			if getString(stmt, "raw") == "\"use strict\"" || getString(stmt, "raw") == "'use strict'" {
				return true
			}
		}
	} else {
		for _, stmt := range stmts {
			if nodeType(stmt) != SyntaxExpressionStatement {
				break
			}
			expr := getNode(stmt, "expression")
			if nodeType(expr) != SyntaxLiteral {
				break
			}
			if _, ok := expr["value"].(string); !ok {
				break
			}
			raw, hasRaw := expr["raw"].(string)
			if hasRaw {
				if raw == "\"use strict\"" || raw == "'use strict'" {
					return true
				}
			} else {
				if expr["value"].(string) == "use strict" {
					return true
				}
			}
		}
	}
	return false
}

// shouldBeStatically reports whether a definition should be resolved statically.
func shouldBeStatically(def *Definition) bool {
	return (def.Type == VarClassName) ||
		(def.Type == VarVariable && (def.Kind == nil || *def.Kind != "var"))
}

func (s *Scope) shouldStaticallyClose(m *ScopeManager) bool {
	return (!s.Dynamic || m.isOptimistic())
}

func (s *Scope) shouldStaticallyCloseForGlobal(ref *Reference) bool {
	name := getString(ref.Identifier, "name")
	variable, ok := s.Set[name]
	if !ok {
		return false
	}
	defs := variable.Defs
	if len(defs) == 0 {
		return false
	}
	for _, d := range defs {
		if !shouldBeStatically(d) {
			return false
		}
	}
	return true
}

func (s *Scope) staticCloseRef(ref *Reference) {
	if !s.resolveRef(ref) {
		s.delegateToUpperScope(ref)
	}
}

func (s *Scope) dynamicCloseRef(ref *Reference) {
	// Notify all names are through to global.
	for current := s; current != nil; current = current.Upper {
		current.Through = append(current.Through, ref)
	}
}

func (s *Scope) globalCloseRef(ref *Reference) {
	if s.shouldStaticallyCloseForGlobal(ref) {
		s.staticCloseRef(ref)
	} else {
		s.dynamicCloseRef(ref)
	}
}

// Close implements Scope.__close, including the GlobalScope and WithScope
// overrides.
func (s *Scope) Close(m *ScopeManager) *Scope {
	if s.Type == ScopeWith && !s.shouldStaticallyClose(m) {
		for _, ref := range s.left {
			ref.Tainted = true
			s.delegateToUpperScope(ref)
		}
		s.left = nil
		return s.Upper
	}

	if s.Type == ScopeGlobal {
		var implicitDefs []*implicitGlobalInfo
		for _, ref := range s.left {
			if ref.maybeImplicitGlobal != nil {
				if _, ok := s.Set[getString(ref.Identifier, "name")]; !ok {
					implicitDefs = append(implicitDefs, ref.maybeImplicitGlobal)
				}
			}
		}
		for _, info := range implicitDefs {
			s.defineImplicit(info.pattern, &Definition{Type: VarImplicitGlobalVariable, Name: info.pattern, Node: info.node, Parent: nil})
		}
		s.implicit.left = s.left
	}

	var closeRef func(*Reference)
	if s.shouldStaticallyClose(m) {
		closeRef = s.staticCloseRef
	} else if s.Type != ScopeGlobal {
		closeRef = s.dynamicCloseRef
	} else {
		closeRef = s.globalCloseRef
	}
	for _, ref := range s.left {
		closeRef(ref)
	}
	s.left = nil
	return s.Upper
}

// isValidResolution is overridden by function scopes (default true), so that
// references in default parameters don't resolve to variables defined in the
// function body.
func (s *Scope) isValidResolution(ref *Reference, variable *Variable) bool {
	if s.Type != ScopeFunction {
		return true
	}
	// If `options.nodejsScope` is true, `block` becomes a Program node.
	if nodeType(s.Block) == SyntaxProgram {
		return true
	}
	body := getNode(s.Block, "body")
	bodyStart := rangeStart(body)
	// Invalid resolution: the reference is in the parameter part, while the
	// variable is defined in the body.
	if variable.Scope == s && rangeStart(ref.Identifier) < bodyStart && everyDefNameInBody(variable.Defs, bodyStart) {
		return false
	}
	return true
}

func everyDefNameInBody(defs []*Definition, bodyStart int) bool {
	for _, d := range defs {
		if rangeStart(d.Name) < bodyStart {
			return false
		}
	}
	return true
}

// resolveRef implements Scope.__resolve.
func (s *Scope) resolveRef(ref *Reference) bool {
	name := getString(ref.Identifier, "name")
	variable, ok := s.Set[name]
	if !ok {
		return false
	}
	if !s.isValidResolution(ref, variable) {
		return false
	}
	variable.References = append(variable.References, ref)
	variable.Stack = variable.Stack && ref.From.variableScope == s.variableScope
	if ref.Tainted {
		variable.Tainted = true
		s.taints[variable.Name] = true
	}
	ref.Resolved = variable
	return true
}

func (s *Scope) delegateToUpperScope(ref *Reference) {
	if s.Upper != nil {
		s.Upper.left = append(s.Upper.left, ref)
	}
	s.Through = append(s.Through, ref)
}

func (s *Scope) addDeclaredVariablesOfNode(variable *Variable, node Node) {
	if node == nil {
		return
	}
	id := nodeID(node)
	existing := s.declaredVariables[id]
	if existing == nil {
		existing = []*Variable{}
	}
	found := false
	for _, v := range existing {
		if v == variable {
			found = true
			break
		}
	}
	if !found {
		s.declaredVariables[id] = append(existing, variable)
	}
}

func (s *Scope) defineGeneric(name string, set map[string]*Variable, variables *[]*Variable, node Node, def *Definition) {
	variable := set[name]
	if variable == nil {
		variable = &Variable{Name: name, Scope: s, Stack: true}
		set[name] = variable
		*variables = append(*variables, variable)
	}
	if def != nil {
		variable.Defs = append(variable.Defs, def)
		s.addDeclaredVariablesOfNode(variable, def.Node)
		s.addDeclaredVariablesOfNode(variable, def.Parent)
	}
	if node != nil {
		variable.Identifiers = append(variable.Identifiers, node)
	}
}

// Define implements Scope.__define.
func (s *Scope) Define(node Node, def *Definition) {
	if node != nil && nodeType(node) == SyntaxIdentifier {
		s.defineGeneric(getString(node, "name"), s.Set, &s.Variables, node, def)
	}
}

func (s *Scope) defineImplicit(node Node, def *Definition) {
	if node != nil && nodeType(node) == SyntaxIdentifier {
		s.defineGeneric(getString(node, "name"), s.implicit.set, &s.implicit.variables, node, def)
	}
}

// Referencing implements Scope.__referencing.
func (s *Scope) Referencing(node Node, assign int, writeExpr Node, maybeImplicitGlobal *implicitGlobalInfo, partial, init bool) {
	if node == nil || nodeType(node) != SyntaxIdentifier {
		return
	}
	if getString(node, "name") == "super" {
		return
	}
	flag := assign
	if flag == 0 {
		flag = RefRead
	}
	ref := &Reference{
		Identifier:          node,
		From:                s,
		Flag:                flag,
		WriteExpr:           writeExpr,
		Partial:             partial,
		Init:                init,
		maybeImplicitGlobal: maybeImplicitGlobal,
	}
	s.References = append(s.References, ref)
	s.left = append(s.left, ref)
}

// DetectEval implements Scope.__detectEval.
func (s *Scope) DetectEval() {
	s.directCallToEvalScope = true
	for current := s; current != nil; current = current.Upper {
		current.Dynamic = true
	}
}

// DetectThis implements Scope.__detectThis.
func (s *Scope) DetectThis() {
	s.thisFound = true
}

// IsClosed reports whether the scope has been closed.
func (s *Scope) IsClosed() bool { return s.left == nil }

// Resolve returns the reference whose identifier is ident, or nil.
func (s *Scope) Resolve(ident Node) *Reference {
	for _, ref := range s.References {
		if nodeID(ref.Identifier) == nodeID(ident) {
			return ref
		}
	}
	return nil
}

// IsStatic reports whether the scope is static.
func (s *Scope) IsStatic() bool { return !s.Dynamic }

// IsArgumentsMaterialized reports whether arguments is materialized.
func (s *Scope) IsArgumentsMaterialized() bool {
	if s.Type != ScopeFunction {
		return true
	}
	if nodeType(s.Block) == SyntaxArrowFunctionExpression {
		return false
	}
	if !s.IsStatic() {
		return true
	}
	variable, ok := s.Set["arguments"]
	if !ok {
		return true
	}
	return variable.Tainted || len(variable.References) != 0
}

// IsThisMaterialized reports whether this is materialized.
func (s *Scope) IsThisMaterialized() bool {
	if s.Type != ScopeFunction {
		return true
	}
	if !s.IsStatic() {
		return true
	}
	return s.thisFound
}

// IsUsedName reports whether the name is used in this scope or through.
func (s *Scope) IsUsedName(name string) bool {
	if _, ok := s.Set[name]; ok {
		return true
	}
	for _, ref := range s.Through {
		if getString(ref.Identifier, "name") == name {
			return true
		}
	}
	return false
}

func (s *Scope) defineArguments() {
	s.defineGeneric("arguments", s.Set, &s.Variables, nil, nil)
	s.taints["arguments"] = true
}

var _ = nodeID // keep helper referenced if unused elsewhere in package file graph
