package eslintscope

// patternInfo is the { topLevel, rest, assignments } payload passed to the
// callback of a PatternVisitor traversal.
type patternInfo struct {
	topLevel    bool
	rest        bool
	assignments []Node
}

func getLastAssignment(xs []Node) Node {
	if len(xs) == 0 {
		return nil
	}
	return xs[len(xs)-1]
}

// isPatternNode is PatternVisitor.isPattern.
func isPatternNode(node Node) bool {
	switch nodeType(node) {
	case "Identifier", "ObjectPattern", "ArrayPattern", "SpreadElement", "RestElement", "AssignmentPattern":
		return true
	}
	return false
}

// PatternVisitor walks a binding pattern (the left-hand side of declarations,
// parameters, destructuring assignments) and reports each identifier plus the
// rest/assignment context to a callback, collecting right-hand nodes for the
// referencer to process afterwards.
type PatternVisitor struct {
	visitorBase
	rootPattern    Node
	callback       func(Node, patternInfo)
	assignments    []Node
	rightHandNodes []Node
	restElements   []Node
}

func newPatternVisitor(options *Options, rootPattern Node, callback func(Node, patternInfo)) *PatternVisitor {
	return &PatternVisitor{
		visitorBase: newVisitorBase(options),
		rootPattern: rootPattern,
		callback:    callback,
	}
}

func (v *PatternVisitor) visit(node Node) {
	if node == nil {
		return
	}
	switch nodeType(node) {
	case "Identifier":
		lastRest := getLastAssignment(v.restElements)
		rest := lastRest != nil && nodeID(getNode(lastRest, "argument")) == nodeID(node)
		v.callback(node, patternInfo{
			topLevel:    nodeID(node) == nodeID(v.rootPattern),
			rest:        rest,
			assignments: v.assignments,
		})
	case "Property":
		if getBool(node, "computed") {
			v.rightHandNodes = append(v.rightHandNodes, getNode(node, "key"))
		}
		v.visit(getNode(node, "value"))
	case "ArrayPattern":
		for _, element := range getNodes(node, "elements") {
			v.visit(element)
		}
	case "AssignmentPattern":
		v.assignments = append(v.assignments, node)
		v.visit(getNode(node, "left"))
		v.rightHandNodes = append(v.rightHandNodes, getNode(node, "right"))
		v.assignments = v.assignments[:len(v.assignments)-1]
	case "RestElement":
		v.restElements = append(v.restElements, node)
		v.visit(getNode(node, "argument"))
		v.restElements = v.restElements[:len(v.restElements)-1]
	case "MemberExpression":
		if getBool(node, "computed") {
			v.rightHandNodes = append(v.rightHandNodes, getNode(node, "property"))
		}
		v.rightHandNodes = append(v.rightHandNodes, getNode(node, "object"))
	case "SpreadElement":
		v.visit(getNode(node, "argument"))
	case "ArrayExpression":
		for _, element := range getNodes(node, "elements") {
			v.visit(element)
		}
	case "AssignmentExpression":
		v.assignments = append(v.assignments, node)
		v.visit(getNode(node, "left"))
		v.rightHandNodes = append(v.rightHandNodes, getNode(node, "right"))
		v.assignments = v.assignments[:len(v.assignments)-1]
	case "CallExpression":
		for _, arg := range getNodes(node, "arguments") {
			v.rightHandNodes = append(v.rightHandNodes, arg)
		}
		v.visit(getNode(node, "callee"))
	default:
		v.visitorBase.readChildren(node, v.visit)
	}
}
