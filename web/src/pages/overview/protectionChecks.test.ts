import { describe, expect, it } from "vitest";

import { tallyEntries } from "../../lib/protectionTally";
import { domain, item } from "./overview.testsupport";
import { domainChecks, sectionsOf, worstTone, type Check, type CheckKey } from "./protectionChecks";

const calm = { copying: false, drilling: false };
const one = tallyEntries([item()]);

function check(key: CheckKey, checks: Check[]): Pick<Check, "tone" | "say"> {
  const { tone, say } = checks.find((c) => c.key === key)!;
  return { tone, say };
}

describe("the local check", () => {
  const local = (over: Parameters<typeof domain>[0], tally = one) => check("local", domainChecks(domain(over), tally, calm));

  it("is green for a current backup of every entry", () => {
    expect(local({})).toEqual({ tone: "ok", say: "overview.check.localCurrent" });
  });

  it("claims nothing where there is nothing to back up", () => {
    expect(local({}, tallyEntries([])).tone).toBe("neutral");
  });

  it("claims nothing before the first backup and without a schedule", () => {
    expect(local({ status: "never" })).toEqual({ tone: "neutral", say: "overview.check.localNever" });
    expect(local({ status: "off" })).toEqual({ tone: "neutral", say: "overview.check.localNoSchedule" });
  });

  it("warns when an entry's last run failed", () => {
    const tally = tallyEntries([item({ lastRunStatus: "failed" })]);
    expect(local({}, tally)).toEqual({ tone: "warn", say: "overview.check.localFailed" });
  });

  it("is red once the backup is overdue", () => {
    expect(local({ status: "overdue" })).toEqual({ tone: "fail", say: "overview.check.localOverdue" });
  });

  it("warns while an entry is not protected", () => {
    const tally = tallyEntries([item(), item({ key: "new", lastBackup: 0 })]);
    expect(local({}, tally)).toEqual({ tone: "warn", say: "overview.check.localPartly" });
  });

  it("warns when the next backup is due soon", () => {
    expect(local({ status: "warn" }).tone).toBe("warn");
  });
});

describe("the off-site check", () => {
  const offsite = (over: Parameters<typeof domain>[0], copying = false) =>
    check("offsite", domainChecks(domain(over), one, { copying, drilling: false }));

  it("is red without any copy off the premises", () => {
    expect(offsite({ offsiteConfigured: false })).toEqual({ tone: "fail", say: "overview.check.offsiteNone" });
  });

  it("is green when every entry already lives off the premises", () => {
    expect(offsite({ offsiteConfigured: false, offPremisesCovered: true }).tone).toBe("ok");
  });

  it("shows a copy that is running", () => {
    expect(offsite({ replicationState: "overdue" }, true)).toEqual({ tone: "run", say: "overview.check.offsiteRunning" });
  });

  it("warns about an overdue copy", () => {
    expect(offsite({ replicationState: "overdue" })).toEqual({ tone: "warn", say: "overview.check.offsiteOverdue" });
  });

  it("warns about a paused copy", () => {
    expect(offsite({ replicationState: "paused" })).toEqual({ tone: "warn", say: "overview.check.offsitePaused" });
  });

  it("claims nothing before the first copy", () => {
    expect(offsite({ replicationState: "never", lastReplicationAt: 0 }).tone).toBe("neutral");
  });

  it("is green for a copy that follows each backup", () => {
    expect(offsite({ replicationState: "" }).tone).toBe("ok");
  });
});

