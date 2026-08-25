// ---------------------------------------------------------------------------
// Fleet page — the READ-ONLY fleet view. Watches a list of PEER BombVault
// instances and shows each one's cached protection scorecard (same red/amber/
// green aggregate the local dashboard shows). Polling GET /api/fleet/status is
// the only thing this box does TO a peer's protection data — never a remote
// backup/restore/drill trigger. The one exception is Mesh off-site: this page
// can also PROPOSE this instance's own off-site storage to a peer (sending
// connection metadata only, never backup data) and review storage offers a
// peer has sent here; BombVault still never hosts storage itself, accepting
// an offer only ever creates a normal named credential set + off-site target.
//
// Gated behind settings.fleetEnabled (the Fleet tab only shows when on).
// Freshness comes from the daily scheduled sweep + the explicit "Poll now"
// button, never an implicit poll from opening this page (a peer poll is a
// real network round-trip to another site). Modeled on Receiver.tsx.
// ---------------------------------------------------------------------------

import { useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  listFleetPeers,
  createFleetPeer,
  updateFleetPeer,
  deleteFleetPeer,
  pollFleetPeer,
  listMeshOffers,
  acceptMeshOffer,
  declineMeshOffer,
  proposeMeshOffer,
} from "../lib/api";
import type { FleetPeer, FleetPeerInput, DomainStatus, MeshOffer, DeploySnippetData } from "../lib/api";
import { credSetsChanged } from "../lib/useCloudCredSets";
import { offsiteTargetsChanged } from "../lib/useOffsiteTargets";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconFleet } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { copyText } from "../lib/clipboard";
import { useToast } from "../lib/toast";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";

type T = ReturnType<typeof useT>["t"];

