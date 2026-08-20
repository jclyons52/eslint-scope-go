package eslintscope

// traverseIdentifierInPattern walks a binding pattern with a PatternVisitor,
// then processes the collected right-hand nodes through the referencer.
func traverseIdentifierInPattern(options *Options, rootPattern Node, referencer *Referencer, callback func(Node, patternInfo)) {
	visitor := newPatternVisitor(options, rootPattern, callback)
	visitor.visit(rootPattern)
	if referencer != nil {
		for _, rhs := range visitor.rightHandNodes {
			referencer.visit(rhs)
		}
	}
}

// Importer visits an ImportDeclaration's specifiers and defines import
// bindings on the current scope.
type Importer struct {
	visitorBase
	declaration Node
	referencer  *Referencer
}

func newImporter(declaration Node, referencer *Referencer) *Importer {
	return &Importer{
		visitorBase: newVisitorBase(referencer.options),
		declaration: declaration,
		referencer:  referencer,
	}
}

func (im *Importer) visitImport(id Node, specifier Node) {
	im.referencer.visitPattern(id, false, func(pattern Node, info patternInfo) {
		im.referencer.currentScope().Define(pattern, &Definition{
			Type:   VarImportBinding,
			Name:   pattern,
			Node:   specifier,
			Parent: im.declaration,
		})
	})
}

func (im *Importer) visit(node Node) {
	if node == nil {
		return
	}
	switch nodeType(node) {
	case "ImportNamespaceSpecifier":
		local := getNode(node, "local")
		if local == nil {
			local = getNode(node, "id")
		}
		if local != nil {
			im.visitImport(local, node)
		}
	case "ImportDefaultSpecifier":
		local := getNode(node, "local")
		if local == nil {
			local = getNode(node, "id")
		}
		if local != nil {
			im.visitImport(local, node)
		}
	case "ImportSpecifier":
		local := getNode(node, "local")
		if local == nil {
			local = getNode(node, "id")
		}
		if name := getNode(node, "name"); name != nil {
			im.visitImport(name, node)
		} else {
			im.visitImport(local, node)
		}
	default:
		im.visitorBase.readChildren(node, im.visit)
	}
}

// Referencer walks an AST creating scopes, defining variables and recording
// identifier references (the eslint-scope engine).
type Referencer struct {
	visitorBase
	options                 *Options
	scopeManager            *ScopeManager
	isInnerMethodDefinition bool
}

func newReferencer(options *Options, sm *ScopeManager) *Referencer {
	return &Referencer{
		visitorBase:  newVisitorBase(options),
		options:      options,
		scopeManager: sm,
	}
}

func (r *Referencer) currentScope() *Scope { return r.scopeManager.currentScope }

func (r *Referencer) close(node Node) {
	for r.currentScope() != nil && nodeID(r.currentScope().Block) == nodeID(node) {
		r.scopeManager.currentScope = r.currentScope().Close(r.scopeManager)
	}
}

func (r *Referencer) pushInnerMethodDefinition(v bool) bool {
	prev := r.isInnerMethodDefinition
	r.isInnerMethodDefinition = v
	return prev
}

func (r *Referencer) popInnerMethodDefinition(v bool) {
	r.isInnerMethodDefinition = v
}

func (r *Referencer) referencingDefaultValue(pattern Node, assignments []Node, maybeImplicitGlobal *implicitGlobalInfo, init bool) {
	scope := r.currentScope()
	for _, a := range assignments {
		scope.Referencing(
			pattern,
			RefWrite,
			getNode(a, "right"),
			maybeImplicitGlobal,
			nodeID(pattern) != nodeID(getNode(a, "left")),
			init,
		)
	}
}

func (r *Referencer) visitPattern(node Node, processRightHand bool, cb func(Node, patternInfo)) {
	var ref *Referencer
	if processRightHand {
		ref = r
	}
	traverseIdentifierInPattern(r.options, node, ref, cb)
}

func (r *Referencer) visitFunction(node Node) {
	// FunctionDeclaration name is defined in upper scope.
	if nodeType(node) == SyntaxFunctionDeclaration {
		r.currentScope().Define(getNode(node, "id"), &Definition{
			Type: VarFunctionName,
			Name: getNode(node, "id"),
			Node: node,
		})
	}
	// FunctionExpression with a name creates its FunctionExpressionNameScope.
	if nodeType(node) == SyntaxFunctionExpression {
		if getNode(node, "id") != nil {
			r.scopeManager.nestFunctionExpressionNameScope(node)
		}
	}
	r.scopeManager.nestFunctionScope(node, r.isInnerMethodDefinition)

	params := getNodes(node, "params")
	for i := range params {
		idx := i
		r.visitPattern(params[i], true, func(pattern Node, info patternInfo) {
			r.currentScope().Define(pattern, &Definition{
				Type:  VarParameter,
				Name:  pattern,
				Node:  node,
				Index: intPtr(idx),
				Rest:  info.rest,
			})
			r.referencingDefaultValue(pattern, info.assignments, nil, true)
		})
	}
	// If there's a rest argument, add it.
	if rest := getNode(node, "rest"); rest != nil {
		synthetic := Node{"type": "RestElement", "argument": rest}
		r.visitPattern(synthetic, false, func(pattern Node, info patternInfo) {
			r.currentScope().Define(pattern, &Definition{
				Type:  VarParameter,
				Name:  pattern,
				Node:  node,
				Index: intPtr(len(params)),
				Rest:  true,
			})
		})
	}

	if body := getNode(node, "body"); body != nil {
		if nodeType(body) == SyntaxBlockStatement {
			r.visitChildren(body)
		} else {
			r.visit(body)
		}
	}
	r.close(node)
}

