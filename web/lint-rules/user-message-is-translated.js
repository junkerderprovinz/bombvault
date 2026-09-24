// user-message-is-translated: a string the user reads goes through t(), error
// paths included.
//
// Error paths are where this slips. A fallback such as
// `res.error ?? "Delete failed"` is only reached when the server says nothing,
// so it shows English in every language at the moment something has gone
// wrong.
//
// Three shapes are checked, each a position whose value is shown to a user:
//
//   1. `push(<literal>, …)` and `setError(<literal>)`, the toast and the
//      section error banner.
//   2. `<anything> ?? <literal>` (or `||`) where the literal is a sentence.
//   3. `<cond> ? <anything> : <literal>` where the literal is a sentence, as in
//      `err instanceof Error ? err.message : "…"`.
//
// JSX text, `title=`, `aria-label=` and object literals are not checked. They
// are i18n surface too, but also where the false positives are (a `title` with
// a technical value, a JSX string that is a class), and a rule that cries wolf
// gets disabled.
//
// `?? "x"` and `: "x"` are common for values no user reads (`?? ""`,
// `?? "all"`, `: "rotate-90"`), so in those positions a literal only counts
// when it starts with a capital letter and contains a space. push() and
// setError() need no heuristic: any string literal there is a finding.
import { escapeHatch, hasException } from "./helpers.js";

const RULE_ID = "user-message-is-translated";

/** Functions whose first argument is shown to a user. */
const MESSAGE_SINKS = new Set(["push", "setError"]);

/**
 * Does this literal read like a sentence rather than an enum value, id or CSS
 * token? Capital plus space admits "Delete failed" and "Failed to load VMs"
 * and rejects `""`, `"all"`, `"rotate-90"` and `"name"`. A one-word message
 * such as "Failed" slips through; dropping the space test to catch it would
 * flag every PascalCase identifier.
 */
function looksLikeSentence(value) {
  return typeof value === "string" && /^[A-Z]/.test(value) && value.includes(" ");
}

/**
 * Does a template's static text carry prose once the interpolations are gone?
 * looksLikeSentence does not fit, because template text often starts
 * mid-sentence: `${ok} ok, ${fail} failed` leaves "ok,  failed", with no
 * capital. Two letters in a row count as language, while `${a}/${b}` and
 * `${n} %` leave only punctuation and are formatting.
 */
function templateReadsAsProse(staticText) {
  return typeof staticText === "string" && /[A-Za-z]{2,}/.test(staticText);
}

function isStringLiteral(node) {
  return node?.type === "Literal" && typeof node.value === "string";
}

/** The callee's plain name for `foo(…)` and `obj.foo(…)` alike. */
function calleeName(node) {
  const callee = node.callee;
  if (callee?.type === "Identifier") return callee.name;
  if (callee?.type === "MemberExpression" && callee.property?.type === "Identifier") {
    return callee.property.name;
  }
  return "";
}

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "A string the user reads goes through t(), including the fallback on an error path.",
    },
    schema: [],
    messages: {
      hardcoded:
        'Hardcoded user-facing string "{{text}}". Wrap it in t("…") and add the key to web/src/lib/i18n.ts and to every file in web/src/lib/locales/. An untranslated string on an error path is the one a user sees at the worst possible moment.{{hatch}}',
    },
  },

  create(context) {
    function report(node, text) {
      if (hasException(context, node, RULE_ID)) return;
      context.report({
        node,
        messageId: "hardcoded",
        data: { text, hatch: escapeHatch(RULE_ID) },
      });
    }

    return {
      // 1. push("…", "fail") / setError("…")
      CallExpression(node) {
        const callee = calleeName(node);
        if (!MESSAGE_SINKS.has(callee)) return;
        // `push` is also Array.prototype.push. The toast's push always takes
        // (message, tone) and `seen.push(x)` takes one argument, so the tone
        // tells them apart without type information.
        if (callee === "push" && node.arguments.length < 2) return;
        const first = node.arguments[0];
        if (isStringLiteral(first)) {
          report(first, first.value);
          return;
        }
        // A template counts too, judged on its static text, so pure formatting
        // such as `${a}/${b}` passes.
        if (first?.type === "TemplateLiteral") {
          const staticText = first.quasis.map((q) => q.value.cooked ?? "").join(" ").trim();
          if (templateReadsAsProse(staticText)) report(first, staticText);
        }
      },

      // 2. res.error ?? "…", or the same fallback written with ||.
      LogicalExpression(node) {
        if (node.operator !== "??" && node.operator !== "||") return;
        if (isStringLiteral(node.right) && looksLikeSentence(node.right.value)) {
          report(node.right, node.right.value);
        }
      },

      // 3. err instanceof Error ? err.message : "…"
      ConditionalExpression(node) {
        for (const branch of [node.consequent, node.alternate]) {
          if (isStringLiteral(branch) && looksLikeSentence(branch.value)) {
            report(branch, branch.value);
          }
        }
      },
    };
  },
};
