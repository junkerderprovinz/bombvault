// What the first steps write to the settings when a section is switched: the
// switch itself, and a nightly schedule for a section nothing runs yet.

import { DEFAULT_CADENCE, buildCadenceString } from "../components/CadenceBuilder";
import type { Settings } from "./api";
import type { EntryKind } from "./backupEntry";

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
