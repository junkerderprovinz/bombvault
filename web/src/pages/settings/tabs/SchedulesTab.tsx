import { useState } from "react";
import {
  ApiError,
  backupEverythingNow,
  patchFileSet,
  setScheduleCadence,
  setVMScheduleCadence,
} from "../../../lib/api";
import { NumberField } from "../../../components/NumberField";
import { InfoBubble } from "../../../components/InfoBubble";
import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { EffectiveScheduleLine } from "../../../components/EffectiveScheduleLine";
import { ItemScheduleOverride } from "../../../components/ItemScheduleOverride";
import { Toggle } from "../../../components/Toggle";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { ScheduleRow, scheduleStatus } from "../../../components/ScheduleBadge";
import type { Settings, Container, VM, FileSetView } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { tLtr } from "../../../lib/ltrFragments";
import { IconBackupNow } from "../../../components/Sidebar";
import { Card, ToggleRow } from "../shared";
import type { SettingsTabProps } from "./types";

function ContainersSection({
  settings,
  containers,
  onChange,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  /** #121: when on, each included container exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = settings.containersSchedule;
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    // Live-review round 5 REVERSES the previous round's "Bei allen
    // Zeitplanpicker Cards soll der Name raus... das ist redundant" removal
    // (jdp: "Den Text in die Cardtitelbadges wieder einfügen, den habe ich
    // nicht gemeint. Den 'Titeltext' aus der Zeitplancard entfernen", put
    // the heading BADGE text back; the duplicate jdp actually meant was the
    // plain-text <legend> INSIDE CadenceBuilder, fixed there instead, see
    // CadenceBuilder.tsx's own header comment for that half of this
    // correction). `title` restored; `hint` stays alongside it exactly as
    // every other title+hint Card in this file already composes both.
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      {/* Cadence row */}
      <ScheduleRow schedule={schedule} />

      {/* Editable cadence builder. `hueIndex` passed straight through, the
          SAME position this Card's own heading notch already got above, not
          a second independent value, so the TimePicker inside picks up this
          Card's own stable rainbow colour (Task 3, jdp: "Der Zeitpicker ist
          nicht im Regenbogenmodus"; see CadenceBuilder's own hueIndex doc). */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {/* Member list */}
      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div key={c.name} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    c.state.toLowerCase() === "running"
                      ? "bg-statusOkSolid"
                      : "bg-carbon-surface3"
                  }`}
                />
                <span className="font-medium text-carbon-text flex-1 truncate">
                  {c.name}
                </span>
                {c.image && (
                  <span className="text-xs text-carbon-textMuted truncate hidden sm:block max-w-xs">
                    {c.image}
                  </span>
                )}
              </div>
              {perItem && (
                <ItemScheduleOverride
                  name={c.name}
                  initial={c.scheduleCadence ?? ""}
                  onSave={(cadence) => setScheduleCadence(c.name, cadence)}
                />
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// Domain section: VMs (editable schedule)
function VMsSection({
  settings,
  syncSchedules,
  onChange,
  vms,
  perItem,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  /** Included VMs, for the per-item override list (#121). */
  vms: VM[];
  /** #121: when on, each included VM exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const included = vms.filter((v) => v.includeInSchedule);

  return (
    // See ContainersSection's own comment above, `title` restored, same
    // Task 3 `hueIndex` threaded into CadenceBuilder below.
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      {/* The editor below stays visible and dimmed while the Containers
          schedule owns this domain, because the cadence it shows still runs -
          so the row says who owns it (see ScheduleRow's own `hint` doc). */}
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {/* Per-item overrides (#121): an included-VM list with a per-VM cadence,
          shown only when the toggle is on so the section is otherwise unchanged. */}
      {perItem && (
        included.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("jobs.noVMsIncluded")}</p>
        ) : (
          <div className="flex flex-col gap-1 divide-y divide-carbon-border">
            {included.map((v) => (
              <div key={v.libvirtName} className="flex flex-col gap-2 py-2 text-sm">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-2 h-2 rounded-full shrink-0 ${
                      v.state.toLowerCase() === "running" ? "bg-statusOkSolid" : "bg-carbon-surface3"
                    }`}
                  />
                  <span className="font-medium text-carbon-text flex-1 truncate">{v.name}</span>
                </div>
                <ItemScheduleOverride
                  name={v.name}
                  initial={v.scheduleCadence ?? ""}
                  // libvirtName, not name: PATCH /api/vms/{name} resolves the
                  // path segment against the raw name (see vmNameParam),
                  // never the TrueNAS display-only friendly name.
                  onSave={(cadence) => setVMScheduleCadence(v.libvirtName, cadence)}
                />
              </div>
            ))}
          </div>
        )
      )}
    </Card>
  );
}

