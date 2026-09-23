package eslintscope

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type parityCase struct {
	ID         string
	Src        string
	SourceType string
	Options    *Options
}

// corpus is the set of analysis scenarios exercised against the oracle.
func corpus() []parityCase {
	es6 := &Options{ECMAVersion: 2020, SourceType: "script"}
	es5 := &Options{ECMAVersion: 5, SourceType: "script"}
	mod := &Options{ECMAVersion: 2022, SourceType: "module"}
	mod6 := &Options{ECMAVersion: 2020, SourceType: "module"}
	cjs := &Options{ECMAVersion: 2020, SourceType: "commonjs"}
	nodejs := &Options{ECMAVersion: 2020, SourceType: "script", NodejsScope: true}
	return []parityCase{
		{ID: "basic-var", Src: "var a = 1; a;", Options: es5},
		{ID: "basic-let", Src: "let x = 1; { let y = 2; x; } y;", Options: es6},
		{ID: "var-hoist", Src: "function f() { var a; a = 1; return a; }", Options: es5},
		{ID: "fn-params-default", Src: "const x = 1; function f(a = x) { const x = 2; return a; }", Options: es6},
		{ID: "fn-shadow", Src: "var v = 1; function f(v) { return v; } f(2);", Options: es5},
		{ID: "arrow", Src: "const g = (p) => { let q = p; return q; };", Options: es6},
		{ID: "arrow-expr-body", Src: "const h = p => p * 2;", Options: es6},
		{ID: "fn-expr-name", Src: "var h = function named() { return named; };", Options: es6},
		{ID: "catch", Src: "try {} catch (e) { e; }", Options: es6},
		{ID: "with", Src: "with (obj) { foo; }", Options: es5},
		{ID: "for-let", Src: "for (let i = 0; i < 3; i++) { i; }", Options: es6},
		{ID: "for-in-let", Src: "var o = {}; for (let k in o) { k; }", Options: es6},
		{ID: "for-of", Src: "var arr = []; for (const x of arr) { x; }", Options: es6},
		{ID: "for-of-destructure", Src: "var pairs = []; for (const [a, b] of pairs) { a; b; }", Options: es6},
		{ID: "switch-let", Src: "switch (v) { case 1: let a = 2; a; }", Options: es6},
		{ID: "class-fields", Src: "class A { x = 1; static y = 2; #p = 3; static { this.z = 4; } method() { return this.x; } }", Options: &Options{ECMAVersion: 2022, SourceType: "script"}},
		{ID: "class-extends", Src: "class B extends C { constructor() { super(); } }", Options: &Options{ECMAVersion: 2020, SourceType: "script"}},
		{ID: "destructure-params", Src: "function f({a, b}, [c, d], {e = 1}) { return a; }", Options: es6},
		{ID: "rest-param", Src: "function f(x, ...rest) { return rest.length; }", Options: es6},
		{ID: "implicit-global", Src: "foo = 5; bar = function(){};", Options: es5},
		{ID: "implicit-global-letconflict", Src: "let z = 1; z2 = 2;", Options: es6},
		{ID: "nodejs-scope", Src: "var a = 1; module.exports = a;", Options: nodejs},
		{ID: "commonjs", Src: "var x = 1; exports.x = x;", Options: cjs},
		{ID: "strict-directive", Src: "'use strict'; var a = 1;", Options: es5},
		{ID: "implied-strict", Src: "var a = 1;", Options: &Options{ECMAVersion: 2020, SourceType: "script", ImpliedStrict: true}},
		{ID: "update", Src: "let i = 0; i++; ++i;", Options: es6},
		{ID: "member", Src: "a.b.c; obj[prop]; this.x;", Options: es6},
		{ID: "eval-global", Src: "eval('x');", Options: es6},
		{ID: "eval-fn", Src: "function f() { eval('y'); return y; }", Options: es6},
		{ID: "ignore-eval", Src: "function f() { eval('y'); }", Options: &Options{ECMAVersion: 2020, SourceType: "script", IgnoreEval: true}},
		{ID: "this-fn", Src: "function f() { return this; }", Options: es6},
		{ID: "labeled-break", Src: "outer: for (;;) { break outer; }", Options: es6},
		{ID: "getter-setter", Src: "var o = { get x() { return 1; }, set x(v) {} };", Options: es6},
		{ID: "module-imports", Src: "import a, { b as c } from 'm'; import * as ns from 'n'; export default a; export { c };", Options: mod6},
		{ID: "module-reexport", Src: "export { x } from 'm'; export * from 'n';", Options: mod6},
		{ID: "module-import-meta", Src: "import('m'); import.meta.url;", Options: mod},
		{ID: "nested-scopes", Src: "function outer(a) { let b = 1; function inner(c) { let d = b; return d + c; } return inner(2); }", Options: es6},
		{ID: "try-catch-finally", Src: "try { throw new Error('x'); } catch (e) { e; } finally { cleanup(); }", Options: es6},
		{ID: "compound-assign", Src: "let n = 1; n += 2;", Options: es6},
		{ID: "for-in-undeclared", Src: "for (k in obj) { k; }", Options: es5},
	}
}

