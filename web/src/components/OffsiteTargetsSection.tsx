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
import { Toggle } from "./Toggle";
import { Badge, type BadgeSize } from "./Badge";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { useToast } from "../lib/toast";

// The storage-class/immutable badges AND the Test/Edit/Remove buttons in a
// target row render through Badge at this ONE shared stage, so their heights
// stay pixel-identical regardless of the <span> vs <button> element
// underneath — the same mechanism (and the same audit finding class) as
// ErrorDetailPanel's count-badge + "Resolve"-button pair. "medium" (20px),
// not "small" (18px, the badges' pre-fix stage) or "large" (24px, the
// buttons' pre-fix stage): it's the dominant weight everywhere else in the
// app (see Badge.tsx's file header), so fixing the parity here lands both
// elements on the app's normal chip size instead of introducing a
// one-off-large row in an otherwise compact settings panel.
const ROW_BADGE_SIZE: BadgeSize = "medium";

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
//
// GlimStone follow-up pass (v8.0.0): the ok/uninit/fail verdict below moved to
// toasts — this button is the exact near-duplicate of Settings.tsx's
// TestConnectionButton (same ok/uninit/fail shape, just probing an additional
// target instead of the primary), which already made this move; this button
// was apparently just missed in that pass.
function TargetTestButton({ id, t }: { id: string; t: T }) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);

  async function go() {
    setBusy(true);
    try {
      const r = await testOffsiteTarget(id);
      if (r.ok && r.reachable && r.initialized) {
        push(t("offsite.testOk"), "success");
      } else if (r.ok && r.reachable) {
        push(t("offsite.testUninitialized"), "warn");
      } else {
        push(r.error ?? t("offsite.testFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.testFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Badge
      as="button"
      tone="neutral"
      size={ROW_BADGE_SIZE}
      onClick={() => void go()}
      disabled={busy}
      title={t("offsite.test")}
    >
      {busy ? t("offsite.testing") : t("offsite.targets.test")}
    </Badge>
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
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const numCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full bv-field-focus-well";

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
            <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">{tgt.repo}</span>
            {/* `wrap` on BOTH chips, for the same reason the Dashboard
                protection badges carry it: they sit in a min-w-0 column that a
                long `repo` string (rendered break-all above) lets collapse to a
                fraction of the chip's natural width, so as flex items they get
                squeezed and their multi-word labels ("Immutable (append-only)",
                "(provider default)", longer still in most locales) wrap to two
                or three lines. Without `wrap` the stage's fixed h-* keeps the
                tinted background one line tall and the extra lines paint
                outside it. */}
            <span className="flex flex-wrap gap-2">
              <Badge tone="neutral" size={ROW_BADGE_SIZE} wrap>
                {tgt.storageClass || t("cloud.storageClass.default")}
              </Badge>
              {tgt.immutable && (
                <Badge tone="ok" size={ROW_BADGE_SIZE} wrap>
                  {t("offsite.immutable")}
                </Badge>
              )}
            </span>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            <TargetTestButton id={tgt.id} t={t} />
            <Badge as="button" tone="neutral" size={ROW_BADGE_SIZE} onClick={() => openEdit(tgt)}>
              {t("offsite.targets.edit")}
            </Badge>
            {confirmRemove === tgt.id ? (
              <Badge
                as="button"
                tone="fail"
                size={ROW_BADGE_SIZE}
                onClick={() => void remove(tgt.id)}
                disabled={removingId === tgt.id}
              >
                {removingId === tgt.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </Badge>
            ) : (
              <Badge
                as="button"
                tone="fail"
                size={ROW_BADGE_SIZE}
                onClick={() => setConfirmRemove(tgt.id)}
              >
                {t("offsite.targets.remove")}
              </Badge>
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
              dir="ltr"
              className={`${inputCls} text-start`}
            />
            <span className="text-xs text-carbon-textMuted">
              {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
            </span>
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
              <span className="text-sm text-carbon-text">{t("offsite.immutable")}</span>
              <span className="text-xs text-carbon-textMuted">{t("offsite.immutableHint")}</span>
            </div>
            <Toggle
              hideLabel
              label={t("offsite.immutable")}
              checked={draft.immutable}
              onChange={(v) => setDraft((d) => (d ? { ...d, immutable: v } : d))}
              className="mt-0.5"
            />
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
