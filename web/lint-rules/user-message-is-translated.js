// ---------------------------------------------------------------------------
// user-message-is-translated — settled convention 8.
//
// "A string the user reads goes through t(). No exceptions for error paths."
//
// This rule exists because the error paths were the ONE place the convention
// had quietly never held. The app ships 42 languages and a translator sweep
// runs on every feature, yet a sweep of the failure tails found 56 hardcoded
// English literals across seven files — `res.error ?? "Delete failed"`,
// `err instanceof Error ? err.message : "Failed to load VMs"`,
// `push("Failed to delete backups", "fail")`. Every one of them rendered
// English into whatever language the user had chosen, and only ever at the
// moment something had already gone wrong, which is the worst moment to hand
// someone a sentence they cannot read.
//
// They survived every previous i18n pass for a structural reason worth naming:
// a translator looks for UI COPY — labels, headings, hints, button text — and
// all of that was translated. A fallback after `??` is not copy. It is the
// thing nobody looks at because it is only reached when the server had nothing
// to say, which is rare in development and common in exactly the failure modes
// users report. Being invisible in the happy path is what made this class
// persist while everything around it was translated.
//
// WHY A LINT RULE AND NOT A GREP. The same reason the sibling rules give
// (lint-rules/index.js): this codebase's comments are longer than its code and
// they quote these very literals while explaining past rounds — Files.tsx
// carries the line `// must not leave "Failed to load file sets" up while the
// UI works`, which any text scanner would flag and any parser ignores. The
// mechanical sweep that fixed the 56 had to special-case comment lines by hand
// to avoid mangling that one. A rule reading the AST never sees prose.
//
// WHAT IT CHECKS, deliberately narrow. Only the three shapes that actually
// produced the defect, each one a position whose value is definitionally shown
// to a user:
//
//   1. `push(<literal>, …)` and `setError(<literal>)` — the toast and the
//      section-error banner, this app's two ways of telling someone something
//      went wrong.
//   2. `<anything> ?? <literal>` where the literal is a sentence — the
//      "server said nothing, say this instead" tail.
//   3. `<cond> ? <anything> : <literal>` where the literal is a sentence — the
//      `err instanceof Error ? err.message : "…"` idiom, the same tail wearing
//      a ternary.
//
// WHAT IT DOES NOT CHECK, and why the line is drawn here. It says nothing
// about JSX text, `title=`, `aria-label=` or object literals. Those are real
// i18n surface, but they are ALSO where the false positives live — an
// `aria-label` on a decorative element, a `title` carrying a technical value,
// a JSX string that is a CSS class. A rule that cries wolf on those gets a
// blanket disable and then protects nothing, which is worse than a narrow rule
// that holds. The narrow version covers 56 of 56 real defects found in the
// sweep. If a future round finds a defect this misses, widen it then, with
// that defect as the test case.
//
// SENTENCE HEURISTIC. `?? "x"` and `: "x"` are extremely common in this
// codebase for non-user values — `?? ""`, `?? "all"`, `?? "graceful"`,
// `: "rotate-90"`, `?? "0"`. So a literal in those two positions only counts
// as a user message when it LOOKS like one: it starts with a capital letter
// and contains a space. That admits "Delete failed" and "Could not load
// current settings" while ignoring every enum value, id, class name and CSS
// token in the tree. `push()`/`setError()` need no heuristic — their argument
// is a user message by construction, so ANY string literal there is a finding.
// ---------------------------------------------------------------------------
import { escapeHatch, hasException } from "./helpers.js";

const RULE_ID = "user-message-is-translated";

/** Functions whose first argument IS, by definition, shown to a user. */
const MESSAGE_SINKS = new Set(["push", "setError"]);

/**
 * Does this literal read like a sentence a person is meant to understand,
 * rather than an enum value, id, or CSS token?
 *
 * Capital + space is the whole test, and it is calibrated against the real
 * tree rather than chosen for tidiness: every one of the 56 defects passes it
 * ("Delete failed", "Network error", "Failed to load VMs"), and every `??`/`:`
 * string fallback that is NOT a user message fails it — `""`, `"all"`,
 * `"graceful"`, `"local"`, `"rotate-90"`, `"0"`, `"name"`. A one-word user
 * message ("Failed", which the sweep also found) slips through, and that is
 * accepted: tightening to catch it would have to drop the space test, which
 * immediately catches every PascalCase identifier in the file.
 */
function looksLikeSentence(value) {
  return typeof value === "string" && /^[A-Z]/.test(value) && value.includes(" ");
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
        'Hardcoded user-facing string "{{text}}". Wrap it in t("…") and add the key to web/src/lib/i18n.ts and to every file in web/src/lib/locales/ — this app ships 42 languages, and an untranslated string on an error path is the one a user sees at the worst possible moment.{{hatch}}',
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
        if (!MESSAGE_SINKS.has(calleeName(node))) return;
        const first = node.arguments[0];
        if (isStringLiteral(first)) report(first, first.value);
      },

      // 2. res.error ?? "…"
      LogicalExpression(node) {
        if (node.operator !== "??") return;
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
