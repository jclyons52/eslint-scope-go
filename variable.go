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
	// Writeable mirrors ESLint's variable.writeable (a configured global
	// declared "writable"). eslint-scope does not maintain it; ESLint sets it
	// after analysis when applying the config's globals.
	Writeable bool
	// ESLintUsed records that ESLint's context.markVariableAsUsed() (or a
	// rule such as no-unused-vars marking an exported binding) has counted
	// this variable as used. eslint-scope itself does not set it — ESLint
	// does — so it lives outside the scope serializer's output.
	ESLintUsed bool
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
