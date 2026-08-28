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

// ---------------------------------------------------------------------------
// #183 (kramttocs): a schedule set to Wednesday was summarised as "weekly (Tue)"
// on the dashboard, one day early, while the Schedules tab showed Wed correctly.
//
// The weekday label is produced by formatting a Date built with Date.UTC, so it
// is midnight UTC. Formatting it in the VIEWER's zone shifts it backwards
// anywhere west of UTC, and midnight minus a few hours lands on the previous
// day. It was therefore wrong for the Americas and right for Europe, which is
// how it survived: it looks correct wherever it was written.
// ---------------------------------------------------------------------------
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
    // A negative UTC offset is what triggers the bug. Rather than depend on the
    // machine's zone, assert the formatter is asked for UTC explicitly, since
    // that is the property that makes every zone agree.
    const seen: Intl.DateTimeFormatOptions[] = [];
    const real = Intl.DateTimeFormat;
    // @ts-expect-error deliberately swapping the constructor for the assertion
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
