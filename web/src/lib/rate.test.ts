import { afterEach, describe, expect, it, vi } from "vitest";
import { rateHint } from "./rate";

function pageIn(lang: string) {
  vi.stubGlobal("document", { documentElement: { lang } });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("rateHint", () => {
  it("says nothing for no limit", () => {
    pageIn("en");
    expect(rateHint(0)).toBe("");
  });

  it("switches from KB/s to MB/s at 1024 KiB/s and from kbit/s to Mbit/s at one megabit", () => {
    pageIn("en");
    expect(rateHint(1)).toBe("≈ 1.0 KB/s (8.2 kbit/s)");
    expect(rateHint(500)).toBe("≈ 500.0 KB/s (4.1 Mbit/s)");
    expect(rateHint(1023)).toBe("≈ 1,023.0 KB/s (8.4 Mbit/s)");
    expect(rateHint(1024)).toBe("≈ 1.0 MB/s (8.4 Mbit/s)");
    expect(rateHint(2048)).toBe("≈ 2.0 MB/s (16.8 Mbit/s)");
    expect(rateHint(125000)).toBe("≈ 122.1 MB/s (1,024.0 Mbit/s)");
  });

  it("writes the numbers in the page's language", () => {
    pageIn("de");
    expect(rateHint(1)).toBe("≈ 1,0 KB/s (8,2 kbit/s)");
    expect(rateHint(2048)).toBe("≈ 2,0 MB/s (16,8 Mbit/s)");
    expect(rateHint(125000)).toBe("≈ 122,1 MB/s (1.024,0 Mbit/s)");
  });
});
