// ---------------------------------------------------------------------------
// Two things parity cannot see ([338], [339]).
//
// The parity test proves every locale carries every key with the same
// placeholders. That is the structural half. Both failures below satisfy it
// completely and are still wrong:
//
//   [338] A value that is still the English string. Present, correctly keyed,
//         correctly placeholdered — and untranslated. Across 42 locales this
//         is the likeliest quiet gap, because nothing about it looks like a
//         gap: the key is there, the app renders, and only a reader of that
//         language sees it.
//
//   [339] Two English keys with the same value. Every one of those is a
//         sentence maintained in two places, and the pair drifts the moment
//         somebody edits one. It already happened: `recovery.pageTitle` was a
//         second copy of `nav.recovery`, and by the time it was found the page
//         heading said "Notfall-Wiederherstellung" while the rail beside it
//         said "Wiederherstellung".
//
// Both carry allow-lists rather than being advisory, because a guard that
// merely warns is a guard nobody reads. An entry on either list is a decision
// somebody made and wrote down.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { en } from "./i18n";
import { allLocales as locales } from "./localesForTests";

/**
 * Values that are legitimately identical to English in some languages.
 *
 * Product names, units, protocol names and loanwords adopted verbatim. This is
 * the escape hatch for "correct and identical", and it is deliberately keyed
 * by KEY rather than by language: a term that stays English in Dutch usually
 * stays English in Danish too, and listing every pair would be noise.
 */
const SAME_AS_EN_IS_FINE = new Set<string>([
  // Literal input a user types or pastes. Translating a file path or a key
  // sample would make the example wrong, not localised.
  "excludes.placeholder",
  "export.encrypt.recipientsPlaceholder",
  "fleet.mesh.dockerRun",

  // Nothing but placeholders and punctuation once the tokens are removed, so
  // there is no prose to translate.
  "activityLog.lineOther",
  "cadence.fmtCron",

  // Protocol and product names every locale writes in Latin script anyway.
  // Checked rather than assumed: each of these appears untranslated in the
  // locale's OWN surrounding prose too.
  "notify.webhook",
  "notify.appriseUrl",
  "notify.matrixHomeserver",
  "notify.healthchecks",
  "notify.smtp",

  // "Containers" is the word el and he use in their own nav entry, so it is
  // their vocabulary rather than a gap. The rule this file applies throughout:
  // consistency INSIDE the language decides, not whether the string looks
  // English from outside.
  "stack.members",
]);

/** A value nobody would translate: a unit, a number, a protocol, a symbol. */
function isUntranslatable(value: string): boolean {
  const v = value.trim();
  if (v.length <= 3) return true; // "OK", "ID", "%", "s"
  if (!/[a-z]/i.test(v)) return true; // digits, punctuation, symbols only
  // A single token with no spaces that looks like an identifier or protocol:
  // "restic", "rclone", "WebDAV", "S3", "SFTP", "BombVault".
  if (!/\s/.test(v)) return true;
  return false;
}

describe("translations are actually translated", () => {
  // Latin-script languages share vocabulary with English far more often than
  // the others, so a hit there is much likelier to be legitimate. The
  // non-Latin ones are where an untranslated string is unambiguous: a Cyrillic,
  // Greek, Hebrew, Arabic, CJK or Thai locale showing a Latin sentence is not
  // a loanword, it is a gap.
  const NON_LATIN = ["ar", "bg", "el", "fa", "he", "hi", "ja", "ko", "ru", "sr", "th", "uk", "zh"];

  it.each(NON_LATIN)("%s does not leave English sentences in place", (code) => {
    const table = locales[code as keyof typeof locales] as Record<string, string>;
    expect(table, `no table for ${code}`).toBeTruthy();
    const leaks: string[] = [];
    for (const [key, value] of Object.entries(en)) {
      if (SAME_AS_EN_IS_FINE.has(key)) continue;
      if (typeof value !== "string" || typeof table[key] !== "string") continue;
      if (table[key] !== value) continue;
      if (isUntranslatable(value)) continue;
      leaks.push(`${key}: ${JSON.stringify(value.slice(0, 60))}`);
    }
    expect(
      leaks,
      `${code} carries English sentences verbatim. Translate them, or add the key to ` +
        "SAME_AS_EN_IS_FINE with a reason if it is genuinely the same word in this language.",
    ).toEqual([]);
  });
});

describe("no sentence lives in two keys", () => {
  // English is not enough to decide this, which the first cut of this guard
  // found out immediately. It flagged two groups, and MEASURED against the
  // other 40 tables both turned out to be legitimately separate:
  //
  //   containers.restoreSelected / vms.restoreSelected  differ in 5 locales,
  //     because the noun's gender changes the agreement — Galician writes
  //     "os seleccionados" for containers and "as seleccionadas" for VMs.
  //
  //   cloud.secretSet / receiver.appKeyKeep / fleet.tokenKeep  differ in ALL
  //     40. They are identical in English by coincidence and in no other
  //     language at all.
  //
  // So the rule is not "same in English". It is "same in English AND in every
  // locale that carries both", which is the only shape that can actually
  // drift: two keys already saying different things in 40 languages are not
  // one sentence in two places, they are two sentences English happens to
  // collapse. Merging those would have flattened a distinction 40 translators
  // kept.
  const TABLES = Object.entries(locales) as [string, Record<string, string>][];

  it("keeps a genuinely shared sentence to one key", () => {
    const byValue = new Map<string, string[]>();
    for (const [key, value] of Object.entries(en)) {
      if (typeof value !== "string") continue;
      const v = value.trim();
      // Short labels repeat legitimately across unrelated controls; the drift
      // risk lives in the long, distinctive ones.
      if (v.length < 25) continue;
      byValue.set(v, [...(byValue.get(v) ?? []), key]);
    }

    const dupes: string[] = [];
    for (const [value, keys] of byValue) {
      if (keys.length < 2) continue;
      // Identical in every table that has more than one of them, or it is a
      // distinction some language is making.
      const differsSomewhere = TABLES.some(([, table]) => {
        const present = keys.map((k) => table[k]).filter((v) => typeof v === "string");
        return present.length > 1 && new Set(present).size > 1;
      });
      if (differsSomewhere) continue;
      dupes.push(`${keys.join(" + ")} -> ${JSON.stringify(value.slice(0, 70))}`);
    }

    expect(
      dupes,
      "These keys hold the same sentence in EVERY language, so it is maintained " +
        "in several places and will drift. Point one at the other.",
    ).toEqual([]);
  });
});
