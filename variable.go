package eslintscope

// Variable represents a locally scoped identifier, including the automatic
// "arguments" binding of function scopes.
type Variable struct {
	Name        string
	Identifiers []Node
	References  []*Reference
	Defs        []*Definition
	Tainted     bool
	Stack       bool
	Scope       *Scope
}

// Variable definition-type constants.
const (
	VarCatchClause            = "CatchClause"
	VarParameter              = "Parameter"
	VarFunctionName           = "FunctionName"
	VarClassName              = "ClassName"
	VarVariable               = "Variable"
	VarImportBinding          = "ImportBinding"
	VarImplicitGlobalVariable = "ImplicitGlobalVariable"
)
