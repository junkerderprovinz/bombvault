import { useEffect, useState } from "react";
import type { OffsiteTarget, CloudCredSetInfo } from "../lib/api";
import {
  listOffsiteTargets,
  createOffsiteTarget,
  updateOffsiteTarget,
  deleteOffsiteTarget,
  testOffsiteTarget,
  getCloudCredSets,
} from "../lib/api";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// OffsiteTargetsSection — per-domain "Additional off-site targets" editor
// (multi-off-site). The PRIMARY off-site target (sortOrder 0, synced from the
// Settings off-site config) is still edited by the single off-site editor above;
// this section lists and manages the EXTRA targets (sortOrder > 0) through the
// off-site-targets CRUD API. It owns no Settings state: every mutation goes
// straight to the CRUD endpoints, and the section re-fetches its own list.
//
// No per-target schedule control is exposed: every target of a domain replicates
// on that domain's off-site schedule (a short help line says so).
// ---------------------------------------------------------------------------

type Domain = "containers" | "vms" | "flash" | "files";
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving" | "error";

// The restore-readable storage-class whitelist (mirrors CloudCard); "" renders as
// the provider-default option.
const STORAGE_CLASSES = [
  "STANDARD",
  "STANDARD_IA",
  "ONEZONE_IA",
  "INTELLIGENT_TIERING",
  "GLACIER_IR",
] as const;

// A blank draft for a new additional target. sortOrder is assigned at save time so
// it never shadows the primary (sortOrder 0).
function emptyDraft(domain: Domain): OffsiteTarget {
  return {
    id: "",
    domain,
    name: "",
    repo: "",
    credsRef: "",
    storageClass: "",
    immutable: false,
    schedule: "",
    retentionKeepLast: 0,
    retentionKeepDaily: 0,
    retentionKeepWeekly: 0,
    retentionKeepMonthly: 0,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: 0,
    sortOrder: 0,
  };
}

// TargetTestButton probes ONE additional target. The primary editor's "Test
// connection" only ever probes the PRIMARY target, so without this an extra
// destination could sit broken behind that button's green verdict (issue #138).
function TargetTestButton({ id, t }: { id: string; t: T }) {
  const [st, setSt] = useState<"idle" | "busy" | "ok" | "uninit" | "fail">("idle");
  const [err, setErr] = useState<string | null>(null);

  async function go() {
    setSt("busy");
    setErr(null);
    try {
      const r = await testOffsiteTarget(id);
      if (r.ok && r.reachable && r.initialized) setSt("ok");
      else if (r.ok && r.reachable) setSt("uninit");
      else {
        setSt("fail");
        setErr(r.error ?? null);
      }
    } catch (e) {
      setSt("fail");
      setErr(e instanceof Error ? e.message : null);
    }
  }

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <button
        type="button"
        onClick={() => void go()}
        disabled={st === "busy"}
        title={t("offsite.test")}
        className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
      >
        {st === "busy" ? t("offsite.testing") : t("offsite.targets.test")}
      </button>
      {st === "ok" && <span className="text-[11px] text-statusOk">{t("offsite.testOk")}</span>}
      {st === "uninit" && (
        <span className="text-[11px] text-statusWarn">{t("offsite.testUninitialized")}</span>
      )}
      {st === "fail" && (
        <span className="text-[11px] text-statusFail max-w-[18rem] wrap-break-word text-right">
          {err ?? t("offsite.testFailed")}
        </span>
      )}
    </span>
  );
}

