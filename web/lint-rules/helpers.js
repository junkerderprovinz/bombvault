// AST helpers shared by the bombvault convention rules. The JSX node types used
// here are identical in espree (which RuleTester uses in the unit tests) and in
// typescript-eslint's parser (which `npm run lint` uses), so a rule tested on
// plain JSX behaves the same on the real tree.

/** The tag name of a JSXElement / JSXOpeningElement, e.g. "div", "Badge". */
export function jsxName(node) {
  const opening = node.type === "JSXElement" ? node.openingElement : node;
  const name = opening?.name;
  if (!name) return "";
  if (name.type === "JSXIdentifier") return name.name;
  // <Foo.Bar />: the last part names the component.
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
 * The string value of an attribute written as `x="lit"` or `x={"lit"}`, or
 * undefined for anything computed. The rules treat a value they cannot see as
 * compliant rather than guess.
 */
export function attrStringValue(node, attr) {
  const a = getAttr(node, attr);
  if (!a) return undefined;
  const v = a.value;
  if (!v) return undefined; // a bare `iconOnly` is boolean true
  if (v.type === "Literal" && typeof v.value === "string") return v.value;
  if (v.type === "JSXExpressionContainer") {
    const e = v.expression;
    if (e.type === "Literal" && typeof e.value === "string") return e.value;
    if (e.type === "TemplateLiteral" && e.expressions.length === 0) return e.quasis[0]?.value?.cooked;
  }
  return undefined;
}

/**
 * The initializer of the `const` an identifier names, or undefined. Only a
 * const with a single definition and no other write is followed; a `let`, a
 * parameter or an import reads as unknown.
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
    if (variable.references.some((r) => r.isWrite() && r.identifier !== def.name)) return undefined;
    return def.node?.init;
  }
  return undefined;
}

/**
 * Every string literal inside a node, template quasis included. With
 * `context`, an identifier is followed to the const it names, so a class list
 * factored out into `const inputCls = "…"` is still checked. `seen` breaks a
 * cycle between two consts.
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
    // `S.btn` uses one entry of S. Following the object would judge the
    // className by every string in the map, so only a computed key is read.
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
 * Every class token an element can render, from every string literal in its
 * `className`, including both arms of a conditional: `hover:text-statusFail`
 * in one branch is still a bespoke colour.
 *
 * With `context`, a bare identifier such as `className={inputCls}` is followed
 * to its const. Without it only literals are read, which the RuleTester cases
 * that call this helper directly rely on.
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
      // A call like t("x"), a member access, a template or a map() can all
      // produce a string. Counting them as text keeps "icon-only" narrow.
      return true;
  }
}

/** Does this element render a child element, such as an <svg> or an icon? */
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
 * Icon-only: renders at least one element and no text. Derived from what the
 * element renders rather than from a prop a caller could forget to pass.
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

// A house convention occasionally has a real exception, and a bare
// eslint-disable says nothing about why. Each rule honours a marker comment
// directly above the offending element instead, as in pages/Dashboard.tsx:
//
//     {/* bv-convention-exception: control-reads-engine-tokens --
//         the heat-map cell is a chart mark, not a control; its colour is the
//         datum and cannot come from the engine. */}
//
// A marker whose reason is shorter than 12 characters is ignored.
const MARKER = /bv-convention-exception:\s*([a-z-]+)\s*(?:--|—|:)\s*(\S[\s\S]*)$/;

/** How many lines above the reported node the marker's comment may end. */
const MARKER_LOOKBACK = 8;

/**
 * The file's comments as the author wrote them: runs of adjacent `//` lines
 * are joined into one block, so a marker's reason can span several lines.
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
    if (m[2].replace(/[\s/*]+/g, " ").trim().length < 12) continue; // too short to be a reason
    return true;
  }
  return false;
}

/** Appended to every message so the fix and the escape hatch are both stated. */
export function escapeHatch(ruleId) {
  return ` If this is a real, reasoned exception, put \`bv-convention-exception: ${ruleId} -- <why>\` in a comment directly above it.`;
}