// CopyBlock mirrors OffsiteWizard's copy pattern (module-private there too): a
// monospace <pre> with a copy button. GlimStone form-engine Task 9 (toasts):
// this was one of the ad-hoc 2000ms-inline-text-swap "copied" patterns the
// audit flagged as a natural toast candidate — the "Copied" feedback now
// surfaces as a routine (quiet-mode-suppressible) toast instead of the
// button's own label flipping for two seconds. While here: switched the raw
// `navigator.clipboard.writeText` call to this repo's shared `copyText()`
// helper (lib/clipboard.ts), which already exists specifically because a
// direct call silently does nothing on a non-secure (plain HTTP) origin —
// this was the one remaining call site still bypassing it (#112).
function CopyBlock({ text, t }: { text: string; t: T }) {
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Copy button alongside its existing toast on a failed copy.
  const [shake, setShake] = useState(0);
  async function copy() {
    if (await copyText(text)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      // "failures always surface" (design-language.md) — copyText() only
      // returns false when BOTH the Clipboard API and the execCommand
      // fallback failed, so this is a real, user-actionable failure, not
      // routine noise a quiet-mode user would want suppressed.
      push(t("vm.ssh.copyFailed"), "fail");
      setShake((n) => n + 1);
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <button
        key={shake}
        type="button"
        onClick={() => void copy()}
        className={`shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover${
          shake ? " glim-shake" : ""
        }`}
      >
        {t("vm.ssh.copy")}
      </button>
    </div>
  );
}

const MESH_DOMAINS = ["containers", "vms", "flash", "config", "files"] as const;

// ---------------------------------------------------------------------------
// Protection chip — mirrors Dashboard's protectionChip mapping (not exported
// there, so duplicated here: green/amber/red/"" -> ok/warn/fail/neutral).
// ---------------------------------------------------------------------------

function protectionTone(level: string): "ok" | "fail" | "warn" | "neutral" {
  switch (level) {
    case "green":
      return "ok";
    case "amber":
      return "warn";
    case "red":
      return "fail";
    default:
      return "neutral";
  }
}

// ---------------------------------------------------------------------------
// Cached scorecard (one row per domain the peer reported)
// ---------------------------------------------------------------------------

// Domain -> label key. An explicit map (not a template literal) so every
// lookup is a real, statically-checked TranslationKey.
const DOMAIN_LABEL_KEYS: Record<string, TranslationKey> = {
  containers: "settings.containersEnabled",
  vms: "settings.vmsEnabled",
  flash: "settings.flashEnabled",
  files: "settings.filesEnabled",
  config: "settings.configEnabled",
};

function domainLabelKey(domain: string): TranslationKey {
  return DOMAIN_LABEL_KEYS[domain] ?? "settings.containersEnabled";
}

function protectionLabelKey(level: string): TranslationKey {
  switch (level) {
    case "green":
      return "fleet.protection.green";
    case "amber":
      return "fleet.protection.amber";
    case "red":
      return "fleet.protection.red";
    default:
      return "fleet.protection.none";
  }
}

function PeerScorecard({ domains, t }: { domains: DomainStatus[]; t: T }) {
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  if (shown.length === 0) {
    return <p className="py-2 text-xs text-carbon-textMuted">{t("fleet.noScorecard")}</p>;
  }
  return (
    <div className="mt-2 flex flex-col gap-1.5">
      {shown.map((d) => (
        <div key={d.domain} className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-carbon-textSub w-20 shrink-0">{t(domainLabelKey(d.domain))}</span>
          <Badge tone={protectionTone(d.protection)}>{t(protectionLabelKey(d.protection))}</Badge>
          {d.lastSuccess > 0 && (
            <span className="text-xs text-carbon-textMuted">
              {t("fleet.lastBackup").replace("{time}", relativeTime(t, d.lastSuccess))}
            </span>
          )}
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Mesh: offers received FROM peers
// ---------------------------------------------------------------------------

function meshStatusTone(status: string): "ok" | "fail" | "warn" | "neutral" {
  switch (status) {
    case "accepted":
      return "ok";
    case "declined":
      return "fail";
    default:
      return "warn"; // pending
  }
}

function meshStatusLabelKey(status: string): TranslationKey {
  switch (status) {
    case "accepted":
      return "fleet.mesh.status.accepted";
    case "declined":
      return "fleet.mesh.status.declined";
    default:
      return "fleet.mesh.status.pending";
  }
}

function MeshOfferRow({ offer, t, onChanged }: { offer: MeshOffer; t: T; onChanged: () => void }) {
  const [domain, setDomain] = useState<string>(offer.suggestedDomain || "containers");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // whichever button was actually clicked — separate nonces since Accept/
  // Decline are two different failable actions.
  const [shakeAccept, setShakeAccept] = useState(0);
  const [shakeDecline, setShakeDecline] = useState(0);

  async function handleAccept() {
    setBusy(true);
    try {
      const res = await acceptMeshOffer(offer.id, domain);
      if (res.ok) {
        // Accepting mints BOTH a named credential set (holding the peer's REST
        // credentials) and an off-site target — announce both so any mounted
        // reader of either list is current, not just this page's offer rows
        // (#173's invalidation contract; see useCloudCredSets).
        credSetsChanged();
        offsiteTargetsChanged();
        onChanged();
      } else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShakeAccept((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeAccept((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  async function handleDecline() {
    setBusy(true);
    try {
      const res = await declineMeshOffer(offer.id);
      if (res.ok) onChanged();
      else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShakeDecline((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeDecline((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const pending = offer.status === "pending";

  return (
    <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="font-semibold text-carbon-text text-sm truncate">{offer.from || t("fleet.mesh.unknownPeer")}</span>
        <Badge tone={meshStatusTone(offer.status)}>{t(meshStatusLabelKey(offer.status))}</Badge>
        <span className="text-xs text-carbon-textMuted ms-auto">{relativeTime(t, offer.receivedAt)}</span>
      </div>
      <p dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate text-start">{offer.repo}</p>
      {pending && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("fleet.mesh.applyTo")}
            <select
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 bv-field-focus-well"
            >
              {MESH_DOMAINS.map((d) => (
                <option key={d} value={d}>{t(domainLabelKey(d))}</option>
              ))}
            </select>
          </label>
          <button
            key={shakeAccept}
            onClick={() => void handleAccept()}
            disabled={busy}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeAccept ? " glim-shake" : ""
            }`}
          >
            {t("fleet.mesh.accept")}
          </button>
          <button
            key={shakeDecline}
            onClick={() => void handleDecline()}
            disabled={busy}
            className={`inline-flex items-center rounded-control bg-carbon-surface3 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50${
              shakeDecline ? " glim-shake" : ""
            }`}
          >
            {t("fleet.mesh.decline")}
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Mesh: propose this instance's own storage TO a peer
// ---------------------------------------------------------------------------

function ProposeMeshDialog({ peer, t, onClose }: { peer: FleetPeer; t: T; onClose: () => void }) {
  const [domain, setDomain] = useState<string>("containers");
  const [baseUrl, setBaseUrl] = useState("");
  const [sending, setSending] = useState(false);
  const { push } = useToast();
  const [snippet, setSnippet] = useState<(DeploySnippetData & { repo: string }) | null>(null);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Send button alongside the toast on a failed send.
  const [shake, setShake] = useState(0);

  // GlimStone follow-up pass (v8.0.0): the form-stage "error" flash below is
  // now a toast (matches this file's already-migrated CopyBlock/MeshOfferRow
  // shape) — the client-side baseUrlRequired check is reachable through the
  // UI (Send isn't disabled while baseUrl is blank), so it gets the same
  // push() treatment as the API failure. The POST-send `snippet` view further
  // down is deliberately UNCHANGED: "Offer sent" + the docker-run/compose
  // blocks are a persistent reference the user copies down, not a one-shot
  // ping — same reasoning as ExportButton/RestoreProgress's reference values.
  async function handleSend() {
    if (baseUrl.trim() === "") {
      push(t("fleet.mesh.baseUrlRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSending(true);
    try {
      const res = await proposeMeshOffer(peer.id, domain, baseUrl.trim());
      if (res.ok && res.snippet) setSnippet(res.snippet);
      else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSending(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus";

  // `items-center`, NOT `items-start` (whole-app sweep — this was one of the
  // three sites Files.tsx's own FileSetDialog comment explicitly recorded as
  // "same fix still owed", after that round scoped itself to the Ordner tab).
  // Top-anchored, this dialog's heading Badge poked to 5px below the literal
  // browser-viewport edge — measured live on the deployed container at a
  // 1000px-tall viewport: badge top = 5px — reading as a flat bar jammed into
  // the screen corner rather than a notch straddling the card. Safe here for
  // the identical reason it is safe in ConfirmDialog/WhatsNewDialog/
  // ErrorDetailPanel/FileSetDialog: the visible box below is capped at
  // `max-h-[90vh]`, strictly under the 100vh flex container, so a centred
  // item's top offset is always positive and never clips off-screen, while
  // `overflow-y-auto` on this backdrop still covers content that grows toward
  // the cap.
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-4" onClick={onClose}>
      {/* GlimStone follow-up pass ("half-overlap card notch"): non-scrolling
          `relative` shell wraps the scrollable dialog box, same split as
          Receiver.tsx's ReceiverDialog — see that call site's comment. */}
      <div className="relative w-full max-w-lg">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>{t("fleet.mesh.proposeTitle")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("fleet.mesh.proposeTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.proposeHint").replace("{peer}", peer.name)}</p>

        {!snippet ? (
          <>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("fleet.mesh.domain")}</label>
              <select value={domain} onChange={(e) => setDomain(e.target.value)} className={inputCls}>
                {MESH_DOMAINS.map((d) => (
                  <option key={d} value={d}>{t(domainLabelKey(d))}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("fleet.mesh.baseUrl")}</label>
              <input
                type="text"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                spellCheck={false}
                autoComplete="off"
                placeholder="http://192.168.1.50:8000"
                dir="ltr"
                className={`${inputCls} font-mono text-start`}
              />
              <p className="text-caption text-carbon-textMuted">{t("fleet.mesh.baseUrlHint")}</p>
            </div>
            <div className="flex items-center justify-end gap-2 pt-1">
              <button
                onClick={onClose}
                disabled={sending}
                className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
              >
                {t("files.cancel")}
              </button>
              <button
                key={shake}
                onClick={() => void handleSend()}
                disabled={sending}
                className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
                  shake ? " glim-shake" : ""
                }`}
              >
                {sending ? t("fleet.mesh.sending") : t("fleet.mesh.send")}
              </button>
            </div>
          </>
        ) : (
          <>
            <p className="text-xs text-statusOk">{t("fleet.mesh.sent").replace("{peer}", peer.name)}</p>
            <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.deployNow")}</p>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("fleet.mesh.dockerRun")}</span>
              <CopyBlock text={snippet.dockerRun} t={t} />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("fleet.mesh.compose")}</span>
              <CopyBlock text={snippet.compose} t={t} />
            </div>
            <div className="flex items-center justify-end pt-1">
              <button
                onClick={onClose}
                className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
              >
                {t("common.close")}
              </button>
            </div>
          </>
        )}
      </div>
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// Peer card
// ---------------------------------------------------------------------------

function FleetPeerCard({
  peer,
  t,
  onRefresh,
  onEdit,
  index,
}: {
  peer: FleetPeer;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  colour engine), matching Containers.tsx's ContainerRow / VMs.tsx's VMRow /
   *  Files.tsx's FileSetRow: a peer list is exactly the case the mode exists
   *  for, a variable, user-configured set someone tracks several of at once.
   *  Assigned by LIST INDEX, never a hash of `peer.name` — see the caller
   *  below. */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [polling, setPolling] = useState(false);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  const [showPropose, setShowPropose] = useState(false);
  // Reversible action: removing a monitoring entry never contacts the peer
  // instance (re-addable in one step), so per the design-language's
  // "reversible actions don't ask" rule this gets the LIGHTER two-click
  // inline-confirm — click "Remove" → button becomes "Confirm remove" —
  // matching OffsiteTargetsSection's / Receiver.tsx's `confirmRemove`
  // pattern exactly, not a full window.confirm()/ConfirmDialog (form-engine
  // Task 7).
  const [confirmRemove, setConfirmRemove] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Poll/Remove buttons alongside their existing toasts on failure.
  const [shakePoll, setShakePoll] = useState(0);
  const [shakeRemove, setShakeRemove] = useState(0);

  // BUG FIX (found alongside this card's handleRemove migration below,
  // GlimStone follow-up pass v8.0.0): this never checked pollFleetPeer's
  // `res.ok`, and had no catch at all — a server-reported poll failure was
  // silently ignored, and a network-level failure threw past the missing
  // catch as an unhandled rejection, skipping onRefresh() with zero feedback
  // to the user either way. Now both are surfaced; onRefresh() still only
  // runs when the request itself resolved (preserving the original "reload
  // only on a real response" behavior), so the persistent pollTone/pollLabel
  // badge below picks up the server-recorded outcome exactly as before.
  async function handlePoll() {
    setPolling(true);
    try {
      const res = await pollFleetPeer(peer.id);
      if (!res.ok) {
        push(res.error ?? t("fleet.saveError"), "fail");
        setShakePoll((n) => n + 1);
      }
      onRefresh();
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShakePoll((n) => n + 1);
    } finally {
      setPolling(false);
    }
  }

  async function handleRemove() {
    setRemoving(true);
    try {
      const res = await deleteFleetPeer(peer.id);
      if (res.ok) {
        onRefresh();
        setConfirmRemove(false);
      } else {
        // Keep the two-click confirm UP on failure (don't reset to "Remove") —
        // otherwise the shake below would fire on a button that unmounts in
        // the same tick, and the user would lose their confirm click for a
        // failure that wasn't their mistake.
        push(res.error ?? t("fleet.saveError"), "fail");
        setShakeRemove((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShakeRemove((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  const pollTone: "ok" | "fail" | "neutral" = peer.lastPollOk === null ? "neutral" : peer.lastPollOk ? "ok" : "fail";
  const pollLabel =
    peer.lastPollOk === null ? t("fleet.pollNever") : peer.lastPollOk ? t("fleet.pollOk") : t("fleet.pollFailed");

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-hue owns the position; glim-tint washes the WHOLE card with it
      // (trap #2, design-language.md's "Rainbow" section) — same
      // relative/overflow-hidden/glim-hue/glim-tint shell as
      // ContainerRow/VMRow/FileSetRow, so a rainbow-mode Fleet list colours
      // each monitored peer instead of leaving every row the flat accent. No
      // glim-active here: unlike those three, a peer card has no
      // progressMap-tracked backup/restore job of its own to key it off —
      // Poll/Propose are quick request/response actions, not a tracked job.
      // bv-stagger-row (GlimStone motion-engine animation 3) — see
      // ContainerRow's identical comment.
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint bv-stagger-row"
    >
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {peer.lastPollInstanceName || peer.name}
            </span>
            {!peer.enabled && <Badge tone="neutral">{t("fleet.monitoringOff")}</Badge>}
            <Badge tone={pollTone}>{pollLabel}</Badge>
          </div>
          <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start">{peer.url}</p>
        </div>
        {peer.lastPollVersion && (
          <span className="text-xs text-carbon-textMuted shrink-0">v{peer.lastPollVersion}</span>
        )}
      </div>

      {peer.lastPollAt > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("fleet.lastPolled").replace("{time}", relativeTime(t, peer.lastPollAt))}
          {peer.lastPollOk === false && peer.lastPollError && (
            <span className="text-statusFail"> · {peer.lastPollError}</span>
          )}
        </p>
      )}

      <div className="flex items-center gap-3 flex-wrap">
        <button
          key={shakePoll}
          onClick={() => void handlePoll()}
          disabled={polling}
          className={`inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shakePoll ? " glim-shake" : ""
          }`}
        >
          {polling ? (
            <>
              <span
                className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("fleet.polling")}
            </>
          ) : (
            t("fleet.pollNow")
          )}
        </button>

        <div className="ms-auto flex items-center gap-2">
          <button
            onClick={() => setShowPropose(true)}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("fleet.mesh.proposeButton")}
          </button>
          <button
            onClick={() => setOpen((v) => !v)}
            className="inline-flex items-center gap-1.5 rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 12 12"
              fill="none"
              className={`transition-transform ${open ? "rotate-90" : "rtl:rotate-180"}`}
            >
              <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
            </svg>
            {t("fleet.details")}
          </button>
          <button
            onClick={onEdit}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("fleet.edit")}
          </button>
          {confirmRemove ? (
            <button
              key={shakeRemove}
              onClick={() => void handleRemove()}
              disabled={removing}
              className={`inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50${
                shakeRemove ? " glim-shake" : ""
              }`}
            >
              {removing ? t("fleet.removing") : t("fleet.confirmRemove")}
            </button>
          ) : (
            <button
              onClick={() => setConfirmRemove(true)}
              className="inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50"
            >
              {t("fleet.remove")}
            </button>
          )}
        </div>
      </div>

      {open && (
        <div className="rounded-card bg-carbon-background px-3 py-2">
          <p className="text-xs font-medium text-carbon-textSub">{t("fleet.scorecardTitle")}</p>
          <PeerScorecard domains={peer.lastPollDomains} t={t} />
        </div>
      )}
      {showPropose && <ProposeMeshDialog peer={peer} t={t} onClose={() => setShowPropose(false)} />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add / edit dialog
// ---------------------------------------------------------------------------

function FleetDialog({
  initial,
  t,
  onClose,
  onSaved,
}: {
  /** null = create; a peer = edit that peer. */
  initial: FleetPeer | null;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [token, setToken] = useState("");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const revealToken = useReveal();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Save button alongside the toast on a failed save.
  const [shake, setShake] = useState(0);

  const editing = initial !== null;
  const canSave = name.trim() !== "" && url.trim() !== "" && (token.trim() !== "" || editing) && !saving;

  // GlimStone follow-up pass (v8.0.0): the "error" flash below is now a
  // toast — same shape as Files.tsx's FileSetDialog.handleSave (a dialog
  // editor that closes on success via onSaved(), so a toast is the only
  // outcome notice left, success or failure). The two client-side
  // nameRequired/urlRequired checks are effectively unreachable through the
  // UI (canSave already disables Save for the same conditions), but get the
  // same push() treatment as the API failure below for consistency.
  async function handleSave() {
    if (name.trim() === "") {
      push(t("fleet.nameRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (url.trim() === "") {
      push(t("fleet.urlRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSaving(true);
    const input: FleetPeerInput = {
      name: name.trim(),
      url: url.trim(),
      token: token.trim(),
      enabled,
      sortOrder: initial?.sortOrder ?? 0,
    };
    try {
      const res = editing ? await updateFleetPeer(initial.id, input) : await createFleetPeer(input);
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("fleet.saveError"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus";

  // `items-center` — same whole-app sweep fix, and for the same measured
  // reason, as this file's own proposeTitle dialog above; see that call site's
  // comment for the full writeup.
  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      {/* GlimStone follow-up pass ("half-overlap card notch"): non-scrolling
          `relative` shell wraps the scrollable dialog box — see
          Receiver.tsx's ReceiverDialog and this file's own proposeTitle
          dialog above for the identical split. */}
      <div className="relative w-full max-w-lg">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>{editing ? t("fleet.editTitle") : t("fleet.addTitle")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={editing ? t("fleet.editTitle") : t("fleet.addTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("fleet.name")}</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className={inputCls}
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("fleet.url")}</label>
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="https://192.168.1.50:3443"
            dir="ltr"
            className={`${inputCls} font-mono text-start`}
          />
          <p className="text-caption text-carbon-textMuted">{t("fleet.urlHint")}</p>
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("fleet.token")}</label>
          <RevealInput
            {...revealToken}
            value={token}
            onChange={(e) => setToken(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder={editing ? t("fleet.tokenKeep") : "a1b2c3…"}
            wrapperClassName="w-full"
            className={`${inputCls} font-mono`}
          />
          <p className="text-caption text-carbon-textMuted">{t("fleet.tokenHint")}</p>
        </div>

        <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("fleet.enabledLabel")}
        </label>

        <div className="flex items-center justify-end gap-2 pt-1">
          <button
            onClick={onClose}
            disabled={saving}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("files.cancel")}
          </button>
          <button
            key={shake}
            onClick={() => void handleSave()}
            disabled={!canSave}
            className={`inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shake ? " glim-shake" : ""
            }`}
          >
            {saving ? t("common.saving") : t("settings.save")}
          </button>
        </div>
      </div>
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// Fleet page
// ---------------------------------------------------------------------------

export function Fleet() {
  const { t } = useT();
  // Registers this page for a re-render on any rainbow-state change (on/off/
  // reactive/rotate/palette edit) — the FleetPeerCard list below reads
  // rainbowAt()/hueVars() directly during render; see lib/useRainbow.ts's own
  // header for why a caller doesn't need the returned value.
  useRainbow();
  const [peers, setPeers] = useState<FleetPeer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a row = edit dialog for that peer.
  const [dialog, setDialog] = useState<"new" | FleetPeer | null>(null);
  const [offers, setOffers] = useState<MeshOffer[]>([]);

  function loadPeers() {
    return listFleetPeers()
      .then((res) => {
        if (res.ok) {
          setPeers(res.peers ?? []);
          setError(null);
        } else {
          setError(res.error ?? t("fleet.loadError"));
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("fleet.loadError")));
  }

  function loadOffers() {
    return listMeshOffers()
      .then((res) => {
        if (res.ok) setOffers(res.offers ?? []);
      })
      .catch(() => undefined);
  }

  useEffect(() => {
    void Promise.all([loadPeers(), loadOffers()]).finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const pendingOffers = offers.filter((o) => o.status === "pending");

  // jdp live review ("Fleet Tab: Button rechts oben ist redundant"): the
  // empty-state Card below already carries its own prominent "Add peer" CTA,
  // so showing the identical button a second time in the top-right actions
  // bar was pure duplication — confirmed both call the exact same handler
  // (`() => setDialog("new")`). Mirrors Files.tsx's/Receiver.tsx's own
  // showEmptyState fix for the identical pattern. Once a peer exists the
  // empty-state Card stops rendering and the top-right button is the page's
  // only entry point again, so "Add" is never unreachable.
  const showEmptyState = !loading && !error && peers.length === 0;

  // hueSeq/nextHue (GlimStone follow-up pass — see VMs.tsx's/Settings.tsx's
  // own identical hueSeq/nextHue comment for the full reasoning): a plain,
  // freshly-reset-every-render counter assigning 0,1,2,... to this page's
  // heading notches in the exact order the JSX below actually evaluates each
  // `hueIndex={nextHue()}` call — safe here because both call sites below are
  // plain `cond && (<div>…)` blocks evaluated directly in this component's
  // own JSX (never handed down as a prop into a child that might itself
  // decide not to render, the one shape that would evaluate a hueIndex
  // expression before its own gate — see VMs.tsx's <Advanced> caution).
  //   Two heading notches can exist on this page at once: the mesh-offers
  // Card (offers a PEER sent TO this instance, independent of whether this
  // instance is itself watching that peer) and the new empty-state Card
  // below (zero watched peers) — genuinely independent conditions, so both
  // CAN render together (a peer proposed storage here while this instance
  // watches no peers of its own yet). The mesh-offers badge used to be this
  // page's only heading notch and correctly rendered flat/un-rainbowed as a
  // genuine singleton (proactive sweep, memory always-integrate-new-
  // elements-into-color-modes: found live while adding the empty-state
  // Card's own badge) — neither is a singleton anymore once both can be
  // visible together, so both are threaded through this one counter instead.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    // PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich breit
    // machen?"): the gap here was already the correct 40px from the earlier
    // "Im Fleet Tab ist die Card zu weit oben" round; only the width changes,
    // max-w-5xl (1024px) → the shared 1152px. This page's heading is a single
    // bare h1+p row, so the one flat shell gap still governs every gap on it.
    // See lib/pageShell.ts for the full before/after table.
    <div className={PAGE_SHELL}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("fleet.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("fleet.subtitle")}</p>
        </div>
        {!showEmptyState && (
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity shrink-0"
          >
            {t("fleet.addPeer")}
          </button>
        )}
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {!loading && pendingOffers.length > 0 && (
        <div className="relative bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
          <div>
            {/* Task 5 (rule 11): outermost heading of this rounded-card p-4
                panel — not nested inside anything already badged — same
                Badge-in-<h2> treatment as every other converted Card
                heading. GlimStone follow-up pass ("half-overlap card
                notch"): `relative` added on the outer p-4 card above (not
                this bare inner div) — the heading Badge is now
                `position: absolute` and needs to straddle the padded card's
                real edge, not just this inner div's own (padding-less)
                position within it. hueIndex={nextHue()} (proactive
                rainbow-hue sweep, this round) — see this page's own
                hueSeq/nextHue comment above for why this is no longer a
                genuine singleton. */}
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>{t("fleet.mesh.offersTitle")}</Badge>
            </h2>
            <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.offersHint")}</p>
          </div>
          <div className="flex flex-col gap-2">
            {pendingOffers.map((o) => (
              <MeshOfferRow key={o.id} offer={o} t={t} onChanged={() => void loadOffers()} />
            ))}
          </div>
        </div>
      )}

      {/* Empty state — GlimStone follow-up pass (jdp live review: "Card hat
          keinen Cardtitelbadge mit dem Infotext der in der Card steht"): this
          card had no heading at all — just the icon, the permanent pitch
          paragraph, and the Add button — the one Card-shaped box on this page
          that never got the tone="heading" notch every other Card in the app
          carries. `relative glim-notch-card` (Files.tsx's own setsTitle Card
          precedent). The old permanent `<p>{t("fleet.empty")}</p>` reads once
          and then costs vertical space forever — moved verbatim onto the new
          heading Badge as an `onAccent` InfoBubble instead, zero new i18n
          keys for the body, only the new title key.
          hueIndex={nextHue()}, not a fixed 0: see this page's own
          hueSeq/nextHue comment above — the mesh-offers Card can render
          simultaneously with this one, so this can't assume it is always the
          sole heading notch on the page the way Receiver.tsx's identical
          empty-state Card can.
          insetStart={6} (GlimStone follow-up pass, jdp: "Empfaenger/Fleet-
          Tab: Cardtitelbadge falsch platziert" — the SAME `text-center
          items-center` collapsed-h2 mismatch as Files.tsx's own setsTitle
          Card and Receiver.tsx's identical empty-state Card; see Files.tsx's
          own call site for the full "why a single-merged-div Card can still
          get this wrong" mechanism and Badge.tsx's `insetStart` doc). */}
      {showEmptyState && (() => {
        // Single nextHue() call (unchanged sequence position — see this
        // page's own hueSeq/nextHue comment above) reused for BOTH the
        // heading Badge and this card's own wrapper: rainbow-mode
        // completeness sweep (jdp, live review: "Es sind nicht alle Buttons
        // in den Regenbogen-Modus eingepflegt"). `glim-notch-card` alone only
        // wires the reactive-mode hover reveal on the Badge's own notch — it
        // never redefines --accent/--focus-ring, so the "Add" button below
        // stayed the flat theme accent regardless of rainbow. Adding
        // `.glim-hue` here too (same mechanism as StepCard.tsx/Dashboard.tsx
        // Card()'s own identical fix) makes it inherit the SAME hue via
        // ordinary CSS custom-property cascade, no button-level change.
        const emptyHue = nextHue();
        return (
          <div
            className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
            style={hueVars(rainbowAt(emptyHue)) as CSSProperties}
          >
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={emptyHue} insetStart={6}>
                {t("fleet.emptyTitle")}
                <InfoBubble tip={t("fleet.empty")} onAccent />
              </Badge>
            </h2>
            <EmptyStateIcon icon={IconFleet} />
            <button
              onClick={() => setDialog("new")}
              className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
            >
              {t("fleet.addPeer")}
            </button>
          </div>
        );
      })()}

      {!loading && peers.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
          {peers.map((p, i) => (
            <FleetPeerCard
              key={p.id}
              peer={p}
              t={t}
              onRefresh={() => void loadPeers()}
              onEdit={() => setDialog(p)}
              index={i}
            />
          ))}
        </div>
      )}

      {dialog !== null && (
        <FleetDialog
          initial={dialog === "new" ? null : dialog}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void loadPeers();
          }}
        />
      )}
    </div>
  );
}
