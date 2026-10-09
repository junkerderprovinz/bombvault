import {
  PLACEMENT_DOMAINS,
  type DefaultChange,
  type DefaultRow,
  type HomeKind,
  type HomeOption,
  type ObservedPlace,
  type PlacementChange,
  type PlacementDomain,
  type PlacementObserved,
  type PlacementOptions,
  type PlacementPlan,
  type PlacementView,
  type PlanKind,
  type SegmentId,
  type SegmentLockReason,
  type SendToOption,
  type StackNote,
} from "./api";
import { repoDisplayName } from "./directRepo";
import type { TranslationKey, useT } from "./i18n";
import { withLtrIsolates } from "./ltrFragments";
import { formatTs } from "./reltime";

type T = ReturnType<typeof useT>["t"];

const ALL = "*";

function skipsAll(skip: string[]): boolean {
  return skip.length === 1 && skip[0] === ALL;
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
 *  is switched off or has no row any more. A direct repository the lists leave
 *  out, because its target is switched off, still takes backups and goes by
 *  its own name. */
export function viewHomeLabel(t: T, host: string, view: PlacementView, options: PlacementOptions): string {
  const listed = homeLabel(t, host, view.repo, options);
  if (listed !== null) return listed;
  if (view.repo === "") return host;
  const name = repoDisplayName(t, view.repoLabel, view.repoDirectOf) || view.repo;
  if (view.repoOff) return t("placement.off").replace("{name}", () => name);
  if (view.repoKind === "direct") return name;
  return t("placement.unknown").replace("{name}", () => name);
}

/** The bracketed "off" word alone, the same one viewHomeLabel and TargetChips
 *  append to a switched-off target's name, for a badge that carries only that
 *  word next to the name rather than folded into it. */
export function offQualifier(t: T): string {
  return t("placement.off").replace("{name}", () => "").trim();
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
    repoTarget: row.homeTarget,
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
  // skip is what the item copies once the direct repository exists.
  | { kind: "direct"; target: SendToOption; skip?: string[] }
  | { kind: "blocked"; reason: "home-fixed" | "last" | "remote" }
  | { kind: "none" };

const NONE: PlacementStep = { kind: "none" };

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

export function chipTicked(view: PlacementView, targetId: string): boolean {
  return !skipsAll(view.skip) && !view.skip.includes(targetId);
}

export function stepForReset(view: PlacementView): PlacementStep {
  const copies = { follow: true } as const;
  return view.locked
    ? { kind: "save", change: { copies }, confirmHome: null, optimistic: { copiesFollow: true } }
    : { kind: "save", change: { home: { follow: true }, copies }, confirmHome: null, optimistic: { homeFollows: true, copiesFollow: true } };
}

const LOCAL_KINDS: readonly (HomeKind | "")[] = ["domain", "domain-remote", "local"];

/** The buttons a placement shows lit: Local when it is the home, the target a
 *  direct home belongs to, and every target a copy goes to. ticked keeps the
 *  order of options.targets and holds the home's own target too. */
export interface PlacementButtons {
  local: boolean;
  home: string | null;
  ticked: string[];
}

export function placementButtons(view: PlacementView, options: PlacementOptions): PlacementButtons {
  const local = LOCAL_KINDS.includes(view.repoKind);
  const home = view.repoKind === "direct" ? homeTargetOf(view, options) : null;
  const copying = local || home !== null;
  const ticked = options.targets.filter((x) => x.id === home || (copying && chipTicked(view, x.id))).map((x) => x.id);
  return { local, home, ticked };
}

function homeTargetOf(view: PlacementView, options: PlacementOptions): string | null {
  return view.repoTarget || options.sendTo.find((s) => s.repoId !== "" && s.repoId === view.repo)?.targetId || null;
}

/** skipFor is the skip list that copies to exactly the given targets. A list
 *  of every other target lets a target added later arrive ticked, as it does
 *  everywhere; nothing at all is ["*"]. */
function skipFor(known: string[], copies: string[]): string[] {
  return copies.length === 0 ? [ALL] : known.filter((id) => !copies.includes(id));
}

const BLOCKED_FIXED: PlacementStep = { kind: "blocked", reason: "home-fixed" };
const BLOCKED_LAST: PlacementStep = { kind: "blocked", reason: "last" };
const BLOCKED_REMOTE: PlacementStep = { kind: "blocked", reason: "remote" };

// moveHomeTo makes a target's direct repository the home and copies to rest.
function moveHomeTo(targetId: string, rest: string[], view: PlacementView, options: PlacementOptions, t: T): PlacementStep {
  if (view.locked) return BLOCKED_FIXED;
  const s = options.sendTo.find((x) => x.kind === "direct" && x.targetId === targetId);
  if (!s) return NONE;
  const skip = skipFor(
    options.targets.map((x) => x.id),
    rest
  );
  if (!s.repoId) return { kind: "direct", target: s, skip };
  return {
    kind: "save",
    change: { home: { repo: s.repoId }, copies: { skip } },
    confirmHome: sendToLabel(t, s),
    optimistic: {
      repo: s.repoId,
      repoKind: "direct",
      repoTarget: targetId,
      repoLabel: s.name,
      homeFollows: false,
      skip,
      copiesFollow: false,
    },
  };
}

/** stepForLocal turns Local on, which brings the item home and keeps every
 *  lit target as a copy, or off, which makes the first lit target the home. */
export function stepForLocal(on: boolean, view: PlacementView, options: PlacementOptions, t: T, host: string): PlacementStep {
  const b = placementButtons(view, options);
  if (on === b.local) return NONE;
  if (!on) {
    const [first, ...rest] = b.ticked;
    return first ? moveHomeTo(first, rest, view, options, t) : BLOCKED_LAST;
  }
  if (view.locked) return BLOCKED_FIXED;
  const d = options.default;
  const repo = LOCAL_KINDS.includes(d.homeKind) ? d.home : "";
  const skip = skipFor(
    options.targets.map((x) => x.id),
    b.ticked
  );
  const home = options.homes.find((h) => h.id === repo);
  return {
    kind: "save",
    change: { home: { repo }, copies: { skip } },
    confirmHome: homeLabel(t, host, repo, options) ?? host,
    optimistic: { repo, repoKind: home?.kind ?? "domain", repoTarget: undefined, repoLabel: home?.name ?? "", homeFollows: false, skip, copiesFollow: false },
  };
}

/** stepForTarget lights or darkens one target. Darkening the home moves it to
 *  the next lit target; a placement without Local or a home gets its first
 *  target as the home. One button always stays lit. */
export function stepForTarget(id: string, on: boolean, view: PlacementView, options: PlacementOptions, t: T): PlacementStep {
  if (view.repoKind === "remote") return BLOCKED_REMOTE;
  const b = placementButtons(view, options);
  if (on === b.ticked.includes(id)) return NONE;
  if (!on && b.home === id) {
    const [next, ...others] = b.ticked.filter((x) => x !== id);
    return next ? moveHomeTo(next, others, view, options, t) : BLOCKED_LAST;
  }
  if (!b.local && b.home === null) return on ? moveHomeTo(id, [], view, options, t) : NONE;
  const lit = on ? [...b.ticked, id] : b.ticked.filter((x) => x !== id);
  if (lit.length === 0 && !b.local) return BLOCKED_LAST;
  const skip = skipFor(
    options.targets.map((x) => x.id),
    lit.filter((x) => x !== b.home)
  );
  return { kind: "save", change: { copies: { skip } }, confirmHome: null, optimistic: { skip, copiesFollow: false } };
}

/** newTargetRefused is the answer to a destination click on a placement that
 *  takes no copies, given before the click creates a target for nothing. */
export function newTargetRefused(view: PlacementView): PlacementStep | null {
  return view.repoKind === "remote" ? BLOCKED_REMOTE : null;
}

/** stepForNewTarget lights a target that a destination has just created for
 *  the domain, which the options read before it existed do not list yet. */
export function stepForNewTarget(id: string, view: PlacementView, options: PlacementOptions): PlacementStep {
  const refused = newTargetRefused(view);
  if (refused) return refused;
  const b = placementButtons(view, options);
  if (!b.local && b.home === null) return NONE;
  const known = [...options.targets.map((x) => x.id), id];
  const skip = skipFor(known, [...b.ticked.filter((x) => x !== b.home), id]);
  return { kind: "save", change: { copies: { skip } }, confirmHome: null, optimistic: { skip, copiesFollow: false } };
}

/** defaultChangeOf is a step's change as a default takes it. */
export function defaultChangeOf(step: PlacementStep): DefaultChange | null {
  if (step.kind !== "save") return null;
  const change: DefaultChange = {};
  if (step.change.home && "repo" in step.change.home) change.home = step.change.home.repo;
  if (step.change.copies && "skip" in step.change.copies) change.skip = step.change.copies.skip;
  return change;
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

export interface StatusLine {
  text: string;
  tone: "normal" | "warn" | "unconfirmed" | "muted";
}

export const STATUS_TONE_CLASS: Record<StatusLine["tone"], string> = {
  normal: "text-carbon-textSub",
  warn: "text-statusFail",
  unconfirmed: "text-statusWarn",
  muted: "text-carbon-textMuted",
};

const WHERE: Record<Exclude<PlanKind, "paused" | "not-backed-up">, TranslationKey> = {
  home: "placement.planHome",
  "stays-domain": "placement.planStays",
  "default-home": "placement.planDefaultHome",
  "decides-at-first-backup": "placement.planDecides",
};

export function planLines(
  t: T,
  lang: string,
  host: string,
  domain: PlacementDomain,
  plan: PlacementPlan,
  noTargets: boolean
): StatusLine[] {
  const home = repoDisplayName(t, plan.home, plan.homeDirectOf) || host;
  const kind = plan.kind;
  if (kind === "paused") return [{ text: t("placement.paused"), tone: "warn" }];
  if (kind === "not-backed-up") {
    const text =
      plan.reason === "default-off"
        ? t("placement.planNotBackedUpOff").replace("{home}", () => home)
        : t("placement.planNotBackedUpMissing");
    return [{ text, tone: "warn" }];
  }
  const tone: StatusLine["tone"] = plan.warn ? "warn" : "normal";
  const lines: StatusLine[] = [{ text: t(WHERE[kind]).replace("{home}", () => home), tone }];
  if (plan.targets.length > 0) {
    lines.push({
      text: t("placement.planCopied").replace("{targets}", () => formatList(lang, plan.targets)),
      tone: "normal",
    });
  } else if (noTargets) {
    lines.push({ text: t("placement.planNoTarget").replace("{domain}", () => domainLabel(t, domain)), tone });
  } else if (plan.noCopy) {
    lines.push({ text: t("placement.planNoCopy"), tone: "warn" });
  }
  return lines;
}

const RULE_321: Record<PlacementObserved["rule321"], { key: TranslationKey; tone: StatusLine["tone"] }> = {
  met: { key: "placement.rule321Met", tone: "normal" },
  "one-copy": { key: "placement.rule321OneCopy", tone: "warn" },
  unconfirmed: { key: "placement.rule321Unconfirmed", tone: "unconfirmed" },
};

function sitesLine(t: T, sites: number): StatusLine {
  return {
    text: sites === 1 ? t("placement.sitesOne") : t("placement.sites").replace("{n}", String(sites)),
    tone: "normal",
  };
}

function ruleLine(t: T, rule321: PlacementObserved["rule321"]): StatusLine {
  const rule = RULE_321[rule321];
  return { text: t(rule.key), tone: rule.tone };
}

export function observedLine(t: T, observed: PlacementObserved): StatusLine[] {
  if (observed.noBackup) return [{ text: t("placement.noBackup"), tone: "muted" }];
  const lines: StatusLine[] = [sitesLine(t, observed.sites)];
  for (const p of observed.places) {
    if (p.place !== "local") lines.push(placeLine(t, p));
  }
  lines.push(ruleLine(t, observed.rule321));
  return lines;
}

/** zfsObservedLine is a ZFS entry's 3-2-1 line. The server counts a current
 *  replica as a site off the premises, but a replica is no backup, so an
 *  entry with nothing else says so. */
export function zfsObservedLine(
  t: T,
  item: { sites: number; rule321: PlacementObserved["rule321"]; lastBackup: number }
): StatusLine[] {
  if (item.lastBackup > 0) return [sitesLine(t, item.sites), ruleLine(t, item.rule321)];
  if (item.sites <= 1) return [{ text: t("placement.noBackup"), tone: "muted" }];
  return [sitesLine(t, item.sites), { text: t("zfs.replica.rule321NoBackup"), tone: "warn" }];
}

// Dates follow the browser's locale, as formatTs and every other date in the app do.
function placeLine(t: T, p: ObservedPlace): StatusLine {
  const at = (key: TranslationKey) => t(key).replace("{place}", () => p.label);
  switch (p.state) {
    case "counts":
      return { text: at("placement.seen").replace("{time}", () => formatTs(p.seenAt)), tone: "normal" };
    case "unreachable":
      return {
        text:
          p.seenAt > 0
            ? at("placement.unreachable")
                .replace("{since}", () => formatTs(p.since))
                .replace("{time}", () => formatTs(p.seenAt))
            : at("placement.stateUnknown").replace("{since}", () => formatTs(p.since)),
        tone: "warn",
      };
    case "unknown":
      return {
        text:
          p.since > 0
            ? at("placement.stateUnknown").replace("{since}", () => formatTs(p.since))
            : at("placement.notListedYet"),
        tone: "muted",
      };
    case "old-copy":
      return {
        text: at("placement.oldCopy").replace("{date}", () => new Date(p.latest * 1000).toLocaleDateString()),
        tone: "muted",
      };
    case "no-copy":
      return { text: at("placement.noCopyYet"), tone: "muted" };
    case "off":
      return { text: t("placement.off").replace("{name}", () => p.label), tone: "muted" };
  }
}

export function stackNoteText(t: T, lang: string, host: string, note: StackNote): string {
  const text =
    note.targets.length > 0
      ? t("placement.stackNote").replace("{targets}", () => formatList(lang, note.targets))
      : t("placement.stackNoteNoCopy");
  return text.replace("{project}", () => note.project).replace("{home}", () => note.home || host);
}