func (r *Referencer) visitClass(node Node) {
	if nodeType(node) == SyntaxClassDeclaration {
		r.currentScope().Define(getNode(node, "id"), &Definition{
			Type: VarClassName,
			Name: getNode(node, "id"),
			Node: node,
		})
	}
	r.visit(getNode(node, "superClass"))
	r.scopeManager.nestClassScope(node)
	if id := getNode(node, "id"); id != nil {
		r.currentScope().Define(id, &Definition{Type: VarClassName, Name: id, Node: node})
	}
	r.visit(getNode(node, "body"))
	r.close(node)
}

func (r *Referencer) visitProperty(node Node) {
	if getBool(node, "computed") {
		r.visit(getNode(node, "key"))
	}
	isMethodDefinition := nodeType(node) == SyntaxMethodDefinition
	var prev bool
	if isMethodDefinition {
		prev = r.pushInnerMethodDefinition(true)
	}
	r.visit(getNode(node, "value"))
	if isMethodDefinition {
		r.popInnerMethodDefinition(prev)
	}
}

func (r *Referencer) visitForIn(node Node) {
	left := getNode(node, "left")
	if nodeType(left) == SyntaxVariableDeclaration && getString(left, "kind") != "var" {
		r.scopeManager.nestForScope(node)
	}
	if nodeType(left) == SyntaxVariableDeclaration {
		r.visit(left)
		decls := getNodes(left, "declarations")
		if len(decls) > 0 {
			r.visitPattern(getNode(decls[0], "id"), false, func(pattern Node, info patternInfo) {
				r.currentScope().Referencing(pattern, RefWrite, getNode(node, "right"), nil, true, true)
			})
		}
	} else {
		r.visitPattern(left, true, func(pattern Node, info patternInfo) {
			var maybe *implicitGlobalInfo
			if !r.currentScope().IsStrict {
				maybe = &implicitGlobalInfo{pattern: pattern, node: node}
			}
			r.referencingDefaultValue(pattern, info.assignments, maybe, false)
			r.currentScope().Referencing(pattern, RefWrite, getNode(node, "right"), maybe, true, false)
		})
	}
	r.visit(getNode(node, "right"))
	r.visit(getNode(node, "body"))
	r.close(node)
}

func (r *Referencer) visitVariableDeclaration(variableTargetScope *Scope, type_ string, node Node, index int) {
	decls := getNodes(node, "declarations")
	if index >= len(decls) {
		return
	}
	decl := decls[index]
	init := getNode(decl, "init")
	r.visitPattern(getNode(decl, "id"), true, func(pattern Node, info patternInfo) {
		variableTargetScope.Define(pattern, &Definition{
			Type:   type_,
			Name:   pattern,
			Node:   decl,
			Parent: node,
			Index:  intPtr(index),
			Kind:   strPtr(getString(node, "kind")),
		})
		r.referencingDefaultValue(pattern, info.assignments, nil, true)
		if init != nil {
			r.currentScope().Referencing(pattern, RefWrite, init, nil, !info.topLevel, true)
		}
	})
}

func (r *Referencer) visitChildren(node Node) {
	r.visitorBase.readChildren(node, r.visit)
}

