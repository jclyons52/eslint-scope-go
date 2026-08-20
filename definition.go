package eslintscope

// Definition describes a defining occurrence of a variable. Rest is only
// meaningful for parameter definitions (it mirrors ParameterDefinition.rest).
type Definition struct {
	Type   string
	Name   Node
	Node   Node
	Parent Node
	Index  *int
	Kind   *string
	Rest   bool
}

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }
