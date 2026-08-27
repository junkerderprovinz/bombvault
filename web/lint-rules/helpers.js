// ---------------------------------------------------------------------------
// Shared AST helpers for the bombvault convention rules.
//
// Everything here works on the JSX nodes that BOTH espree (with
// `ecmaFeatures.jsx`, what RuleTester uses in the unit tests) and
// typescript-eslint's parser (what `npm run lint` actually uses on .tsx)
// produce — JSXElement / JSXOpeningElement / JSXAttribute / JSXText /
// JSXExpressionContainer are identical between the two, so a rule tested
// against plain JSX is the same rule that runs against the real tree.
//
// Why AST and not a grep script: this codebase's comment blocks are longer
// than its code and they QUOTE the very patterns these rules look for
// (`shape="square"`, `tone="fail"`, `rounded-full` and `size="icon"` all
// appear dozens of times inside prose explaining why a past round did or did
// not use them). A text scanner cannot tell a call site from a paragraph
// about a call site; a parser never sees the paragraphs at all.
// ---------------------------------------------------------------------------

/** The tag name of a JSXElement / JSXOpeningElement, e.g. "div", "Badge". */
export function jsxName(node) {
  const opening = node.type === "JSXElement" ? node.openingElement : node;
  const name = opening?.name;
  if (!name) return "";
  if (name.type === "JSXIdentifier") return name.name;
  // <Foo.Bar /> — the member expression's last part is the useful half.
  if (name.type === "JSXMemberExpression") return name.property?.name ?? "";
  return "";
}

/** The JSXAttribute node for `attr` on a JSXElement/JSXOpeningElement. */
export function getAttr(node, attr) {
  const opening = node.type === "JSXElement" ? node.openingElement : node;
  return (opening?.attributes ?? []).find(
    (a) => a.type === "JSXAttribute" && a.name?.type === "JSXIdentifier" && a.name.name === attr
  );
}

export function hasAttr(node, attr) {
  return getAttr(node, attr) !== undefined;
}

/**
 * The literal string value of an attribute written as `x="lit"` or `x={"lit"}`.
 * Returns undefined for anything computed — a rule must never guess at a value
 * it cannot see, so "unknown" is always treated as "not a violation".
 */
export function attrStringValue(node, attr) {
  const a = getAttr(node, attr);
  if (!a) return undefined;
  const v = a.value;
  if (!v) return undefined; // bare `iconOnly` — boolean true, not a string
  if (v.type === "Literal" && typeof v.value === "string") return v.value;
  if (v.type === "JSXExpressionContainer") {
    const e = v.expression;
    if (e.type === "Literal" && typeof e.value === "string") return e.value;
    if (e.type === "TemplateLiteral" && e.expressions.length === 0) return e.quasis[0]?.value?.cooked;
  }
  return undefined;
}

/**
 * The `const x = "…"` a className identifier stands for, or undefined.
 *
 * Only a CONST with a single definition and no later reassignment is followed:
 * anything a rule cannot see for certain must read as "unknown", and unknown is
 * never a violation. A `let` that is reassigned, a function parameter, an import
 * from another module — all decline here rather than guess.
 */
function constInitializer(idNode, context) {
  const sourceCode = context?.sourceCode ?? context?.getSourceCode?.();
  if (!sourceCode?.getScope) return undefined;
  let scope;
  try {
    scope = sourceCode.getScope(idNode);
  } catch {
    return undefined;
  }
  for (let s = scope; s; s = s.upper) {
    const variable = s.variables.find((v) => v.name === idNode.name);
    if (!variable) continue;
    if (variable.defs.length !== 1) return undefined;
    const def = variable.defs[0];
    if (def.type !== "Variable" || def.parent?.kind !== "const") return undefined;
    // A const can still be mutated through its own reference in TS? No — but a
    // write reference at all means this is not the single settled value.
    if (variable.references.some((r) => r.isWrite() && r.identifier !== def.name)) return undefined;
    return def.node?.init;
  }
  return undefined;
}

/**
 * Every string literal anywhere inside a node (template quasis included).
 *
 * `context` is optional and, when given, lets an IDENTIFIER be followed to the
 * `const` it names. That is not a nicety: a class list moved into a local (the
 * `const inputCls = "rounded-control bg-carbon-surface2 …"` idiom this codebase
 * uses at a dozen interactive call sites) was invisible to every rule built on
 * this helper, so those controls were compliant by luck and unchecked in fact —
 * a guard that reads only literals silently stops guarding the moment someone
 * factors the literal out. `seen` breaks a cycle between two consts.
 */
function collectStrings(node, out, context, seen) {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const n of node) collectStrings(n, out, context, seen);
    return;
  }
  if (node.type === "Literal" && typeof node.value === "string") out.push(node.value);
  else if (node.type === "TemplateLiteral") {
    for (const q of node.quasis) if (q.value?.cooked) out.push(q.value.cooked);
    for (const e of node.expressions) collectStrings(e, out, context, seen);
  } else if (node.type === "MemberExpression") {
    // A property ACCESS is not the whole map. Resolving `S.btn` by expanding its
    // object through constInitializer pulled in every string of the class map,
    // so all three class-reading rules (which run at "error") judged a className
    // by strings the element never referenced. The property name is the useful
    // part and is collected on its own when it is a plain literal key; the
    // object is deliberately not followed.
    if (node.computed) collectStrings(node.property, out, context, seen);
    return;
  } else if (node.type === "Identifier" && context && seen && !seen.has(node.name)) {
    seen.add(node.name);
    collectStrings(constInitializer(node, context), out, context, seen);
    return;
  }
  for (const key of Object.keys(node)) {
    if (key === "parent" || key === "loc" || key === "range" || key === "type") continue;
    const child = node[key];
    if (child && typeof child === "object") collectStrings(child, out, context, seen);
  }
}

/**
 * Every whitespace-separated class token an element can render, gathered from
 * every string literal in its `className` — a plain string, a template
 * literal, and both arms of a `cond ? "a" : "b"` inside one. Conditional arms
 * are included deliberately: `hover:text-statusFail` is just as much a
 * bespoke colour when it only appears in one branch.
 *
 * Pass `context` to also follow a bare identifier to the `const` it names, so
 * `className={inputCls}` is read rather than skipped. Without it the old
 * literals-only behaviour is unchanged, which is what the RuleTester cases that
 * exercise this helper directly rely on.
 */
export function classTokens(node, context) {
  const a = getAttr(node, "className");
  if (!a || !a.value) return [];
  const strings = [];
  collectStrings(a.value, strings, context, context ? new Set() : undefined);
  return strings.flatMap((s) => s.split(/\s+/)).filter(Boolean);
}

/** Strips Tailwind variant prefixes: "hover:focus:text-x" -> "text-x". */
export function baseUtility(token) {
  const i = token.lastIndexOf(":");
  return i === -1 ? token : token.slice(i + 1);
}

/**
 * Does this element's subtree render any human-readable text? A JSXText run
 * with non-whitespace content, or any expression that is not itself just more
 * JSX (`{t("x")}`, `{label}`, `{`${a} b`}`, `{"x"}`).
 */
export function subtreeHasText(node) {
  if (node.type !== "JSXElement" && node.type !== "JSXFragment") return false;
  for (const child of node.children ?? []) {
    if (child.type === "JSXText") {
      if (child.value.trim() !== "") return true;
    } else if (child.type === "JSXElement" || child.type === "JSXFragment") {
      if (subtreeHasText(child)) return true;
    } else if (child.type === "JSXExpressionContainer") {
      if (expressionRendersText(child.expression)) return true;
    }
  }
  return false;
}

function expressionRendersText(e) {
  if (!e) return false;
  switch (e.type) {
    case "JSXEmptyExpression": // {/* a comment */}
      return false;
    case "JSXElement":
    case "JSXFragment":
      return subtreeHasText(e);
    case "LogicalExpression":
      return expressionRendersText(e.right);
    case "ConditionalExpression":
      return expressionRendersText(e.consequent) || expressionRendersText(e.alternate);
    case "Literal":
      // `{null}` / `{false}` render nothing; a string literal is text.
      return typeof e.value === "string" && e.value.trim() !== "";
    case "Identifier":
      return e.name !== "undefined";
    default:
      // A call (`t("x")`), a member expression, a template literal, a map()
      // — anything that can produce a string. Treated as text, which makes
      // "icon-only" the conservative, narrow classification it should be.
      return true;
  }
}

