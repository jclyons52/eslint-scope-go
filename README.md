# eslint-scope-go

A faithful Go port of [`eslint-scope`](https://github.com/estools/escope)
**7.2.2** (the version ESLint 8.x depends on), validated by a JS-oracle
parity test against the real npm package.

`eslint-scope` is the scope analyzer underneath ESLint: it walks an ESTree
AST (as produced by espree) and produces a `ScopeManager` — the nested lexical
scopes, the variables each defines, and the identifier references resolved
against them. This is one of the last big dependencies needed before ESLint's
rules and `Linter.verify` can run on top of the Go parser chain
(`espree-go` → `acorn-go` → `estraverse-go`).

## What's here

- `eslint_scope.go` — public `Analyze(tree, options)` entry point + options.
- `referencer.go` — the `Referencer` / `Importer` walkers (scope nesting,
  binding creation, reference recording), including every node-type handler.
- `pattern_visitor.go` — pattern walking for destructuring/params (the
  `PatternVisitor` and `isPattern`).
- `scope.go` — the `Scope` class and all its subtypes (`global`, `module`,
  `function`, `function-expression-name`, `block`, `switch`, `catch`, `with`,
  `for`, `class`, `class-field-initializer`, `class-static-block`).
- `scope_manager.go` — `ScopeManager` (scopes, `acquire`/`release`,
  `getDeclaredVariables`).
- `variable.go`, `reference.go`, `definition.go` — data model.
- `visitor.go` — the esrecurse-style traversal base (visitor-keys dispatch +
  iteration fallback).
- `visitor_keys.go` — the `estraverse.VisitorKeys` table (transcribed;
  kept local so the port is standalone and oracle-testable).
- `original/` — the vendored real npm source (eslint-scope 7.2.2 `lib/*` +
  `esrecurse.js`).
- `oracle/` — a Node environment (`espree` + `eslint-scope`) used as the
  parity oracle.

## Parity

`parity_test.go` runs the oracle (`node oracle/driver.js`) over a 40-case
corpus covering every scope type and analysis branch, then compares a
canonical serialization of the resulting `ScopeManager` — scopes, variables,
definitions, identifiers, references, resolved bindings, `through`,
`nodeToScope`, `declaredVariables`, and global implicit variables — between
the Go port and real eslint-scope.

```
PARITY PASS: 40 cases, 0 mismatches
```

## Verify

```
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

The parity test skips cleanly when `node` or the oracle deps aren't installed
(`npm install` in `oracle/` first).
