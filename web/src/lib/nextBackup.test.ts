// ---------------------------------------------------------------------------
// nextBackup — tests for the "next backup" summary cell's pure selection.
//
// #186 (manilx) has two halves, and both are regression-guarded below. The
// reported half: every domain switched on with no cadence of its own, the whole
// server backed up nightly by the "Backup Everything" pass, and the cell saying
// "not scheduled" while the Settings card two clicks away said "Daily at 05:00".
// The mirror half: every domain on a weekly cadence and the pass running daily,
// where the cell named Sunday although the real next backup was that night.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { nextBackupCadence } from "./nextBackup";
import type { DomainStatus } from "./api";

// domain builds a DomainStatus carrying only the three fields
// nextBackupCadence actually reads. The rest of the interface (RPO ages, drill
// outcomes, the ransomware scorecard) has no bearing on which cadence fires
// soonest, and the cast already waives type completeness, so spelling those
// fields out would be ceremony that has to be maintained for nothing.
function domain(over: Partial<DomainStatus>): DomainStatus {
  return { domain: "containers", enabled: true, schedule: "off", ...over } as DomainStatus;
}

describe("nextBackupCadence", () => {
  it("returns null when nothing is scheduled at all", () => {
    expect(nextBackupCadence([], "")).toBeNull();
    expect(nextBackupCadence([domain({}), domain({ domain: "vms" })], "off")).toBeNull();
  });

  it("picks a domain's own cadence and does not claim the pass covers it", () => {
    expect(nextBackupCadence([domain({ schedule: "daily 03:00" })], "")).toEqual({
      cadence: "daily 03:00",
      viaEverything: false,
    });
  });

  // #186, reported half. Before the fix this returned null and the cell rendered
  // "Not scheduled" for a server backed up every night.
  it("names the pass when no domain has a cadence of its own", () => {
    const domains = [
      domain({ domain: "containers" }),
      domain({ domain: "vms" }),
      domain({ domain: "flash" }),
    ];
    expect(nextBackupCadence(domains, "daily 05:00")).toEqual({
      cadence: "daily 05:00",
      viaEverything: true,
    });
  });

  // #186, mirror half. The backend deliberately leaves coveredBy empty here, so
  // this case is only reachable through the separate everythingSchedule field.
  it("names the pass when it fires sooner than every domain's own cadence", () => {
    const domains = [
      domain({ domain: "containers", schedule: "weekly Sun 04:00" }),
      domain({ domain: "vms", schedule: "weekly Sun 04:00" }),
    ];
    expect(nextBackupCadence(domains, "daily 05:00")).toEqual({
      cadence: "daily 05:00",
      viaEverything: true,
    });
  });

  it("keeps a domain's own cadence when that one fires sooner", () => {
    const domains = [
      domain({ domain: "containers", schedule: "daily 02:00" }),
      domain({ domain: "vms", schedule: "weekly Sun 04:00" }),
    ];
    expect(nextBackupCadence(domains, "daily 05:00")).toEqual({
      cadence: "daily 02:00",
      viaEverything: false,
    });
  });

  it("prefers the domain's own schedule on an exact tie", () => {
    // Both fire daily at 05:00. Naming the schedule the user set on the domain
    // is the less surprising of two equally true answers.
    expect(nextBackupCadence([domain({ schedule: "daily 05:00" })], "daily 05:00")).toEqual({
      cadence: "daily 05:00",
      viaEverything: false,
    });
  });

  it("ignores a pass that has no domain left to back up", () => {
    // The pass skips every domain switched off in Settings and then writes
    // nothing at all (internal/api/everything.go), so its cadence must not be
    // reported as an upcoming backup.
    expect(nextBackupCadence([domain({ enabled: false })], "daily 05:00")).toBeNull();
  });

  it("ignores a disabled domain's own cadence", () => {
    const domains = [
      domain({ domain: "containers", enabled: false, schedule: "daily 01:00" }),
      domain({ domain: "vms", schedule: "daily 06:00" }),
    ];
    expect(nextBackupCadence(domains, "")).toEqual({
      cadence: "daily 06:00",
      viaEverything: false,
    });
  });

  it("survives a server that omits everythingSchedule entirely", () => {
    // Fleet polls peer instances, which may run an older build with no such
    // field; the cell must fall back to the domain cadences rather than throw.
    expect(nextBackupCadence([domain({ schedule: "daily 03:00" })])).toEqual({
      cadence: "daily 03:00",
      viaEverything: false,
    });
    expect(nextBackupCadence([domain({})])).toBeNull();
  });

  it("handles a raw cron cadence on both sides", () => {
    // Hourly cron on a domain beats the pass's daily.
    expect(nextBackupCadence([domain({ schedule: "0 * * * *" })], "daily 05:00")).toEqual({
      cadence: "0 * * * *",
      viaEverything: false,
    });
    // And an hourly pass beats a domain's daily.
    expect(nextBackupCadence([domain({ schedule: "daily 05:00" })], "0 * * * *")).toEqual({
      cadence: "0 * * * *",
      viaEverything: true,
    });
  });
});
