// ---------------------------------------------------------------------------
// forecast — tests for the pure buildForecastLine selection/formatting.
// Same stub-resolver convention as activityLog.test.ts: resolve renders
// "key a=1 b=2", which keeps the translation key AND the interpolated params
// assertable without any i18n context.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { buildForecastLine, humanBytes } from "./forecast";

const resolve = (key: string, params?: Record<string, string>): string =>
  params
    ? `${key} ${Object.entries(params)
        .map(([k, v]) => `${k}=${v}`)
        .join(" ")}`
    : key;

const GIB = 1024 * 1024 * 1024;

describe("buildForecastLine", () => {
  it("returns null when the forecast is absent (no empty shells)", () => {
    expect(buildForecastLine(null, resolve)).toBeNull();
    expect(buildForecastLine(undefined, resolve)).toBeNull();
    expect(buildForecastLine({}, resolve)).toBeNull();
  });

  it("renders growth + projection + free space for a growing repo", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: 2 * GIB, freeBytes: 40 * GIB, weeksToFull: 20 },
      resolve
    );
    expect(line).not.toBeNull();
    expect(line!.growth).toBe("dashboard.forecastGrowth bytes=2.0 GB");
    expect(line!.projection).toBe("dashboard.forecastFull weeks=20");
    expect(line!.free).toBe("dashboard.forecastFree bytes=40.0 GB");
    expect(line!.warn).toBe(false);
  });

  it("picks the shrinking wording with the unsigned magnitude for negative growth", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: -512 * 1024 * 1024, freeBytes: 40 * GIB },
      resolve
    );
    expect(line!.growth).toBe("dashboard.forecastShrink bytes=512.0 MB");
    // A shrinking repo never fills the disk — no projection, no warn.
    expect(line!.projection).toBeNull();
    expect(line!.warn).toBe(false);
  });

  it("still shows free space alone when only freeBytes is known", () => {
    const line = buildForecastLine({ freeBytes: GIB }, resolve);
    expect(line!.growth).toBeNull();
    expect(line!.projection).toBeNull();
    expect(line!.free).toBe("dashboard.forecastFree bytes=1.0 GB");
  });

  it("rounds the projection to whole weeks", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: GIB, freeBytes: 10 * GIB, weeksToFull: 10.4 },
      resolve
    );
    expect(line!.projection).toBe("dashboard.forecastFull weeks=10");
  });

  it("caps the projection at > 1 year beyond 52 weeks", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: GIB, freeBytes: 100 * GIB, weeksToFull: 104.5 },
      resolve
    );
    expect(line!.projection).toBe("dashboard.forecastFullOverYear");
    expect(line!.warn).toBe(false);
  });

  it("does not cap at exactly 52 weeks", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: GIB, freeBytes: 52 * GIB, weeksToFull: 52 },
      resolve
    );
    expect(line!.projection).toBe("dashboard.forecastFull weeks=52");
  });

  it("warns below 8 weeks and uses the count-neutral ~1 week key at the floor", () => {
    const warned = buildForecastLine(
      { growthBytesPerWeek: 4 * GIB, freeBytes: 30 * GIB, weeksToFull: 7.5 },
      resolve
    );
    expect(warned!.warn).toBe(true);
    expect(warned!.projection).toBe("dashboard.forecastFull weeks=8"); // rounds up, still warns

    const imminent = buildForecastLine(
      { growthBytesPerWeek: 10 * GIB, freeBytes: 4 * GIB, weeksToFull: 0.4 },
      resolve
    );
    expect(imminent!.warn).toBe(true);
    expect(imminent!.projection).toBe("dashboard.forecastFullOneWeek"); // floor of 1, own key
  });

  it("does not warn at exactly 8 weeks", () => {
    const line = buildForecastLine(
      { growthBytesPerWeek: GIB, freeBytes: 8 * GIB, weeksToFull: 8 },
      resolve
    );
    expect(line!.warn).toBe(false);
  });
});

describe("humanBytes", () => {
  it("formats with a binary unit and one decimal, collapsing non-positives to 0 B", () => {
    expect(humanBytes(0)).toBe("0 B");
    expect(humanBytes(-5)).toBe("0 B");
    expect(humanBytes(512)).toBe("512 B");
    expect(humanBytes(1536)).toBe("1.5 KB");
    expect(humanBytes(2 * GIB)).toBe("2.0 GB");
  });
});