// Domain section: Flash (editable schedule)
function FlashSection({
  settings,
  syncSchedules,
  onChange,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.flashSchedule;

  return (
    // See ContainersSection's own comment above, `title` restored
    // (jobs.flashScheduleHint's own text, added when the title was dropped,
    // stays too: unlike Containers/VMs/Folders, Flash has no per-item member
    // list, so the hint states what a Flash backup actually covers rather
    // than explaining a list). Same Task 3 `hueIndex` threaded into
    // CadenceBuilder below.
    <Card title={t("jobs.flashSection")} hint={tLtr(t, "jobs.flashScheduleHint")} hueIndex={hueIndex}>
      {/* Same synced-owner bubble as VMsSection above. */}
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
        {/* GlimStone follow-up pass: stays permanent text, NOT bubbled, a
            behavioural caveat ("this control looks live but silently does
            nothing yet") someone hits while confused about why a saved
            Flash schedule never runs, not a one-time "what does this do"
            explainer. Same carve-out category as notify.healthchecksLifecycle
            above (NotifyCard's own header comment).
              Live-review round (jdp, Task 7: "Wieso steht in der Flash
            Zeitplan Card die Zeile mit dem Text 'Unraid Flash-
            Konfiguration'? Kann das nicht weg?"): that was a SEPARATE
            trailing "member row" below this paragraph (dot + name +
            "planned" status, styled like a ContainersSection/VMsSection
            member-list row), removed outright, along with its now-orphaned
            jobs.flashRow/jobs.flashPlanned keys, since Flash has no actual
            per-item collection to list and the row conveyed nothing this
            paragraph doesn't already say. The paragraph that stood here is
            gone as well, and so is the carve-out that kept it. It read "the
            Flash backup executor is not yet implemented in Phase 1, the
            schedule is stored but not executed", and it had been false for
            some time: main.go calls SetFlashJob at startup, the scheduler
            runs it on the flash cadence, and the service does the work. It
            told a user their boot drive was unprotected while it was being
            backed up every night.
            Worth keeping the reason it survived: this very comment argued
            for it, citing a task description from back when it was true. A
            caveat that outlives the thing it warned about is a lie with an
            alibi - correct once, and nobody re-reads a sentence that already
            has a justification written next to it. When a caveat's condition
            is fixed, the caveat is part of the fix. */}
      </div>
    </Card>
  );
}

// Domain section: Files (editable schedule + per-set include list). Mirrors
// VMsSection for the cadence and ContainersSection for the member list, except
// the per-set "include in schedule" toggles PATCH each file set directly (the
// same {enabled} flag the Files tab edits), they are not part of the SaveBar.
function FilesSection({
  settings,
  syncSchedules,
  fileSets,
  perItem,
  onChange,
  onSetsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  /** #?: "Container-Zeitplan auch für VMs, Flash und Ordner verwenden"
   *  (jdp, live-review): Folders now follows the same sync toggle VMs/Flash
   *  already had, mirroring their exact pattern below (schedule resolves to
   *  the Containers cadence while synced, its own CadenceBuilder disabled
   *  meanwhile). Previously this section had no syncSchedules concept at
   *  all and always used its own independent settings.filesSchedule. */
  syncSchedules: boolean;
  fileSets: FileSetView[];
  /** Whether per-item schedules are switched on (#199): the row's cadence
   *  editor is hidden entirely while off, exactly as the container rows do it,
   *  so nobody can set a cadence that the scheduler would then ignore. */
  perItem: boolean;
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set, reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.filesSchedule;
  // Per-set toggle busy state, keyed by set id.
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  // GlimStone follow-up pass (v8.0.0): the persistent (never auto-cleared)
  // error paragraph below is now a toast, a toggle failure is a one-shot
  // completion notice like every other migrated site here.
  // Mirrors ContainersSection's setScheduleCadence: the PATCH carries only the
  // cadence, so an edit here cannot disturb the set's name, path or excludes,
  // and the list is reloaded because the server may have reloaded the scheduler.
  async function setFileSetCadence(id: string, cadence: string) {
    const res = await patchFileSet(id, { scheduleCadence: cadence });
    if (res.ok) onSetsChanged();
    return res;
  }

  async function toggle(set: FileSetView) {
    setBusy((b) => ({ ...b, [set.id]: true }));
    try {
      const res = await patchFileSet(set.id, { enabled: !set.enabled });
      if (res.ok) onSetsChanged();
      else push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [set.id]: false }));
    }
  }

  return (
    // See ContainersSection's own comment above, `title` restored, same
    // Task 3 `hueIndex` threaded into CadenceBuilder below.
    <Card title={t("jobs.filesSection")} hint={t("jobs.filesIncludeHint")} hueIndex={hueIndex}>
      {/* Same synced-owner bubble as VMsSection above. */}
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.filesSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {/* Member list: every file set with its live include-in-schedule toggle.  */}
      {fileSets.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noFileSetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {fileSets.map((s) => (
            // #199 gave a folder set its own cadence, so the row grew a second
            // line the way the container rows already had one. The layout
            // follows ContainersSection exactly: the identifying row stays a
            // single flex line, the override sits under it, and it appears only
            // while the per-item-schedules toggle is on.
            <div key={s.id} className="flex flex-col gap-2 py-2 text-sm">
            <div className="flex items-center gap-3">
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  s.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                }`}
              />
              <span className="font-medium text-carbon-text flex-1 min-w-0 truncate">{s.name}</span>
              {s.path && (
                <span dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs text-start">
                  {s.path}
                </span>
              )}
              {/* No-empty-toggles audit (jdp): this row used to `hideLabel`
                  with no visible caption anywhere in the row at all, worse
                  than the Card-title-redundant pattern found elsewhere, since
                  there wasn't even a duplicate label to point to, only the
                  set's own NAME (which identifies the row, not what the
                  switch does). Wrapped in the same `<label>` + sibling
                  `<span>` shape FileSetEnabledToggle then
                  used for a per-row switch: `hideLabel` stays on the bare
                  Toggle (legitimate here, the caller right beside it now
                  draws the same text), but the text is genuinely visible in
                  the row, always, not just conveyed via aria-label. */}
              <label className="flex items-center gap-2 shrink-0 cursor-pointer">
                <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
                <Toggle
                  hideLabel
                  label={`${t("files.enabled")}: ${s.name}`}
                  checked={s.enabled}
                  onChange={() => void toggle(s)}
                  disabled={!!busy[s.id]}
                />
              </label>
            </div>
            {/* #199: the one line that says what actually happens to this set.
                Rendered ALWAYS, not only under `perItem`, because the worst
                outcome ("not backed up automatically") is reachable with the
                per-item toggle off, that is what "Include in schedule" does
                on its own, and it is exactly the state manilx put three of his
                four folders into while believing Backup Everything still
                covered them. */}
            <EffectiveScheduleLine effective={s.effectiveSchedule} />
            {perItem && (
              <ItemScheduleOverride
                name={s.name}
                initial={s.scheduleCadence ?? ""}
                onSave={(cadence) => setFileSetCadence(s.id, cadence)}
              />
            )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

export function EverythingSection({
  settings,
  update,
  t,
  hueIndex,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed action toasts AND shakes its button. `key={shake}` remounts the
  // badge so the CSS animation restarts on a repeat failure.
  const [shake, setShake] = useState(0);

  // The overlap this card warns about is only real when THIS cadence is on
  // AND at least one of the five domain cadences above is too, configSchedule
  // included, since the pass ends with the self-backup.
  const everythingOn = scheduleStatus(settings.everythingSchedule) !== "off";
  const anyDomainOn = [
    settings.containersSchedule,
    settings.vmsSchedule,
    settings.flashSchedule,
    settings.filesSchedule,
    settings.configSchedule,
  ].some((s) => scheduleStatus(s) !== "off");
  const overlapWarning = everythingOn && anyDomainOn;

  async function runNow() {
    if (busy) return; // guard the in-flight window (badge also disables)
    setBusy(true);
    try {
      const res = await backupEverythingNow();
      if (res.ok) {
        push(t("settings.everythingStarted"), "success");
      } else {
        push(res.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        push(t("settings.everythingAlreadyRunning"), "fail");
      } else {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
      }
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title={t("settings.everythingTitle")} hint={t("settings.everythingHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={settings.everythingSchedule} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("settings.everythingTitle")}
          value={settings.everythingSchedule}
          onChange={(v) => update({ everythingSchedule: v })}
          hueIndex={hueIndex}
        />
        {/* Conditional overlap warning, see this component's header for why
            it is no longer permanent. Same markup as the tamper-schedule-
            inactive warning further down: status amber on a plain readout
            surface, which rule 5 keeps OUT of the accent/rainbow engine on
            purpose (a status colour, not control chrome). */}
        {overlapWarning && (
          <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settings.everythingDuplicateWarning")}
          </div>
        )}
      </div>
      <div className="flex flex-col gap-2">
        {/* The group label these two fields never had, carrying the
            what-do-these-do explanation as an InfoBubble instead of the
            permanent paragraph that used to sit here. `hooks.title`
            ("Backup hooks") is an EXISTING key, already translated in all 42
            locales, so this adds no new i18n surface. */}
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("hooks.title")}
          <InfoBubble tip={t("settings.everythingHooksHint")} />
        </span>
        {/* A stored hook is never echoed back, so the field arrives blank with
            ...Set true. It then shows the same "already set" placeholder every
            write-only secret field on this page uses, and a Remove badge, which
            is the only way to actually delete one, a blank field means "keep"
            on save. Both labels are existing keys, so this adds no new i18n. */}
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPreHook}
              onChange={(e) => update({ everythingPreHook: e.target.value })}
              spellCheck={false}
              placeholder={settings.everythingPreHookSet ? t("cloud.secretSet") : "echo starting"}
              className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPreHookSet && (
              <Badge
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPreHook: "", everythingPreHookClear: true })}
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.post")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPostHook}
              onChange={(e) => update({ everythingPostHook: e.target.value })}
              spellCheck={false}
              placeholder={
                settings.everythingPostHookSet ? t("cloud.secretSet") : "curl -fsS https://hc-ping.com/your-uuid"
              }
              className="flex-1 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPostHookSet && (
              <Badge
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPostHook: "", everythingPostHookClear: true })}
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </label>
      </div>
      {/* `justify-end` on the row rather than `ms-auto` on the badge: this
          app's flush-right idiom is ms-auto only when the badge has a leading
          sibling to push away from (Containers' BackupButton/ExportButton
          pair), and there is none here, byte-identical to how Flash's and
          Config's own backup-now cards do it. */}
      <div className="flex justify-end">
        <Button
          key={shake}
          label={t("settings.everythingRunNow")}
          labelKey="settings.everythingRunNow"
          glyph={<IconBackupNow />}
          tone="accent"
          onClick={() => void runNow()}
          disabled={busy}
          busy={busy}
          title={busy ? t("settings.everythingBusy") : undefined}
          className={shake ? "glim-shake" : ""}
        />
      </div>
    </Card>
  );
}

export function SchedulesTab({
  t,
  settings,
  containers,
  vms,
  fileSets,
  syncSchedules,
  schedFieldBusy,
  schedFieldShake,
  syncToggleBusy,
  syncToggleShake,
  configScheduleToggleBusy,
  configScheduleToggleShake,
  loadFileSets,
  fieldPulse,
  scheduleField,
  autoSaveScheduleField,
  scheduleUpdate,
  handleSyncSchedulesToggle,
  toggleConfigSchedule,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* ------------------------------------------------------------------ */}
      {/* SCHEDULES: the single owner of every cadence (migrated from Plans).   */}
      {/* Backup schedules reuse the proven per-domain sections + sync toggle;  */}
      {/* off-site / self-backup / restore-check cadences are edited here too.   */}
      {/* Task 5 (live-review: "Speichern-Buttons können weg, es soll immer     */}
      {/* alles live gespeichert werden"): every field on this tab auto-saves   */}
      {/* itself now (scheduleField/autoSaveScheduleField/handleSyncSchedules-  */}
      {/* Toggle above), there is no tab-wide SaveBar left to persist them.     */}
      {/* ------------------------------------------------------------------ */}
      {/* Schedule options (jdp, live-review: "Die beiden Toggle sollen in
          eine eigene Card"): perItemSchedules (#121) and the Containers-
          sync toggle used to be two raw <input type="checkbox"> rows
          sitting directly in this tab, outside any Card. Both are now the
          shared ToggleRow component, grouped in their own Card, first on
          this tab, directly above ContainersSection, perItem first, sync
          directly below it, per jdp's own ordering. (The group-level
          "Backup-Zeitpläne" Badge heading that used to sit above this
          Card was removed on jdp's live-review ask, the four domain
          schedule Cards below already carry their own clear headings, so
          the group label was redundant; nextHue()'s sequence starts
          directly with this Card now, one call short of before.) A
          genuine two-member list, so each ToggleRow gets its own LOCAL
          hueIndex (0/1, independent of this Card's own nextHue() notch),
          the same "own local 0-based index per group" rule the Domains
          card's seven rows and the merged Colors Card's three rainbow
          toggles already follow. */}
      <Card title={t("settings.schedulesOptions")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.perItemSchedules")}
          hint={t("settings.perItemSchedulesHint")}
          checked={settings.perItemSchedules}
          onChange={(v) => void autoSaveScheduleField("perItemSchedules", v)}
          disabled={schedFieldBusy.perItemSchedules}
          shakeNonce={schedFieldShake.perItemSchedules}
          pulseNonce={fieldPulse.perItemSchedules}
          hueIndex={0}
        />
        {/* Sync toggle: applies the Containers cadence to VMs, Flash AND
            Folders (Task 2 extended this from "VMs + Flash" to also cover
            Folders, see FilesSection's own new syncSchedules prop). */}
        <ToggleRow
          label={t("jobs.syncSchedules")}
          hint={t("jobs.syncSchedulesHint")}
          checked={syncSchedules}
          onChange={(v) => void handleSyncSchedulesToggle(v)}
          disabled={syncToggleBusy}
          shakeNonce={syncToggleShake || undefined}
          // handleSyncSchedulesToggle's own save() patch touches vms/
          // flash/filesSchedule together (see that function), any one
          // of the three is bumped by save()'s success branch, so
          // vmsSchedule works as well as either of the others here.
          pulseNonce={fieldPulse.vmsSchedule}
          hueIndex={1}
        />
      </Card>
      {/* Backup Everything (schedulesEverything): a 6th, independent pass over
          all five domains BELOW + a manual trigger. See EverythingSection's
          own doc comment for the convention pass this card needed after the
          merge, the conditional overlap warning included.
            `update={scheduleUpdate}` rather than the bare setSettings merge
          this shipped with on main: the Schedules tab has no Save button any
          more (jdp: cadences "sollen live gespeichert werden"), so a patch
          that only touched local state here would look saved and be lost on
          reload. scheduleUpdate keeps the same Partial<Settings> shape and
          debounces each key through the shared save(), which is what the
          cadence editor and the two hook text inputs want.

          SECOND on the tab, not last ([413]). It shipped at the bottom,
          below five per-domain cadences, the off-site and self-backup
          schedules and two detail cards, and a forum user could not find it
          at all after being told it was "in Settings", with seven tabs, the
          bottom of the third is not somewhere anybody lands by accident.
          Someone who wants "back the whole server up on one schedule" now
          meets that before the parts it is made of.

          A parallel session made this same move on feature/accent-ink-
          whatsnew (f23cc8e1). It is repeated here because this branch has
          been beside main since 2026-07-18, and its version of this file
          wins for this region on merge: leaving it would have silently
          undone their fix.

            `hueIndex={nextHue()}`, the counter runs in RENDER order, so
          moving the call shifts every rainbow position after it by one and
          needs no renumbering anywhere. That is what the counter is for. */}
      <EverythingSection settings={settings} update={scheduleUpdate} t={t} hueIndex={nextHue()} />
      <ContainersSection
        settings={settings}
        containers={containers}
        onChange={(v) => scheduleField("containersSchedule", v)}
        perItem={settings.perItemSchedules}
        t={t}
        hueIndex={nextHue()}
      />
      <VMsSection
        settings={settings}
        syncSchedules={syncSchedules}
        onChange={(v) => scheduleField("vmsSchedule", v)}
        vms={vms}
        perItem={settings.perItemSchedules}
        t={t}
        hueIndex={nextHue()}
      />
      <FlashSection
        settings={settings}
        syncSchedules={syncSchedules}
        onChange={(v) => scheduleField("flashSchedule", v)}
        t={t}
        hueIndex={nextHue()}
      />
      <FilesSection
        settings={settings}
        syncSchedules={syncSchedules}
        fileSets={fileSets}
        perItem={settings.perItemSchedules}
        onChange={(v) => scheduleField("filesSchedule", v)}
        onSetsChanged={loadFileSets}
        t={t}
        hueIndex={nextHue()}
      />

      {/* Off-site replication schedules (schedulesOffsite): one cadence per
          domain (+ config + files). Editors here are the sole owner of these
          fields. */}
      <Card title={t("settings.schedulesOffsite")} hueIndex={nextHue()}>
        {([
          ["containersOffsiteSchedule", "nav.containers"],
          ["vmsOffsiteSchedule", "nav.vms"],
          ["flashOffsiteSchedule", "nav.flash"],
          ["configOffsiteSchedule", "nav.config"],
          ["filesOffsiteSchedule", "nav.files"],
        ] as const).map(([key, label]) => (
          <div key={key} className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t(label)}</span>
            <input
              value={settings[key]}
              spellCheck={false}
              onChange={(e) => scheduleField(key, e.target.value)}
              placeholder={t("offsite.schedulePlaceholder")}
              dir="ltr"
              className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
            />
          </div>
        ))}
      </Card>

      {/* Self-backup schedule (schedulesSelfBackup): BombVault's own config.
          Live-review (jdp: "Selbst-Backup-Zeitplan bitte mit Toggle für
          an/aus, und Zeitplancard wie bei Container, VMs usw."): this was
          the one schedule editor left in this tab as a bare hand-typed
          cadence <input> (had to type e.g. "daily 02:00" yourself, and
          "off" was only reachable by typing the word), no on/off control
          of its own. Rebuilt to match ContainersSection/VMsSection/
          FlashSection/FilesSection's own shape: the same status row +
          CadenceBuilder-in-a-well, one IIFE-captured `hueIdx` feeding
          both this Card's heading notch and the CadenceBuilder's
          TimePicker inside it, identical to the schedulesChecks Card
          just below (see that IIFE's own comment for why a bare inline
          `hueIndex={nextHue()}` can't feed two hue-aware children from
          one call).
            The toggle's label is NOT hidden behind this Card's own title:
          RestoreChecksSection's `verify.auto` ToggleRow right below
          this one used to hide its own caption the same way (reasoning:
          "the Card's title already says the same thing"), and jdp
          explicitly reversed that exact pattern there ("Bei erstem
          Toggle bitte 'Automatische Restore-Prüfungen' hinschreiben"),
          so this toggle reuses that Card's own corrected shape instead:
          the SAME string as both the Card's `title` and the ToggleRow's
          visible `label`, no `hideLabel`. No `hueIndex` on the toggle
          itself either, matching that same corrected ToggleRow (and
          FlashZipExportCard's lone ToggleRow), a single stand-alone
          switch with no sibling toggles of its own kind in this Card is
          the one case ToggleRow's own hueIndex doc carves out as having
          no list to walk. See toggleConfigSchedule's own comment above
          for why this reuses the cadence string's existing "off" mode
          instead of a new configScheduleEnabled field. */}
      {(() => {
        const hueIdx = nextHue();
        const schedule = settings.configSchedule;
        const status = scheduleStatus(schedule);
        return (
          <Card title={t("settings.schedulesSelfBackup")} hint={t("config.scheduleHint")} hueIndex={hueIdx}>
            <ToggleRow
              label={t("settings.schedulesSelfBackup")}
              checked={status !== "off"}
              onChange={(v) => void toggleConfigSchedule(v)}
              disabled={configScheduleToggleBusy}
              shakeNonce={configScheduleToggleShake}
              pulseNonce={fieldPulse.configSchedule}
            />
            <ScheduleRow schedule={schedule} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.schedulesSelfBackup")}
                value={schedule}
                onChange={(v) => scheduleField("configSchedule", v)}
                hueIndex={hueIdx}
              />
            </div>
          </Card>
        );
      })()}

      {/* Restore-check drills (RestoreChecksSection) moved to the Integrity
          tab (jdp, live-review: "Gehört die 'Automatische Restore-
          Prüfungen' Card nicht in den Integritäts-Tab?"), it configures
          WHAT gets verified and how often, which fits that tab's existing
          verify/unlock/prune/drill actions better than this tab's own
          "when do backup jobs run" focus. See the `tab === "integrity"`
          block below for its new call site; removing it here also frees
          up one `nextHue()` notch, automatically renumbering every Card
          still below on this tab (see that counter's own doc comment for
          why no manual re-numbering is needed). */}

      {/* Missed schedules: anacron-style catch-up after start. Backend runs
          the missed domain job ~2 minutes after boot (see internal/schedule
          CatchUpMissed). */}
      <Card title={t("settings.missedSchedulesTitle")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.catchUpMissed")}
          hint={t("settings.catchUpMissedHint")}
          checked={settings.catchUpMissed}
          onChange={(v) => void autoSaveScheduleField("catchUpMissed", v)}
          disabled={schedFieldBusy.catchUpMissed}
          shakeNonce={schedFieldShake.catchUpMissed}
          pulseNonce={fieldPulse.catchUpMissed}
        />
      </Card>

      {/* Health-gated ordered restart (#119): after a backup that stopped
          other containers ("Stop other containers during backup"), they
          restart in compose depends_on order and each must report
          healthy/running before its dependents start. The wait also holds
          through the post-backup update recreate (see internal/backup
          orchestrator WhileDependentsStopped). */}
      <Card title={t("settings.restartHealthTitle")} hueIndex={nextHue()}>
        {/* Full-page Speichern-Button sweep (jdp, live review, emphatic:
            "Die Speicher-Buttons sollen in allen Tabs weg. Überall soll
            es automatisch speichern."): this was the one Card left on
            the Schedules tab still batched into its own manual SaveBar
            after Task 5 converted every other field here, that task's
            own comment named it as a deliberate exception at the time;
            this pass closes it out with the exact same shapes Task 5
            already established one Card up (autoSaveScheduleField for
            the discrete toggle, scheduleField's debounce for the
            continuously-typed number). */}
        <ToggleRow
          label={t("settings.restartHealthWait")}
          hint={t("settings.restartHealthWaitHint")}
          checked={settings.restartHealthWait}
          onChange={(v) => void autoSaveScheduleField("restartHealthWait", v)}
          disabled={schedFieldBusy.restartHealthWait}
          shakeNonce={schedFieldShake.restartHealthWait}
          pulseNonce={fieldPulse.restartHealthWait}
        />
        {settings.restartHealthWait && (
          <label className="flex flex-col gap-1 sm:w-1/2">
            {/* Live-review round 3 sweep: the range explainer used to sit
                as a permanent caption below the field. Moved beside the
                field's own label as an InfoBubble, the exact pattern the
                retention grid further down already uses for a field-
                level "what does this number mean" note. */}
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("settings.restartHealthTimeoutLabel")}
              <InfoBubble tip={t("settings.restartHealthTimeoutHint")} />
            </span>
            <NumberField
              min={5}
              max={3600}
              value={settings.restartHealthTimeoutSec}
              onChange={(e) => {
                const raw = (e.target as unknown as { value: string }).value;
                // Clamp to the field minimum (5): never let a transient sub-5
                // value sit in component state. The server clamps to 5..3600.
                const n = Math.max(5, parseInt(raw, 10) || 0);
                scheduleField("restartHealthTimeoutSec", n);
              }}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
            />
          </label>
        )}
      </Card>

      {/* Restore-check schedule (schedulesChecks) moved to the Integrity
          tab alongside RestoreChecksSection above (jdp, live-review,
          same "belongs with WHAT/how-often gets verified, not WHEN
          backup jobs run" reasoning). See the `tab === "integrity"`
          block below for its new call site. */}

      {/* Backup Everything used to be here, as the last card on the tab.
          It now renders SECOND, right after Schedule options, see its call
          site up there for why ([413]). */}

      {/* No SaveBar: every field in this tab auto-saves, see scheduleField
          / autoSaveScheduleField. main's buildSchedulePatch() is gone. */}
    </>
  );
}
