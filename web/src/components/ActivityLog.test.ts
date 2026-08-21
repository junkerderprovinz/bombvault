// ---------------------------------------------------------------------------
// colorFor — the activity log's status → colour mapping (issue #164).
//
// Pure logic, node environment: only the exported mapping function is called,
// nothing is rendered.
//
// The bug this guards: GlimStone Phase 2 Task 7 merged "offsite" into the
// "running" arm on the premise that both mean "activity happening right now".
// activityLog.ts's finishedLineText returns status "offsite" for a FINISHED,
// successful replication ("Off-site replication done — Containers"), so that
// merge painted completed runs with the in-progress accent — and, because the
// default accent gold and the warn amber used for "info" are ~11 RGB apart,
// it also made off-site and info lines near-indistinguishable in the same log.
// ---------------------------------------------------------------------------
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
