// Receiver monitors the repositories other BombVault instances push off-site
// copies to: snapshots per source, last-received time, an independent restic
// check on this hardware and the dead man's switch. A repo is opened read-only
// with the sending instance's APP_KEY, stored encrypted and never shown again.

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
import { dbDumpNameOf, isDbDumpIdentity } from "../lib/dbdump";
import { useT } from "../lib/i18n";
import { PAGE_SHELL, PAGE_SHELL_TABBED } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { humanBytes } from "../lib/forecast";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { NumberField } from "../components/NumberField";
import { IconReceiver } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { useToast } from "../lib/toast";
import { hueVars } from "../lib/appearance";
import { Button } from "../components/Button";
import { Toggle } from "../components/Toggle";
import { ToggleRow } from "./settings/shared";
import { IconDisclosure } from "../components/IconDisclosure";

type T = ReturnType<typeof useT>["t"];

/** The name a received item goes by: its identity tag, or the dumps of a
 *  container in words. */
function itemLabel(item: string, t: T): string {
  if (isDbDumpIdentity(item)) return t("dbdump.retentionItem").replace("{name}", dbDumpNameOf(item));
  return item || "-";
}

// Mirrors the backend's foreignKeyRe for instant feedback; the server checks
// the key again and probes the repo with it.
const APP_KEY_RE = /^[0-9a-f]{64}$/;

