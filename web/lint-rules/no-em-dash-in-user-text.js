// no-em-dash-in-user-text: no em dash (U+2014) in text a user reads. The
// locales ru, uk, bg and sr are exempt.
//
// An em dash looks ordinary in the sentence being typed and only stands out
// across all 42 locale files at once, which a linter sees and a reviewer does
// not. The rule reads the AST, so it never sees a comment, it tests the cooked
// value (an escaped U+2014 counts), and in JSX it also catches the entities
// React decodes (&mdash;, &#8212;, &#x2014;).
//
// What is checked:
//
// 1. The translation tables, `src/lib/locales/*.ts` and `src/lib/i18n.ts`:
//    every string and template chunk except object keys, which are lookup keys
//    like "dashboard.title". No sentence heuristic is needed, because every
//    value there is shown to a person. Outside the tables string literals are
//    class lists, API paths, storage keys and fixtures, and a rule that had to
//    guess which of them a person reads would be too noisy to keep.
//
// 2. The exemption for ru, uk, bg and sr, which goes by the file's locale code
//    rather than by script. In Russian, Ukrainian, Bulgarian and Serbian (in
//    both of its scripts) the em dash stands in for the absent copula, and
//    removing it makes the sentence ungrammatical.
//
// 3. JSX text. user-message-is-translated stays away from JSX text because a
//    string there may be a class or a technical value, but an em dash that
//    renders is user-facing whatever the string is.
//
// 4. `title`, `aria-label`, `placeholder` and `alt`: text a person reads or a
//    screen reader speaks.
//
// push() and setError() are left to user-message-is-translated, which reports
// any string literal there; its fix moves the sentence into the tables, where
// check 1 covers it.
//
// A sentence assembled at runtime is only partly visible: the literal chunks
// of a template are checked, but a separator from the server or from a
// variable the rule cannot resolve is not.
//
// The replacement depends on the language and on what the dash joins, so there
// is no autofix and the messages spell out the choices.
import { escapeHatch, hasException } from "./helpers.js";

const RULE_ID = "no-em-dash-in-user-text";

/** U+2014, written as an escape so this line cannot itself trip a text scan. */
const EM_DASH = "\u2014";

/**
 * The spellings React decodes into an em dash at render time. Only meaningful
 * in JSX, where the parser hands over the raw source run and the browser does
 * the decoding. A plain `.ts` string containing "&mdash;" is just those seven
 * characters and is left alone.
 */
const EM_DASH_ENTITY = /&(?:mdash|#8212|#[xX]2014);/;

/**
 * A translation table: `src/lib/locales/<code>.ts`, or `src/lib/i18n.ts`, which
 * holds the `en` and `de` tables inline as the source of truth. Capture group 1
 * is the locale code, and is undefined for i18n.ts. Both path separators,
 * because `context.filename` comes back backslashed on Windows.
 */
const TRANSLATION_TABLE = /(?:^|[\\/])src[\\/]lib[\\/](?:i18n\.ts|locales[\\/]([a-z][a-z-]*)\.ts)$/;

/** The locales whose typography uses the em dash as ordinary punctuation. */
const EXEMPT_LOCALES = new Set(["ru", "uk", "bg", "sr"]);

/** Attributes whose value a person reads or a screen reader speaks. */
const TEXT_ATTRIBUTES = new Set(["title", "aria-label", "placeholder", "alt"]);

/** A short, recognisable slice of the offending text, for the message. */
function excerpt(value) {
  const flat = value.replace(/\s+/g, " ").trim();
  return flat.length > 60 ? `${flat.slice(0, 57)}...` : flat;
}

/** Is this literal an object key (`"dashboard.title":`) rather than a value? */
function isObjectKey(node) {
  const parent = node.parent;
  return parent?.type === "Property" && parent.key === node && !parent.computed;
}

export default {
  meta: {
    type: "problem",
    docs: {
      description:
        "No em dash (U+2014) in text a user reads. ru, uk, bg and sr are exempt: there it is ordinary punctuation.",
    },
    schema: [],
    messages: {
      inTranslation:
        'Em dash in a translated string: "{{text}}". Replace it with what this language would actually use: a comma where it joins a clause to the one before it, a full stop and a new sentence where it introduces a standalone explanation, a colon before a list or a definition, parentheses around an aside. Not a hyphen. ja/ko/zh and th take their own punctuation, ar/he/fa their own comma and no directional control characters, hi the danda. Keep every placeholder ({path}, {count}) spelled exactly as it is. ru, uk, bg and sr are exempt and must not be touched.{{hatch}}',
      inJsxText:
        'Em dash in rendered text: "{{text}}". It is on screen, so the house rule applies here exactly as it does in the locale files, and a separator hardcoded in a component is worse: the translators cannot reach it at all. Move the sentence into a t() key, or use the punctuation the two halves actually call for.{{hatch}}',
      inAttribute:
        'Em dash in `{{attr}}`: "{{text}}". A screen reader speaks this and the browser shows it on hover, which makes it user-facing text under the same rule.{{hatch}}',
    },
  },

  create(context) {
    const filename = context.filename ?? context.getFilename();
    const table = TRANSLATION_TABLE.exec(filename);
    // A locale file whose language uses the mark legitimately is not checked at
    // all. i18n.ts, whose capture group is undefined, always is.
    const checkTable = table !== null && !EXEMPT_LOCALES.has(table[1] ?? "");

    function report(node, messageId, data) {
      if (hasException(context, node, RULE_ID)) return;
      context.report({ node, messageId, data: { ...data, hatch: escapeHatch(RULE_ID) } });
    }

    /** Every literal chunk of a value written as `"x"` or as `` `x${y}z` ``. */
    function literalChunks(node) {
      if (!node) return [];
      if (node.type === "Literal" && typeof node.value === "string") return [node.value];
      if (node.type === "TemplateLiteral") return node.quasis.map((q) => q.value.cooked ?? "");
      if (node.type === "JSXExpressionContainer") return literalChunks(node.expression);
      return [];
    }

    const visitors = {
      // Rendered text between tags. `value` is the raw source run, so the
      // entity spellings the browser decodes are visible here, and only here.
      JSXText(node) {
        if (!node.value.includes(EM_DASH) && !EM_DASH_ENTITY.test(node.value)) return;
        report(node, "inJsxText", { text: excerpt(node.value) });
      },

      JSXAttribute(node) {
        const name = node.name?.type === "JSXIdentifier" ? node.name.name : "";
        if (!TEXT_ATTRIBUTES.has(name)) return;
        const offending = literalChunks(node.value).find(
          (chunk) => chunk.includes(EM_DASH) || EM_DASH_ENTITY.test(chunk)
        );
        if (offending === undefined) return;
        report(node, "inAttribute", { attr: name, text: excerpt(offending) });
      },
    };

    if (checkTable) {
      // Every value in the table except object keys; here every string is a
      // user message.
      visitors.Literal = (node) => {
        if (typeof node.value !== "string" || !node.value.includes(EM_DASH)) return;
        if (isObjectKey(node)) return;
        report(node, "inTranslation", { text: excerpt(node.value) });
      };
      visitors.TemplateLiteral = (node) => {
        for (const quasi of node.quasis) {
          const chunk = quasi.value.cooked ?? "";
          if (!chunk.includes(EM_DASH)) continue;
          report(quasi, "inTranslation", { text: excerpt(chunk) });
          return; // one finding per string, not one per chunk
        }
      };
    }

    return visitors;
  },
};
