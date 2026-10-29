// Which section is backed up to which storage location, as the lines of one
// picture. A line of a section with placement is read off the section's
// default; flash, ZFS and the self-backup have no placement, and their lines
// are the switch of their own folder and of their targets. Every line can be
// switched, and switching one off moves nothing that is already stored.

import type { DefaultChange, OffsiteDomain, PlacementOptions, PlacementView, StorageLocation, StorageLocationSection } from "./api";
import { isPlacementDomain, placementButtons, type PlacementButtons } from "./placement";
import { ownFolderSection } from "./storageLocations";

export const FLOW_DOMAINS: readonly OffsiteDomain[] = ["containers", "vms", "flash", "files", "zfs", "config"];

/** The default home that gives new entries no place at all. */
export const HOME_NOWHERE = "none";

/** What a click on a line does. */
export type FlowAction =
  /** A new default for a section with placement; `enable` switches a target
   *  on first. */
  | { kind: "default"; change: DefaultChange; enable?: string }
  /** The target's direct repository becomes the place backups are written
   *  to, made first where it does not exist yet. */
  | { kind: "direct"; targetId: string; skip: string[]; enable?: string }
  /** The destination has no target for the section yet. */
  | { kind: "destination"; destinationId: string }
  | { kind: "enable"; targetId: string; on: boolean }
  | { kind: "create"; destinationId: string }
  | { kind: "local"; on: boolean };

export interface FlowLine {
  key: string;
  domain: OffsiteDomain;
  location: StorageLocation;
  on: boolean;
  action: FlowAction;
}

export interface FlowCounts {
  copies: number;
  media: number;
  offsite: number;
}

export interface FlowRow {
  domain: OffsiteDomain;
  lines: FlowLine[];
  counts: FlowCounts;
  /** The storage locations at which the section holds rules of its own. */
  ownRules: StorageLocation[];
  /** The section's default could not be read, so it has no lines. */
  unreadable: boolean;
  /** Every line of the section is off, so nothing new of it is backed up. */
  nowhere: boolean;
}

/** A placement section's default, as its buttons read it. */
export interface FlowDefault {
  options: PlacementOptions;
  view: PlacementView;
}

export function objectId(location: StorageLocation): string {
  return location.id.slice(location.id.indexOf(":") + 1);
}

/** The sections that are backed up at all: the ones whose own folder is in
 *  use, on this host or in a storage location. A switched-off module has none. */
export function flowDomains(locations: StorageLocation[]): OffsiteDomain[] {
  const live = new Set(
    locations.flatMap((location) => FLOW_DOMAINS.filter((domain) => ownFolderSection(location, domain)?.enabled))
  );
  return FLOW_DOMAINS.filter((domain) => live.has(domain));
}

function copyOf(location: StorageLocation, domain: OffsiteDomain): StorageLocationSection | undefined {
  return location.sections.find((section) => section.domain === domain && section.use === "copy");
}


/** skipFor is the skip list that copies to exactly the given targets. A list
 *  of every other target lets a target added later arrive ticked; nothing at
 *  all is ["*"]. */
function skipFor(options: PlacementOptions, copies: string[]): string[] {
  return copies.length === 0 ? ["*"] : options.targets.filter((x) => !copies.includes(x.id)).map((x) => x.id);
}

/** One placement default with what its lines are read from. `copies` are the
 *  targets copies go to now, the home's own target left out, and `copying`
 *  says whether backups are written somewhere copies can be made from. */
interface Placed {
  def: FlowDefault;
  buttons: PlacementButtons;
  copies: string[];
  copying: boolean;
}

function placedOf(def: FlowDefault): Placed {
  const buttons = placementButtons(def.view, def.options);
  return {
    def,
    buttons,
    copies: buttons.ticked.filter((id) => id !== buttons.home),
    copying: buttons.local || buttons.home !== null,
  };
}

function lit(placed: Placed, id: string): boolean {
  return placed.buttons.ticked.includes(id) && placed.def.options.targets.some((x) => x.id === id && x.enabled);
}

/** Writes backups straight to a target, copying to rest from there. */
function homeAt(placed: Placed, targetId: string, rest: string[], enable?: string): FlowAction {
  const { options } = placed.def;
  const skip = skipFor(options, rest);
  const made = options.sendTo.find((s) => s.kind === "direct" && s.targetId === targetId && s.repoId !== "");
  return made ? { kind: "default", change: { home: made.repoId, skip }, enable } : { kind: "direct", targetId, skip, enable };
}

/** Takes away the place backups are written to: the first target that is
 *  lit takes over, or nothing does. */
function leaveHome(placed: Placed): FlowAction {
  const [next, ...rest] = placed.copies.filter((id) => lit(placed, id));
  return next ? homeAt(placed, next, rest) : { kind: "default", change: { home: HOME_NOWHERE } };
}