function fmtReceived(iso: string, t: T): string {
  if (!iso) return t("receiver.never");
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

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
                <span className="font-medium">{itemLabel(s.item, t)}</span>
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
  /** Position in the list, which picks the card's rainbow hue. */
  index: number;
}) {
  const [open, setOpen] = useState(false);
  const [deepCheck, setDeepCheck] = useState(false);
  const [checking, setChecking] = useState(false);
  const [removing, setRemoving] = useState(false);
  const { push } = useToast();
  // Removing only drops the monitoring entry and leaves the repo on disk, so a
  // two-click inline confirm is enough.
  const [confirmRemove, setConfirmRemove] = useState(false);
  // Bumped on a failure and used as the button key, so the shake replays.
  const [shakeCheck, setShakeCheck] = useState(0);
  const [shakeRemove, setShakeRemove] = useState(0);

  // The outcome is only a toast: onRefresh() reloads the repo, and its badge
  // and "last checked" line carry the lasting result.
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
        // The confirm stays armed, so the shake lands on a mounted button and a
        // retry needs no second click.
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

  const checkTone = repo.lastCheckOk === null ? "neutral" : repo.lastCheckOk ? "ok" : "fail";
  const checkLabel =
    repo.lastCheckOk === null
      ? t("receiver.checkNever")
      : repo.lastCheckOk
      ? t("receiver.checkOk")
      : t("receiver.checkFailed");

  return (
    <div
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // Unlike ContainerRow, no glim-active: a check is a quick request, not a
      // tracked backup or restore job.
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row"
    >
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

        <div className="text-end shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("receiver.lastReceived")}</p>
          <p className="text-xs text-carbon-textSub">{fmtReceived(repo.lastReceived, t)}</p>
          <p className="text-xs text-carbon-textMuted mt-0.5">
            {t("receiver.snapshotsCount", repo.snapshotCount)}
          </p>
        </div>
      </div>

      {repo.lastCheckAt > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("receiver.lastChecked").replace("{time}", relativeTime(t, repo.lastCheckAt))}
          {repo.lastCheckOk === false && repo.lastCheckError && (
            <span className="text-statusFail"> · {repo.lastCheckError}</span>
          )}
        </p>
      )}

      {/* A received repo's password derives from the sending instance's
          APP_KEY, which is the first wall anyone debugging a failed check from
          a shell on this box runs into. */}
      {repo.lastCheckOk === false && (
        <p className="text-xs text-carbon-textSub wrap-break-word">{t("receiver.checkFailedHelp")}</p>
      )}

      <div className="flex items-center gap-3 flex-wrap">
        <Button
          key={shakeCheck}
          label={t("receiver.checkNow")}
          labelKey="receiver.checkNow"
          tone="accent"
          onClick={() => void handleCheck()}
          disabled={checking}
          busy={checking}
          title={checking ? t("dashboard.checking") : undefined}
          className={shakeCheck ? "glim-shake" : ""}
        />
        <Toggle
          checked={deepCheck}
          onChange={setDeepCheck}
          disabled={checking}
          label={t("receiver.deepCheck")}
        />

        <div className="ms-auto flex items-center gap-2">
          <Button
            label={t("receiver.details")}
            labelKey="receiver.details"
            tone="neutral"
            onClick={() => setOpen((v) => !v)}
            glyph={<IconDisclosure open={open} />}
          />
          <Button
            label={t("receiver.edit")}
            labelKey="receiver.edit"
            tone="neutral"
            onClick={onEdit}
          />
          {/* A text button rather than an icon: the two-click confirm needs a
              label to flip. */}
          {confirmRemove ? (
            <Button
              key={shakeRemove}
              label={t("receiver.confirmRemove")}
              labelKey="receiver.confirmRemove"
              tone="neutral"
              onClick={() => void handleRemove()}
              disabled={removing}
              busy={removing}
              title={removing ? t("receiver.removing") : undefined}
              className={shakeRemove ? "glim-shake" : ""}
            />
          ) : (
            <Button
              label={t("receiver.remove")}
              labelKey="receiver.remove"
              tone="neutral"
              onClick={() => setConfirmRemove(true)}
            />
          )}
        </div>
      </div>

      {open && (
        <div className="rounded-card bg-carbon-background px-3 py-2">
          <p className="text-xs font-medium text-carbon-textSub">{t("receiver.inventoryTitle")}</p>
          <InventoryPanel repo={repo} t={t} />
        </div>
      )}
    </div>
  );
}

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
  const [shake, setShake] = useState(0);

  const editing = initial !== null;
  // On edit an empty key keeps the stored one; on create a key is required.
  const keyOk = appKey === "" ? editing : APP_KEY_RE.test(appKey);
  const canSave = name.trim() !== "" && repo.trim() !== "" && keyOk && !saving;

  // The dialog closes on success, so a toast is the only notice either way.
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
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

  // Centring is safe: the box is capped at 90vh, so its top never goes
  // negative, and the backdrop scrolls if the content grows.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      {/* The box scrolls and would clip the heading badge that pokes above
          its top edge, so a non-clipping shell carries the badge. */}
      <div className="relative w-full max-w-lg">
        {/* px-5 matches the box's p-5, so the notch sits where a Card's does. */}
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>{editing ? t("receiver.editTitle") : t("receiver.addTitle")}</Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={editing ? t("receiver.editTitle") : t("receiver.addTitle")}
          onClick={(e) => e.stopPropagation()}
          className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
        >
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

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("receiver.deadManHours")}</label>
              <NumberField
                min={1}
                value={deadManHours}
                onChange={(e) => setDeadManHours(parseInt(e.target.value, 10))}
                className={inputCls}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("receiver.readDataPercent")}</label>
              <NumberField
                min={0}
                max={100}
                value={readDataPercent}
                onChange={(e) => setReadDataPercent(parseInt(e.target.value, 10))}
                className={inputCls}
              />
            </div>
          </div>
          <p className="text-caption text-carbon-textMuted -mt-2">{t("receiver.deadManHoursHint")}</p>

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

          {/* ToggleRow puts the label at the start and the switch at the end,
              like every other settings row. */}
          <ToggleRow checked={enabled} onChange={setEnabled} label={t("receiver.enabledLabel")} />

          <div className="flex items-center justify-end gap-2 pt-1">
            <Button
              label={t("files.cancel")}
              labelKey="files.cancel"
              tone="neutral"
              onClick={onClose}
              disabled={saving}
            />
            <Button
              key={shake}
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void handleSave()}
              disabled={!canSave}
              busy={saving}
              title={saving ? t("common.saving") : undefined}
              className={shake ? "glim-shake" : ""}
            />
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/** With `embedded`, the Instances page shows this as a tab and owns the outer
 *  shell and the heading, so the tab does not repeat the strip's label. */
export function Receiver({ embedded = false }: { embedded?: boolean } = {}) {
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

  // The empty state carries its own Add button, so the header one hides then.
  const showEmptyState = !loading && !error && repos.length === 0;

  return (
    <div className={embedded ? PAGE_SHELL_TABBED : PAGE_SHELL}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          {!embedded && <h1 className="text-2xl font-semibold text-carbon-text">{t("receiver.title")}</h1>}
          <p className="mt-1 text-sm text-carbon-textSub">{t("receiver.subtitle")}</p>
        </div>
        {!showEmptyState && (
          <Button
            label={t("receiver.addRepo")}
            labelKey="receiver.addRepo"
            tone="accent"
            onClick={() => setDialog("new")}
            className="shrink-0"
          />
        )}
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {/* Hue 0 cannot collide with a card's, since this only shows while the
          list is empty. glim-hue sets the accent for the Add button, which
          glim-notch-card alone does not. insetStart corrects the notch in a
          centred card, see Badge.tsx. */}
      {showEmptyState && (
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(0) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("receiver.emptyTitle")}
              <InfoBubble tip={t("receiver.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconReceiver} />
          <Button
            label={t("receiver.addRepo")}
            labelKey="receiver.addRepo"
            tone="accent"
            onClick={() => setDialog("new")}
          />
        </div>
      )}

      {!loading && repos.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
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