// jsOptions renders the Go options into the JSON object the oracle driver
// passes to eslint-scope.analyze. Zero-value fields are omitted so the oracle
// uses its own defaults (mirroring analyze()'s deep-merge).
func jsOptions(o *Options) map[string]any {
	m := map[string]any{}
	if o.Optimistic {
		m["optimistic"] = true
	}
	if o.Directive {
		m["directive"] = true
	}
	if o.IgnoreEval {
		m["ignoreEval"] = true
	}
	if o.NodejsScope {
		m["nodejsScope"] = true
	}
	if o.ImpliedStrict {
		m["impliedStrict"] = true
	}
	if o.SourceType != "" {
		m["sourceType"] = o.SourceType
	}
	if o.ECMAVersion != 0 {
		m["ecmaVersion"] = o.ECMAVersion
	}
	if o.Fallback != "" {
		m["fallback"] = o.Fallback
	}
	return m
}

func TestParity(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping oracle parity")
	}
	wd, _ := os.Getwd()
	driver := filepath.Join(wd, "oracle", "driver.js")
	if _, err := os.Stat(driver); err != nil {
		t.Skip("oracle driver not installed; skipping")
	}
	// The driver requires the real espree/eslint-scope/estraverse. They are not
	// committed (test-time dependencies), so a checkout without `npm ci` in
	// oracle/ skips — a skip is never a pass. CI installs them from the lockfile.
	if _, err := os.Stat(filepath.Join(wd, "oracle", "node_modules", "eslint-scope")); err != nil {
		t.Skip("oracle/node_modules missing: run `npm ci` in oracle/ to verify against the npm original")
	}

	cases := corpus()
	// Build the corpus JSON for the JS side.
	type jcase struct {
		ID         string         `json:"id"`
		Src        string         `json:"src"`
		SourceType string         `json:"sourceType,omitempty"`
		Options    map[string]any `json:"options"`
	}
	jc := make([]jcase, 0, len(cases))
	for _, c := range cases {
		st := "script"
		if c.Options != nil && c.Options.SourceType == "module" {
			st = "module"
		}
		jc = append(jc, jcase{ID: c.ID, Src: c.Src, SourceType: st, Options: jsOptions(c.Options)})
	}
	corpusBytes, _ := json.Marshal(map[string]any{"cases": jc})
	tmp, err := os.CreateTemp(t.TempDir(), "corpus-*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write(corpusBytes)
	tmp.Close()

	// Run `node driver.js corpusfile` to produce the oracle's tree + expect.
	out, runErr := exec.Command(nodeBin, driver, tmp.Name()).CombinedOutput()
	if runErr != nil {
		t.Fatalf("oracle run failed: %v\n%s", runErr, out)
	}

	var res struct {
		Cases []struct {
			ID     string         `json:"id"`
			Tree   map[string]any `json:"tree"`
			Expect any            `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal oracle output: %v", err)
	}
	if len(res.Cases) != len(cases) {
		t.Fatalf("oracle returned %d cases, expected %d", len(res.Cases), len(cases))
	}

	failures := 0
	for i, rc := range res.Cases {
		goSM := Analyze(Node(rc.Tree), cases[i].Options)
		goSer := SerializeSM(goSM)
		goBytes, _ := json.Marshal(goSer)
		var goAny any
		json.Unmarshal(goBytes, &goAny)
		if !reflect.DeepEqual(goAny, rc.Expect) {
			failures++
			t.Errorf("CASE %s: MISMATCH\n--- expect ---\n%v\n--- got ---\n%v", rc.ID, mustIndent(rc.Expect), string(goBytes))
		} else {
			t.Logf("CASE %s: OK", rc.ID)
		}
	}
	if failures > 0 {
		t.Fatalf("parity: %d/%d cases mismatched", failures, len(cases))
	}
	t.Logf("PARITY PASS: %d cases, 0 mismatches", len(cases))
}

func mustIndent(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
