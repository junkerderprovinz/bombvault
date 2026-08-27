// ---------------------------------------------------------------------------
// no-em-dash-in-user-text -- the house prose rule, enforced.
//
// "No em dash (U+2014) in text a user reads. The four Cyrillic-script locales
//  (ru, uk, bg, sr) are exempt."
//
// This header is written without em dashes on purpose. Comments are exempt from
// the rule (see WHAT IS NEVER CHECKED below) and every sibling file here uses
// the mark freely, so this is a choice rather than a requirement. It is made
// because a rule whose own explanation is the densest patch of the thing it
// forbids is a rule nobody quite believes in.
//
// WHY IT EXISTS. The convention had no gate at all, and the cost of that showed
// up three times in one session: three separate "fix the em dashes" rounds,
// each of which found more, because each one was a person reading files and
// each one stopped when the reader ran out of files rather than when the tree
// ran out of dashes. The sweep that finally cleared it removed roughly 4,900 em
// dashes from user-facing strings across 42 languages. Nothing prevented the
// next one being typed the following afternoon.
//
// Those three rounds are the whole argument. This is not a convention anyone
// disputes. It is a convention that is invisible at the moment of writing,
// because an em dash looks perfectly ordinary inside the sentence you are
// typing and only looks like a violation to someone auditing 42 files at once.
// That is exactly the shape of defect a linter is good at and a reviewer is not.
//
// WHY A LINT RULE AND NOT A GREP, measured rather than asserted. `src/**/*.ts*`
// contains 4,708 em dashes spread over 180 of its 181 files. Exactly 3 of them
// reach a user. Every other one is prose: this codebase's comment blocks are
// longer than its code, the sibling rules' headers say so, and those headers
// are themselves written in em dashes. A text scanner returns 4,708 hits at a
// 0.06% signal rate, which is not a check, it is a haystack. A parser is never
// handed a comment at all, so the same question has 3 answers and no noise.
//
// Two things a parser also gets for free. A string written with the escape
// sequence for U+2014 is read as an em dash, because the rule tests the cooked
// value and not the source text. And the HTML entity spellings React decodes at
// render time (&mdash;, &#8212;, &#x2014;) are checked in JSX, which is the only
// place they would actually render. Neither appears in the tree today; both are
// the obvious way this rule gets routed around otherwise.
//
// ---------------------------------------------------------------------------
// WHERE THE BOUNDARY IS
// ---------------------------------------------------------------------------
//
// 1. THE TRANSLATION TABLES: `src/lib/locales/*.ts` and `src/lib/i18n.ts`.
//    This is the target, and it needs no heuristic at all. A value in a
//    translation table is by construction a sentence shown to a person. There
//    is no enum value, no CSS token and no id in those files to confuse with
//    one, which is what forced the sibling rule (user-message-is-translated)
//    into its capital-plus-space test and is simply not needed here. Every
//    string literal and every template chunk in those files is checked, with
//    one exception: an object key. `"dashboard.title"` is an i18n key, read by
//    the lookup and never by a human.
//
//    Why file-scoped rather than "every string literal in src": because that is
//    the version that gets switched off. Outside the tables the string literals
//    in this tree are class lists, API paths, storage keys, router paths and
//    test fixtures, and a rule that has to guess which of those a person reads
//    is precisely the noisy rule that lint-rules/README.md's "what is not
//    checked" section exists to refuse. Inside the tables the guess never
//    arises.
//
// 2. THE CYRILLIC EXEMPTION: ru, uk, bg, sr, keyed off the locale file's own
//    name. This is not an oversight waiting to be tidied up. In Russian,
//    Ukrainian, Bulgarian and Serbian typography the em dash is ordinary,
//    load-bearing punctuation: it stands in for the absent copula, as in
//    "Мой брат — врач", and taking it out does not make the sentence plainer,
//    it makes it ungrammatical. Those four files hold 561 em dashes inside
//    strings right now and every one of them is correct. The house rule is
//    about an English typographic habit leaking into UI copy. It was never
//    about those four languages, and a rule that flagged them would be teaching
//    a translator to write badly.
//
//    The size of that is worth stating, because it is the whole reason the
//    exemption cannot be left to a reviewer's memory: dropping the four codes
//    out of EXEMPT_LOCALES turns a clean tree into 538 errors on the next run,
//    all of them wrong. It is checked here, once, against the file's own name,
//    so it cannot drift out of step with a later sweep.
//
//    Note what the exemption is NOT. It is not "Cyrillic script", it is four
//    named locales: `sr` is on the list because Serbian uses the mark this way
//    in both of its scripts, and the test is the file's locale code, never the
//    characters inside the string.
//
// 3. JSX TEXT: checked, and this is a deliberate widening past where
//    user-message-is-translated drew its line. That rule declined JSX text on a
//    stated false-positive cost: an ordinary JSX string is often a CSS class or
//    a technical value, so "is this hardcoded English copy" cannot be settled
//    from the node. The question THIS rule asks is a different one and does not
//    inherit that cost. An em dash in rendered JSX text is a mark a person sees,
//    whatever the surrounding string is "about". There is no such thing as an em
//    dash that renders on screen and is not user-facing.
//
//    Measured before widening, in the sibling's own style. Enabling this over
//    the clean post-sweep tree produces 3 findings and all 3 are real:
//    `pages/Containers.tsx` joining a timestamp to a result inside a <p>,
//    `pages/Recovery.tsx` joining a date to a snapshot id inside an <option>,
//    and `pages/Recovery.tsx` prefixing an error string inside a <span>. Zero
//    false positives. Those three are also the ones the locale sweep could not
//    structurally have found: they are separators living in the components, not
//    values living in the tables, so no amount of care over the 42 locale files
//    would ever have reached them. Three real defects at zero noise is the case
//    for widening that user-message-is-translated asked a future round to bring.
//
// 4. `title`, `aria-label`, `placeholder`, `alt`: checked, same reasoning.
//    These four carry text a person reads or a screen reader speaks. The
//    sibling's worry about `title` holding a technical value is real for "is
//    this untranslated copy" and irrelevant for "does this contain an em dash",
//    because a technical value has no business containing one either. Zero
//    findings on the clean tree, so this costs nothing today and closes the door
//    that checking JSX text alone would leave open.
//
// 5. NOT the message sinks, `push()` and `setError()`. Not an omission:
//    user-message-is-translated already reports ANY string literal in those
//    positions, em dash or not, so a second rule firing on the same line adds a
//    second squiggle and no information. The fix for both is the same one, and
//    it moves the sentence into the tables, where check 1 covers it.
//
// WHAT IS NEVER CHECKED: a comment. Comments are not user-facing and this
// codebase's are enormous (see the 4,708 above, and see the header of every
// rule in this directory, each written in the very mark it describes). The rule
// reads the AST, so this is not a special case it has to implement, it is a
// property of never being handed a comment in the first place. It is stated
// here because it is the single most important fact about the rule's scope, and
// because the by-hand sweep that preceded it had to special-case comment lines
// and got that wrong before it got it right.
//
// KNOWN LIMIT. A sentence assembled at runtime is only partly visible. The
// literal chunks of a template literal ARE checked, so `{a} — {b}` is caught,
// but a separator arriving from the server or from a variable this rule cannot
// resolve is not. Same rule as everywhere else in this directory: a value it
// cannot see reads as unknown, and unknown is never a violation.
//
// THE FIX IS NOT A SEARCH AND REPLACE, and the messages below say so, because
// mechanical substitution is how some of the damage in the earlier rounds got
// made. An em dash joining a clause to the one before it wants a comma. One
// introducing an explanation that stands on its own wants a full stop and a new
// sentence. One before a list or a definition wants a colon. One around an aside
// wants parentheses. ja, ko, zh and th never take a spaced hyphen as a
// substitute, they take their own script's punctuation. ar, he and fa take their
// own comma and no directional control characters. In Hindi the sentence break
// is the danda. And every placeholder ({path}, {count}) has to come through the
// edit spelled exactly as it went in.
// ---------------------------------------------------------------------------
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

/**
 * The locales whose typography uses the em dash as ordinary punctuation. In
 * these four it stands in for an absent copula and carries grammatical weight,
 * so "removing" it produces a broken sentence rather than a plainer one.
 */
const EXEMPT_LOCALES = new Set(["ru", "uk", "bg", "sr"]);

/** Attributes whose value a person reads or a screen reader speaks. */
const TEXT_ATTRIBUTES = new Set(["title", "aria-label", "placeholder", "alt"]);

/** A short, recognisable slice of the offending text, for the message. */
function excerpt(value) {
  const flat = value.replace(/\s+/g, " ").trim();
  return flat.length > 60 ? `${flat.slice(0, 57)}...` : flat;
}

/** Is this literal an object KEY (`"dashboard.title":`) rather than a value? */
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
      // Every value in the table, object keys excepted. No sentence heuristic is
      // needed or wanted here: in these files, a string IS a user message.
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
