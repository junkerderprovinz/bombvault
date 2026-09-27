import { describe, expect, it } from "vitest";
import { foreignZone, zoneLabel } from "./scheduleZone";

describe("the server's schedule clock", () => {
  it("is named only when it differs from the browser's", () => {
    expect(foreignZone({ name: "UTC", offsetSeconds: 0 }, 7200)).toEqual({ name: "UTC", offsetSeconds: 0 });
    expect(foreignZone({ name: "CEST", offsetSeconds: 7200 }, 7200)).toBeNull();
    expect(foreignZone(undefined, 7200)).toBeNull();
  });

  it("reads as an offset from UTC", () => {
    expect(zoneLabel({ name: "UTC", offsetSeconds: 0 })).toBe("UTC");
    expect(zoneLabel({ name: "CEST", offsetSeconds: 7200 })).toBe("UTC+02:00");
    const west = zoneLabel({ name: "NST", offsetSeconds: -12600 });
    expect(west.startsWith("UTC")).toBe(true);
    expect(west.slice(3)).toBe("-03:30");
    expect(zoneLabel({ name: "IST", offsetSeconds: 19800 })).toBe("UTC+05:30");
  });
});
