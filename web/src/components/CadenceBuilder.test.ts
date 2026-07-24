// ---------------------------------------------------------------------------
// Cadence string round-trip (#107): a raw cron cadence loaded from settings
// must survive the builder untouched — parse to mode "cron" with the value
// preserved, and re-emit EXACTLY the same string. The old behavior mapped
// anything unrecognized to "off", which silently destroyed a stored cron
// schedule on the next save.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { buildCadenceString, formatCadence, parseCadenceString } from "./CadenceBuilder";
import { en, type TranslationKey } from "../lib/i18n";

const t = (key: TranslationKey): string => en[key];

describe("raw cron round-trip", () => {
  it.each(["0 */6 * * *", "30 2 * * 1-5", "0 3 1 * *", "15 4 * * MON-FRI"])(
    "parses %j to cron mode and re-emits it unchanged",
    (raw) => {
      const s = parseCadenceString(raw);
      expect(s.mode).toBe("cron");
      expect(s.cron).toBe(raw);
      expect(buildCadenceString(s)).toBe(raw);
    }
  );

  it("preserves even an unrecognized string instead of destroying it", () => {
    // The backend would reject this on save — but the builder must never eat
    // a stored value; the cron editor shows it with an inline error instead.
    const s = parseCadenceString("not-a-cadence");
    expect(s.mode).toBe("cron");
    expect(s.cron).toBe("not-a-cadence");
    expect(buildCadenceString(s)).toBe("not-a-cadence");
  });
});

describe("builder-grammar round-trip stays intact", () => {
  it.each(["off", "daily 02:00", "weekly Mon,Fri 03:30", "everyN 3 04:00"])(
    "round-trips %j",
    (raw) => {
      expect(buildCadenceString(parseCadenceString(raw))).toBe(raw);
    }
  );

  it("still maps empty to off", () => {
    expect(parseCadenceString("").mode).toBe("off");
    expect(parseCadenceString("  ").mode).toBe("off");
  });
});

describe("formatCadence", () => {
  it("shows a raw cron cadence verbatim with a cron prefix", () => {
    expect(formatCadence("0 */6 * * *", t, "en")).toBe("cron: 0 */6 * * *");
  });

  it("still renders the builder modes as prose", () => {
    expect(formatCadence("daily 04:00", t, "en")).toBe("daily at 4:00");
    expect(formatCadence("off", t, "en")).toBe("");
  });
});
