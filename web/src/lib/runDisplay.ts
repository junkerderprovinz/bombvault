// ---------------------------------------------------------------------------
// Run display; the shared run-presentation helpers.
//
// Six helpers: the run kind and target labels (runDomainLabel, runKindLabel,
// isDomainOpRunKind, runTargetText) and the status chip pair (statusTone,
// statusLabel).
//
// Why a lib at all: the RunDetailSheet renders the same
// kind/target title and the same status Badge the dashboard's RunsCard
// renders. A second copy of these helpers inside a component would be a
// forked vocabulary; the exact drift this extraction exists to prevent;
// so a shared module is the alternative, and the dashboard keeps
// compiling against the same functions from their new home.
//
// Layering note; the one import from components/ is type-only:
//
//   import type { BadgeTone } from "../components/Badge";
//
// statusTone's return type is the shared Badge's tone union; re-spelling it
// as a local string union would create a second definition that can drift
// from the Badge's. Type-only imports are erased at compile time, so no
// runtime edge from lib upward to components exists; which is the substance
// of the "lib imports nothing from pages or components" law. Precedent:
// lib/navModel.ts imports icon components from components/navGlyphs, the
// documented load-bearing exception; this is that exception in its weakest
// possible form (a type position only, gone in the emitted JS).
// ---------------------------------------------------------------------------
import type { Run } from "./api";
import type { useT } from "./i18n";
// Type-only import; see the layering note in the header comment.
import type { BadgeTone } from "../components/Badge";

// ---------------------------------------------------------------------------
// Run kind/target label helpers; shared by every surface that renders a
// Run's kind and target (dashboard RunsCard, SummaryTier's "Last result"
// cell, the RunDetailSheet). A prune/verify run's targetId is the domain
// literal it ran against ("containers"/"vms"/"files", or
// store.FlashTargetID/ConfigTargetID; "flash"/"config"; see
// internal/api/service.go domainRunTargetID), never a resolvable item id, so
// it needs its own kind label + domain-name resolution instead of falling
// through to the generic backup/restore/update display (which would
// otherwise show it mislabeled as "Restore" with a blank/truncated target;
// #run-activity-log finding 1).
// ---------------------------------------------------------------------------

export function runDomainLabel(t: ReturnType<typeof useT>["t"], domain: string): string {
  switch (domain) {
    case "containers":
      return t("activityLog.domainContainers");
    case "vms":
      return t("activityLog.domainVMs");
    case "flash":
      return t("activityLog.domainFlash");
    case "config":
      return t("activityLog.domainConfig");
    case "files":
      return t("activityLog.domainFiles");
    default:
      return domain;
  }
}

export function runKindLabel(t: ReturnType<typeof useT>["t"], kind: string): string {
  switch (kind) {
    case "backup":
      return t("run.kindBackup");
    case "restore":
      return t("run.kindRestore");
    case "update":
      return t("run.kindUpdate");
    case "prune":
      return t("activityLog.typePrune");
    case "verify":
      return t("activityLog.typeVerify");
    case "offsite":
      return t("activityLog.typeOffsite");
    case "drill":
      return t("activityLog.jobDrill");
    case "drdrill":
      return t("run.kindDRDrill");
    case "tamper":
      return t("activityLog.jobTamper");
    case "export":
      return t("run.kindExport");
    default:
      // An unknown future kind shows its raw literal rather than a wrong label.
      return kind;
  }
}

// isDomainOpRunKind mirrors the backend's domainRunTargetID users: these kinds
// carry the domain literal (or the flash/config singleton id) in targetId, never
// a resolvable item id.
export function isDomainOpRunKind(kind: string): boolean {
  return kind === "prune" || kind === "verify" || kind === "offsite" || kind === "drill" || kind === "drdrill" || kind === "tamper" || kind === "export";
}

// runTargetText resolves what to show in a run's "target" column. Domain-op
// runs (prune/verify/offsite/drill/tamper/export) carry the domain literal in
// targetId (see above); reuse the same domain-name keys the activity log uses
// instead of the generic target/targetId fallback, which would show the raw
// literal (e.g. "containers…") since it is never in the backend's
// target-name map.
export function runTargetText(t: ReturnType<typeof useT>["t"], run: Run): string {
  if (isDomainOpRunKind(run.kind)) {
    return runDomainLabel(t, run.targetId);
  }
  // The Backup Everything parent run's target carries the backend's own
  // server-side literal, English text whatever the UI language. One action,
  // one name: settings.everythingTitle, the same key the effective-schedule
  // sentence resolves, so the run sheet's title reads Gesamt-Backup in
  // German instead of pairing a translated kind with the raw name.
  if (run.domain === "everything") {
    return t("settings.everythingTitle");
  }
  return run.target || `${run.targetId.slice(0, 12)}…`;
}