/** The line to a place backups are written to: the section's own folder or a
 *  named repository. Only one of them is the home at a time. */
function homeLine(base: Omit<FlowLine, "on" | "action">, repo: string, placed: Placed): FlowLine | null {
  const { options, view } = placed.def;
  const listed = options.homes.some((home) => home.id === repo);
  const remote = options.sendTo.some((option) => option.kind === "remote" && option.repoId === repo);
  const on = view.repo === repo && view.repoKind !== "missing";
  if (on) return { ...base, on, action: leaveHome(placed) };
  if (!listed && !remote) return null;
  // A remote repository is read with its own credentials, so nothing is
  // copied from it and the copies stay as they are for the next home.
  if (remote) return { ...base, on, action: { kind: "default", change: { home: repo } } };
  return { ...base, on, action: { kind: "default", change: { home: repo, skip: skipFor(options, placed.buttons.ticked) } } };
}

function placementLine(base: Omit<FlowLine, "on" | "action">, def: FlowDefault): FlowLine | null {
  const { location, domain } = base;
  const placed = placedOf(def);
  // A storage location that holds the section's own folder is its home,
  // never a place to copy to as well.
  if (ownFolderSection(location, domain)) return homeLine(base, "", placed);
  if (location.object === "path") return null;
  if (location.object === "repo") return homeLine(base, objectId(location), placed);

  const copy = copyOf(location, domain);
  if (location.object === "target" && !copy) return null;
  const target = copy?.targetId ? def.options.targets.find((option) => option.id === copy.targetId) : undefined;
  if (!target) {
    if (location.object !== "destination") return null;
    return { ...base, on: false, action: { kind: "destination", destinationId: objectId(location) } };
  }
  const { options } = def;
  const { buttons, copies } = placed;
  if (lit(placed, target.id)) {
    const action: FlowAction =
      buttons.home === target.id
        ? leaveHome(placed)
        : { kind: "default", change: { skip: skipFor(options, copies.filter((id) => id !== target.id)) } };
    return { ...base, on: true, action };
  }
  // A target that is ticked and switched off only needs its switch.
  if (buttons.ticked.includes(target.id)) return { ...base, on: false, action: { kind: "enable", targetId: target.id, on: true } };
  const enable = target.enabled ? undefined : target.id;
  const action: FlowAction = placed.copying
    ? { kind: "default", change: { skip: skipFor(options, [...copies, target.id]) }, enable }
    : homeAt(placed, target.id, [], enable);
  return { ...base, on: false, action };
}

/** The line of a section without placement: the switch of its own folder or
 *  of a target. */
function switchLine(base: Omit<FlowLine, "on" | "action">): FlowLine | null {
  const { location, domain } = base;
  const own = ownFolderSection(location, domain);
  if (own) return { ...base, on: !own.off, action: { kind: "local", on: !!own.off } };
  if (location.object === "path") return null;
  if (location.object === "repo") return null;
  const copy = copyOf(location, domain);
  if (copy?.targetId) return { ...base, on: copy.enabled, action: { kind: "enable", targetId: copy.targetId, on: !copy.enabled } };
  return location.object === "destination" ? { ...base, on: false, action: { kind: "create", destinationId: objectId(location) } } : null;
}

/** The original counts as a copy on a medium of its own. A local place shares
 *  that medium, every other place is one more. */
export function flowCounts(lines: FlowLine[]): FlowCounts {
  const used = lines.filter((line) => line.on && line.location.enabled);
  const far = used.filter((line) => line.location.kind !== "local");
  return {
    copies: 1 + used.length,
    media: 1 + far.length,
    offsite: far.filter((line) => line.location.offPremises).length,
  };
}

export function flowRows(locations: StorageLocation[], defaults: Partial<Record<OffsiteDomain, FlowDefault | "unreadable">>): FlowRow[] {
  return flowDomains(locations).map((domain) => {
    const def = defaults[domain];
    const lines: FlowLine[] = [];
    for (const location of locations) {
      const base = { key: `${domain}@${location.id}`, domain, location };
      let line: FlowLine | null;
      if (!isPlacementDomain(domain)) line = switchLine(base);
      else if (def && def !== "unreadable") line = placementLine(base, def);
      else line = null;
      if (line) lines.push(line);
    }
    const ownRules = lines
      .filter((line) => line.on && line.location.object === "destination")
      .filter((line) => (copyOf(line.location, domain)?.own.length ?? 0) > 0)
      .map((line) => line.location);
    const unreadable = def === "unreadable";
    return { domain, lines, counts: flowCounts(lines), ownRules, unreadable, nowhere: !unreadable && !lines.some((line) => line.on) };
  });
}