export function OffsiteTargetsSection({ domain, t }: { domain: Domain; t: T }) {
  const [targets, setTargets] = useState<OffsiteTarget[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  // The target being edited: null = editor closed; id "" = a new target.
  const [draft, setDraft] = useState<OffsiteTarget | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveErr, setSaveErr] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  // Additional named credential sets (#141 stage 2) this target's CredsRef can
  // pick from — loaded once, shared across every domain's section instance
  // isn't needed here since each mount is cheap and the list rarely changes.
  const [credSets, setCredSets] = useState<CloudCredSetInfo[]>([]);
  useEffect(() => {
    getCloudCredSets()
      .then((r) => { if (r.ok) setCredSets(r.sets ?? []); })
      .catch(() => undefined);
  }, []);

  function refresh() {
    listOffsiteTargets(domain)
      .then((r) => {
        if (r.ok) {
          // Additional targets only: the primary (sortOrder 0, synced from the
          // Settings off-site config) is owned by the single editor above.
          setTargets((r.targets ?? []).filter((x) => x.sortOrder > 0));
          setLoaded(true);
          setLoadErr(null);
        } else {
          setLoadErr(r.error ?? t("offsite.targets.loadError"));
        }
      })
      .catch(() => setLoadErr(t("offsite.targets.loadError")));
  }
  // domain is fixed for a mounted instance (one per off-site domain block).
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(refresh, [domain]);

  function openNew() {
    setDraft(emptyDraft(domain));
    setSaveState("idle");
    setSaveErr(null);
  }

  function openEdit(tgt: OffsiteTarget) {
    setDraft({ ...tgt });
    setSaveState("idle");
    setSaveErr(null);
  }

  function closeEditor() {
    setDraft(null);
    setSaveState("idle");
    setSaveErr(null);
  }

  async function saveDraft() {
    if (!draft) return;
    if (draft.repo.trim() === "") {
      setSaveState("error");
      setSaveErr(t("offsite.targets.repoRequired"));
      return;
    }
    setSaveState("saving");
    setSaveErr(null);
    try {
      if (draft.id === "") {
        // New target: give it a sortOrder strictly greater than 0 (and above any
        // existing additional target) so a later Settings save can never mistake
        // it for the primary and overwrite it.
        const maxSort = targets.reduce((m, x) => Math.max(m, x.sortOrder), 0);
        const r = await createOffsiteTarget({
          domain: draft.domain,
          name: draft.name.trim(),
          repo: draft.repo.trim(),
          credsRef: draft.credsRef,
          storageClass: draft.storageClass,
          immutable: draft.immutable,
          schedule: draft.schedule,
          retentionKeepLast: draft.retentionKeepLast,
          retentionKeepDaily: draft.retentionKeepDaily,
          retentionKeepWeekly: draft.retentionKeepWeekly,
          retentionKeepMonthly: draft.retentionKeepMonthly,
          limitUpload: draft.limitUpload,
          limitDownload: draft.limitDownload,
          growthBudgetGb: draft.growthBudgetGb,
          enabled: draft.enabled,
          sortOrder: maxSort + 1,
        });
        if (!r.ok) throw new Error(r.error ?? t("settings.error"));
      } else {
        const r = await updateOffsiteTarget(draft.id, {
          ...draft,
          name: draft.name.trim(),
          repo: draft.repo.trim(),
        });
        if (!r.ok) throw new Error(r.error ?? t("settings.error"));
      }
      closeEditor();
      refresh();
    } catch (e) {
      setSaveState("error");
      setSaveErr(e instanceof Error ? e.message : t("settings.error"));
    }
  }

  async function remove(id: string) {
    setRemovingId(id);
    try {
      await deleteOffsiteTarget(id);
      setConfirmRemove(null);
      refresh();
    } catch {
      /* best-effort: refresh() below re-syncs the visible list */
    } finally {
      setRemovingId(null);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  const numCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";

  return (
    <div className="mt-2 flex flex-col gap-3 rounded-card bg-carbon-surface2 p-3">
      <div className="flex flex-col gap-0.5">
        <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("offsite.targets.title")}
        </span>
        <p className="text-xs text-carbon-textMuted">{t("offsite.targets.hint")}</p>
        <p className="text-xs text-carbon-textMuted">{t("offsite.targets.scheduleNote")}</p>
      </div>

      {loadErr && <span className="text-xs text-statusFail wrap-break-word">{loadErr}</span>}

      {loaded && targets.length === 0 && !draft && (
        <span className="text-xs text-carbon-textMuted">{t("offsite.targets.none")}</span>
      )}

      {/* Existing additional targets */}
      {targets.map((tgt) => (
        <div
          key={tgt.id}
          className="flex items-start justify-between gap-3 rounded-card bg-carbon-surface p-3"
        >
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-sm text-carbon-text truncate">{tgt.name || tgt.repo}</span>
            <span className="text-xs text-carbon-textMuted font-mono break-all">{tgt.repo}</span>
            <span className="flex flex-wrap gap-2 text-[11px] text-carbon-textSub">
              <span className="rounded-control bg-carbon-surface2 px-1.5 py-0.5">
                {tgt.storageClass || t("cloud.storageClass.default")}
              </span>
              {tgt.immutable && (
                <span className="rounded-control bg-carbon-surface2 px-1.5 py-0.5 text-statusOk">
                  {t("offsite.immutable")}
                </span>
              )}
            </span>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            <TargetTestButton id={tgt.id} t={t} />
            <button
              type="button"
              onClick={() => openEdit(tgt)}
              className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
            >
              {t("offsite.targets.edit")}
            </button>
            {confirmRemove === tgt.id ? (
              <button
                type="button"
                onClick={() => void remove(tgt.id)}
                disabled={removingId === tgt.id}
                className="rounded-control bg-statusFailBg px-2.5 py-1 text-xs font-medium text-statusFail hover:bg-statusFailBgHover disabled:opacity-50"
              >
                {removingId === tgt.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </button>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmRemove(tgt.id)}
                className="rounded-control bg-carbon-surface2 px-2.5 py-1 text-xs text-statusFail hover:bg-carbon-hover"
              >
                {t("offsite.targets.remove")}
              </button>
            )}
          </div>
        </div>
      ))}

      {/* Editor form (new or edit) */}
      {draft && (
        <div className="flex flex-col gap-3 rounded-card bg-carbon-surface p-3">
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.targets.name")}</span>
            <input
              value={draft.name}
              onChange={(e) => setDraft((d) => (d ? { ...d, name: e.target.value } : d))}
              spellCheck={false}
              placeholder={t("offsite.targets.namePlaceholder")}
              className={inputCls}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.wizard.repoUrl")}</span>
            <input
              value={draft.repo}
              onChange={(e) => setDraft((d) => (d ? { ...d, repo: e.target.value } : d))}
              spellCheck={false}
              placeholder={t("offsite.wizard.repoUrlPlaceholder")}
              className={inputCls}
            />
            <span className="text-xs text-carbon-textMuted">{t("offsite.repoLocalHint")}</span>
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.targets.credsLabel")}</span>
            <select
              value={draft.credsRef}
              onChange={(e) => setDraft((d) => (d ? { ...d, credsRef: e.target.value } : d))}
              className={inputCls}
            >
              <option value="">{t("offsite.targets.credsDefault")}</option>
              {credSets.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("cloud.storageClass.label")}</span>
            <select
              value={draft.storageClass}
              onChange={(e) => setDraft((d) => (d ? { ...d, storageClass: e.target.value } : d))}
              className={inputCls}
            >
              <option value="">{t("cloud.storageClass.default")}</option>
              {STORAGE_CLASSES.map((sc) => (
                <option key={sc} value={sc}>
                  {sc}
                </option>
              ))}
            </select>
          </label>

          {/* Append-only (immutable) toggle */}
          <div className="flex items-start justify-between gap-4">
            <div className="flex flex-col gap-0.5">
              <span id={`tgt-imm-${domain}`} className="text-sm text-carbon-text">
                {t("offsite.immutable")}
              </span>
              <span className="text-xs text-carbon-textMuted">{t("offsite.immutableHint")}</span>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={draft.immutable}
              aria-labelledby={`tgt-imm-${domain}`}
              onClick={() => setDraft((d) => (d ? { ...d, immutable: !d.immutable } : d))}
              className={`relative inline-flex h-5 w-9 shrink-0 mt-0.5 items-center rounded-[min(var(--radius-pill),50%)] transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-statusInfoSolid ${
                draft.immutable ? "bg-accent" : "bg-carbon-surface3"
              }`}
            >
              <span
                className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
                  draft.immutable ? "translate-x-[18px]" : "translate-x-[3px]"
                }`}
              />
            </button>
          </div>

          {/* Retention */}
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("offsite.targets.retentionTitle")}</span>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {([
                ["retentionKeepLast", "settings.retentionLast"],
                ["retentionKeepDaily", "settings.retentionDaily"],
                ["retentionKeepWeekly", "settings.retentionWeekly"],
                ["retentionKeepMonthly", "settings.retentionMonthly"],
              ] as const).map(([key, label]) => (
                <label key={key} className="flex flex-col gap-1">
                  <span className="text-xs text-carbon-textSub">{t(label)}</span>
                  <input
                    type="number"
                    min={0}
                    value={draft[key]}
                    onChange={(e) => {
                      const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                      setDraft((d) => (d ? { ...d, [key]: n } : d));
                    }}
                    className={numCls}
                  />
                </label>
              ))}
            </div>
          </div>

          {/* Growth budget */}
          <label className="flex flex-col gap-1 max-w-56">
            <span className="text-xs text-carbon-textSub">{t("offsite.retention.budget")}</span>
            <input
              type="number"
              min={0}
              value={draft.growthBudgetGb}
              onChange={(e) => {
                const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                setDraft((d) => (d ? { ...d, growthBudgetGb: n } : d));
              }}
              className={numCls}
            />
          </label>

          <div className="flex items-center gap-3 flex-wrap">
            <button
              type="button"
              onClick={() => void saveDraft()}
              disabled={saveState === "saving"}
              className="rounded-control bg-accent px-3 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 disabled:opacity-50"
            >
              {saveState === "saving" ? t("common.saving") : t("offsite.targets.save")}
            </button>
            <button
              type="button"
              onClick={closeEditor}
              className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover"
            >
              {t("offsite.targets.cancel")}
            </button>
            {saveState === "error" && saveErr && (
              <span className="text-xs text-statusFail wrap-break-word">{saveErr}</span>
            )}
          </div>
        </div>
      )}

      {/* Add button (hidden while the editor is open) */}
      {!draft && (
        <button
          type="button"
          onClick={openNew}
          className="self-start rounded-control bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover"
        >
          {t("offsite.targets.add")}
        </button>
      )}
    </div>
  );
}
