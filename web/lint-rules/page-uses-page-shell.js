// page-uses-page-shell: every page uses the shared shell from
// src/lib/pageShell.ts, one max-width and one card gap. Two ways to drift are
// checked:
//
//   1. The root of the page component, the one the file is named for, must be
//      `className={PAGE_SHELL}`, so a new page in src/pages is covered as soon
//      as it exists. A page that also renders as a tab panel of another page
//      may pick between the two shared shells with a ternary
//      (`embedded ? PAGE_SHELL_TABBED : PAGE_SHELL`), as long as one arm is the
//      shell this file requires.
//   2. Nothing in src/pages may hand-roll the shell: a `className` that pairs
//      a `max-w-*xl` cap with `flex-col` and a card-sized `gap-*` is a page
//      shell, whatever it is called.
//
// Exceptions are configured in eslint.config.js rather than inferred:
//
//     exceptions: {
//       "Settings.tsx": "PAGE_SHELL_TABBED",  // its 7-tab strip needs 1424px
//       "Login.tsx": null,                    // not a routed page at all
//     }
//
// The rule only visits src/pages/*.tsx and only recognises the default export
// or an export named after the file (`Fleet` or `FleetPage` in Fleet.tsx). A
// routed page outside that scope would go unchecked, so
// src/lib/uiConventions.test.ts resolves every route to its file and fails
// when this rule would not see it.
import { baseUtility, escapeHatch, getAttr, hasException } from "./helpers.js";

const RULE_ID = "page-uses-page-shell";

const DEFAULT_SHELL = "PAGE_SHELL";

/**
 * `flex flex-col gap-10 max-w-6xl` written out instead of imported.
 *
 * All three parts have to be page-scale, or this would fire on every small
 * stacked column in a settings row:
 *   * `max-w-*xl` only, a content column cap; `max-w-40`, `max-w-xs` and
 *     `max-w-full` are element widths.
 *   * a card-rhythm gap of at least gap-5 (20px); `gap-1` or `gap-2` is a
 *     label stack.
 */
const PAGE_WIDTH_CAP = /^max-w-\d*xl$/;
const CARD_RHYTHM_GAP = /^gap(?:-x|-y)?-(\d+)$/;

