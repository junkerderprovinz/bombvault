// ---------------------------------------------------------------------------
// Receiver page — the READ-ONLY receiver dashboard. On the box that RECEIVES
// immutable off-site copies (an append-only rest-server / repo another BombVault
// pushes to), this registers that repo and monitors it read-only: snapshot
// inventory grouped by source, last-received time, an independent restic check
// on the receiving hardware, and dead-mans-switch + integrity status.
//
// Gated behind settings.receiverEnabled (the Receiver tab only shows when on).
// Nothing here writes to the received repo: it is opened read-only with the
// SENDING instance's APP_KEY (encrypted at rest, never shown again). Modeled on
// Files.tsx — one card per received repo with an expandable inventory panel.
// ---------------------------------------------------------------------------

import { useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  listReceivedRepos,
  createReceivedRepo,
  updateReceivedRepo,
  deleteReceivedRepo,
  receiverInventory,
  checkReceivedRepo,
} from "../lib/api";
import type {
  ReceivedRepoStatus,
  ReceivedRepoInput,
  ReceiverInventory,
} from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { humanBytes } from "../lib/forecast";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconReceiver } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { useToast } from "../lib/toast";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";

type T = ReturnType<typeof useT>["t"];

// The sending APP_KEY shape guard mirrors the backend foreignKeyRe (64 lowercase
// hex). The server re-validates + probes; this just gives instant feedback.
const APP_KEY_RE = /^[0-9a-f]{64}$/;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Render an RFC3339 (or empty) received-time string as a localized date/time. */
function fmtReceived(iso: string, t: T): string {
  if (!iso) return t("receiver.never");
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

// ---------------------------------------------------------------------------
// Inventory panel (grouped by source) — lazy-loaded on expand
// ---------------------------------------------------------------------------

function InventoryPanel({ repo, t }: { repo: ReceivedRepoStatus; t: T }) {
  const [inv, setInv] = useState<ReceiverInventory | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    receiverInventory(repo.id)
      .then((res) => {
        if (!active) return;
        if (res.ok && res.inventory) setInv(res.inventory);
        else setError(res.error ?? t("receiver.inventoryError"));
      })
      .catch((err) => {
        if (active) setError(err instanceof Error ? err.message : t("receiver.inventoryError"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [repo.id, t]);

  if (loading) {
    return <p className="py-3 text-xs text-carbon-textMuted">{t("receiver.inventoryLoading")}</p>;
  }
  if (error) {
    return <p className="py-3 text-xs text-statusFail wrap-break-word">{error}</p>;
  }
  if (!inv || inv.sources.length === 0) {
    return <p className="py-3 text-xs text-carbon-textMuted">{t("receiver.inventoryEmpty")}</p>;
  }

  return (
    <div className="mt-2 overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-carbon-textMuted text-start">
            <th className="font-medium py-1.5 pe-3">{t("receiver.colSource")}</th>
            <th className="font-medium py-1.5 pe-3 text-end">{t("receiver.colSnapshots")}</th>
            <th className="font-medium py-1.5 pe-3">{t("receiver.colLastReceived")}</th>
            <th className="font-medium py-1.5 text-end">{t("receiver.colSize")}</th>
          </tr>
        </thead>
        <tbody>
          {inv.sources.map((s, i) => (
            <tr key={`${s.host}/${s.item}/${i}`} className="border-t border-carbon-border">
              <td className="py-1.5 pe-3 text-carbon-text">
                <span className="font-medium">{s.item || "-"}</span>
                {s.host && <span className="text-carbon-textMuted"> · {s.host}</span>}
              </td>
              <td className="py-1.5 pe-3 text-end text-carbon-textSub font-mono">{s.snapshotCount}</td>
              <td className="py-1.5 pe-3 text-carbon-textSub">{fmtReceived(s.lastReceived, t)}</td>
              <td className="py-1.5 text-end text-carbon-textSub font-mono">{humanBytes(s.totalSize)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr className="border-t border-carbon-border text-carbon-text">
            <td className="py-1.5 pe-3 font-medium">{t("receiver.total")}</td>
            <td className="py-1.5 pe-3 text-end font-mono">{inv.snapshotCount}</td>
            <td className="py-1.5 pe-3 text-carbon-textSub">{fmtReceived(inv.lastReceived, t)}</td>
            <td className="py-1.5 text-end font-mono">{humanBytes(inv.totalSize)}</td>
          </tr>
        </tfoot>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Repo card
// ---------------------------------------------------------------------------

function ReceivedRepoCard({
  repo,
  t,
  onRefresh,
  onEdit,
  index,
}: {
  repo: ReceivedRepoStatus;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Position in the rendered list — the rainbow palette position (GlimStone
   *  colour engine), matching Containers.tsx's ContainerRow / VMs.tsx's VMRow /
   *  Files.tsx's FileSetRow (and now Fleet.tsx's FleetPeerCard): a list of
   *  received repos is exactly the case the mode exists for, a variable,
   *  user-configured set someone tracks several of at once. Assigned by LIST
   *  INDEX, never a hash of `repo.name` — see the caller below. */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [deepCheck, setDeepCheck] = useState(false);
  const [checking, setChecking] = useState(false);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  // Reversible action: removing a monitoring entry never touches the repo on
  // disk (re-addable in one step), so per the design-language's "reversible
  // actions don't ask" rule this gets the LIGHTER two-click inline-confirm —
  // click "Remove" → button becomes "Confirm remove" — matching
  // OffsiteTargetsSection's `confirmRemove` pattern exactly, not a full
  // window.confirm()/ConfirmDialog (form-engine Task 7).
  const [confirmRemove, setConfirmRemove] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Check/Remove buttons alongside their existing toasts on failure.
  const [shakeCheck, setShakeCheck] = useState(0);
  const [shakeRemove, setShakeRemove] = useState(0);

  // GlimStone follow-up pass (v8.0.0): the ok/fail checkMsg result below moved
  // to a toast — found alongside this card's handleRemove migration (same
  // component). Same ok/uninit/fail shape as the already-migrated
  // TestConnectionButton/TargetTestButton: onRefresh() reloads the repo,
  // whose persistent checkTone/checkLabel Badge + "last checked" line already
  // carry this exact outcome, so the ephemeral checkMsg was pure duplicate
  // (and, since it never auto-cleared, a stale one could linger next to the
  // button until the NEXT check).
  async function handleCheck() {
    setChecking(true);
    try {
      const res = await checkReceivedRepo(repo.id, deepCheck);
      if (res.ok && res.result) {
        if (res.result.ok) push(t("receiver.checkOk"), "success");
        else {
          push(res.result.error || t("receiver.checkFailed"), "fail");
          setShakeCheck((n) => n + 1);
        }
      } else {
        push(res.error ?? t("receiver.checkFailed"), "fail");
        setShakeCheck((n) => n + 1);
      }
      onRefresh();
    } catch (err) {
      push(err instanceof Error ? err.message : t("receiver.checkFailed"), "fail");
      setShakeCheck((n) => n + 1);
    } finally {
      setChecking(false);
    }
  }

  async function handleRemove() {
    setRemoving(true);
    try {
      const res = await deleteReceivedRepo(repo.id);
      if (res.ok) {
        onRefresh();
        setConfirmRemove(false);
      } else {
        // Keep the two-click confirm UP on failure (don't reset to "Remove")
        // — see FleetPeerCard's identical handleRemove for the fuller reason.
        push(res.error ?? t("receiver.saveError"), "fail");
        setShakeRemove((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("receiver.saveError"), "fail");
      setShakeRemove((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  // Check-result badge tone: never checked / passed / failed.
  const checkTone = repo.lastCheckOk === null ? "neutral" : repo.lastCheckOk ? "ok" : "fail";
  const checkLabel =
    repo.lastCheckOk === null
      ? t("receiver.checkNever")
      : repo.lastCheckOk
      ? t("receiver.checkOk")
      : t("receiver.checkFailed");

  return (
    <div
      style={{ ...hueVars(rainbowAt(index)), "--row-i": String(index) } as CSSProperties}
      // glim-hue owns the position; glim-tint washes the WHOLE card with it
      // (trap #2, design-language.md's "Rainbow" section) — same
      // relative/overflow-hidden/glim-hue/glim-tint shell as
      // ContainerRow/VMRow/FileSetRow/FleetPeerCard, so a rainbow-mode
      // Receiver list colours each monitored repo instead of leaving every
      // row the flat accent. No glim-active here: unlike those first three,
      // a received-repo card has no progressMap-tracked backup/restore job of
      // its own to key it off — Check is a quick request/response action,
      // not a tracked job.
      // bv-stagger-row (GlimStone motion-engine animation 3) — see
      // ContainerRow's identical comment.
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-tint bv-stagger-row"
    >
      {/* Header: name + badges */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">{repo.name}</span>
            {!repo.enabled && <Badge tone="neutral">{t("receiver.monitoringOff")}</Badge>}
            {repo.enabled &&
              (repo.reachable ? (
                <Badge tone="ok">{t("receiver.reachable")}</Badge>
              ) : (
                <Badge tone="fail">{t("receiver.unreachable")}</Badge>
              ))}
            <Badge tone={checkTone}>{checkLabel}</Badge>
          </div>
          <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start">{repo.repo}</p>
        </div>

        {/* Last received + snapshot count */}
        <div className="text-end shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("receiver.lastReceived")}</p>
          <p className="text-xs text-carbon-textSub">{fmtReceived(repo.lastReceived, t)}</p>
          <p className="text-xs text-carbon-textMuted mt-0.5">
            {t("receiver.snapshotsCount").replace("{n}", String(repo.snapshotCount))}
          </p>
        </div>
      </div>

      {/* Last check line */}
      {repo.lastCheckAt > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("receiver.lastChecked").replace("{time}", relativeTime(t, repo.lastCheckAt))}
          {repo.lastCheckOk === false && repo.lastCheckError && (
            <span className="text-statusFail"> · {repo.lastCheckError}</span>
          )}
        </p>
      )}

      {/* Actions row */}
      <div className="flex items-center gap-3 flex-wrap">
        <button
          key={shakeCheck}
          onClick={() => void handleCheck()}
          disabled={checking}
          className={`inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shakeCheck ? " glim-shake" : ""
          }`}
        >
          {checking ? (
            <>
              <span
                className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
                style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
              />
              {t("dashboard.checking")}
            </>
          ) : (
            t("receiver.checkNow")
          )}
        </button>
        <label className="flex items-center gap-1.5 text-xs text-carbon-textSub cursor-pointer">
          <input
            type="checkbox"
            checked={deepCheck}
            onChange={(e) => setDeepCheck(e.target.checked)}
            disabled={checking}
            className="h-3.5 w-3.5 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("receiver.deepCheck")}
        </label>

        <div className="ms-auto flex items-center gap-2">
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
            {t("receiver.details")}
          </button>
          <button
            onClick={onEdit}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("receiver.edit")}
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
              {removing ? t("receiver.removing") : t("receiver.confirmRemove")}
            </button>
          ) : (
            <button
              onClick={() => setConfirmRemove(true)}
              className="inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50"
            >
              {t("receiver.remove")}
            </button>
          )}
        </div>
      </div>

      {/* Inventory disclosure */}
      {open && (
        <div className="rounded-card bg-carbon-background px-3 py-2">
          <p className="text-xs font-medium text-carbon-textSub">{t("receiver.inventoryTitle")}</p>
          <InventoryPanel repo={repo} t={t} />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Add / edit dialog
// ---------------------------------------------------------------------------

function ReceiverDialog({
  initial,
  t,
  onClose,
  onSaved,
}: {
  /** null = create; a status row = edit that repo. */
  initial: ReceivedRepoStatus | null;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [repo, setRepo] = useState(initial?.repo ?? "");
  const [appKey, setAppKey] = useState("");
  const revealAppKey = useReveal();
  const [deadManHours, setDeadManHours] = useState(initial?.deadManHours ?? 26);
  const [checkCadence, setCheckCadence] = useState(initial?.checkCadence ?? "");
  const [readDataPercent, setReadDataPercent] = useState(initial?.readDataPercent ?? 0);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): shake
  // the Save button alongside the toast on a failed save.
  const [shake, setShake] = useState(0);

  const editing = initial !== null;
  // On edit an empty key keeps the stored one; on create a key is required.
  const keyOk = appKey === "" ? editing : APP_KEY_RE.test(appKey);
  const canSave = name.trim() !== "" && repo.trim() !== "" && keyOk && !saving;

  // GlimStone follow-up pass (v8.0.0): the "error" flash below is now a
  // toast — same shape as Files.tsx's FileSetDialog.handleSave (a dialog
  // editor that closes on success via onSaved(), so a toast is the only
  // outcome notice left, success or failure). The three client-side checks
  // are effectively unreachable through the UI (canSave already disables
  // Save for the same conditions), but get the same push() treatment as the
  // API failure below for consistency. The separate appKey-format hint
  // further down (`appKey !== "" && !APP_KEY_RE.test(appKey)`) is untouched —
  // that's a live field-validation hint recomputed every render, not a
  // submit-triggered one-shot notice.
  async function handleSave() {
    if (name.trim() === "") {
      push(t("receiver.nameRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (repo.trim() === "") {
      push(t("receiver.repoRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (!keyOk) {
      push(t("receiver.appKeyInvalid"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSaving(true);
    const input: ReceivedRepoInput = {
      name: name.trim(),
      repo: repo.trim(),
      appKey: appKey.trim(),
      deadManHours: Number.isFinite(deadManHours) ? deadManHours : 26,
      checkCadence: checkCadence.trim(),
      readDataPercent: Math.max(0, Math.min(100, Number.isFinite(readDataPercent) ? readDataPercent : 0)),
      enabled,
      sortOrder: initial?.sortOrder ?? 0,
    };
    try {
      const res = editing
        ? await updateReceivedRepo(initial.id, input)
        : await createReceivedRepo(input);
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("receiver.saveError"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("receiver.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 bv-field-focus";

  // `items-center` — the third and last of the three sites Files.tsx's own
  // FileSetDialog comment recorded as "same fix still owed" when that round
  // scoped itself to the Ordner tab (Fleet.tsx's two dialogs are the other
  // two, fixed in the same pass as this). Top-anchored, the heading Badge
  // poked to within a few px of the browser-viewport edge instead of
  // straddling the card with any breathing room. Safe for the identical
  // reason it is safe in every other dialog in this app: the box below is
  // capped at `max-h-[90vh]`, strictly under the 100vh flex container, so a
  // centred item's top offset is always positive, and this backdrop's own
  // `overflow-y-auto` still covers content that grows toward the cap.
  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      {/* GlimStone follow-up pass ("half-overlap card notch"): the dialog box
          itself scrolls (`max-h-[90vh] overflow-y-auto`), which would clip
          the heading Badge's own -11px poke above it — a scrollable box
          can't reveal content positioned above its own top edge at
          scrollTop 0. So this wraps in a non-scrolling, non-clipping
          `relative` shell that hosts the badge, with the ORIGINAL
          scrollable box moved one level in as its only child. `w-full
          max-w-lg` moves to this outer shell (it's now the actual flex item
          inside the centring backdrop below) and the inner box gets a plain
          `w-full` instead, so the rendered width/centring is pixel-identical
          to before this split. */}
      <div className="relative w-full max-w-lg">
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap>{editing ? t("receiver.editTitle") : t("receiver.addTitle")}</Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={editing ? t("receiver.editTitle") : t("receiver.addTitle")}
          onClick={(e) => e.stopPropagation()}
          className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
        >
          {/* Name */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("receiver.name")}</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower off-site"
            className={inputCls}
          />
        </div>

        {/* Repository location */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("receiver.repoLocation")}</label>
          <input
            type="text"
            value={repo}
            onChange={(e) => setRepo(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder="rest:http://192.168.x.x:8000/tower-containers"
            dir="ltr"
            className={`${inputCls} font-mono text-start`}
          />
          <p className="text-caption text-carbon-textMuted">{t("receiver.repoLocationHint")}</p>
        </div>

        {/* Sending APP_KEY */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("receiver.appKey")}</label>
          <RevealInput
            {...revealAppKey}
            value={appKey}
            onChange={(e) => setAppKey(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder={editing ? t("receiver.appKeyKeep") : "0123456789abcdef…"}
            wrapperClassName="w-full"
            className={`${inputCls} font-mono`}
          />
          <p className="text-caption text-carbon-textMuted">{t("receiver.appKeyHint")}</p>
          {appKey !== "" && !APP_KEY_RE.test(appKey) && (
            <p className="text-caption text-statusFail">{t("receiver.appKeyInvalid")}</p>
          )}
        </div>

        {/* Dead-mans-switch + check cadence + deep-check percent */}
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">{t("receiver.deadManHours")}</label>
            <input
              type="number"
              min={1}
              value={deadManHours}
              onChange={(e) => setDeadManHours(parseInt(e.target.value, 10))}
              className={inputCls}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">{t("receiver.readDataPercent")}</label>
            <input
              type="number"
              min={0}
              max={100}
              value={readDataPercent}
              onChange={(e) => setReadDataPercent(parseInt(e.target.value, 10))}
              className={inputCls}
            />
          </div>
        </div>
        <p className="text-caption text-carbon-textMuted -mt-2">{t("receiver.deadManHoursHint")}</p>

        {/* Check cadence */}
        <div className="flex flex-col gap-1.5">
          <label className="text-xs text-carbon-textSub">{t("receiver.checkCadence")}</label>
          <input
            type="text"
            value={checkCadence}
            onChange={(e) => setCheckCadence(e.target.value)}
            spellCheck={false}
            autoComplete="off"
            placeholder={t("receiver.checkCadencePlaceholder")}
            dir="ltr"
            className={`${inputCls} font-mono text-start`}
          />
          <p className="text-caption text-carbon-textMuted">{t("receiver.checkCadenceHint")}</p>
        </div>

        {/* Monitor toggle */}
        <label className="flex items-center gap-2 text-xs text-carbon-textSub cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 cursor-pointer"
            style={{ accentColor: "var(--accent)" }}
          />
          {t("receiver.enabledLabel")}
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
// Receiver page
// ---------------------------------------------------------------------------

export function Receiver() {
  const { t } = useT();
  // Registers this page for a re-render on any rainbow-state change (on/off/
  // reactive/rotate/palette edit) — the ReceivedRepoCard list below reads
  // rainbowAt()/hueVars() directly during render; see lib/useRainbow.ts's own
  // header for why a caller doesn't need the returned value.
  useRainbow();
  const [repos, setRepos] = useState<ReceivedRepoStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a row = edit dialog for that repo.
  const [dialog, setDialog] = useState<"new" | ReceivedRepoStatus | null>(null);

  function loadRepos() {
    return listReceivedRepos()
      .then((res) => {
        if (res.ok) {
          setRepos(res.repos ?? []);
          setError(null);
        } else {
          setError(res.error ?? t("receiver.loadError"));
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("receiver.loadError")));
  }

  useEffect(() => {
    void loadRepos().finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // jdp live review ("Empfänger Tab: Button rechts oben ist redundant"): the
  // empty-state Card below already carries its own prominent "Add received
  // repo" CTA, so showing the identical button a second time in the
  // top-right actions bar was pure duplication — confirmed both call the
  // exact same handler (`() => setDialog("new")`). Gate the top-right button
  // on NOT being in that empty state — mirrors Files.tsx's own
  // showEmptyState fix for the identical pattern. Once a repo exists the
  // empty-state Card stops rendering and the top-right button is the page's
  // only entry point again, so "Add" is never unreachable.
  const showEmptyState = !loading && !error && repos.length === 0;

  return (
    // PAGE_SHELL (jdp live-review, "Können wir die nicht überall gleich breit
    // machen?"): the gap here was already the correct 40px from the earlier
    // "Im Empfänger Tab ist die Card zu weit oben" round; only the width
    // changes, max-w-5xl (1024px) → the shared 1152px. This page's heading is
    // a single bare h1+p row, so the one flat shell gap still governs every
    // gap on it. See lib/pageShell.ts for the full before/after table.
    <div className={PAGE_SHELL}>
      {/* Heading + Add */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("receiver.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("receiver.subtitle")}</p>
        </div>
        {!showEmptyState && (
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity shrink-0"
          >
            {t("receiver.addRepo")}
          </button>
        )}
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {/* Empty state — GlimStone follow-up pass (jdp live review: "Card hat
          keinen Cardtitelbadge mit dem Infotext der in der Card steht"): this
          card had no heading at all — just the icon, the permanent pitch
          paragraph, and the Add button — the one Card-shaped box on this page
          that never got the tone="heading" notch every other Card in the app
          carries. `relative glim-notch-card` (Files.tsx's own setsTitle Card
          precedent — no separate inner overflow-hidden box needed, this card
          was never overflow-hidden to begin with). The old permanent
          `<p>{t("receiver.empty")}</p>` reads once and then costs vertical
          space forever — moved verbatim onto the new heading Badge as an
          `onAccent` InfoBubble instead, zero new i18n keys for the body, only
          the new title key. hueIndex={0}: the only tone="heading" notch on
          this page's own body (ReceiverDialog's own h2 badge deliberately
          carries no hueIndex, same as every other dialog title in the app),
          and mutually exclusive with ReceivedRepoCard's OWN rainbowAt(index)
          tint (this card only renders while the list is empty), so there is
          no position to collide with.
          insetStart={6} (GlimStone follow-up pass, jdp: "Empfaenger/Fleet-
          Tab: Cardtitelbadge falsch platziert" — the SAME `text-center
          items-center` collapsed-h2 mismatch as Files.tsx's own setsTitle
          Card and Fleet.tsx's identical empty-state Card; see Files.tsx's
          own call site for the full "why a single-merged-div Card can still
          get this wrong" mechanism and Badge.tsx's `insetStart` doc). */}
      {showEmptyState && (
        // `.glim-hue` added (rainbow-mode completeness sweep, jdp live
        // review: "Es sind nicht alle Buttons in den Regenbogen-Modus
        // eingepflegt"): `glim-notch-card` alone only wires the reactive-mode
        // hover reveal on the Badge's own notch, never --accent/--focus-ring
        // itself, so the "Add" button below stayed flat regardless of
        // rainbow. Same hueIndex={0} the Badge already uses (Fleet.tsx's own
        // identical fix for the same reasoning).
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(rainbowAt(0)) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("receiver.emptyTitle")}
              <InfoBubble tip={t("receiver.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconReceiver} />
          <button
            onClick={() => setDialog("new")}
            className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity"
          >
            {t("receiver.addRepo")}
          </button>
        </div>
      )}

      {/* Repo cards */}
      {!loading && repos.length > 0 && (
        <div className="flex flex-col gap-3 bv-content-fade">
          {repos.map((r, i) => (
            <ReceivedRepoCard
              key={r.id}
              repo={r}
              t={t}
              onRefresh={() => void loadRepos()}
              onEdit={() => setDialog(r)}
              index={i}
            />
          ))}
        </div>
      )}

      {/* Add / edit dialog */}
      {dialog !== null && (
        <ReceiverDialog
          initial={dialog === "new" ? null : dialog}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void loadRepos();
          }}
        />
      )}
    </div>
  );
}
