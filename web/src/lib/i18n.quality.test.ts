// Gaps the parity test cannot see. A value still identical to English is
// keyed and placeholdered correctly, yet untranslated, and only a reader of that
// language notices. Two en keys holding the same sentence are maintained twice
// and drift apart the first time one is edited. A German string can name a
// thing differently from the rest of the German UI. The checks fail rather than
// warn, and every exception sits on an allow-list.
import { describe, expect, it } from "vitest";
import { de, en } from "./i18n";
import { allLocales as locales } from "./localesForTests";

/**
 * SAME_AS_EN_IS_FINE lists values that are legitimately identical to English:
 * product names, units, protocol names and loanwords. It is keyed by key rather
 * than by language, because a term that stays English in Dutch usually does in
 * Danish too.
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
  "dbdump.versionLabel",
  "zfs.safety.row",

  // Protocol and product names every locale writes in Latin script. Each also
  // appears untranslated in the locales' own surrounding prose.
  "notify.webhook",
  "notify.appriseUrl",
  "notify.matrixHomeserver",
  "notify.healthchecks",
  "notify.smtp",
  "zfs.connection.version",

  // "Containers" is the word el and he use in their own nav entry. Consistency
  // within the language decides, not whether the string looks English.
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
  // Latin-script languages share much vocabulary with English. In a Cyrillic,
  // Greek, Hebrew, Arabic, CJK or Thai locale a Latin sentence is always a gap.
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
  // English alone cannot decide this. containers.restoreSelected and
  // vms.restoreSelected differ in five locales because of noun gender (Galician
  // "os seleccionados" and "as seleccionadas"), and cloud.secretSet,
  // receiver.appKeyKeep and fleet.tokenKeep differ in every other language. So
  // a pair counts only when it is identical in English and in every locale
  // that carries both; anything else is two sentences English happens to
  // collapse.
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

describe("German keeps one name for each thing", () => {
  // The German UI says Backup and Snapshot throughout. A single dialog that
  // switches to Sicherung and Schnappschuss reads like a different action.
  it("calls a snapshot a Snapshot and the cancel action a Backup", () => {
    const odd = Object.entries(de)
      .filter(([key, value]) => /Schnappschuss/.test(value) || (key.startsWith("backup.cancel") && /Sicherung/.test(value)))
      .map(([key, value]) => `${key}: ${value}`);
    expect(odd).toEqual([]);
  });
});