function isHandRolledShell(tokens) {
  // The patterns are anchored, so variant prefixes like `md:` come off first.
  const bases = tokens.map(baseUtility);
  const has = (re) => bases.some((t) => re.test(t));
  const gap = bases.map((t) => CARD_RHYTHM_GAP.exec(t)).find(Boolean);
  return has(PAGE_WIDTH_CAP) && has(/^flex-col$/) && gap !== undefined && Number(gap[1]) >= 5;
}

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "A routed page's root element uses the shared PAGE_SHELL constant, and no page hand-rolls the shell as literal width/gap classes.",
    },
    schema: [
      {
        type: "object",
        properties: {
          // filename -> required shell identifier, or null to exempt the file.
          exceptions: {
            type: "object",
            additionalProperties: { type: ["string", "null"] },
          },
        },
        additionalProperties: false,
      },
    ],
    messages: {
      notShelled:
        "The `{{component}}` page's root element does not use the shared page shell. Return `<div className={{{shell}}}>` from lib/pageShell.ts, which holds the one app-wide max-width and Card gap, so this page cannot drift from the others.{{hatch}}",
      handRolled:
        "This element hand-rolls the page shell in literal classes (`{{tokens}}`). Import PAGE_SHELL from lib/pageShell.ts instead; every literal copy of the width and gap is one more page width to drift.{{hatch}}",
    },
  },

  create(context) {
    const { exceptions = {} } = context.options[0] ?? {};
    const filename = (context.filename ?? context.getFilename() ?? "").replace(/\\/g, "/");
    const base = filename.slice(filename.lastIndexOf("/") + 1);

    if (!/\/src\/pages\/[^/]+\.tsx$/.test(filename)) return {};
    if (/\.test\.tsx$/.test(base)) return {};

    const exempt = Object.prototype.hasOwnProperty.call(exceptions, base);
    const requiredShell = exempt ? exceptions[base] : DEFAULT_SHELL;
    // An explicit `null` in the config means "this file is not a routed page".
    if (exempt && requiredShell === null) return {};

    const stem = base.replace(/\.tsx$/, "");
    const wanted = new Set([stem, `${stem}Page`]);

    /** Every function in this file that could be the page component. */
    const candidates = [];

    function noteExport(decl, isDefault) {
      if (!decl) return;
      if (decl.type === "FunctionDeclaration") {
        if (isDefault || wanted.has(decl.id?.name)) candidates.push(decl);
      } else if (decl.type === "VariableDeclaration") {
        for (const d of decl.declarations) {
          const isFn =
            d.init &&
            (d.init.type === "ArrowFunctionExpression" || d.init.type === "FunctionExpression");
          if (isFn && (isDefault || wanted.has(d.id?.name))) candidates.push(d.init);
        }
      } else if (isDefault && (decl.type === "ArrowFunctionExpression" || decl.type === "FunctionExpression")) {
        candidates.push(decl);
      } else if (isDefault && decl.type === "Identifier") {
        // `export default Fleet;` with the function declared elsewhere.
        const scope = context.sourceCode.getScope(decl);
        const variable = scope.references.find((r) => r.identifier === decl)?.resolved
          ?? scope.set.get(decl.name);
        for (const def of variable?.defs ?? []) {
          if (def.node?.type === "FunctionDeclaration") candidates.push(def.node);
          else if (
            def.node?.init &&
            (def.node.init.type === "ArrowFunctionExpression" || def.node.init.type === "FunctionExpression")
          ) {
            candidates.push(def.node.init);
          }
        }
      }
    }

    /** The `return` statements written directly in a function's own body. */
    function ownReturns(fn) {
      const out = [];
      // An arrow with an expression body has no ReturnStatement; its body is
      // the return.
      if (fn.type === "ArrowFunctionExpression" && fn.body?.type !== "BlockStatement") {
        return [{ type: "ReturnStatement", argument: fn.body }];
      }
      (function walk(node) {
        if (!node || typeof node !== "object") return;
        if (Array.isArray(node)) return node.forEach(walk);
        if (node.type === "ReturnStatement") out.push(node);
        // A nested function's returns (a row renderer, a useMemo callback) are
        // not the page's root.
        if (
          node !== fn &&
          (node.type === "FunctionDeclaration" ||
            node.type === "FunctionExpression" ||
            node.type === "ArrowFunctionExpression")
        )
          return;
        for (const key of Object.keys(node)) {
          if (key === "parent") continue;
          const child = node[key];
          if (child && typeof child === "object") walk(child);
        }
      })(fn.body);
      return out;
    }

    // The two shells lib/pageShell.ts exports, listed here because a rule only
    // sees one file's AST.
    const SHARED_SHELLS = ["PAGE_SHELL", "PAGE_SHELL_TABBED"];

    function usesShell(returnStatement) {
      const arg = returnStatement.argument;
      if (!arg || arg.type !== "JSXElement") return false;
      const attr = getAttr(arg, "className");
      const v = attr?.value;
      if (!v || v.type !== "JSXExpressionContainer") return false;
      const e = v.expression;
      if (e.type === "Identifier") return e.name === requiredShell;
      // `embedded ? PAGE_SHELL_TABBED : PAGE_SHELL` for a page that is also a
      // tab panel of another (Receiver, Fleet and Pull inside Instances). Both
      // arms must be shared shells and one must be the required one; a literal
      // in either arm is a violation.
      if (e.type === "ConditionalExpression") {
        const arms = [e.consequent, e.alternate];
        return (
          arms.every((a) => a.type === "Identifier" && SHARED_SHELLS.includes(a.name)) &&
          arms.some((a) => a.name === requiredShell)
        );
      }
      return false;
    }

    return {
      ExportNamedDeclaration: (n) => noteExport(n.declaration, false),
      ExportDefaultDeclaration: (n) => noteExport(n.declaration, true),

      JSXAttribute(node) {
        if (node.name?.name !== "className") return;
        const strings = [];
        (function collect(n) {
          if (!n || typeof n !== "object") return;
          if (Array.isArray(n)) return n.forEach(collect);
          if (n.type === "Literal" && typeof n.value === "string") strings.push(n.value);
          if (n.type === "TemplateLiteral") for (const q of n.quasis) strings.push(q.value.cooked ?? "");
          for (const k of Object.keys(n)) {
            if (k === "parent") continue;
            const c = n[k];
            if (c && typeof c === "object") collect(c);
          }
        })(node.value);
        for (const s of strings) {
          const tokens = s.split(/\s+/).filter(Boolean);
          if (!isHandRolledShell(tokens)) continue;
          if (hasException(context, node, RULE_ID)) return;
          context.report({
            node,
            messageId: "handRolled",
            data: { tokens: tokens.join(" "), hatch: escapeHatch(RULE_ID) },
          });
          return;
        }
      },

      "Program:exit"() {
        if (candidates.length === 0) return; // no component named for the file
        for (const fn of candidates) {
          const returns = ownReturns(fn);
          // A component with no JSX return at all is not a page root.
          const jsxReturns = returns.filter((r) => r.argument?.type === "JSXElement");
          if (jsxReturns.length === 0) continue;
          if (jsxReturns.some(usesShell)) continue;
          if (hasException(context, fn, RULE_ID)) continue;
          context.report({
            node: fn.id ?? fn,
            messageId: "notShelled",
            data: {
              component: fn.id?.name ?? stem,
              shell: requiredShell,
              hatch: escapeHatch(RULE_ID),
            },
          });
        }
      },
    };
  },
};