/** Does this element have a glyph child (an <svg>, an <Icon*/ /*> component)? */
export function hasElementChild(node) {
  return (node.children ?? []).some(
    (c) =>
      c.type === "JSXElement" ||
      c.type === "JSXFragment" ||
      (c.type === "JSXExpressionContainer" && containsJsx(c.expression))
  );
}

function containsJsx(e) {
  if (!e) return false;
  if (e.type === "JSXElement" || e.type === "JSXFragment") return true;
  if (e.type === "LogicalExpression") return containsJsx(e.right);
  if (e.type === "ConditionalExpression") return containsJsx(e.consequent) || containsJsx(e.alternate);
  return false;
}

/**
 * Icon-only: renders at least one element and no text at all. This is exactly
 * the shape Badge.tsx's own `tip` doc describes ("an icon-only trigger has
 * nothing else a name could come from"), derived from what the element
 * actually renders rather than from a prop a caller could forget to pass.
 */
export function isIconOnly(node) {
  return hasElementChild(node) && !subtreeHasText(node);
}

/** Interactive: a real control, or anything wired to a click. */
export function isInteractive(node) {
  const name = jsxName(node);
  if (INTERACTIVE_TAGS.has(name)) return true;
  if (name === "Badge") {
    const as = attrStringValue(node, "as");
    return as === "button" || as === "a";
  }
  if (hasAttr(node, "onClick")) return true;
  return false;
}

const INTERACTIVE_TAGS = new Set([
  "button",
  "a",
  "input",
  "select",
  "textarea",
  "summary",
  "IconTipButton",
]);

// ---------------------------------------------------------------------------
// The escape hatch.
//
// Every rule here is a house convention, and a house convention occasionally
// has a real exception. Rather than a bare `eslint-disable` (which says
// nothing about WHY and is invisible to anyone auditing the conventions), each
// rule honours one marker comment placed directly above the offending element:
//
//     {/* bv-convention-exception: no-status-color-on-control --
//         ConfirmDialog's own destructive-variant button IS the status
//         surface; the dialog exists to state the severity. */}
//
// The reason text is mandatory (>= 12 characters) — a marker with no reason is
// not accepted and the rule still fires. Every exception in the app is one
// `grep -rn "bv-convention-exception" web/src` away, which a blanket
// eslint-disable comment never is.
// ---------------------------------------------------------------------------
const MARKER = /bv-convention-exception:\s*([a-z-]+)\s*(?:--|—|:)\s*(\S[\s\S]*)$/;

/** How many lines above the reported node the marker BLOCK may end. */
const MARKER_LOOKBACK = 8;

/**
 * Consecutive `//` lines are separate comment nodes to the parser but one
 * paragraph to a reader, and a real reason spills over several of them. Glue
 * runs of adjacent line comments (and each block comment) back into the block
 * the author actually wrote, so the marker's reason can span lines.
 */
function commentBlocks(sourceCode) {
  const blocks = [];
  let current = null;
  for (const c of sourceCode.getAllComments()) {
    const joinable =
      current !== null && c.type === "Line" && current.type === "Line" && c.loc.start.line === current.endLine + 1;
    if (joinable) {
      current.text += `\n${c.value}`;
      current.endLine = c.loc.end.line;
      continue;
    }
    current = { type: c.type, text: c.value, endLine: c.loc.end.line };
    blocks.push(current);
  }
  return blocks;
}

export function hasException(context, node, ruleId) {
  const sourceCode = context.sourceCode ?? context.getSourceCode();
  const anchor = node.loc.start.line;
  for (const block of commentBlocks(sourceCode)) {
    if (block.endLine > anchor || block.endLine < anchor - MARKER_LOOKBACK) continue;
    const m = MARKER.exec(block.text);
    if (!m) continue;
    if (m[1] !== ruleId) continue;
    if (m[2].replace(/[\s/*]+/g, " ").trim().length < 12) continue; // a reason, not a shrug
    return true;
  }
  return false;
}

/** Appended to every message so the fix and the escape hatch are both stated. */
export function escapeHatch(ruleId) {
  return ` If this is a real, reasoned exception, put \`bv-convention-exception: ${ruleId} -- <why>\` in a comment directly above it.`;
}
