// What a restore can ask for, worked out before the window draws it: which
// backup it starts on, what of it can come back, and where that can go.

import { anomalyShortLine } from "./anomalies";
import type { AnomalyView, OffsiteDomain, Settings, TimelineRow } from "./api";
import { entryKey, kindOfDomain, type EntryKind } from "./backupEntry";
import { rowHolds, rowTime } from "./entry";
import type { TranslationKey } from "./i18n";

export type RestoreWhat = "all" | "files" | "dumps" | "plugins";

export type RestoreWhere = "beside" | "replace" | "folder" | "newDataset" | "zip";

/** What the server needs to know about the entry before it offers a target. */
export interface TargetFacts {
  /** The container or VM is on the host, or the folder set knows its folder. */
  hasOriginal: boolean;
  /** ZFS: the chosen dataset is mounted and may be written into. */
  writable: boolean;
  /** ZFS: one dataset is chosen and this backup holds it. */
  oneDataset: boolean;
}

export function whatsFor(kind: EntryKind, facts: { database: boolean }): RestoreWhat[] {
  switch (kind) {
    case "container":
      return facts.database ? ["all", "files", "dumps"] : ["all", "files"];
    case "files":
    case "zfs":
      return ["all", "files"];
    case "flash":
      return ["all", "plugins"];
    default:
      return ["all"];
  }
}

/**
 * wheresFor lists the targets the server takes for this kind, the safe one
 * first. A part of a backup goes where its restore route can put it, which
 * is fewer places than the whole backup has.
 */
export function wheresFor(kind: EntryKind, what: RestoreWhat, facts: TargetFacts): RestoreWhere[] {
  if (what === "dumps" || what === "plugins") return [];
  switch (kind) {
    case "container":
      if (what === "files") return ["replace", "folder"];
      return facts.hasOriginal ? ["beside", "replace", "folder"] : ["replace", "folder"];
    case "vm":
      return facts.hasOriginal ? ["beside", "replace"] : ["replace"];
    case "files":
      if (what === "files" || !facts.hasOriginal) return ["folder"];
      return ["beside", "replace", "folder"];
    case "zfs": {
      if (what === "files") return facts.writable ? ["replace"] : [];
      const targets: RestoreWhere[] = [];
      if (facts.writable) targets.push("replace");
      if (facts.oneDataset) targets.push("newDataset");
      return [...targets, "folder"];
    }
    case "flash":
      return ["zip"];
    default:
      return [];
  }
}

/** The safe target is the first one that leaves the original alone. Where
 *  there is no original, bringing the entry back replaces nothing. */
export function defaultWhere(wheres: RestoreWhere[], hasOriginal: boolean): RestoreWhere | undefined {
  if (!hasOriginal && wheres.includes("replace")) return "replace";
  return wheres.find((where) => where !== "replace") ?? wheres[0];
}

/** A backup can be taken first where a whole entry is replaced. ZFS keeps a
 *  snapshot of its own instead. */
export function backsUpFirst(kind: EntryKind, what: RestoreWhat, where: RestoreWhere, hasOriginal: boolean): boolean {
  return where === "replace" && what === "all" && hasOriginal && (kind === "container" || kind === "vm" || kind === "files");
}

const SECTION: Record<EntryKind, OffsiteDomain> = {
  container: "containers",
  vm: "vms",
  files: "files",
  zfs: "zfs",
  flash: "flash",
  config: "config",
};

/** Matches the server's default for the setting. */
export const DEFAULT_RESTORE_FOLDER = "user/bombvault/restore";

/** The folder a restore of this kind into a folder starts with. */
export function restoreFolderFor(settings: Pick<Settings, "restoreFolder" | "restoreFolders">, kind: EntryKind): string {
  return settings.restoreFolders?.[SECTION[kind]] || settings.restoreFolder || DEFAULT_RESTORE_FOLDER;
}

/** Every id the backup goes by: its own and those of its copies. */
export function snapshotIds(row: TimelineRow): string[] {
  return [row.key, ...row.places.flatMap((mark) => mark.snapshotIds)];
}

/** The open findings that are more than a note and belong to this entry. */
export function findingsOf(list: AnomalyView[], kind: EntryKind, key: string): AnomalyView[] {
  return list.filter(
    (a) => a.severity !== "info" && kindOfDomain(a.domain) === kind && entryKey(kind, { name: a.name, id: a.targetId }) === key
  );
}

export interface Recommendation {
  /** The row to restore from, counted from the newest. */
  index: number;
  /** The finding that moves the recommendation off the newest backup. */
  finding?: AnomalyView;
}

/**
 * recommend picks the backup to restore from: the newest one, unless an open
 * finding says data was lost. Then it is the last good backup the finding
 * names, or the one before the oldest backup it was raised on.
 */
export function recommend(rows: TimelineRow[], findings: AnomalyView[]): Recommendation {
  const lossy = findings
    .filter((a) => a.lastGood !== undefined)
    .sort((a, b) => (a.lastGood?.at ?? 0) - (b.lastGood?.at ?? 0));
  for (const finding of lossy) {
    const good = finding.lastGood;
    if (!good) continue;
    const named = rows.findIndex((row) => rowHolds(row, good.snapshotId));
    if (named >= 0) return { index: named, finding };
    // Retention may have removed the named backup since.
    const before = rows.findIndex((row) => rowTime(row) <= good.at);
    if (before >= 0) return { index: before, finding };
  }
  let oldest = -1;
  let raisedBy: AnomalyView | undefined;
  rows.forEach((row, i) => {
    const finding = findings.find((a) => (a.flaggedSnapshots ?? []).some((id) => rowHolds(row, id)));
    if (finding) {
      oldest = i;
      raisedBy = finding;
    }
  });
  if (raisedBy && oldest + 1 < rows.length) return { index: oldest + 1, finding: raisedBy };
  return { index: 0 };
}

/** The row a restore link asks for, or the one closest to its time. */
export function requestedRow(rows: TimelineRow[], snapshot: string, at: number): { row?: TimelineRow; missing: boolean } {
  const asked = rows.find((row) => rowHolds(row, snapshot));
  if (asked) return { row: asked, missing: false };
  let nearest: TimelineRow | undefined;
  for (const row of rows) {
    if (!nearest || Math.abs(rowTime(row) - at) < Math.abs(rowTime(nearest) - at)) nearest = row;
  }
  return { row: at > 0 ? nearest : rows[0], missing: true };
}

type Translate = (key: TranslationKey, n?: number) => string;

/** Why the recommended backup is the one to take, in a sentence. */
export function recommendationWhy(recommendation: Recommendation, t: Translate, lang: string): string {
  const { finding } = recommendation;
  if (!finding) return t("restoreWindow.whyNewest");
  return t("restoreWindow.whyBefore").replace("{finding}", () => anomalyShortLine(finding, t, lang).text);
}

export const EMERGENCY_STEPS = ["access", "settings", "selection", "restore"] as const;

export type EmergencyStep = (typeof EMERGENCY_STEPS)[number];

/** The step a link asks for in ?emergency=, or null for no window. */
export function parseEmergencyStep(value: string | null): EmergencyStep | null {
  return EMERGENCY_STEPS.find((step) => step === value) ?? null;
}

/** Where the Restore page opens the emergency window on a step. */
export function emergencyPath(step: EmergencyStep): string {
  return `/restore?emergency=${step}`;
}