describe("the delete protection check", () => {
  const lock = (over: Parameters<typeof domain>[0]) => check("lock", domainChecks(domain(over), one, calm));

  it("does not apply without an off-site copy", () => {
    expect(lock({ offsiteConfigured: false, tamperState: "" }).tone).toBe("neutral");
  });

  it("is green once the far side refused a delete", () => {
    expect(lock({ tamperState: "ok" })).toEqual({ tone: "ok", say: "overview.check.lockChecked" });
  });

  it("is a gap, not a grey mark, while an off-site copy can be deleted", () => {
    expect(lock({ offsiteImmutable: false, tamperState: "" })).toEqual({ tone: "warn", say: "overview.check.lockUnproven" });
  });

  it("warns while the proof is missing or stale", () => {
    expect(lock({ tamperState: "never" }).tone).toBe("warn");
    expect(lock({ tamperState: "stale" }).tone).toBe("warn");
  });

  it("is red when the test deleted something", () => {
    expect(lock({ tamperState: "failed" })).toEqual({ tone: "fail", say: "overview.check.lockFailed" });
  });
});

describe("the trial restore check", () => {
  const drill = (over: Parameters<typeof domain>[0], drilling = false) =>
    check("drill", domainChecks(domain(over), one, { copying: false, drilling }));

  it("does not apply without an off-site copy", () => {
    expect(drill({ offsiteConfigured: false }).tone).toBe("neutral");
  });

  it("does not apply to VMs and the self-backup, which cannot be restored into a sandbox", () => {
    expect(drill({ domain: "vms" })).toEqual({ tone: "neutral", say: "overview.check.drillUnavailable" });
    expect(drill({ domain: "config" }).tone).toBe("neutral");
  });

  it("applies to ZFS like to every section that can be restored", () => {
    expect(drill({ domain: "zfs" }).tone).toBe("ok");
  });

  it("is red after a failed restore, however recent", () => {
    expect(drill({ lastDrDrillOK: false, drillState: "failed" })).toEqual({ tone: "fail", say: "overview.check.drillFailed" });
  });

  it("warns when the last passed restore is overdue", () => {
    expect(drill({ drillState: "overdue" })).toEqual({ tone: "warn", say: "overview.check.drillOverdue" });
  });

  it("tells a restore that never ran from one that only runs by hand", () => {
    const never = { lastDrDrillAt: 0, lastDrDrillOK: false };
    expect(drill({ ...never, offsiteDrillScheduled: true }).say).toBe("overview.check.drillNever");
    expect(drill({ ...never, offsiteDrillScheduled: false }).say).toBe("overview.check.drillManual");
  });

  it("shows a restore that is running", () => {
    expect(drill({ lastDrDrillOK: false }, true).tone).toBe("run");
  });
});

describe("worstTone", () => {
  const tones = (...list: Check["tone"][]) => list.map((tone) => ({ tone }) as Check);

  it("lets the worst check decide", () => {
    expect(worstTone(tones("ok", "warn", "fail"))).toBe("fail");
    expect(worstTone(tones("ok", "run", "warn"))).toBe("warn");
    expect(worstTone(tones("ok", "run"))).toBe("run");
  });

  it("stays neutral where nothing applies yet, instead of looking safe", () => {
    expect(worstTone(tones("neutral", "neutral"))).toBe("neutral");
  });
});

describe("sectionsOf", () => {
  it("leaves out a switched-off section", () => {
    const sections = sectionsOf([domain(), domain({ domain: "vms", enabled: false })], [], {}, null);
    expect(sections.map((s) => s.kind)).toEqual(["container"]);
  });

  it("counts each section's own entries", () => {
    const items = [item(), item({ kind: "vm", key: "win", lastBackup: 0 })];
    const [containers, vms] = sectionsOf([domain(), domain({ domain: "vms" })], items, {}, null);
    expect(containers.tally).toMatchObject({ total: 1, ok: 1 });
    expect(vms.tally).toMatchObject({ total: 1, ok: 0 });
  });

  it("follows the off-site copy and the trial restore that are under way", () => {
    const progress = { "offsite:containers": { phase: "replicate" as const, percent: 10, active: true, lastSeen: 0 } };
    const [section] = sectionsOf([domain()], [item()], progress, "containers");
    expect(section.checks.find((c) => c.key === "offsite")!.tone).toBe("run");
    expect(section.checks.find((c) => c.key === "drill")!.tone).toBe("run");
    expect(section.tone).toBe("run");
  });
});
