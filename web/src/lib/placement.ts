import type { SelectorItem } from "../components/Selector";
import {
  PLACEMENT_DOMAINS,
  type DefaultChange,
  type DefaultRow,
  type HomeKind,
  type HomeOption,
  type PlacementChange,
  type PlacementDomain,
  type PlacementOptions,
  type PlacementView,
  type SegmentId,
  type SegmentLockReason,
  type SegmentLocks,
  type SendToOption,
} from "./api";
import type { TranslationKey, useT } from "./i18n";
import { withLtrIsolates } from "./ltrFragments";

type T = ReturnType<typeof useT>["t"];

const ALL = "*";

function skipsAll(skip: string[]): boolean {
  return skip.length === 1 && skip[0] === ALL;
}

function copySource(kind: HomeKind | ""): boolean {
  return kind === "domain" || kind === "domain-remote" || kind === "local";
}

export function isPlacementDomain(domain: string): domain is PlacementDomain {
  return (PLACEMENT_DOMAINS as readonly string[]).includes(domain);
}

const DOMAIN_KEYS: Record<PlacementDomain, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  files: "nav.files",
};

export function domainLabel(t: T, domain: PlacementDomain): string {
  return t(DOMAIN_KEYS[domain]);
}

export function formatList(lang: string, names: string[]): string {
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(names);
}

export function homeOptionLabel(t: T, host: string, h: HomeOption): string {
  switch (h.kind) {
    case "domain":
      return withLtrIsolates(
        t("placement.homeDomain").replace("{host}", () => host).replace("{path}", () => h.location),
        [h.location]
      );
    case "domain-remote":
      return t("placement.homeDomainRemote").replace("{scheme}", () => h.scheme);
    case "local":
      return t("placement.homeLocal").replace("{name}", () => h.name);
  }
}

export function sendToLabel(t: T, s: SendToOption): string {
  if (s.kind === "remote") return t("placement.homeRemote").replace("{name}", () => s.name);
  const label = t("placement.homeDirect").replace("{target}", () => s.name);
  return s.repoId ? label : `${label} · ${t("placement.directNotYet")}`;
}

// homeLabel names a repository id the way the lists offer it, null when neither does.
function homeLabel(t: T, host: string, repoId: string, options: PlacementOptions): string | null {
  const home = options.homes.find((h) => h.id === repoId);
  if (home) return homeOptionLabel(t, host, home);
  const sendTo = options.sendTo.find((s) => s.repoId !== "" && s.repoId === repoId);
  return sendTo ? sendToLabel(t, sendTo) : null;
}

/** viewHomeLabel names a card's home: as the lists offer it, or marked when it
 *  is switched off or has no row any more. */
export function viewHomeLabel(t: T, host: string, view: PlacementView, options: PlacementOptions): string {
  const listed = homeLabel(t, host, view.repo, options);
  if (listed !== null) return listed;
  if (view.repo === "") return host;
  return t(view.repoOff ? "placement.off" : "placement.unknown").replace("{name}", () => view.repoLabel || view.repo);
}

const LOCK_KEYS: Record<SegmentLockReason, TranslationKey> = {
  "no-target": "placement.noTarget",
  "own-credentials": "placement.lockOwnCredentials",
  "at-target": "placement.lockAtTarget",
  "home-fixed": "placement.lockHomeFixed",
};

export function lockHint(t: T, reason: SegmentLockReason, home: string): string {
  return t(LOCK_KEYS[reason]).replace("{home}", () => home);
}

const SEGMENTS: readonly [SegmentId, TranslationKey][] = [
  ["local", "placement.segLocal"],
  ["local-offsite", "placement.segLocalOffsite"],
  ["offsite-only", "placement.segOffsiteOnly"],
];

/** lockedSegments is what the bar disables: the locks the server sent, and
 *  off-site only whenever the options offer nothing to send to. */
export function lockedSegments(view: PlacementView, options: PlacementOptions): SegmentLocks {
  if (options.sendTo.length > 0) return view.segmentLocks;
  return { ...view.segmentLocks, "offsite-only": "no-target" };
}

export function segmentItems(t: T, locks: SegmentLocks, home: string): SelectorItem[] {
  return SEGMENTS.map(([id, key]) => {
    const reason = locks[id];
    return { id, label: t(key), disabled: reason !== undefined, title: reason ? lockHint(t, reason, home) : undefined };
  });
}

