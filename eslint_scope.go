package eslintscope

// Version mirrors the ported eslint-scope version.
const Version = "7.2.2"

func defaultOptions() *Options {
	return &Options{
		SourceType:  "script",
		ECMAVersion: 5,
		Fallback:    "iteration",
	}
}

// optionsFrom applies a provided options struct over the defaults. Zero-value
// fields leave the default unchanged (matching the deep-merge semantics of
// the original updateDeeply for the option set we model).
func optionsFrom(provided *Options) *Options {
	o := defaultOptions()
	if provided == nil {
		return o
	}
	if provided.Optimistic {
		o.Optimistic = true
	}
	if provided.Directive {
		o.Directive = true
	}
	if provided.IgnoreEval {
		o.IgnoreEval = true
	}
	if provided.NodejsScope {
		o.NodejsScope = true
	}
	if provided.ImpliedStrict {
		o.ImpliedStrict = true
	}
	if provided.SourceType != "" {
		o.SourceType = provided.SourceType
	}
	if provided.ECMAVersion != 0 {
		o.ECMAVersion = provided.ECMAVersion
	}
	if provided.ChildVisitorKeys != nil {
		o.ChildVisitorKeys = provided.ChildVisitorKeys
	}
	if provided.Fallback != "" {
		o.Fallback = provided.Fallback
	}
	return o
}

// Analyze takes an ESTree AST (as produced by espree) and returns the
// analyzed ScopeManager.
func Analyze(tree Node, provided *Options) *ScopeManager {
	options := optionsFrom(provided)
	sm := newScopeManager(options)
	referencer := newReferencer(options, sm)
	referencer.visit(tree)
	return sm
}
