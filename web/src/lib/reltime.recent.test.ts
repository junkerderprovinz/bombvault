import { describe, expect, it } from "vitest";
import { formatRecent } from "./reltime";

// Local times, so the day boundaries fall where the clock of the reader puts them.
const at = (y: number, m: number, d: number, h = 3, min = 50) => new Date(y, m - 1, d, h, min).getTime();
const NOW = at(2026, 10, 10, 12, 0);
const recent = (ms: number, lang = "en-GB") => formatRecent(ms / 1000, lang, NOW);

describe("formatRecent", () => {
  it("says today and yesterday in the language asked for, with the clock time", () => {
    expect(recent(at(2026, 10, 10))).toBe("today 03:50");
    expect(recent(at(2026, 10, 9, 23, 59))).toBe("yesterday 23:59");
    expect(recent(at(2026, 10, 10), "de")).toBe("heute 03:50");
  });

  it("names the weekday within the last week", () => {
    expect(recent(at(2026, 10, 8))).toBe("Thu 03:50");
    expect(recent(at(2026, 10, 4))).toBe("Sun 03:50");
  });

  it("gives day and month for anything older in the same year", () => {
    expect(recent(at(2026, 10, 3))).toBe("03/10 03:50");
    expect(recent(at(2026, 8, 14), "de")).toBe("14.08. 03:50");
  });

  it("gives the whole date and no time for another year", () => {
    expect(recent(at(2025, 12, 31))).toBe("31/12/2025");
  });

  it("says tomorrow and names the weekday for what is coming up", () => {
    expect(recent(at(2026, 10, 11, 3, 0))).toBe("tomorrow 03:00");
    expect(recent(at(2026, 10, 11, 3, 0), "de")).toBe("morgen 03:00");
    expect(recent(at(2026, 10, 14, 4, 0))).toBe("Wed 04:00");
    expect(recent(at(2026, 10, 20, 4, 0))).toBe("20/10 04:00");
  });
});
