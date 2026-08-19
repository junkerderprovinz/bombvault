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

import { useEffect, useState } from "react";
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
import { useT, type TranslationKey } from "../lib/i18n";
import { relativeTime } from "../lib/reltime";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconFleet } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { copyText } from "../lib/clipboard";
import { useToast } from "../lib/toast";

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
  async function copy() {
    if (await copyText(text)) {
      push(t("vm.ssh.copied"), "success");
    }
  }
  return (
    <div className="flex items-start gap-2">
      <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-[11px] leading-snug text-carbon-text whitespace-pre">
        {text}
      </pre>
      <button
        type="button"
        onClick={() => void copy()}
        className="shrink-0 rounded-control bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover"
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
  const [error, setError] = useState<string | null>(null);

  async function handleAccept() {
    setBusy(true);
    setError(null);
    try {
      const res = await acceptMeshOffer(offer.id, domain);
      if (res.ok) onChanged();
      else setError(res.error ?? t("fleet.mesh.saveError"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("fleet.mesh.saveError"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDecline() {
    setBusy(true);
    setError(null);
    try {
      const res = await declineMeshOffer(offer.id);
      if (res.ok) onChanged();
      else setError(res.error ?? t("fleet.mesh.saveError"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("fleet.mesh.saveError"));
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
        <span className="text-xs text-carbon-textMuted ml-auto">{relativeTime(t, offer.receivedAt)}</span>
      </div>
      <p className="text-xs font-mono text-carbon-textMuted truncate">{offer.repo}</p>
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
            onClick={() => void handleAccept()}
            disabled={busy}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {t("fleet.mesh.accept")}
          </button>
          <button
            onClick={() => void handleDecline()}
            disabled={busy}
            className="inline-flex items-center rounded-control bg-carbon-surface3 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {t("fleet.mesh.decline")}
          </button>
        </div>
      )}
      {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
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
  const [error, setError] = useState<string | null>(null);
  const [snippet, setSnippet] = useState<(DeploySnippetData & { repo: string }) | null>(null);

  async function handleSend() {
    if (baseUrl.trim() === "") {
      setError(t("fleet.mesh.baseUrlRequired"));
      return;
    }
    setSending(true);
    setError(null);
    try {
      const res = await proposeMeshOffer(peer.id, domain, baseUrl.trim());
      if (res.ok && res.snippet) setSnippet(res.snippet);
      else setError(res.error ?? t("fleet.mesh.saveError"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("fleet.mesh.saveError"));
    } finally {
      setSending(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus";

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/60 p-4" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("fleet.mesh.proposeTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <h2 className="text-lg font-semibold text-carbon-text">{t("fleet.mesh.proposeTitle")}</h2>
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
                className={`${inputCls} font-mono`}
              />
              <p className="text-[11px] text-carbon-textMuted">{t("fleet.mesh.baseUrlHint")}</p>
            </div>
            {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
            <div className="flex items-center justify-end gap-2 pt-1">
              <button
                onClick={onClose}
                disabled={sending}
                className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
              >
                {t("files.cancel")}
              </button>
              <button
                onClick={() => void handleSend()}
                disabled={sending}
                className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
}: {
  peer: FleetPeer;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [polling, setPolling] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [removeErr, setRemoveErr] = useState<string | null>(null);
  const [showPropose, setShowPropose] = useState(false);
  // Reversible action: removing a monitoring entry never contacts the peer
  // instance (re-addable in one step), so per the design-language's
  // "reversible actions don't ask" rule this gets the LIGHTER two-click
  // inline-confirm — click "Remove" → button becomes "Confirm remove" —
  // matching OffsiteTargetsSection's / Receiver.tsx's `confirmRemove`
  // pattern exactly, not a full window.confirm()/ConfirmDialog (form-engine
  // Task 7).
  const [confirmRemove, setConfirmRemove] = useState(false);

  async function handlePoll() {
    setPolling(true);
    try {
      await pollFleetPeer(peer.id);
      onRefresh();
    } finally {
      setPolling(false);
    }
  }

  async function handleRemove() {
    setRemoving(true);
    setRemoveErr(null);
    try {
      const res = await deleteFleetPeer(peer.id);
      if (res.ok) onRefresh();
      else setRemoveErr(res.error ?? t("fleet.saveError"));
    } catch (err) {
      setRemoveErr(err instanceof Error ? err.message : t("fleet.saveError"));
    } finally {
      setRemoving(false);
      setConfirmRemove(false);
    }
  }

  const pollTone: "ok" | "fail" | "neutral" = peer.lastPollOk === null ? "neutral" : peer.lastPollOk ? "ok" : "fail";
  const pollLabel =
    peer.lastPollOk === null ? t("fleet.pollNever") : peer.lastPollOk ? t("fleet.pollOk") : t("fleet.pollFailed");

  return (
    <div className="bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {peer.lastPollInstanceName || peer.name}
            </span>
            {!peer.enabled && <Badge tone="neutral">{t("fleet.monitoringOff")}</Badge>}
            <Badge tone={pollTone}>{pollLabel}</Badge>
          </div>
          <p className="mt-1 text-xs font-mono text-carbon-textMuted truncate">{peer.url}</p>
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
          onClick={() => void handlePoll()}
          disabled={polling}
          className="inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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

        <div className="ml-auto flex items-center gap-2">
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
              className={`transition-transform ${open ? "rotate-90" : ""}`}
            >
              <path d="M4 2l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
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
              onClick={() => void handleRemove()}
              disabled={removing}
              className="inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50"
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
      {removeErr && <p className="text-xs text-statusFail wrap-break-word">{removeErr}</p>}

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
  const [name, setName] = useState(initial?.name ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [token, setToken] = useState("");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const revealToken = useReveal();

  const editing = initial !== null;
  const canSave = name.trim() !== "" && url.trim() !== "" && (token.trim() !== "" || editing) && !saving;

  async function handleSave() {
    if (name.trim() === "") {
      setError(t("fleet.nameRequired"));
      return;
    }
    if (url.trim() === "") {
      setError(t("fleet.urlRequired"));
      return;
    }
    setSaving(true);
    setError(null);
    const input: FleetPeerInput = {
      name: name.trim(),
      url: url.trim(),
      token: token.trim(),
      enabled,
      sortOrder: initial?.sortOrder ?? 0,
    };
    try {
      const res = editing ? await updateFleetPeer(initial.id, input) : await createFleetPeer(input);
      if (res.ok) onSaved();
      else setError(res.error ?? t("fleet.saveError"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("fleet.saveError"));
    } finally {
      setSaving(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus";

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={editing ? t("fleet.editTitle") : t("fleet.addTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <h2 className="text-lg font-semibold text-carbon-text">
          {editing ? t("fleet.editTitle") : t("fleet.addTitle")}
        </h2>

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
            className={`${inputCls} font-mono`}
          />
          <p className="text-[11px] text-carbon-textMuted">{t("fleet.urlHint")}</p>
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
          <p className="text-[11px] text-carbon-textMuted">{t("fleet.tokenHint")}</p>
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

        {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}

        <div className="flex items-center justify-end gap-2 pt-1">
          <button
            onClick={onClose}
            disabled={saving}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-50"
          >
            {t("files.cancel")}
          </button>
          <button
            onClick={() => void handleSave()}
            disabled={!canSave}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {saving ? t("common.saving") : t("settings.save")}
          </button>
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

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("fleet.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("fleet.subtitle")}</p>
        </div>
        <button
          onClick={() => setDialog("new")}
          className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity shrink-0"
        >
          {t("fleet.addPeer")}
        </button>
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {!loading && pendingOffers.length > 0 && (
        <div className="bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
          <div>
            <p className="text-sm font-semibold text-carbon-text">{t("fleet.mesh.offersTitle")}</p>
            <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.offersHint")}</p>
          </div>
          <div className="flex flex-col gap-2">
            {pendingOffers.map((o) => (
              <MeshOfferRow key={o.id} offer={o} t={t} onChanged={() => void loadOffers()} />
            ))}
          </div>
        </div>
      )}

      {!loading && !error && peers.length === 0 && (
        <div className="bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3">
          <EmptyStateIcon icon={IconFleet} />
          <p className="text-sm text-carbon-textMuted max-w-xl">{t("fleet.empty")}</p>
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
          >
            {t("fleet.addPeer")}
          </button>
        </div>
      )}

      {!loading && peers.length > 0 && (
        <div className="flex flex-col gap-3">
          {peers.map((p) => (
            <FleetPeerCard
              key={p.id}
              peer={p}
              t={t}
              onRefresh={() => void loadPeers()}
              onEdit={() => setDialog(p)}
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
