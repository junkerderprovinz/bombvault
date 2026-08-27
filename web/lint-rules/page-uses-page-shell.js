// ---------------------------------------------------------------------------
// page-uses-page-shell — settled convention 7.
//
// "Every page uses the shared page shell (web/src/lib/pageShell.ts): one
// max-width, one card gap. Settings is the one documented exception."
//
// pageShell.ts records what this cost to find: FIVE different rendered card
// widths and TWO different card gaps across ten pages, measured live, after
// several earlier rounds had each "made it consistent" on whichever single
// page jdp happened to open that day. The constant fixed the ten pages that
// existed. Nothing stopped page eleven.
//
// Two halves, because there are two ways to drift:
//
//   1. The page component's root must be `className={PAGE_SHELL}`. Checked on
//      the component the file is named for, so a NEW page in src/pages is
//      covered the moment it is created.
//   2. Nothing anywhere in src/pages may hand-roll the shell as literal
//      classes — a `className` that pairs a `max-w-*` cap with `flex-col` and
//      a `gap-*` IS a page shell, whatever it is called. That is the exact
//      shape of every one of the ten originals.
//
// The exceptions are DATA, in eslint.config.js, not something a rule infers:
//
//     exceptions: {
//       "Settings.tsx": "PAGE_SHELL_TABBED",  // its 7-tab strip needs 1424px
//       "Login.tsx": null,                    // not a routed page at all
//     }
//
// so "Settings is allowed to differ" is a line someone wrote on purpose and
// can read back, not a hole a page fell through. A page not listed there gets
// no latitude.
//
// The rule's one structural blind spot is what it is HANDED. It returns `{}`
// for anything outside src/pages/*.tsx, and inside such a file it recognises
// the page component only as the default export or an export named after the
// file (`Fleet` / `FleetPage` in Fleet.tsx). A routed page that is neither — a
// Reports.tsx exporting `function ReportsView()`, or a routed component living
// outside src/pages — would ship with its own width and gap and this rule would
// say nothing at all. src/lib/uiConventions.test.ts closes that gap: its
// "page-uses-page-shell sees every routed page" block reads the REAL router,
// resolves every routed element back to its file, and fails when that file is
// not one this rule would visit and recognise. (This citation previously named
// a test that had never been written; it now names one that has, and that has
// been seen to fail on exactly the shape above.)
// ---------------------------------------------------------------------------
import { baseUtility, escapeHatch, getAttr, hasException } from "./helpers.js";

const RULE_ID = "page-uses-page-shell";

const DEFAULT_SHELL = "PAGE_SHELL";

/**
 * `flex flex-col gap-10 max-w-6xl` written out by hand instead of imported.
 *
 * All three parts have to be the PAGE-scale versions, or this fires on every
 * small stacked column in a settings row. Calibrated against the real tree:
 *   * `max-w-*xl` only — a CONTENT COLUMN cap. Every one of the ten original
 *     page roots was max-w-3xl / max-w-5xl / max-w-6xl. `max-w-40`,
 *     `max-w-xs`, `max-w-full` are element widths and are left alone (four
 *     such columns exist in Settings.tsx today, and none of them is a page).
 *   * a Card-rhythm gap (>= gap-5, i.e. 20px). The two real values in the
 *     survey were gap-6 (24px) and gap-10 (40px); `gap-1`/`gap-2` is a label
 *     stack, not a page.
 */
const PAGE_WIDTH_CAP = /^max-w-\d*xl$/;
const CARD_RHYTHM_GAP = /^gap(?:-x|-y)?-(\d+)$/;

function isHandRolledShell(tokens) {
  // Matched on the BASE utility, so a variant prefix cannot hide a token from
  // the patterns: they are ^-anchored, and `md:max-w-6xl` therefore missed
  // PAGE_WIDTH_CAP entirely. helpers.js exports baseUtility for exactly this and
  // every sibling rule already uses it.
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
        "The `{{component}}` page's root element does not use the shared page shell. Return `<div className={{{shell}}}>` from lib/pageShell.ts — one app-wide max-width and one Card gap, so this page cannot drift the way the original ten did.{{hatch}}",
      handRolled:
        "This element hand-rolls the page shell in literal classes (`{{tokens}}`). Import PAGE_SHELL from lib/pageShell.ts instead — a second literal copy of the width and gap is exactly how five different page widths accumulated.{{hatch}}",
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

    /** Every function in this file that could be THE page component. */
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
        // `export default Fleet;` — the function is declared elsewhere in the
        // file. Dropping it here meant the whole page went unchecked, which is
        // the same silent pass the arrow-body gap produced.
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
      // An arrow with an EXPRESSION body has no ReturnStatement at all, so the
      // walk below found nothing and the component was skipped in silence — a
      // page written as `const Fleet = () => (<div>…</div>)` was never checked.
      // Synthesised here as the return it is.
      if (fn.type === "ArrowFunctionExpression" && fn.body?.type !== "BlockStatement") {
        return [{ type: "ReturnStatement", argument: fn.body }];
      }
      (function walk(node) {
        if (!node || typeof node !== "object") return;
        if (Array.isArray(node)) return node.forEach(walk);
        if (node.type === "ReturnStatement") out.push(node);
        // Do not descend into a nested function — its returns are not the
        // page's root (a row renderer, a useMemo callback, an event handler).
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

    function usesShell(returnStatement) {
      const arg = returnStatement.argument;
      if (!arg || arg.type !== "JSXElement") return false;
      const attr = getAttr(arg, "className");
      const v = attr?.value;
      if (!v || v.type !== "JSXExpressionContainer") return false;
      return v.expression.type === "Identifier" && v.expression.name === requiredShell;
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
