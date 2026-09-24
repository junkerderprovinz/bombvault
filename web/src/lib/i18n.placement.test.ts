// Catches what the parity and quality guards do not: an English sentence left
// in a Latin-script locale (the non-Latin check in i18n.quality.test.ts skips
// those), a term that drifts between two placement sentences in the same
// language, and the 3-2-1 phrasing dropped from a translation.
import { describe, expect, it } from "vitest";
import { en } from "./i18n";
import { allLocales } from "./localesForTests";

type Key = keyof typeof en;

const PREFIXES = [
  "placement.",
  "placementDefaults.",
  "placementCode.",
  "directRepo.",
  "newTarget.",
  "offsiteRemoval.",
  "saveWarning.",
  "timeline.",
  "discover.",
];

const SINGLE_KEYS = new Set<string>([
  "repos.offPremises",
  "repos.offPremisesHint",
  "repos.directOf",
  "repos.companionLost",
  "repos.mirroredLocked",
  "offsite.alsoDirect",
  "offsite.directRetentionAsk",
  "offsite.directAppendOnlyAsk",
  "ransomware.replicationPaused",
  "settingsIO.previewCopyRules",
  "settingsIO.previewNotInFile",
]);

// placement.rule321NothingOff does not exist: every target counts as a site
// of its own, so with two counting places one is always off the premises,
// leaving no path that reaches that message.
const RULE_321: Key[] = [
  "placement.rule321Met",
  "placement.rule321OneCopy",
  "placement.rule321Unconfirmed",
];

const placementKeys = (Object.keys(en) as Key[]).filter(
  (k) => SINGLE_KEYS.has(k) || PREFIXES.some((p) => k.startsWith(p)),
);

// Four words or more are a sentence, and no language shares a whole sentence
// with English. Shorter labels such as "Local" or "Host" can be the same word.
function isSentence(value: string): boolean {
  const words = value
    .replace(/\{\w+\}/g, " ")
    .split(/\s+/)
    .filter((w) => /\p{L}/u.test(w));
  return words.length >= 4;
}

const translated = Object.entries(allLocales).filter(([code]) => code !== "en");

describe("placement texts", () => {
  it("are found by their prefixes", () => {
    expect(placementKeys).toContain("placement.title");
    expect(placementKeys).toContain("timeline.check");
  });

  it.each(translated)("%s translates every placement sentence", (_code, table) => {
    const english = placementKeys.filter((k) => isSentence(en[k]) && table[k] === en[k]);
    expect(english).toEqual([]);
  });

  it.each(Object.entries(allLocales))("%s writes 3-2-1 as it is", (_code, table) => {
    for (const key of RULE_321) {
      expect(table[key], key).toContain("3-2-1");
    }
  });

  it("keeps off-site and append-only in German as the rest of the interface does", () => {
    for (const key of placementKeys) {
      if (/off-site/i.test(en[key])) expect(allLocales.de[key], key).toMatch(/off-site/i);
      if (en[key].includes("append-only")) expect(allLocales.de[key], key).toContain("append-only");
    }
  });

  it("uses the same delete verb in Lithuanian as every other delete button", () => {
    const verb = allLocales.lt["common.delete"];
    expect(allLocales.lt["offsiteRemoval.delete"]).toContain(verb);
    expect(allLocales.lt["timeline.deleteRow"]).toContain(verb);
  });

  it("names the target the Lithuanian off-site delete texts act on", () => {
    for (const key of ["offsiteRemoval.delete", "offsiteRemoval.ask", "offsiteRemoval.done"] as const) {
      expect(allLocales.lt[key], key).toMatch(/iš \{target\}/);
    }
  });

  it("names append-only the same way a locale already does in placement.droppedAppendOnly", () => {
    const keys = ["placementCode.appendOnly", "timeline.deleteSkipped", "offsiteRemoval.appendOnly"] as const;
    for (const [code, table] of Object.entries(allLocales)) {
      if (code === "en") continue;
      const reference = table["placement.droppedAppendOnly"];
      if (!reference?.includes("append-only")) continue;
      for (const key of keys) {
        expect(table[key], `${code} ${key}`).toContain("append-only");
      }
    }
  });
});
