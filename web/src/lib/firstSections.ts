// What the first steps write when a section is switched or the step is ticked
// off: the switch itself, a nightly schedule for a section nothing runs yet,
// and the found containers and VMs taken into that schedule.

import { DEFAULT_CADENCE, buildCadenceString } from "../components/CadenceBuilder";
import { setIncludeAll, setVMIncludeAll, type BackupItem, type OkEnvelope, type Settings } from "./api";
import type { EntryKind } from "./backupEntry";
import { isGone } from "./backupList";

type SectionKey = "containers" | "vms" | "flash" | "files" | "zfs" | "config";

export interface Section {
  kind: EntryKind;
  enabled: `${SectionKey}Enabled`;
  schedule: `${SectionKey}Schedule`;
}

export const SECTIONS: Section[] = [
  { kind: "container", enabled: "containersEnabled", schedule: "containersSchedule" },
  { kind: "vm", enabled: "vmsEnabled", schedule: "vmsSchedule" },
  { kind: "flash", enabled: "flashEnabled", schedule: "flashSchedule" },
  { kind: "files", enabled: "filesEnabled", schedule: "filesSchedule" },
  { kind: "zfs", enabled: "zfsEnabled", schedule: "zfsSchedule" },
  { kind: "config", enabled: "configEnabled", schedule: "configSchedule" },
];

/** The daily run the schedule editor offers first. */
export const NIGHTLY = buildCadenceString({ ...DEFAULT_CADENCE, mode: "daily" });

function off(cadence: string): boolean {
  return cadence === "" || cadence === "off";
}

// A section the "Backup Everything" schedule covers runs already, and a
// schedule of its own would back it up twice.
function unscheduled(settings: Settings, section: Section): boolean {
  return off(settings[section.schedule]) && off(settings.everythingSchedule);
}

/**
 * switchSection is what switching a section in the first steps saves. One
 * that nothing runs yet gets the nightly schedule along with the switch, so
 * what is on there is also what runs. Switching it off takes that schedule
 * back and leaves any other alone, so the section starts without one when it
 * is switched on in the settings later.
 */
export function switchSection(settings: Settings, section: Section, on: boolean): Partial<Settings> {
  if (on && unscheduled(settings, section)) return { [section.enabled]: true, [section.schedule]: NIGHTLY };
  if (!on && settings[section.schedule] === NIGHTLY) return { [section.enabled]: false, [section.schedule]: "off" };
  return { [section.enabled]: on };
}

/**
 * scheduleChosen gives the nightly schedule to every section that is on and
 * that nothing runs. Containers are on from the first start, so no switch
 * ever asked for theirs.
 */
export function scheduleChosen(settings: Settings): Partial<Settings> {
  const left = SECTIONS.filter((section) => settings[section.enabled] && unscheduled(settings, section));
  return Object.fromEntries(left.map((section) => [section.schedule, NIGHTLY]));
}

/** The kinds whose entries are found on the host and stand outside the
 *  schedule until somebody takes them in. */
export const FOUND_KINDS = ["container", "vm"] as const;

export type FoundKind = (typeof FOUND_KINDS)[number];

export const INCLUDE_FOUND: Record<FoundKind, () => Promise<OkEnvelope>> = {
  container: () => setIncludeAll(true),
  vm: () => setVMIncludeAll(true),
};

/**
 * kindsToInclude names the kinds whose found entries the first steps take
 * into the schedule: the section is on and not one of its entries is in yet.
 * Where some are, somebody has chosen, and the choice stands.
 */
export function kindsToInclude(settings: Settings, reported: BackupItem[]): FoundKind[] {
  return FOUND_KINDS.filter((kind) => {
    const section = SECTIONS.find((s) => s.kind === kind)!;
    const found = reported.filter((item) => item.kind === kind && !item.self && !isGone(item));
    return settings[section.enabled] && found.length > 0 && found.every((item) => !item.included);
  });
}
