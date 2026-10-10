// Which section is backed up to which storage location, as the lines of one
// picture. A line of a section with placement is read off the section's
// default; flash, ZFS and the self-backup have no placement, and their line is
// the switch of their target.

import type {
  DefaultChange,
  OffsiteDomain,
  PlacementOptions,
  PlacementView,
  StorageLocation,
  StorageLocationSection,
} from "./api";
import type { useT } from "./i18n";
import {
  defaultChangeOf,
  isPlacementDomain,
  lockHint,
  newTargetRefused,
  placementButtons,
  stepForLocal,
  stepForTarget,
  viewHomeLabel,
  type PlacementStep,
} from "./placement";
import { blockedText } from "./placementButtons";
import { DOMAIN_LABEL } from "./storageLocations";

type T = ReturnType<typeof useT>["t"];

export const FLOW_DOMAINS: readonly OffsiteDomain[] = ["containers", "vms", "flash", "files", "zfs", "config"];

/** What a click on a line does. */
export type FlowAction =
  | { kind: "step"; step: PlacementStep }
  | { kind: "default"; change: DefaultChange }
  /** The destination has no target for the section yet. */
  | { kind: "destination"; destinationId: string }
  | { kind: "enable"; targetId: string; on: boolean }
  | { kind: "create"; destinationId: string };

export interface FlowLine {
  key: string;
  domain: OffsiteDomain;
  location: StorageLocation;
  on: boolean;
  /** Why the line cannot be switched. A line with a reason has no action. */
  locked?: string;
  action?: FlowAction;
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
  /** The section's default could not be read, so its lines only show. */
  unreadable: boolean;
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
 *  use. A switched-off module has none. */
export function flowDomains(locations: StorageLocation[]): OffsiteDomain[] {
  const live = new Set(
    locations
      .filter((location) => location.object === "path")
      .flatMap((location) => location.sections.filter((section) => section.enabled).map((section) => section.domain))
  );
  return FLOW_DOMAINS.filter((domain) => live.has(domain));
}

function copyOf(location: StorageLocation, domain: OffsiteDomain): StorageLocationSection | undefined {
  return location.sections.find((section) => section.domain === domain && section.use === "copy");
}

function locked(step: PlacementStep, t: T, home: string, fallback: string): string | undefined {
  if (step.kind === "blocked") return blockedText(t, step, home);
  return step.kind === "none" ? fallback : undefined;
}

function stepLine(base: Omit<FlowLine, "locked" | "action">, step: PlacementStep, t: T, home: string, fallback: string): FlowLine {
  const reason = locked(step, t, home, fallback);
  return reason ? { ...base, locked: reason } : { ...base, action: { kind: "step", step } };
}

/** The line to a place backups are written to: the section's own folder or a
 *  named repository. Only one of them is the home at a time. */
function homeLine(
  base: Omit<FlowLine, "on" | "locked" | "action">,
  repo: string,
  def: FlowDefault,
  t: T,
  host: string
): FlowLine | null {
  const { options, view } = def;
  const listed = options.homes.some((home) => home.id === repo);
  const remote = options.sendTo.some((option) => option.kind === "remote" && option.repoId === repo);
  const on = view.repo === repo && view.repoKind !== "missing";
  if (!on && !listed && !remote) return null;
  const home = viewHomeLabel(t, host, view, options);
  const buttons = placementButtons(view, options);
  if (on) {
    const step = stepForLocal(!buttons.local, view, options, t, host);
    return stepLine({ ...base, on }, step, t, home, t("placement.lastButton"));
  }
  if (remote || buttons.local) return { ...base, on, action: { kind: "default", change: { home: repo } } };
  const step = stepForLocal(true, view, options, t, host);
  const change = defaultChangeOf(step);
  if (!change) return stepLine({ ...base, on }, step, t, home, t("placement.lastButton"));
  return { ...base, on, action: { kind: "default", change: { ...change, home: repo } } };
}

function placementLine(
  base: Omit<FlowLine, "on" | "locked" | "action">,
  def: FlowDefault,
  t: T,
  host: string
): FlowLine | null {
  const { location, domain } = base;
  const { options, view } = def;
  if (location.object === "path") {
    const own = location.sections.some((section) => section.domain === domain && section.use === "home");
    return own ? homeLine(base, "", def, t, host) : null;
  }
  if (location.object === "repo") return homeLine(base, objectId(location), def, t, host);

  const copy = copyOf(location, domain);
  if (location.object === "target" && !copy) return null;
  const home = viewHomeLabel(t, host, view, options);
  const target = copy?.targetId ? options.targets.find((option) => option.id === copy.targetId) : undefined;
  if (!target) {
    const off = { ...base, on: false };
    if (!location.enabled) return { ...off, locked: t("storage.flow.lockOff") };
    if (newTargetRefused(view)) return { ...off, locked: lockHint(t, "own-credentials", home) };
    return location.object === "destination"
      ? { ...off, action: { kind: "destination", destinationId: objectId(location) } }
      : null;
  }
  const on = placementButtons(view, options).ticked.includes(target.id);
  if (!on && !target.enabled) return { ...base, on, locked: t("storage.flow.lockOff") };
  const step = stepForTarget(target.id, !on, view, options, t);
  return stepLine({ ...base, on }, step, t, home, t("placementDefaults.homeMissing"));
}

/** The line of a section without placement: its folder is fixed, and a copy
 *  is the switch of its target. */
function switchLine(base: Omit<FlowLine, "on" | "locked" | "action">, t: T): FlowLine | null {
  const { location, domain } = base;
  if (location.object === "path") {
    const own = location.sections.some((section) => section.domain === domain && section.use === "home");
    if (!own) return null;
    const reason = t("storage.flow.lockOwnFolder").replace("{domain}", () => t(DOMAIN_LABEL[domain]));
    return { ...base, on: true, locked: reason };
  }
  if (location.object === "repo") return null;
  const copy = copyOf(location, domain);
  if (!copy?.targetId) {
    if (location.object !== "destination") return null;
    const off = { ...base, on: false };
    if (!location.enabled) return { ...off, locked: t("storage.flow.lockOff") };
    return { ...off, action: { kind: "create", destinationId: objectId(location) } };
  }
  const line = { ...base, on: copy.enabled };
  if (copy.primary) return { ...line, locked: t("storage.own.primary") };
  if (!copy.enabled && !location.enabled) return { ...line, locked: t("storage.flow.lockOff") };
  return { ...line, action: { kind: "enable", targetId: copy.targetId, on: !copy.enabled } };
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

export function flowRows(
  locations: StorageLocation[],
  defaults: Partial<Record<OffsiteDomain, FlowDefault | "unreadable">>,
  t: T,
  host: string
): FlowRow[] {
  return flowDomains(locations).map((domain) => {
    const def = defaults[domain];
    const lines: FlowLine[] = [];
    for (const location of locations) {
      const base = { key: `${domain}@${location.id}`, domain, location };
      let line: FlowLine | null;
      if (!isPlacementDomain(domain)) line = switchLine(base, t);
      else if (def && def !== "unreadable") line = placementLine(base, def, t, host);
      else line = null;
      if (line) lines.push(line);
    }
    const ownRules = lines
      .filter((line) => line.on && line.location.object === "destination")
      .filter((line) => (copyOf(line.location, domain)?.own.length ?? 0) > 0)
      .map((line) => line.location);
    return { domain, lines, counts: flowCounts(lines), ownRules, unreadable: def === "unreadable" };
  });
}
