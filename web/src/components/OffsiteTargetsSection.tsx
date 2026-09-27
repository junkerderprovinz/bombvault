import { useEffect, useState } from "react";
import type { OffsiteTarget } from "../lib/api";
import {
  listOffsiteTargets,
  createOffsiteTarget,
  updateOffsiteTarget,
  deleteOffsiteTarget,
  testOffsiteTarget,
} from "../lib/api";
import { useCloudCredSets } from "../lib/useCloudCredSets";
import { offsiteTargetsChanged, subscribeOffsiteTargets, type OffsiteDomain } from "../lib/useOffsiteTargets";
import { useT } from "../lib/i18n";
import { STORAGE_CLASSES } from "../lib/storageClasses";
import { SelectField } from "./SelectField";
import { Toggle } from "./Toggle";
import { NumberField } from "./NumberField";
import { InfoBubble } from "./InfoBubble";
import { Badge, type BadgeSize } from "./Badge";
import { Button } from "./Button";
import { IconAdd } from "./Sidebar";
import { withLtrFragments, REPO_LOCAL_HINT_LTR_FRAGMENTS } from "../lib/ltrFragments";
import { useToast } from "../lib/toast";

// The badges and the Test, Edit and Remove buttons of a target row share one
// size, so spans and buttons in the row have the same height. Medium is the
// app's usual chip size.
const ROW_BADGE_SIZE: BadgeSize = "medium";

// Editor for a domain's additional off-site targets (sortOrder > 0). The
// primary target (sortOrder 0, synced from the Settings off-site config) has
// its own editor above. This section owns no Settings state: it calls the
// off-site-targets API directly and re-reads its list. Every target of a
// domain replicates on that domain's schedule.
//
// Unlike the rest of Settings, the editor keeps an explicit Save button. A new
// target is created through the API when saved, so saving while the user types
// would create a half-configured destination, and Cancel has to be able to
// throw a draft away.

type Domain = OffsiteDomain;
type T = ReturnType<typeof useT>["t"];
type SaveState = "idle" | "saving";

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
    retentionKeepYearly: 0,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: 0,
    sortOrder: 0,
  };
}

// TargetTestButton probes one additional target. The primary editor's "Test
// connection" probes only the primary.
function TargetTestButton({ id, t }: { id: string; t: T }) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  // Bumped on a failure to replay the shake. A reachable but uninitialised
  // repo is a warning, not a failure, and does not shake.
  const [shake, setShake] = useState(0);

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
        setShake((n) => n + 1);
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.testFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Badge
      key={shake}
      as="button"
      tone="neutral"
      size={ROW_BADGE_SIZE}
      onClick={() => void go()}
      disabled={busy}
      title={t("offsite.test")}
      className={shake ? "glim-tile-raise glim-shake" : "glim-tile-raise"}
    >
      {busy ? t("offsite.testing") : t("offsite.targets.test")}
    </Badge>
  );
}

