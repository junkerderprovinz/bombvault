// colorFor maps an activity-log status to its text colour. "offsite" is also
// the status of a finished off-site replication, so it must not wear the
// in-progress accent of "running", and it has to stay apart from the amber of
// "info" lines, which sits about 11 RGB units from the default accent.
import { describe, expect, it } from "vitest";
import { colorFor } from "./ActivityLog";
import type { LogStatus } from "../lib/activityLog";

describe("colorFor", () => {
  it("does not paint off-site lines with the in-progress accent", () => {
    expect(colorFor("offsite")).not.toBe(colorFor("running"));
    expect(colorFor("offsite")).toBe("text-statusOffsite");
    expect(colorFor("running")).toBe("text-accentText");
  });

  it("keeps off-site distinct from every other status in the same log", () => {
    const others: LogStatus[] = ["success", "failed", "running", "info"];
    for (const status of others) {
      expect(colorFor("offsite")).not.toBe(colorFor(status));
    }
  });

  it("gives every status its own class (no two buckets share a colour)", () => {
    const all: LogStatus[] = ["success", "failed", "running", "offsite", "info"];
    expect(new Set(all.map(colorFor)).size).toBe(all.length);
  });
});
