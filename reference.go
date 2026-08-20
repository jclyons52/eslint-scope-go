package eslintscope

// Reference access-mode flags.
const (
	RefRead  = 0x1
	RefWrite = 0x2
	RefRW    = RefRead | RefWrite
)

// Reference represents a single occurrence of an identifier in code.
type Reference struct {
	Identifier          Node
	From                *Scope
	Tainted             bool
	Resolved            *Variable
	Flag                int
	WriteExpr           Node
	Partial             bool
	Init                bool
	maybeImplicitGlobal *implicitGlobalInfo
}

// implicitGlobalInfo mirrors the { pattern, node } object produced when a
// non-strict script assigns to an undeclared name (only meaningful on global
// scope).
type implicitGlobalInfo struct {
	pattern Node
	node    Node
}

// IsStatic reports whether the reference is static.
func (r *Reference) IsStatic() bool {
	return !r.Tainted && r.Resolved != nil && r.Resolved.Scope.IsStatic()
}

// IsWrite reports whether the reference is writeable.
func (r *Reference) IsWrite() bool { return r.Flag&RefWrite != 0 }

// IsRead reports whether the reference is readable.
func (r *Reference) IsRead() bool { return r.Flag&RefRead != 0 }

// IsReadOnly reports whether the reference is read-only.
func (r *Reference) IsReadOnly() bool { return r.Flag == RefRead }

// IsWriteOnly reports whether the reference is write-only.
func (r *Reference) IsWriteOnly() bool { return r.Flag == RefWrite }

// IsReadWrite reports whether the reference is read-write.
func (r *Reference) IsReadWrite() bool { return r.Flag == RefRW }