/** viewSegment reads a segment off the stored rule, the way the server does. */
export function viewSegment(kind: HomeKind | "", skip: string[], hasTargets: boolean): SegmentId {
  if (kind === "remote" || kind === "direct") return "offsite-only";
  return !hasTargets || skipsAll(skip) ? "local" : "local-offsite";
}

export function defaultSegment(row: DefaultRow, options: PlacementOptions): SegmentId {
  return viewSegment(row.homeKind, row.skip, options.targets.length > 0);
}

function repoName(repoId: string, options: PlacementOptions): string {
  const home = options.homes.find((h) => h.id === repoId);
  const sendTo = options.sendTo.find((s) => s.repoId !== "" && s.repoId === repoId);
  return home?.name ?? sendTo?.name ?? repoId;
}

/** defaultView is a default as the bar shows it. */
export function defaultView(row: DefaultRow, options: PlacementOptions): PlacementView {
  return {
    segment: defaultSegment(row, options),
    repo: row.home,
    repoLabel: repoName(row.home, options),
    repoKind: row.homeKind,
    repoOff: row.homeOff,
    homeFollows: false,
    copiesFollow: false,
    skip: row.skip,
    locked: false,
    lockReason: "",
    segmentLocks: options.segmentLocks,
    paused: row.paused,
    unreadable: row.unreadable,
  };
}

/** draftView is a new item before anything is chosen: it follows the default. */
export function draftView(options: PlacementOptions): PlacementView {
  return { ...defaultView(options.default, options), homeFollows: true, copiesFollow: true, paused: false };
}

export type PlacementStep =
  | { kind: "save"; change: PlacementChange; confirmHome: string | null; optimistic: Partial<PlacementView> }
  | { kind: "direct"; target: SendToOption }
  | { kind: "none" };

const NONE: PlacementStep = { kind: "none" };

// homeBack is where an item leaving a remote or direct repository before its
// first backup goes: the default's home when that is a copy source, else the
// domain path. Null when the home stays.
function homeBack(view: PlacementView, options: PlacementOptions, t: T, host: string) {
  if (view.locked || (view.repoKind !== "remote" && view.repoKind !== "direct")) return null;
  const d = options.default;
  if (copySource(d.homeKind)) {
    return {
      change: { follow: true } as const,
      label: homeLabel(t, host, d.home, options) ?? host,
      optimistic: { repo: d.home, repoKind: d.homeKind, repoLabel: repoName(d.home, options), homeFollows: true },
    };
  }
  return {
    change: { repo: "" },
    label: homeLabel(t, host, "", options) ?? host,
    optimistic: { repo: "", repoKind: options.homes[0].kind, repoLabel: "", homeFollows: false },
  };
}

export function stepForSegment(seg: SegmentId, view: PlacementView, options: PlacementOptions, t: T, host: string): PlacementStep {
  if (seg === view.segment) return NONE;
  if (seg === "offsite-only") {
    // The bar locks this step while the list is empty, so a click can only
    // come from a list that emptied after it was drawn.
    const first = options.sendTo[0];
    if (!first) return NONE;
    if (!first.repoId) return { kind: "direct", target: first };
    return {
      kind: "save",
      change: { home: { repo: first.repoId }, copies: { skip: [ALL] } },
      confirmHome: sendToLabel(t, first),
      optimistic: {
        segment: "offsite-only",
        repo: first.repoId,
        repoKind: first.kind,
        repoLabel: first.name,
        homeFollows: false,
        skip: [ALL],
        copiesFollow: false,
      },
    };
  }
  const skip = seg === "local" ? [ALL] : [];
  const back = homeBack(view, options, t, host);
  return {
    kind: "save",
    change: back ? { home: back.change, copies: { skip } } : { copies: { skip } },
    confirmHome: back ? back.label : null,
    optimistic: { ...back?.optimistic, segment: seg, skip, copiesFollow: false },
  };
}

