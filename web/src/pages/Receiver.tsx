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

import { useEffect, useState } from "react";
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
import { relativeTime } from "../lib/reltime";
import { humanBytes } from "../lib/forecast";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconReceiver } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";

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
          <tr className="text-carbon-textMuted text-left">
            <th className="font-medium py-1.5 pr-3">{t("receiver.colSource")}</th>
            <th className="font-medium py-1.5 pr-3 text-right">{t("receiver.colSnapshots")}</th>
            <th className="font-medium py-1.5 pr-3">{t("receiver.colLastReceived")}</th>
            <th className="font-medium py-1.5 text-right">{t("receiver.colSize")}</th>
          </tr>
        </thead>
        <tbody>
          {inv.sources.map((s, i) => (
            <tr key={`${s.host}/${s.item}/${i}`} className="border-t border-carbon-border">
              <td className="py-1.5 pr-3 text-carbon-text">
                <span className="font-medium">{s.item || "-"}</span>
                {s.host && <span className="text-carbon-textMuted"> · {s.host}</span>}
              </td>
              <td className="py-1.5 pr-3 text-right text-carbon-textSub font-mono">{s.snapshotCount}</td>
              <td className="py-1.5 pr-3 text-carbon-textSub">{fmtReceived(s.lastReceived, t)}</td>
              <td className="py-1.5 text-right text-carbon-textSub font-mono">{humanBytes(s.totalSize)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr className="border-t border-carbon-border text-carbon-text">
            <td className="py-1.5 pr-3 font-medium">{t("receiver.total")}</td>
            <td className="py-1.5 pr-3 text-right font-mono">{inv.snapshotCount}</td>
            <td className="py-1.5 pr-3 text-carbon-textSub">{fmtReceived(inv.lastReceived, t)}</td>
            <td className="py-1.5 text-right font-mono">{humanBytes(inv.totalSize)}</td>
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
}: {
  repo: ReceivedRepoStatus;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [deepCheck, setDeepCheck] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkMsg, setCheckMsg] = useState<string | null>(null);
  const [removing, setRemoving] = useState(false);
  const [removeErr, setRemoveErr] = useState<string | null>(null);

  async function handleCheck() {
    setChecking(true);
    setCheckMsg(null);
    try {
      const res = await checkReceivedRepo(repo.id, deepCheck);
      if (res.ok && res.result) {
        setCheckMsg(res.result.ok ? t("receiver.checkOk") : res.result.error || t("receiver.checkFailed"));
      } else {
        setCheckMsg(res.error ?? t("receiver.checkFailed"));
      }
      onRefresh();
    } catch (err) {
      setCheckMsg(err instanceof Error ? err.message : t("receiver.checkFailed"));
    } finally {
      setChecking(false);
    }
  }

  async function handleRemove() {
    if (!window.confirm(t("receiver.removeConfirm"))) return;
    setRemoving(true);
    setRemoveErr(null);
    try {
      const res = await deleteReceivedRepo(repo.id);
      if (res.ok) onRefresh();
      else setRemoveErr(res.error ?? t("receiver.saveError"));
    } catch (err) {
      setRemoveErr(err instanceof Error ? err.message : t("receiver.saveError"));
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
    <div className="bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
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
          <p className="mt-1 text-xs font-mono text-carbon-textMuted truncate">{repo.repo}</p>
        </div>

        {/* Last received + snapshot count */}
        <div className="text-right shrink-0">
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
          onClick={() => void handleCheck()}
          disabled={checking}
          className="inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
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
        {checkMsg && <span className="text-xs text-carbon-textSub wrap-break-word">{checkMsg}</span>}

        <div className="ml-auto flex items-center gap-2">
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
            {t("receiver.details")}
          </button>
          <button
            onClick={onEdit}
            className="inline-flex items-center rounded-control bg-carbon-surface2 px-3 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("receiver.edit")}
          </button>
          <button
            onClick={() => void handleRemove()}
            disabled={removing}
            className="inline-flex items-center rounded-control bg-statusFailBg px-3 py-1.5 text-xs font-medium text-statusFail hover:bg-statusFailBgHover transition-colors disabled:opacity-50"
          >
            {removing ? t("receiver.removing") : t("receiver.remove")}
          </button>
        </div>
      </div>
      {removeErr && <p className="text-xs text-statusFail wrap-break-word">{removeErr}</p>}

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
  const [name, setName] = useState(initial?.name ?? "");
  const [repo, setRepo] = useState(initial?.repo ?? "");
  const [appKey, setAppKey] = useState("");
  const revealAppKey = useReveal();
  const [deadManHours, setDeadManHours] = useState(initial?.deadManHours ?? 26);
  const [checkCadence, setCheckCadence] = useState(initial?.checkCadence ?? "");
  const [readDataPercent, setReadDataPercent] = useState(initial?.readDataPercent ?? 0);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const editing = initial !== null;
  // On edit an empty key keeps the stored one; on create a key is required.
  const keyOk = appKey === "" ? editing : APP_KEY_RE.test(appKey);
  const canSave = name.trim() !== "" && repo.trim() !== "" && keyOk && !saving;

  async function handleSave() {
    if (name.trim() === "") {
      setError(t("receiver.nameRequired"));
      return;
    }
    if (repo.trim() === "") {
      setError(t("receiver.repoRequired"));
      return;
    }
    if (!keyOk) {
      setError(t("receiver.appKeyInvalid"));
      return;
    }
    setSaving(true);
    setError(null);
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
      if (res.ok) onSaved();
      else setError(res.error ?? t("receiver.saveError"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("receiver.saveError"));
    } finally {
      setSaving(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={editing ? t("receiver.editTitle") : t("receiver.addTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <h2 className="text-lg font-semibold text-carbon-text">
          {editing ? t("receiver.editTitle") : t("receiver.addTitle")}
        </h2>

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
            className={`${inputCls} font-mono`}
          />
          <p className="text-[11px] text-carbon-textMuted">{t("receiver.repoLocationHint")}</p>
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
          <p className="text-[11px] text-carbon-textMuted">{t("receiver.appKeyHint")}</p>
          {appKey !== "" && !APP_KEY_RE.test(appKey) && (
            <p className="text-[11px] text-statusFail">{t("receiver.appKeyInvalid")}</p>
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
        <p className="text-[11px] text-carbon-textMuted -mt-2">{t("receiver.deadManHoursHint")}</p>

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
            className={`${inputCls} font-mono`}
          />
          <p className="text-[11px] text-carbon-textMuted">{t("receiver.checkCadenceHint")}</p>
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
// Receiver page
// ---------------------------------------------------------------------------

export function Receiver() {
  const { t } = useT();
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

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Heading + Add */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">{t("receiver.title")}</h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("receiver.subtitle")}</p>
        </div>
        <button
          onClick={() => setDialog("new")}
          className="inline-flex items-center rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity shrink-0"
        >
          {t("receiver.addRepo")}
        </button>
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {/* Empty state */}
      {!loading && !error && repos.length === 0 && (
        <div className="bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3">
          <EmptyStateIcon icon={IconReceiver} />
          <p className="text-sm text-carbon-textMuted max-w-xl">{t("receiver.empty")}</p>
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
        <div className="flex flex-col gap-3">
          {repos.map((r) => (
            <ReceivedRepoCard
              key={r.id}
              repo={r}
              t={t}
              onRefresh={() => void loadRepos()}
              onEdit={() => setDialog(r)}
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
