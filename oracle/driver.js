// eslint-scope-go parity oracle driver.
// Reads a corpus JSON file (argv[2]) of the form
//   { "cases": [ { "id": "...", "src": "...", "sourceType": "...",
//                  "options": { ecmaVersion, sourceType, nodejsScope, ... } } ] }
// For each case it parses src with real espree, analyzes with real
// eslint-scope, annotates the tree with per-node $id, and outputs
//   { "cases": [ { id, tree, expect } ] }
// where `expect` is a canonical serialization of the ScopeManager.
'use strict';

const fs = require('fs');
const espree = require('espree');
const escope = require('eslint-scope');
const estraverse = require('estraverse');

function annotate(root) {
  let n = 0;
  (function assign(node) {
    if (!node || typeof node !== 'object' || typeof node.type !== 'string') return;
    if (node.$id !== undefined) return;
    node.$id = n++;
    const keys = estraverse.VisitorKeys[node.type];
    if (!keys) return;
    for (const k of keys) {
      const c = node[k];
      if (Array.isArray(c)) {
        for (const el of c) assign(el);
      } else {
        assign(c);
      }
    }
  })(root);
  return root;
}

function serialize(sm) {
  const scopeIndex = new Map(sm.scopes.map((s, i) => [s, i]));
  const varIndex = new Map();
  for (const s of sm.scopes) {
    for (const v of s.variables) if (!varIndex.has(v)) varIndex.set(v, varIndex.size);
    if (s.implicit) for (const v of s.implicit.variables) if (!varIndex.has(v)) varIndex.set(v, varIndex.size);
  }
  const refIndex = new Map();
  let refId = 0;
  for (const s of sm.scopes) for (const r of s.references) refIndex.set(r, refId++);

  const defSer = d => ({
    type: d.type,
    name: d.name ? d.name.$id : null,
    node: d.node ? d.node.$id : null,
    parent: d.parent ? d.parent.$id : null,
    index: (d.index === undefined || d.index === null) ? null : d.index,
    kind: (d.kind === undefined || d.kind === null) ? null : d.kind,
    rest: ('rest' in d) ? d.rest : null
  });
  const varSer = (v, si) => ({
    id: varIndex.get(v),
    name: v.name,
    stack: v.stack,
    tainted: v.tainted,
    scope: si,
    identifiers: v.identifiers.map(x => x.$id),
    defs: v.defs.map(defSer),
    references: v.references.map(r => refIndex.get(r))
  });

  const scopes = sm.scopes.map((s, si) => {
    const obj = {
      index: si,
      type: s.type,
      blockId: s.block ? s.block.$id : null,
      blockType: s.block ? s.block.type : null,
      dynamic: s.dynamic,
      isStrict: s.isStrict,
      functionExpressionScope: s.functionExpressionScope,
      directCallToEvalScope: s.directCallToEvalScope,
      thisFound: s.thisFound,
      upper: s.upper ? scopeIndex.get(s.upper) : null,
      variableScope: scopeIndex.get(s.variableScope),
      childScopes: s.childScopes.map(c => scopeIndex.get(c)),
      references: s.references.map(r => refIndex.get(r)),
      through: s.through.map(r => refIndex.get(r)),
      isArgumentsMaterialized: s.isArgumentsMaterialized(),
      isThisMaterialized: s.isThisMaterialized(),
      variables: s.variables.map(v => varSer(v, si))
    };
    if (s.implicit) {
      obj.implicitVariables = s.implicit.variables.map(v => varSer(v, si));
      obj.implicitLeft = (s.implicit.left || []).map(r => refIndex.get(r));
    }
    return obj;
  });

  const nodeToScope = {};
  for (const [si, s] of sm.scopes.entries()) {
    if (!s.block) continue;
    const id = s.block.$id;
    if (!nodeToScope[id]) nodeToScope[id] = [];
    nodeToScope[id].push(si);
  }
  const declared = {};
  function addDecl(node, vi) {
    if (!node) return;
    const id = node.$id;
    if (!declared[id]) declared[id] = [];
    if (!declared[id].includes(vi)) declared[id].push(vi);
  }
  for (const s of sm.scopes) {
    for (const v of s.variables) {
      const vi = varIndex.get(v);
      for (const d of v.defs) { addDecl(d.node, vi); addDecl(d.parent, vi); }
    }
  }
  if (sm.globalScope) {
    for (const v of sm.globalScope.implicit.variables) {
      const vi = varIndex.get(v);
      for (const d of v.defs) { addDecl(d.node, vi); addDecl(d.parent, vi); }
    }
  }

  return {
    scopes,
    globalScope: sm.globalScope ? scopeIndex.get(sm.globalScope) : null,
    nodeToScope,
    declaredVariables: declared
  };
}

const corpus = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const out = { cases: [] };
for (const c of corpus.cases) {
  const tree = annotate(espree.parse(c.src, {
    ecmaVersion: 'latest',
    sourceType: c.sourceType || 'script',
    range: true,
    loc: true,
    comment: false
  }));
  const sm = escope.analyze(tree, c.options || {});
  out.cases.push({ id: c.id, tree, expect: serialize(sm) });
}
console.log(JSON.stringify(out));