export function stepForHome(repoId: string, view: PlacementView, options: PlacementOptions, t: T, host: string): PlacementStep {
  if (repoId === view.repo && !view.homeFollows) return NONE;
  const home = options.homes.find((h) => h.id === repoId);
  return {
    kind: "save",
    change: { home: { repo: repoId } },
    confirmHome: homeLabel(t, host, repoId, options) ?? repoId,
    optimistic: { repo: repoId, repoKind: home?.kind ?? view.repoKind, repoLabel: home?.name ?? "", homeFollows: false },
  };
}

export function stepForSendTo(opt: SendToOption, view: PlacementView, t: T): PlacementStep {
  if (!opt.repoId) return { kind: "direct", target: opt };
  if (opt.repoId === view.repo && !view.homeFollows) return NONE;
  return {
    kind: "save",
    change: { home: { repo: opt.repoId } },
    confirmHome: sendToLabel(t, opt),
    optimistic: { repo: opt.repoId, repoKind: opt.kind, repoLabel: opt.name, homeFollows: false },
  };
}

export function chipTicked(view: PlacementView, targetId: string): boolean {
  return !skipsAll(view.skip) && !view.skip.includes(targetId);
}

export function lastChipLocked(view: PlacementView, options: PlacementOptions, targetId: string): boolean {
  const ticked = options.targets.filter((x) => x.enabled && chipTicked(view, x.id));
  return ticked.length === 1 && ticked[0].id === targetId;
}

export function stepForChip(targetId: string, on: boolean, view: PlacementView, options: PlacementOptions): PlacementStep {
  if (!on && lastChipLocked(view, options, targetId)) return NONE;
  const known = options.targets.map((x) => x.id);
  const excluded = skipsAll(view.skip) ? known : view.skip.filter((id) => known.includes(id));
  const skip = on ? excluded.filter((id) => id !== targetId) : [...excluded, targetId];
  return { kind: "save", change: { copies: { skip } }, confirmHome: null, optimistic: { skip, copiesFollow: false } };
}

export function stepForReset(view: PlacementView): PlacementStep {
  const copies = { follow: true } as const;
  return view.locked
    ? { kind: "save", change: { copies }, confirmHome: null, optimistic: { copiesFollow: true } }
    : { kind: "save", change: { home: { follow: true }, copies }, confirmHome: null, optimistic: { homeFollows: true, copiesFollow: true } };
}

/** addsTargets reports whether a change can hand the item to a target it does
 *  not reach now, which is when uploads may start. */
export function addsTargets(view: PlacementView, change: PlacementChange): boolean {
  const copies = change.copies;
  if (!copies) return false;
  if ("follow" in copies) return true;
  if (skipsAll(copies.skip)) return false;
  return skipsAll(view.skip) || view.skip.some((id) => !copies.skip.includes(id));
}

/** noCopyNow names the switched-off targets an item on Local + off-site is left
 *  with when no enabled target is ticked, so the card can say nothing is copied. */
export function noCopyNow(view: PlacementView, options: PlacementOptions): string[] {
  if (view.segment !== "local-offsite") return [];
  if (options.targets.some((x) => x.enabled && chipTicked(view, x.id))) return [];
  return options.targets.filter((x) => !x.enabled && chipTicked(view, x.id)).map((x) => x.name);
}

export type FollowLine = "follows" | "own-copies" | "home-set" | "copies-follow";

export function followLine(view: PlacementView): FollowLine {
  if (view.homeFollows) return view.copiesFollow ? "follows" : "own-copies";
  if (!view.locked) return "home-set";
  return view.copiesFollow ? "copies-follow" : "own-copies";
}

export type DefaultStep = { kind: "change"; change: DefaultChange } | { kind: "direct"; target: SendToOption } | { kind: "none" };

/** stepForDefaultSegment changes a default. Off-site only moves only its home;
 *  the copies it gives items on a copy source stay as they are. */
export function stepForDefaultSegment(seg: SegmentId, row: DefaultRow, options: PlacementOptions): DefaultStep {
  if (seg === defaultSegment(row, options)) return { kind: "none" };
  if (seg === "offsite-only") {
    const first = options.sendTo[0];
    if (!first) return { kind: "none" };
    return first.repoId ? { kind: "change", change: { home: first.repoId } } : { kind: "direct", target: first };
  }
  const change: DefaultChange = { skip: seg === "local" ? [ALL] : [] };
  if (!copySource(row.homeKind)) change.home = "";
  return { kind: "change", change };
}