// visit dispatches by node type exactly as the esrecurse.Visitor base did for
// the Referencer: types with a handler route to it; all others traverse
// children generically. Handler-less (no-op) types intentionally do not
// recurse.
func (r *Referencer) visit(node Node) {
	if node == nil {
		return
	}
	switch nodeType(node) {
	case "Program":
		r.scopeManager.nestGlobalScope(node)
		if r.scopeManager.IsGlobalReturn() {
			r.currentScope().IsStrict = false
			r.scopeManager.nestFunctionScope(node, false)
		}
		if r.scopeManager.isES6() && r.scopeManager.IsModule() {
			r.scopeManager.nestModuleScope(node)
		}
		if r.scopeManager.IsStrictModeSupported() && r.scopeManager.IsImpliedStrict() {
			r.currentScope().IsStrict = true
		}
		r.visitChildren(node)
		r.close(node)
	case "ClassExpression":
		r.visitClass(node)
	case "ClassDeclaration":
		r.visitClass(node)
	case "CallExpression":
		callee := getNode(node, "callee")
		if !r.scopeManager.ignoreEval() && nodeType(callee) == SyntaxIdentifier && getString(callee, "name") == "eval" {
			r.currentScope().variableScope.DetectEval()
		}
		r.visitChildren(node)
	case "BlockStatement":
		if r.scopeManager.isES6() {
			r.scopeManager.nestBlockScope(node)
		}
		r.visitChildren(node)
		r.close(node)
	case "ThisExpression":
		r.currentScope().variableScope.DetectThis()
	case "WithStatement":
		r.visit(getNode(node, "object"))
		r.scopeManager.nestWithScope(node)
		r.visit(getNode(node, "body"))
		r.close(node)
	case "VariableDeclaration":
		variableTargetScope := r.currentScope()
		if getString(node, "kind") == "var" {
			variableTargetScope = r.currentScope().variableScope
		}
		decls := getNodes(node, "declarations")
		for i := range decls {
			r.visitVariableDeclaration(variableTargetScope, VarVariable, node, i)
			if init := getNode(decls[i], "init"); init != nil {
				r.visit(init)
			}
		}
	case "SwitchStatement":
		r.visit(getNode(node, "discriminant"))
		if r.scopeManager.isES6() {
			r.scopeManager.nestSwitchScope(node)
		}
		for _, c := range getNodes(node, "cases") {
			r.visit(c)
		}
		r.close(node)
	case "FunctionDeclaration":
		r.visitFunction(node)
	case "FunctionExpression":
		r.visitFunction(node)
	case "ForOfStatement":
		r.visitForIn(node)
	case "ForInStatement":
		r.visitForIn(node)
	case "ArrowFunctionExpression":
		r.visitFunction(node)
	case "ImportDeclaration":
		importer := newImporter(node, r)
		importer.visit(node)
	case "ExportDeclaration", "ExportAllDeclaration", "ExportDefaultDeclaration", "ExportNamedDeclaration":
		r.visitExportDeclaration(node)
	case "ExportSpecifier":
		local := getNode(node, "id")
		if local == nil {
			local = getNode(node, "local")
		}
		r.visit(local)
	case "MetaProperty", "PrivateIdentifier", "BreakStatement", "ContinueStatement":
		// Intentionally do nothing.
	case "Identifier":
		r.currentScope().Referencing(node, 0, nil, nil, false, false)
	case "UpdateExpression":
		if isPatternNode(getNode(node, "argument")) {
			r.currentScope().Referencing(getNode(node, "argument"), RefRW, nil, nil, false, false)
		} else {
			r.visitChildren(node)
		}
	case "MemberExpression":
		r.visit(getNode(node, "object"))
		if getBool(node, "computed") {
			r.visit(getNode(node, "property"))
		}
	case "Property":
		r.visitProperty(node)
	case "PropertyDefinition":
		if getBool(node, "computed") {
			r.visit(getNode(node, "key"))
		}
		if value := getNode(node, "value"); value != nil {
			r.scopeManager.nestClassFieldInitializerScope(value)
			r.visit(value)
			r.close(value)
		}
	case "StaticBlock":
		r.scopeManager.nestClassStaticBlockScope(node)
		r.visitChildren(node)
		r.close(node)
	case "MethodDefinition":
		r.visitProperty(node)
	case "LabeledStatement":
		r.visit(getNode(node, "body"))
	case "ForStatement":
		if init := getNode(node, "init"); nodeType(init) == SyntaxVariableDeclaration && getString(init, "kind") != "var" {
			r.scopeManager.nestForScope(node)
		}
		r.visitChildren(node)
		r.close(node)
	case "CatchClause":
		r.scopeManager.nestCatchScope(node)
		r.visitPattern(getNode(node, "param"), true, func(pattern Node, info patternInfo) {
			r.currentScope().Define(pattern, &Definition{
				Type: VarCatchClause,
				Name: getNode(node, "param"),
				Node: node,
			})
			r.referencingDefaultValue(pattern, info.assignments, nil, true)
		})
		r.visit(getNode(node, "body"))
		r.close(node)
	case "AssignmentExpression":
		left := getNode(node, "left")
		if isPatternNode(left) {
			if getString(node, "operator") == "=" {
				r.visitPattern(left, true, func(pattern Node, info patternInfo) {
					var maybe *implicitGlobalInfo
					if !r.currentScope().IsStrict {
						maybe = &implicitGlobalInfo{pattern: pattern, node: node}
					}
					r.referencingDefaultValue(pattern, info.assignments, maybe, false)
					r.currentScope().Referencing(pattern, RefWrite, getNode(node, "right"), maybe, !info.topLevel, false)
				})
			} else {
				r.currentScope().Referencing(left, RefRW, getNode(node, "right"), nil, false, false)
			}
		} else {
			r.visit(left)
		}
		r.visit(getNode(node, "right"))
	default:
		r.visitChildren(node)
	}
}

func (r *Referencer) visitExportDeclaration(node Node) {
	if getNode(node, "source") != nil {
		return
	}
	if decl := getNode(node, "declaration"); decl != nil {
		r.visit(decl)
		return
	}
	r.visitChildren(node)
}
