// A cadence outside the builder grammar, such as a raw cron expression, parses
// to mode "cron" and is emitted unchanged, so saving never destroys it.
import { describe, expect, it } from "vitest";
import { buildCadenceString, formatCadence, parseCadenceString } from "./CadenceBuilder";
import { countText, en, type TranslationKey } from "../lib/i18n";

const t = (key: TranslationKey, n?: number): string => countText(en[key], "en", n);

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

  it("preserves an unrecognized string", () => {
    // The backend rejects this on save; the cron editor shows it with an inline
    // error.
    const s = parseCadenceString("not-a-cadence");
    expect(s.mode).toBe("cron");
    expect(s.cron).toBe("not-a-cadence");
    expect(buildCadenceString(s)).toBe("not-a-cadence");
  });
});

describe("builder grammar round-trip", () => {
  it.each(["off", "daily 02:00", "weekly Mon,Fri 03:30", "everyN 3 04:00"])(
    "round-trips %j",
    (raw) => {
      expect(buildCadenceString(parseCadenceString(raw))).toBe(raw);
    }
  );

  it("maps empty to off", () => {
    expect(parseCadenceString("").mode).toBe("off");
    expect(parseCadenceString("  ").mode).toBe("off");
  });
});

describe("formatCadence", () => {
  it("shows a raw cron cadence verbatim with a cron prefix", () => {
    expect(formatCadence("0 */6 * * *", t, "en")).toBe("cron: 0 */6 * * *");
  });

  it("renders the builder modes as prose", () => {
    expect(formatCadence("daily 04:00", t, "en")).toBe("daily at 4:00");
    expect(formatCadence("off", t, "en")).toBe("");
  });
});

// The weekday label is formatted from a Date built with Date.UTC, which is
// midnight UTC. Formatted in the viewer's zone, it falls on the previous day
// anywhere west of UTC.
describe("weekday labels are zone-independent", () => {
  const days: [string, string][] = [
    ["Mon", "Mon"],
    ["Tue", "Tue"],
    ["Wed", "Wed"],
    ["Thu", "Thu"],
    ["Fri", "Fri"],
    ["Sat", "Sat"],
    ["Sun", "Sun"],
  ];

  it.each(days)("names %s as %s regardless of the viewer's timezone", (stored, label) => {
    // Rather than depend on the machine's zone, check that the formatter is
    // asked for UTC, which is what makes every zone agree.
    const seen: Intl.DateTimeFormatOptions[] = [];
    const real = Intl.DateTimeFormat;
    // @ts-expect-error a plain function stands in for the constructor
    Intl.DateTimeFormat = function (loc: string, opts: Intl.DateTimeFormatOptions) {
      seen.push(opts);
      return new real(loc, opts);
    };
    try {
      expect(formatCadence(`weekly ${stored} 02:00`, t, "en")).toBe(`weekly (${label}) at 2:00`);
    } finally {
      Intl.DateTimeFormat = real;
    }
    expect(seen.length).toBeGreaterThan(0);
    for (const opts of seen) {
      expect(opts.timeZone).toBe("UTC");
    }
  });
});