// ---------------------------------------------------------------------------
// Status chip. statusTone maps a raw status string to the shared Badge's
// tone, statusLabel maps the same string to translated, badge-length text,
// and every call site renders the pair together:
// `<Badge tone={statusTone(s)}>{statusLabel(s, t)}</Badge>`. A Badge given
// only the tone prints the backend's English word, which is what
// `run.statusRunning`, `run.statusSuccess` and `run.statusFailed` exist for
// in all 42 locales. chipFor, chipForRpo and protectionChip below return this
// file's own derived vocabulary rather than backend text, so they are
// translated the same way.
//
// Known limitation, documented on purpose (index.css's comment on
// --status-warn-text's dark value carries the full writeup): tone="warn" and
// tone="active" render as near-identical amber wherever the accent is gold or
// yellow. Dark with the default accent puts them an RGB distance of about 11
// apart (#f1c21b against #FCC419, 1.05:1 between them), and light on a gold
// accent mixes --accent-text down to roughly the warn hue (#8e6a00 against
// about #71580b on the default Sunflower). SummaryTier is a live site where
// both can sit in one row: "Overall health" on tone="warn" for a lapsing RPO
// beside "Last result" on tone="active" for a run that is going. Each badge's
// own text still differs, so this is not an SC 1.4.1 violation, but it is a
// glance-level one. Any other accent separates the two by hue, because
// --accent-text is derived from the accent the user picked. Recolouring warn
// off Carbon's yellow token, changing the default accent or giving Badge an
// icon system all reach well past a contrast fix, so it stays as it is.
// ---------------------------------------------------------------------------

export function statusTone(status: string): BadgeTone {
  switch (status.toLowerCase()) {
    case "success":
    case "ok":
      return "ok";
    case "failed":
    case "degraded":
      return "fail";
    // Genuine activity (the fifth hue); a run that is
    // literally in progress right now. "active" is the accent-soft Badge
    // tone, not a solid fill: this list can show several running rows at
    // once (independent domains backing up concurrently), and rule 3's
    // "at most one solid accent" cap doesn't apply to a soft/tinted chip
    // reading at the same weight as its ok/fail/warn siblings.
    case "running":
    case "checking":
      return "active";
    // The literal backend/derived string "info" (chipForRpo's "warn" SLA
    // lapse, protectionChip's "amber" aggregate, chipFor's best-effort
    // check failure) always meant a real caution, never activity; routes
    // to warn, matching SpikePanel.tsx's own hard-coded tone="warn" for the
    // identical best-effort-fail case. Unaffected by the "active" rename
    // above; this is a separate switch arm.
    case "info":
      return "warn";
    // A skip is neither success nor failure: a muted, neutral chip so a removed
    // container's scheduled target reads as "intentionally not run", distinct
    // from green success and red failure (#57).
    case "skipped":
      return "neutral";
    default:
      return "neutral";
  }
}

// statusLabel is statusTone's translation-side twin. It keys off
// the same tone bucket statusTone already computes; not the raw string a
// second time; so every raw word any helper below can produce (chipFor:
// ok/info/failed; chipForRpo: success/info/failed/neutral; protectionChip:
// ok/info/failed/neutral; run.status/newestRun.status: success/failed/
// running/skipped) resolves to translated, badge-length text without a
// second switch to keep in sync. Reuses spike.ok/spike.fail/spike.info;
// SpikePanel.tsx's own ok/fail/info wording; for the
// ok/fail/warn buckets, and run.statusRunning for active (added alongside
// statusSuccess/statusFailed back when this bug was first anticipated, then
// never wired up until now). neutral/default gets the one genuinely new key
// the neutral bucket needs, run.statusSkipped, translated into all 42 locales.
export function statusLabel(status: string, t: ReturnType<typeof useT>["t"]): string {
  // Checked before the tone lookup, because "cancelled" and "skipped" share the
  // neutral tone and the tone is all the switch below can see. A restore the
  // user cancelled therefore read "Skipped" in Last Result and in the run
  // history; a different claim about a different event, and the one the user
  // themselves had just caused.
  if (status === "cancelled") return t("run.statusCancelled");
  switch (statusTone(status)) {
    case "ok":
      return t("spike.ok");
    case "fail":
      return t("spike.fail");
    case "warn":
      return t("spike.info");
    case "active":
      return t("run.statusRunning");
    default:
      return t("run.statusSkipped");
  }
}
