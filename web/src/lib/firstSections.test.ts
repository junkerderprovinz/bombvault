import { describe, expect, it } from "vitest";

import type { BackupItem, Settings } from "./api";
import { NIGHTLY, SECTIONS, kindsToInclude, scheduleChosen, switchSection } from "./firstSections";

/** The settings of a new install: containers on, nothing scheduled. */
function settings(over: Partial<Settings> = {}): Settings {
  return {
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    zfsEnabled: false,
    configEnabled: false,
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    zfsSchedule: "off",
    configSchedule: "off",
    everythingSchedule: "off",
    ...over,
  } as Settings;
}

const section = (kind: string) => SECTIONS.find((s) => s.kind === kind)!;

describe("the nightly schedule", () => {
  it("is the daily run the schedule editor starts from", () => {
    expect(NIGHTLY).toBe("daily 02:00");
  });
});

describe("switchSection", () => {
  it("switches a section on together with a nightly schedule when nothing runs it", () => {
    expect(switchSection(settings(), section("flash"), true)).toEqual({ flashEnabled: true, flashSchedule: NIGHTLY });
  });

  it("takes an empty schedule for none", () => {
    expect(switchSection(settings({ vmsSchedule: "" }), section("vm"), true)).toEqual({ vmsEnabled: true, vmsSchedule: NIGHTLY });
  });

  it("keeps the schedule a section already has", () => {
    const weekly = settings({ filesSchedule: "weekly Sun 04:00" });
    expect(switchSection(weekly, section("files"), true)).toEqual({ filesEnabled: true });
  });

  it("adds no schedule to a section the Backup Everything schedule runs", () => {
    const covered = settings({ everythingSchedule: "daily 01:00" });
    expect(switchSection(covered, section("zfs"), true)).toEqual({ zfsEnabled: true });
  });

  it("takes the nightly schedule back when the section is switched off", () => {
    const on = settings({ flashEnabled: true, flashSchedule: NIGHTLY });
    expect(switchSection(on, section("flash"), false)).toEqual({ flashEnabled: false, flashSchedule: "off" });
  });

  it("leaves another schedule alone when the section is switched off", () => {
    const weekly = settings({ filesEnabled: true, filesSchedule: "weekly Sun 04:00" });
    expect(switchSection(weekly, section("files"), false)).toEqual({ filesEnabled: false });
    expect(switchSection(settings(), section("container"), false)).toEqual({ containersEnabled: false });
  });
});

describe("scheduleChosen", () => {
  it("gives the nightly schedule to the sections that are on and that nothing runs", () => {
    const chosen = settings({ flashEnabled: true, configEnabled: true, configSchedule: "weekly Mon 05:00" });
    expect(scheduleChosen(chosen)).toEqual({ containersSchedule: NIGHTLY, flashSchedule: NIGHTLY });
  });

  it("leaves the sections that are off without a schedule", () => {
    expect(scheduleChosen(settings())).toEqual({ containersSchedule: NIGHTLY });
  });

  it("writes nothing where every section that is on has a schedule", () => {
    expect(scheduleChosen(settings({ containersSchedule: "daily 03:00" }))).toEqual({});
    expect(scheduleChosen(settings({ everythingSchedule: "daily 01:00" }))).toEqual({});
  });
});

describe("kindsToInclude", () => {
  const entry = (over: Partial<BackupItem>) => ({ kind: "container", key: "plex", included: false, ...over }) as BackupItem;
  const both = settings({ vmsEnabled: true });

  it("names the kinds that are on and of whose found entries none is in the schedule", () => {
    const reported = [entry({}), entry({ key: "gitea" }), entry({ kind: "vm", key: "win11" })];
    expect(kindsToInclude(both, reported, [])).toEqual(["container", "vm"]);
  });

  it("leaves out a kind that was taken in once, though all its entries are out again", () => {
    const reported = [entry({}), entry({ key: "gitea" }), entry({ kind: "vm", key: "win11" })];
    expect(kindsToInclude(both, reported, ["container"])).toEqual(["vm"]);
    expect(kindsToInclude(both, reported, ["container", "vm"])).toEqual([]);
  });

  it("leaves out a kind whose section is off", () => {
    const reported = [entry({}), entry({ kind: "vm", key: "win11", kindDisabled: true })];
    expect(kindsToInclude(settings(), reported, [])).toEqual(["container"]);
  });

  it("leaves a kind alone once somebody has taken one of its entries in", () => {
    const reported = [entry({ included: true }), entry({ key: "gitea" }), entry({ kind: "vm", key: "win11" })];
    expect(kindsToInclude(both, reported, [])).toEqual(["vm"]);
  });

  it("asks for nothing where nothing was found", () => {
    const reported = [entry({ key: "bombvault", self: true }), entry({ key: "old-wiki", installed: false })];
    expect(kindsToInclude(both, reported, [])).toEqual([]);
    expect(kindsToInclude(both, [], [])).toEqual([]);
  });

  it("never names a kind whose entries are added by hand", () => {
    const all = settings({ filesEnabled: true, zfsEnabled: true, flashEnabled: true, configEnabled: true });
    const reported = [entry({ kind: "files", key: "set-1" }), entry({ kind: "zfs", key: "zfs-1" }), entry({ kind: "flash", key: "flash" })];
    expect(kindsToInclude(all, reported, [])).toEqual([]);
  });
});