export function OffsiteTargetsSection({
  domain,
  t,
  hueIndex,
}: {
  domain: Domain;
  t: T;
  /** The enclosing Card's hue, for the add-target button. The row buttons act
   *  on an existing target and stay neutral. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [targets, setTargets] = useState<OffsiteTarget[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  // The target being edited: null = editor closed; id "" = a new target.
  const [draft, setDraft] = useState<OffsiteTarget | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  // One shake nonce per action is enough: only one editor and one remove
  // confirmation can be open at a time.
  const [saveShake, setSaveShake] = useState(0);
  const [removeShake, setRemoveShake] = useState(0);
  // The shared hook rather than a local copy, because CloudCredSetsCard on the
  // same page can add a set while this section is mounted.
  const credSets = useCloudCredSets();

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
  // domain is fixed for a mounted instance. The shared broadcast brings in
  // writes from any section, including a target minted by accepting a fleet
  // mesh offer.
  useEffect(() => {
    refresh();
    return subscribeOffsiteTargets(refresh);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [domain]);

  function openNew() {
    setDraft(emptyDraft(domain));
    setSaveState("idle");
  }

  function openEdit(tgt: OffsiteTarget) {
    setDraft({ ...tgt });
    setSaveState("idle");
  }

  function closeEditor() {
    setDraft(null);
    setSaveState("idle");
  }

  // Save stays enabled while the repo is blank, so the check below is
  // reachable.
  async function saveDraft() {
    if (!draft) return;
    if (draft.repo.trim() === "") {
      push(t("offsite.targets.repoRequired"), "fail");
      setSaveShake((n) => n + 1);
      return;
    }
    setSaveState("saving");
    try {
      if (draft.id === "") {
        // A sortOrder above every existing target, so a later Settings save
        // cannot mistake the new one for the primary and overwrite it.
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
          retentionKeepYearly: draft.retentionKeepYearly,
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
      push(t("settings.saved"), "success");
      closeEditor();
      offsiteTargetsChanged();
    } catch (e) {
      setSaveState("idle");
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
      setSaveShake((n) => n + 1);
    }
  }

  // deleteOffsiteTarget resolves {ok: false} instead of throwing when the
  // server refuses, for example for an append-only target.
  async function remove(id: string) {
    setRemovingId(id);
    try {
      const r = await deleteOffsiteTarget(id);
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        setRemoveShake((n) => n + 1);
        return;
      }
      setConfirmRemove(null);
      offsiteTargetsChanged();
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
      setRemoveShake((n) => n + 1);
    } finally {
      setRemovingId(null);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
  const numCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus-well";

  return (
    <div className="mt-2 flex flex-col gap-3">
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
          className="glim-tile flex items-start justify-between gap-3 rounded-card p-3"
        >
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-sm text-carbon-text truncate">{tgt.name || tgt.repo}</span>
            <span dir="ltr" className="text-xs text-carbon-textMuted font-mono break-all text-start">{tgt.repo}</span>
            {/* A long repo can squeeze this column until the chip labels wrap
                to several lines; without `wrap` the tinted background would
                stay one line tall. */}
            <span className="flex flex-wrap gap-2">
              <Badge tone="neutral" size={ROW_BADGE_SIZE} wrap className="glim-tile-raise">
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
            <Badge as="button" tone="neutral" size={ROW_BADGE_SIZE} onClick={() => openEdit(tgt)} className="glim-tile-raise">
              {t("offsite.targets.edit")}
            </Badge>
            {/* Neutral like Edit, not red. The two-click confirm, whose label
                changes, is what guards the removal. */}
            {confirmRemove === tgt.id ? (
              <Badge
                key={removeShake}
                as="button"
                tone="neutral"
                size={ROW_BADGE_SIZE}
                onClick={() => void remove(tgt.id)}
                disabled={removingId === tgt.id}
                className={removeShake ? "glim-tile-raise glim-shake" : "glim-tile-raise"}
              >
                {removingId === tgt.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </Badge>
            ) : (
              <Badge
                as="button"
                tone="neutral"
                size={ROW_BADGE_SIZE}
                onClick={() => setConfirmRemove(tgt.id)}
                className="glim-tile-raise"
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </div>
      ))}

      {/* Editor form (new or edit) */}
      {draft && (
        <div className="glim-tile flex flex-col gap-3 rounded-card p-3">
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
            <SelectField
              value={draft.credsRef}
              onChange={(v) => setDraft((d) => (d ? { ...d, credsRef: v } : d))}
              label={t("offsite.targets.credsLabel")}
              options={[
                { value: "", label: t("offsite.targets.credsDefault") },
                ...credSets.map((c) => ({ value: c.id, label: c.name })),
              ]}
              className={inputCls}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("cloud.storageClass.label")}</span>
            <SelectField
              value={draft.storageClass}
              onChange={(v) => setDraft((d) => (d ? { ...d, storageClass: v } : d))}
              label={t("cloud.storageClass.label")}
              options={[
                { value: "", label: t("cloud.storageClass.default") },
                ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc })),
              ]}
              className={inputCls}
            />
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
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
              {([
                ["retentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
                ["retentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
                ["retentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
                ["retentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
                ["retentionKeepYearly", "settings.retentionYearly", "settings.retentionYearlyInfo"],
              ] as const).map(([key, label, info]) => (
                <label key={key} className="flex flex-col gap-1">
                  <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                    {t(label)}
                    <InfoBubble tip={t(info)} />
                  </span>
                  <NumberField
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
            <NumberField
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
            <Button
              label={t("offsite.targets.cancel")}
              labelKey="offsite.targets.cancel"
              tone="neutral"
              onClick={closeEditor}
            />
            <Button
              key={saveShake}
              label={t("offsite.targets.save")}
              labelKey="offsite.targets.save"
              tone="accent"
              onClick={() => void saveDraft()}
              disabled={saveState === "saving"}
              busy={saveState === "saving"}
              title={saveState === "saving" ? t("common.saving") : undefined}
              className={saveShake ? "glim-shake" : ""}
            />
          </div>
        </div>
      )}

      {!draft && (
        <Button
          label={t("offsite.targets.add")}
          labelKey="offsite.targets.add"
          glyph={<IconAdd />}
          tone="accent"
          onClick={openNew}
          hueIndex={hueIndex}
          className={"self-start"}
        />
      )}
    </div>
  );
}
