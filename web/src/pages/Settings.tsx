import { useEffect, useRef, useState, type CSSProperties } from "react";
import {
  ApiError,
  backupEverythingNow,
  getAuth,
  patchFileSet,
  patchZFSDataset,
  replicateOffsite,
  setAuthPassword,
  setScheduleCadence,
  setVMScheduleCadence,
  testOffsite,
} from "../lib/api";
import { useOffsiteTargets, type OffsiteDomain } from "../lib/useOffsiteTargets";
import { AccentCard, IconResetArrow } from "./settings/AccentCard";
import { PasskeyCard } from "./settings/PasskeyCard";
import { TwoFactorCard } from "./settings/TwoFactorCard";
import { ThemeCard } from "./settings/ThemeCard";
import { RestoreChecksSection } from "./settings/RestoreChecksSection";
import { AnomalyCard } from "./settings/AnomalyCard";
import { useAnomalySummary } from "../lib/useAnomalies";
import { NumberField } from "../components/NumberField";
import { OffsiteWizard } from "../components/OffsiteWizard";
import { DestinationsCard } from "./settings/DestinationsCard";
import { OffsiteLocationInput } from "../components/placement/OffsiteLocationInput";
import { CompressionSelector, saveCompression } from "../components/CompressionSelector";
import {
  CONTROL_AXES,
  LABEL_MODES,
  getLabelMode,
  setLabelMode,
  type ControlAxis,
  type LabelMode,
} from "../lib/controls";
import { labelModeChanged } from "../lib/useLabelMode";
import { InfoBubble } from "../components/InfoBubble";
import { OffsiteTargetsSection } from "../components/OffsiteTargetsSection";
import { PageTitle } from "../components/PageTitle";
import { StreamingCard } from "./settings/StreamingCard";
import { IdleCard } from "./settings/IdleCard";
// Every cadence picker on this page edits a schedule that can count an
// interval (#166): the six domains and Backup Everything from their last
// successful backup, drills, tamper test and digest from schedule_job_runs.
// The off-site cadences are raw text inputs; rejectEveryNSchedules refuses
// everyN for them on the server.
import { CadenceBuilder } from "../components/CadenceBuilder";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { TestButton, VerdictLine } from "../components/TestButton";
import { useTestVerdict } from "../lib/useTestVerdict";
import { offsiteVerdict } from "../lib/offsiteVerdict";
import { EffectiveScheduleLine } from "../components/EffectiveScheduleLine";
import { ItemScheduleOverride } from "../components/ItemScheduleOverride";
import { Toggle } from "../components/Toggle";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { ScheduleRow, scheduleStatus } from "../components/ScheduleBadge";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import type { Settings, Container, VM, FileSetView, ZFSDatasetView } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { REPO_LOCAL_HINT_LTR_FRAGMENTS, tLtr, withLtrFragments } from "../lib/ltrFragments";
import { useAdvanced } from "../lib/advanced";
import { SpikePanel } from "../components/SpikePanel";
import { ColorPickerSwatch } from "../components/ColorPickerPopover";
import { RAINBOW, getRainbow, setRainbow, type RainbowState } from "../lib/appearance";
import { SHAPES, getShape, leafTap, setShape, type Shape } from "../lib/shape";
import {
  MOTION_INTENSITIES,
  getMotionIntensity,
  setMotionIntensity,
  stormTap,
  type MotionIntensity,
} from "../lib/motion";
import { applyStoredDisco, discoTap, getDisco, setDisco } from "../lib/disco";
import { HUE_OFFSET, Selector } from "../components/Selector";
import { Navigate, useLocation, useParams } from "react-router-dom";
import { SettingsRail } from "../components/SettingsRail";
import { SettingsSearch, jumpTarget, markHit, type SearchJump } from "./settings/SettingsSearch";
import {
  FALLBACK_PAGE,
  LEGACY_HASH,
  isSettingsPage,
  orderPages,
  readLastPage,
  readOrder,
  writeLastPage,
  writeOrder,
  type SettingsPageId,
} from "./settings/settingsPages";
import { IconBackupNow, IconCheck, IconSync, IconGear, IconClose } from "../components/Sidebar";
import { NotifyCard } from "./settings/NotifyCard";
import { Card, LOGIN_PASSWORD_FIELD, ToggleRow, type SaveState } from "./settings/shared";
import { IntegrityCard } from "./settings/IntegrityCard";
import { VMSSHCard } from "./settings/VMSSHCard";
import { DashboardWidgetCard } from "./settings/DashboardWidgetCard";
import { McpServerCard } from "./settings/McpServerCard";
import { ApiTokensCard } from "./settings/ApiTokensCard";
import { HomeAssistantCard } from "./settings/HomeAssistantCard";
import { NetworkCard } from "./settings/NetworkCard";
import { mcpShipped } from "../lib/mcpSwitch";
import { SettingsStoreContext, useSettingsStore } from "./settings/settingsStore";
import { GeneralPage } from "./settings/pages/GeneralPage";
import { StoragePage } from "./settings/pages/StoragePage";
import { RetentionPage } from "./settings/pages/RetentionPage";
import { ContainersPage } from "./settings/pages/ContainersPage";
import { CloudPage } from "./settings/pages/CloudPage";
import { PairingPage } from "./settings/pages/PairingPage";
import { AppsPage } from "./settings/pages/AppsPage";
import { SystemPage } from "./settings/pages/SystemPage";

// PaletteSwatch is one editable colour in the rainbow palette editor. It opens
// the shared colour popover instead of a native colour input, which would open
// a separate OS window, and uses `rounded-pill` rather than `rounded-full` so
// it follows the shape setting through var(--radius-pill).
function PaletteSwatch({
  hex,
  index,
  disabled,
  onChange,
  t,
}: {
  hex: string;
  index: number;
  disabled?: boolean;
  onChange: (hex: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const label = `${t("settings.rainbowPalette")} ${index + 1}`;
  // h-8 w-8 matches the reset badge in the same row: every square icon badge
  // in the app is 32px, so the swatch follows the badge.
  return (
    <ColorPickerSwatch
      value={hex}
      onChange={onChange}
      label={label}
      disabled={disabled}
      className="h-8 w-8 shrink-0 rounded-pill border-2 border-carbon-border transition-transform hover:scale-110 disabled:cursor-not-allowed disabled:opacity-50"
    />
  );
}

function ReplicateNowButton({
  domain,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  t: ReturnType<typeof useT>["t"];
  /** Hue position of the enclosing domain card, so the button takes its colour. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  async function go() {
    setBusy(true);
    try {
      const r = await replicateOffsite(domain);
      if (r.ok) {
        push(t("offsite.replicateStarted"), "success");
      } else {
        push(r.error ?? t("offsite.replicateFailed"), "fail");
      }
    } catch (e) {
      push(e instanceof Error ? e.message : t("offsite.replicateFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }
  return (
    // The label is the stable name; the running state rides in `title` and in
    // the spinner. A label that changed to "Replicating…" mid-action would
    // resize the button while you look at it, which is what the width stages
    // exist to prevent (#178).
    <Button
      label={t("offsite.replicateNow")}
      labelKey="offsite.replicateNow"
      glyph={<IconSync />}
      tone="accent"
      hueIndex={hueIndex}
      onClick={() => void go()}
      disabled={busy}
      busy={busy}
      title={busy ? t("offsite.replicating") : undefined}
    />
  );
}

// OffsiteDomainBar heads a domain's off-site card: the connection test,
// replicate now and the setup switch, with the test's verdict line above them.
function OffsiteDomainBar({
  domain,
  label,
  repo,
  wizardOpen,
  onToggleWizard,
  t,
  hueIndex,
}: {
  domain: OffsiteDomain;
  label: string;
  repo: string;
  wizardOpen: boolean;
  onToggleWizard: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  // The test probes the primary target only, and each additional target has
  // its own Test in OffsiteTargetsSection (#138). With more than one
  // destination the tooltip says so; as a label it would change the button's
  // width the moment a second destination is added.
  const multiTarget = useOffsiteTargets(domain).length > 1;
  // Opening the wizard counts as a change: credentials edited there are not
  // part of `repo`.
  const test = useTestVerdict([repo, wizardOpen], t("offsite.testFailed"));
  return (
    <>
      <VerdictLine verdict={test.verdict} />
      <div className="flex items-center justify-between">
        <span className="text-xs text-carbon-textSub">{label}</span>
        <span className="inline-flex items-center gap-2">
          {repo && !wizardOpen && (
            <>
              <TestButton
                label={t("offsite.test")}
                labelKey="offsite.test"
                glyph={<IconCheck />}
                tone="accent"
                hueIndex={hueIndex}
                test={test}
                onClick={() => void test.run(async () => offsiteVerdict(await testOffsite(domain), t))}
                title={multiTarget ? t("offsite.testPrimary") : undefined}
              />
              <ReplicateNowButton domain={domain} t={t} hueIndex={hueIndex} />
            </>
          )}
          {/* The one place a swapping label is right: open and close are
              two different actions with two different glyphs, not one
              action reporting its state. Both names are short enough to
              share a width stage, so the control does not jump. */}
          <Button
            label={wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
            labelKey={wizardOpen ? "offsite.wizard.close" : "offsite.wizard.setup"}
            glyph={wizardOpen ? <IconClose /> : <IconGear />}
            tone="accent"
            hueIndex={hueIndex}
            onClick={onToggleWizard}
          />
        </span>
      </div>
    </>
  );
}

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
    <Card title={t("jobs.containersSection")} hint={t("containers.scheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} />

      {/* The card's own hueIndex, so the time picker takes the card's colour
          in rainbow mode. */}
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

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
    <Card title={t("jobs.vmsSection")} hint={t("jobs.vmIncludeHint")} hueIndex={hueIndex}>
      {/* The editor stays visible and dimmed while the Containers schedule
          owns this domain, because the cadence it shows still runs, so the row
          says who owns it. */}
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
    // jobs.flashScheduleHint says what a Flash backup covers: unlike
    // Containers, VMs, Folders and ZFS, Flash has no per-item list to show it.
    <Card title={t("jobs.flashSection")} hint={tLtr(t, "jobs.flashScheduleHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>
    </Card>
  );
}

// FilesSection mirrors VMsSection for the cadence and ContainersSection for the
// member list, except that each set's "include in schedule" toggle PATCHes the
// set directly (the same {enabled} flag the Files page edits).
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
  /** While synced, the section follows the Containers cadence and its own
   *  editor is disabled, as for VMs and Flash. */
  syncSchedules: boolean;
  fileSets: FileSetView[];
  /** Whether per-item schedules are switched on (#199): the row's cadence
   *  editor is hidden while off, as the container rows do it, so nobody can
   *  set a cadence that the scheduler would then ignore. */
  perItem: boolean;
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set; reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.filesSchedule;
  const [busy, setBusy] = useState<Record<string, boolean>>({});

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
    <Card title={t("jobs.filesSection")} hint={t("jobs.filesIncludeHint")} hueIndex={hueIndex}>
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

      {fileSets.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noFileSetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {fileSets.map((s) => (
            // Same layout as ContainersSection: the identifying row stays one
            // flex line and the override (#199) sits under it while per-item
            // schedules are on.
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
              {/* The set name identifies the row, not what the switch does,
                  so the switch gets a visible caption beside it. */}
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
            {/* Says what actually happens to this set (#199). Rendered even
                with per-item schedules off, because "Include in schedule" on
                its own can leave a set not backed up automatically while
                users believe Backup Everything still covers it. */}
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

// Domain section for ZFS, in the same shape as FilesSection: the cadence card
// plus one row per item whose "include in schedule" toggle PATCHes the item
// directly.
function ZFSSection({
  settings,
  syncSchedules,
  items,
  perItem,
  onChange,
  onItemsChanged,
  t,
  hueIndex,
}: {
  settings: Settings;
  syncSchedules: boolean;
  items: ZFSDatasetView[];
  perItem: boolean;
  onChange: (schedule: string) => void;
  onItemsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
}) {
  const { push } = useToast();
  const schedule = syncSchedules ? settings.containersSchedule : settings.zfsSchedule;
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  async function setItemCadence(id: string, cadence: string) {
    const res = await patchZFSDataset(id, { scheduleCadence: cadence });
    if (res.ok) onItemsChanged();
    return res;
  }

  async function toggle(item: ZFSDatasetView) {
    setBusy((b) => ({ ...b, [item.id]: true }));
    try {
      const res = await patchZFSDataset(item.id, { enabled: !item.enabled });
      if (res.ok) onItemsChanged();
      else push(res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [item.id]: false }));
    }
  }

  return (
    <Card title={t("jobs.zfsSection")} hint={t("jobs.zfsIncludeHint")} hueIndex={hueIndex}>
      <ScheduleRow schedule={schedule} hint={syncSchedules ? t("jobs.syncSchedulesHint") : undefined} />
      <div className="rounded-card bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.zfsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
          hueIndex={hueIndex}
        />
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noZfsDatasetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {items.map((d) => (
            <div key={d.id} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    d.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                  }`}
                />
                <span dir="ltr" className="font-medium text-carbon-text flex-1 min-w-0 truncate text-start">
                  {d.dataset}
                </span>
                {d.hostMountpoint && (
                  <span dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs text-start">
                    {d.hostMountpoint}
                  </span>
                )}
                <label className="flex items-center gap-2 shrink-0 cursor-pointer">
                  <span className="text-xs text-carbon-textSub">{t("files.enabled")}</span>
                  <Toggle
                    hideLabel
                    label={`${t("files.enabled")}: ${d.dataset}`}
                    checked={d.enabled}
                    onChange={() => void toggle(d)}
                    disabled={!!busy[d.id]}
                  />
                </label>
              </div>
              <EffectiveScheduleLine effective={d.effectiveSchedule} domainLabelKey="jobs.zfsSection" />
              {perItem && (
                <ItemScheduleOverride
                  name={d.dataset}
                  initial={d.scheduleCadence}
                  onSave={(cadence) => setItemCadence(d.id, cadence)}
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
  // A failed action toasts and shakes its button. `key={shake}` remounts the
  // button so the CSS animation restarts on a repeat failure.
  const [shake, setShake] = useState(0);

  // The overlap this card warns about is only real when this cadence is on
  // and at least one domain cadence is too, the self-backup included, since
  // the pass ends with it.
  const everythingOn = scheduleStatus(settings.everythingSchedule) !== "off";
  const anyDomainOn = [
    settings.containersSchedule,
    settings.vmsSchedule,
    settings.flashSchedule,
    settings.filesSchedule,
    settings.zfsSchedule,
    settings.configSchedule,
  ].some((s) => scheduleStatus(s) !== "off");
  const overlapWarning = everythingOn && anyDomainOn;

  async function runNow() {
    if (busy) return; // guard the in-flight window (the button also disables)
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
        {/* Status amber on a plain surface, outside the accent and rainbow
            engine: it is a status colour, not control chrome. */}
        {overlapWarning && (
          <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settings.everythingDuplicateWarning")}
          </div>
        )}
      </div>
      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-1 text-xs text-carbon-textSub">
          {t("hooks.title")}
          <InfoBubble tip={t("settings.everythingHooksHint")} />
        </span>
        {/* A stored hook is never echoed back, so the field arrives blank with
            ...Set true and shows the "already set" placeholder. A blank field
            means "keep" on save, so the Remove badge is the only way to delete
            a hook. */}
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
          <div className="flex items-center gap-2">
            <input
              value={settings.everythingPreHook}
              onChange={(e) => update({ everythingPreHook: e.target.value })}
              spellCheck={false}
              placeholder={settings.everythingPreHookSet ? t("cloud.secretSet") : "echo starting"}
              className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPreHookSet && (
              <Badge
                as="button"
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPreHook: "", everythingPreHookClear: true })}
                className="pointer-coarse:h-(--btn-h) pointer-coarse:px-3"
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
              className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus"
            />
            {settings.everythingPostHookSet && (
              <Badge
                as="button"
                tone="active"
                size="small"
                hueIndex={hueIndex}
                onClick={() => update({ everythingPostHook: "", everythingPostHookClear: true })}
                className="pointer-coarse:h-(--btn-h) pointer-coarse:px-3"
              >
                {t("offsite.targets.remove")}
              </Badge>
            )}
          </div>
        </label>
      </div>
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

// bv-convention-exception: page-uses-page-shell: the rail stands beside the
// page, and the content column next to it carries PAGE_SHELL_RESPONSIVE.
export function SettingsPage() {
  const { t } = useT();
  const { summary: anomalySummary } = useAnomalySummary();
  const { advanced } = useAdvanced();
  const { push } = useToast();
  const store = useSettingsStore();
  const {
    settings,
    setSettings,
    savedBaseline,
    loadError,
    platformKind,
    confirmDialog,
    allTargets,
    authEnabled,
    setAuthEnabled,
    totpEnabled,
    setTotpEnabled,
    recoveryLeft,
    setRecoveryLeft,
    minPasswordLen,
    containers,
    vms,
    fileSets,
    zfsItems,
    loadFileSets,
    loadZFSItems,
    save,
    debouncedSave,
    fieldPulse,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
    scheduleField,
    autoSaveScheduleField,
    scheduleUpdate,
    schedFieldBusy,
    schedFieldShake,
    syncSchedules,
    handleSyncSchedulesToggle,
    syncToggleBusy,
    syncToggleShake,
    toggleConfigSchedule,
    configScheduleToggleBusy,
    configScheduleToggleShake,
  } = store;

  const { page: param } = useParams();
  const page: SettingsPageId = isSettingsPage(param) ? param : FALLBACK_PAGE;
  const location = useLocation();
  const { hash } = location;
  // /settings alone names no page: a legacy hash from an old link leads to the
  // page holding that card, otherwise the page opened last.
  const redirect = isSettingsPage(param)
    ? null
    : param === undefined
      ? (LEGACY_HASH[hash.replace(/^#/, "")] ?? readLastPage())
      : FALLBACK_PAGE;
  const [order, setOrder] = useState(readOrder);
  const pages = orderPages(order);
  // A page slides in from the side its tile sits on in the rail.
  const shownPage = useRef(page);
  const pageDir = useRef<1 | -1>(1);
  if (shownPage.current !== page) {
    const ids = pages.map((p) => p.id);
    pageDir.current = ids.indexOf(page) > ids.indexOf(shownPage.current) ? 1 : -1;
    shownPage.current = page;
  }
  // Only a page named in the address counts: /settings alone renders the
  // fallback for a moment before its redirect, and must not overwrite it.
  useEffect(() => {
    if (isSettingsPage(param)) writeLastPage(param);
  }, [param]);

  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwSaveState, setPwSaveState] = useState<SaveState>("idle");
  const [pwSaveMsg, setPwSaveMsg] = useState<string | null>(null);
  const [pwSaveShake, setPwSaveShake] = useState(0);
  const revealPwNew = useReveal();
  const revealPwConfirm = useReveal();

  const revealMetricsToken = useReveal();

  const [shape, setShapeLocal] = useState<Shape>(() => getShape());
  // The hidden leaf follows the storm below: found and counted in this
  // screen's state, never in storage, so it is offered only while chosen or
  // until this page is left.
  const [leafFound, setLeafFound] = useState(false);
  const leafClicks = useRef({ taps: 0 });

  const [motion, setMotionLocal] = useState<MotionIntensity>(() => getMotionIntensity());
  // The storm is the hidden fourth motion level. Both of these are component
  // state: `stormFound` must not survive leaving this page (an egg that
  // changes behaviour has to be switchable back off, never a permanent picker
  // entry), and the click counter has nothing to remember past the gesture.
  const [stormFound, setStormFound] = useState(false);
  const stormClicks = useRef({ taps: 0 });
  // Disco, the colour engine's own hidden mode, with the same two-part shape
  // the storm above uses: `discoFound` is component state so a found egg is
  // not a permanent row, and the counter has nothing to remember once the
  // gesture completes. Unlike the storm's, this counter carries a timestamp,
  // because its gesture is five turn-ons of Rainbow Mode and somebody merely
  // comparing the mode on and off would otherwise unlock it by accident.
  const [discoFound, setDiscoFound] = useState(false);
  const [disco, setDiscoLocal] = useState<boolean>(() => getDisco());
  const discoClicks = useRef({ taps: 0, last: 0 });
  // The label modes (#178), mirrored into local state so the selectors show
  // the current choice. The controls read through useLabelMode, which the
  // labelModeChanged() call below wakes.
  const [labelModes, setLabelModes] = useState<Record<ControlAxis, LabelMode>>(() => ({
    buttons: getLabelMode("buttons"),
    sidebar: getLabelMode("sidebar"),
    tabs: getLabelMode("tabs"),
    bottombar: getLabelMode("bottombar"),
  }));

  // setRainbow() persists, applies and returns the validated state, so local
  // state is updated from that return value rather than a second read.
  const [rainbow, setRainbowLocal] = useState<RainbowState>(() => getRainbow());
  function updateRainbow(patch: Partial<RainbowState>) {
    setRainbowLocal(setRainbow(patch));
    // The disco walk reads the rainbow state, so a rainbow change has to
    // re-decide whether it runs: switching rainbow off parks it, switching
    // rainbow back on resumes it without touching the disco switch itself.
    applyStoredDisco();
  }

  /** Rainbow Mode's own onChange, which doubles as the disco unlock gesture:
   *  five turn-ons inside disco.ts's window. Only turn-ons count, so the
   *  gesture ends with rainbow on, which is the one state where a walking
   *  palette is visible at all. */
  function rainbowToggled(on: boolean) {
    updateRainbow({ on });
    if (discoTap(discoClicks.current, on, { now: Date.now() })) setDiscoFound(true);
  }

  const [, setOffsiteSaveState] = useState<SaveState>("idle");
  const [, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<OffsiteDomain | null>(null);

  const [, setLimSaveState] = useState<SaveState>("idle");
  const [, setLimSaveError] = useState<string | null>(null);

  const [, setMetricsSaveState] = useState<SaveState>("idle");
  const [, setMetricsSaveError] = useState<string | null>(null);

  // Weekly digest and overdue-backup watchdog on the notifications page.
  const [, setDigestSaveState] = useState<SaveState>("idle");
  const [, setDigestSaveError] = useState<string | null>(null);

  const [, setWatchdogSaveState] = useState<SaveState>("idle");
  const [, setWatchdogSaveError] = useState<string | null>(null);

  // Anomalies card (integrity page): every field saves on its own through
  // autoSaveToggle, which puts the old value back when the save is refused.
  const [, setAnomalySaveState] = useState<SaveState>("idle");
  const [, setAnomalySaveError] = useState<string | null>(null);

  // A hash such as /settings/integrity#anomalies names a card below the fold,
  // or a field that should take the cursor, and a search result names a card
  // and row to mark. Anything else opens a page at its top, since the scroller
  // is shared by every page.
  const settingsLoaded = settings !== null;
  const handledJump = useRef("");
  useEffect(() => {
    if (!settingsLoaded) return;
    const jump = (location.state as { jump?: SearchJump } | null)?.jump;
    const anchor = hash.replace(/^#/, "");
    if (jump && handledJump.current !== location.key) {
      handledJump.current = location.key;
      const content = document.querySelector<HTMLElement>("[data-settings-content]");
      const target = content && (jump.card || jump.row) ? jumpTarget(content, jump) : null;
      if (target) {
        // jsdom has no scrollIntoView.
        target.scrollIntoView?.({ block: "center" });
        markHit(target);
        // Cards that load their own settings grow after the jump, this one or
        // those above it, so the target is centred again while they settle.
        if (content && typeof ResizeObserver !== "undefined") {
          const settle = new ResizeObserver(() => target.scrollIntoView({ block: "center" }));
          settle.observe(content);
          window.setTimeout(() => settle.disconnect(), 1200);
        }
      } else if (jump.card) {
        push(t("settings.search.notShown").replace("{name}", jump.row ?? jump.card), "warn");
      }
      if (target || jump.card) return;
    }
    if (anchor) {
      const target = document.getElementById(anchor);
      if (target instanceof HTMLInputElement) {
        target.scrollIntoView?.({ block: "center" });
        target.focus();
      } else {
        target?.scrollIntoView?.({ block: "start" });
      }
      return;
    }
    document.getElementById("bv-main")?.scrollTo?.({ top: 0 });
  }, [location, hash, settingsLoaded, page]); // eslint-disable-line react-hooks/exhaustive-deps

  if (loadError) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-statusFail">{loadError}</p>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      </div>
    );
  }

  // The password keeps a manual Save button. Two fields must agree before a
  // write is safe, so there is no keystroke to auto-save on, and
  // setAuthPassword takes effect at once for the whole instance (a blank
  // password switches login off). A mismatch stays inline next to the fields
  // rather than in a toast that could vanish while the user is still typing.
  async function handleSetPassword() {
    if (pwNew !== pwConfirm) {
      setPwSaveMsg(t("auth.passwordMismatch"));
      setPwSaveState("error");
      return;
    }
    // The server refuses a short password too, and its answer is what the
    // user would eventually see. Checking here as well saves a round trip and
    // puts the message next to the field instead of in a toast. An empty
    // password is not "too short": it means "switch authentication off".
    if (pwNew !== "" && [...pwNew].length < minPasswordLen) {
      setPwSaveMsg(t("auth.passwordMinHint", minPasswordLen));
      setPwSaveState("error");
      setPwSaveShake((n) => n + 1);
      return;
    }
    setPwSaveState("saving");
    setPwSaveMsg(null);
    try {
      const res = await setAuthPassword(pwNew);
      if (res.ok) {
        setAuthEnabled(res.enabled ?? false);
        // The same response carries the session, so the card can go straight to
        // its signed-in state; otherwise enabling the second factor would answer
        // 401 until a reload, because the new login had issued no session yet.
        setPwSaveState("idle");
        push(pwNew === "" ? t("auth.passwordCleared") : t("auth.passwordSaved"), "success");
        setPwNew("");
        setPwConfirm("");
      } else {
        setPwSaveState("idle");
        push(res.error ?? t("auth.saveError"), "fail");
        setPwSaveShake((n) => n + 1);
      }
    } catch {
      setPwSaveState("idle");
      push(t("auth.saveError"), "fail");
      setPwSaveShake((n) => n + 1);
    }
  }

  // Tamper-test schedule eligibility (#109) mirrors immutableOffsiteDomains in
  // internal/schedule/schedule.go: the scheduler only wires the tamper-test job
  // when at least one domain's off-site repo is set and flagged immutable.
  // Otherwise the cadence editor below would silently never run.
  const tamperScheduleActive =
    (settings.containersOffsite !== "" && settings.containersOffsiteImmutable) ||
    (settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable) ||
    (settings.flashOffsite !== "" && settings.flashOffsiteImmutable) ||
    (settings.configOffsite !== "" && settings.configOffsiteImmutable) ||
    (settings.filesOffsite !== "" && settings.filesOffsiteImmutable) ||
    (settings.zfsOffsite !== "" && settings.zfsOffsiteImmutable);

  // Each card's heading takes the next palette position in render order.
  // Only the current page's gates run, so the count starts at 0 on every page.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <div className="flex flex-1 gap-3 md:gap-10">
      {redirect && <Navigate to={`/settings/${redirect}`} replace />}
      <SettingsRail
        items={pages.map((p) => ({ id: p.id, label: t(p.label), icon: p.icon, to: `/settings/${p.id}` }))}
        active={page}
        label={t("settings.railLabel")}
        onReorder={(ids) => {
          setOrder(ids);
          writeOrder(ids);
        }}
      />
      <div data-settings-content className={`${PAGE_SHELL_RESPONSIVE} min-w-0 flex-1`}>
      <PageTitle>{t("settings.title")}</PageTitle>
      <SettingsSearch pages={pages} />
      {/* Keyed on the page, so the slide replays on every change of page. */}
      <div
        key={page}
        data-settings-page
        className="flex flex-col gap-6 md:gap-10 glim-tab-slide flex-1"
        style={{ "--tab-dir": pageDir.current } as CSSProperties}
      >
      <SettingsStoreContext.Provider value={{ ...store, settings }}>

      {/* Every cadence lives here, and every field saves as it changes. */}
      {page === "schedules" && (
        <>
          {/* A two-member list, so each toggle takes a local hueIndex (0, 1)
              independent of the card's own position. */}
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
            <ToggleRow
              label={t("jobs.syncSchedules")}
              hint={t("jobs.syncSchedulesHint")}
              checked={syncSchedules}
              onChange={(v) => void handleSyncSchedulesToggle(v)}
              disabled={syncToggleBusy}
              shakeNonce={syncToggleShake || undefined}
              // The sync save writes all mirrored cadences together, so any of
              // them carries the pulse.
              pulseNonce={fieldPulse.vmsSchedule}
              hueIndex={1}
            />
          </Card>
          {/* Backup Everything, an independent pass over all domains plus a
              manual trigger. It sits second rather than last, so someone who
              wants one schedule for the whole server finds it before the parts
              it is made of. scheduleUpdate saves each key through the shared
              save(), since nothing on this page has a Save button. */}
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
          <ZFSSection
            settings={settings}
            syncSchedules={syncSchedules}
            items={zfsItems}
            perItem={settings.perItemSchedules}
            onChange={(v) => scheduleField("zfsSchedule", v)}
            onItemsChanged={loadZFSItems}
            t={t}
            hueIndex={nextHue()}
          />

          {/* Off-site replication schedules, one cadence per domain. These
              editors are the only owner of these fields. */}
          <Card title={t("settings.schedulesOffsite")} hueIndex={nextHue()}>
            {([
              ["containersOffsiteSchedule", "nav.containers"],
              ["vmsOffsiteSchedule", "nav.vms"],
              ["flashOffsiteSchedule", "nav.flash"],
              ["configOffsiteSchedule", "nav.config"],
              ["filesOffsiteSchedule", "nav.files"],
              ["zfsOffsiteSchedule", "nav.zfs"],
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

          {/* BombVault's own config backup, in the same shape as the domain
              sections. One hueIdx feeds both the heading and the time picker,
              which a nextHue() call per prop could not. The toggle shows the
              card title as its visible label and has no hueIndex, being a lone
              switch with no list to walk. */}
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

          {advanced && <IdleCard t={t} hueIndex={nextHue()} />}
        </>
      )}

      {page === "general" && <GeneralPage />}

      {page === "storage" && <StoragePage />}

      {page === "retention" && <RetentionPage />}

      {page === "containers" && <ContainersPage />}

      {/* Off-site copies are part of the default view, since ransomware */}
      {/* protection depends on them. The id is the target of /settings/offsite. */}
      {page === "offsite" && (
      <div id="offsite" className="flex flex-col gap-6">
      <DestinationsCard hueIndex={nextHue()} />
      {/* Self-backup ("config") is listed with the other domains (#176): the
          backend gives it its own off-site repo and targets like any other,
          so it gets the wizard, the connection test and per-destination
          credentials too. */}
      {([
        ["containersOffsite", "nav.containers", "containers"],
        ["vmsOffsite", "nav.vms", "vms"],
        ["flashOffsite", "nav.flash", "flash"],
        ["filesOffsite", "nav.files", "files"],
        ["zfsOffsite", "nav.zfs", "zfs"],
        ["configOffsite", "nav.config", "config"],
      ] as const).map(([repoKey, label, domain]) => {
        const wizardOpen = offsiteWizard === domain;
        // One hue position per domain, shared by the card heading and every
        // control inside it, so a domain's buttons match its card.
        const hueIdx = nextHue();
        const fieldTarget = allTargets.find((x) => x.domain === domain && x.sortOrder === 0);
        return (
        <Card key={repoKey} title={t("offsite.copyDomainTitle").replace("{domain}", t(label))} hueIndex={hueIdx}>
          {/* The repo URL prefixes (rest:, s3:, b2:) are visible reference
              text. They apply to every domain, so they are shown once, in the
              first card. */}
          {domain === "containers" && (
            <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
          )}
          <div className="flex flex-col gap-1">
            <OffsiteDomainBar
              domain={domain}
              label={t(label)}
              repo={settings[repoKey]}
              wizardOpen={wizardOpen}
              onToggleWizard={() => setOffsiteWizard(wizardOpen ? null : domain)}
              t={t}
              hueIndex={hueIdx}
            />
            {wizardOpen ? (
              <OffsiteWizard
                domain={domain}
                settings={settings}
                setSettings={setSettings}
                save={save}
                t={t}
                hueIndex={hueIdx}
              />
            ) : (
              <>
                <OffsiteLocationInput
                  domain={domain}
                  value={settings[repoKey]}
                  targetId={fieldTarget?.id}
                  targetName={fieldTarget?.name}
                  following={fieldTarget?.destinationId && fieldTarget.enabled ? fieldTarget.name : undefined}
                  placeholder="rest:http://host:8000/repo"
                  className="rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono glim-field-focus text-start"
                  onSave={(v) => save({ [repoKey]: v } as Partial<Settings>, setOffsiteSaveState, setOffsiteSaveError)}
                  onFromDestination={(location, immutable) => {
                    // The server already holds both, so the next save of
                    // another card must not send the old values back.
                    const patch = { [repoKey]: location, [`${domain}OffsiteImmutable`]: immutable } as Partial<Settings>;
                    setSettings((s) => (s ? { ...s, ...patch } : s));
                    if (savedBaseline.current) savedBaseline.current = { ...savedBaseline.current, ...patch };
                  }}
                />
                {/* A mounted share is a valid off-site target, but the
                    placeholder shows a REST URL, so this says a bare relative
                    path works too (#138). */}
                <span className="text-xs text-carbon-textMuted">
                  {withLtrFragments(t("offsite.repoLocalHint"), REPO_LOCAL_HINT_LTR_FRAGMENTS)}
                </span>
                {settings[repoKey] && (
                  <CompressionSelector
                    value={settings.compression[`offsite:${domain}`]}
                    onChange={(c) => void saveCompression(`offsite:${domain}`, c, settings, setSettings, save)}
                  />
                )}
              </>
            )}
            {/* Extra copies of this domain beyond the primary above, managed
                through the CRUD API. */}
            <OffsiteTargetsSection domain={domain} t={t} hueIndex={hueIdx} />
          </div>
        </Card>
        );
      })}
      </div>
      )}

      {/* `advanced &&` inline for the same reason as the cache card. */}
      {page === "offsite" && advanced && (
      <Card title={t("settings.offsiteLimits")} hint={t("settings.limitHint")} hueIndex={nextHue()}>
        <div className="grid grid-cols-2 gap-3">
          {([
            ["offsiteLimitUpload", "settings.limitUpload"],
            ["offsiteLimitDownload", "settings.limitDownload"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <NumberField
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                  debouncedSave(key, () => void save({ [key]: n } as Partial<Settings>, setLimSaveState, setLimSaveError));
                }}
                className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus"
              />
            </label>
          ))}
        </div>
      </Card>
      )}

      {page === "offsite" && advanced && <StreamingCard t={t} hueIndex={nextHue()} />}

      {/* `advanced &&` inline for the same reason as the cache card. */}
      {page === "integrations" && advanced && (
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* The /metrics and bearer-token syntax can sit in a bubble: it is
            used right here, not typed into another page from memory. */}
        <ToggleRow
          label={tLtr(t, "settings.metricsEnable")}
          hint={tLtr(t, "settings.metricsHint")}
          checked={settings.metricsEnabled}
          onChange={(v) => void autoSaveToggle("metricsEnabled", v, setMetricsSaveState, setMetricsSaveError)}
          disabled={fieldBusy.metricsEnabled}
          shakeNonce={fieldShake.metricsEnabled}
          pulseNonce={fieldPulse.metricsEnabled}
        />
        {/* Write-only secret (the GET never echoes it): a blank save keeps the
            stored token, so a stored one shows the same "already set"
            placeholder the cloud-credential secrets use. */}
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <RevealInput
            {...revealMetricsToken}
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => prev ? { ...prev, metricsToken: v } : prev);
              // A non-blank token marks itself set; a blank save keeps
              // whatever was stored.
              debouncedSave("metricsToken", () =>
                void save(
                  { metricsToken: v, metricsTokenSet: v.trim() !== "" || settings.metricsTokenSet },
                  setMetricsSaveState,
                  setMetricsSaveError
                )
              );
            }}
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus"
          />
        </label>
      </Card>
      )}

      {/* The dashboard widget is an end-user feature, so it is outside the */}
      {/* advanced view, unlike the metrics. */}
      {page === "integrations" && (
        <>
        <DashboardWidgetCard
          t={t}
          tokenSet={settings.widgetTokenSet}
          onTokenSet={(set) => {
            // Keep both the live state and the saved baseline in sync: the token
            // is managed by its own endpoints, so a later save (which merges onto
            // the baseline) must not carry a stale widgetTokenSet.
            setSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
            if (savedBaseline.current) {
              savedBaseline.current = { ...savedBaseline.current, widgetTokenSet: set };
            }
          }}
          hueIndex={nextHue()}
        />
        {mcpShipped && <McpServerCard hueIndex={nextHue()} passwordSet={authEnabled} />}
        <ApiTokensCard hueIndex={nextHue()} passwordSet={authEnabled} />
        <HomeAssistantCard hueIndex={nextHue()} />
        <NetworkCard hueIndex={nextHue()} />
        </>
      )}

      {/* Host SSH shows whenever VMs or ZFS are on, since their backups need it. */}
      {page === "integrations" && (advanced || settings.vmsEnabled || settings.zfsEnabled) && (
        <VMSSHCard t={t} hueIndex={nextHue()} />
      )}

      {page === "cloud" && <CloudPage />}

      {/* The channel and Healthchecks cards only paint in the advanced view, so */}
      {/* their hue positions are counted inside that condition and none is spent */}
      {/* while it is off. */}
      {page === "notifications" && (() => {
        const settingsHue = nextHue();
        const channelsHue = advanced ? nextHue() : undefined;
        const healthchecksHue = advanced ? nextHue() : undefined;
        return (
          <NotifyCard
            t={t}
            platformKind={platformKind}
            hueIndex={settingsHue}
            channelsHueIndex={channelsHue}
            healthchecksHueIndex={healthchecksHue}
          />
        );
      })()}

      {/* Weekly digest: one summary message per week through the channels
          above. One hueIdx feeds both the heading and the time picker. */}
      {page === "notifications" && (() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={hueIdx}>
            <ToggleRow
              label={t("settings.digestToggle")}
              checked={settings.digestEnabled}
              onChange={(v) => void autoSaveToggle("digestEnabled", v, setDigestSaveState, setDigestSaveError)}
              disabled={fieldBusy.digestEnabled}
              shakeNonce={fieldShake.digestEnabled}
              pulseNonce={fieldPulse.digestEnabled}
            />
            {/* `enabled` follows digestEnabled: the on/off is a separate
                toggle here, not the cadence string's own "off" mode. */}
            <ScheduleRow schedule={settings.digestSchedule} enabled={settings.digestEnabled} />
            {/* The editor goes with the toggle above, as in
                RestoreChecksSection: an editor greyed because a switch
                elsewhere is off offers an edit nobody can make. The badge
                stays either way, so switching the report off still shows what
                would have run. */}
            {settings.digestEnabled && (
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.schedule")}
                value={settings.digestSchedule}
                onChange={(v) => {
                  setSettings((prev) => (prev ? { ...prev, digestSchedule: v } : prev));
                  debouncedSave("digestSchedule", () =>
                    void save({ digestSchedule: v }, setDigestSaveState, setDigestSaveError)
                  );
                }}
                hueIndex={hueIdx}
              />
            </div>
            )}
          </Card>
        );
      })()}

      {/* Overdue-backup watchdog: a fixed daily check at 09:00 that sends one
          notification per overdue episode through the channels above; a new
          successful backup re-arms it. */}
      {page === "notifications" && (
        <Card title={t("settings.watchdogTitle")} hint={t("settings.watchdogHint")} hueIndex={nextHue()}>
          <ToggleRow
            label={t("settings.watchdogToggle")}
            checked={settings.watchdogEnabled}
            onChange={(v) => void autoSaveToggle("watchdogEnabled", v, setWatchdogSaveState, setWatchdogSaveError)}
            disabled={fieldBusy.watchdogEnabled}
            shakeNonce={fieldShake.watchdogEnabled}
            pulseNonce={fieldPulse.watchdogEnabled}
          />
        </Card>
      )}

      {/* `advanced &&` inline for the same reason as the cache card. */}
      {page === "integrations" && advanced && (() => {
        // One hueIdx for the heading and SpikePanel's button.
        const hueIdx = nextHue();
        return (
          <Card title={t("spike.title")} hueIndex={hueIdx}>
            <SpikePanel t={t} hueIndex={hueIdx} />
          </Card>
        );
      })()}

      {/* Restore drills, the off-site DR restore among them, are part of the */}
      {/* default view. */}
      {page === "integrity" && (
      <>
        <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={nextHue()} />

        <RestoreChecksSection
          settings={settings}
          update={scheduleUpdate}
          busy={schedFieldBusy}
          shake={schedFieldShake}
          pulse={fieldPulse}
          t={t}
          hueIndex={nextHue()}
        />

        {/* The scheduled off-site append-only tamper test. One hueIdx feeds
            both the heading and the time picker; a second nextHue() call
            would give the one card two colours. */}
        {(() => {
          const hueIdx = nextHue();
          return (
            <Card title={t("settings.schedulesChecks")} hueIndex={hueIdx}>
              {/* No `enabled` here: the cadence string's own "off" mode is the
                  control, as in the domain cards. tamperScheduleActive stays
                  out of the badge: it is not this card's on/off, it has its
                  own warning below, and a "no schedule" badge would contradict
                  the cadence visible in the editor. */}
              <ScheduleRow schedule={settings.tamperTestSchedule} />
              <div className="rounded-card bg-carbon-surface2 p-4">
                <CadenceBuilder
                  label={t("settings.tamperTestSchedule")}
                  value={settings.tamperTestSchedule}
                  onChange={(v) => scheduleField("tamperTestSchedule", v)}
                  hueIndex={hueIdx}
                />
                {/* The scheduler stays inert without a qualifying domain, and
                    this is the only place that says why the test never runs
                    (#109). */}
                {!tamperScheduleActive && (
                  <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                    {t("settings.tamperScheduleInactive")}
                  </div>
                )}
              </div>
            </Card>
          );
        })()}

        {/* Last, so the restore checks and their schedule stay next to each
            other. The target of /settings/integrity#anomalies; the margin keeps the
            heading badge, which straddles the card's top edge, in view. */}
        <div id="anomalies" className="scroll-mt-6">
          <AnomalyCard
            t={t}
            settings={settings}
            summary={anomalySummary}
            save={(key, next) => void autoSaveToggle(key, next, setAnomalySaveState, setAnomalySaveError)}
            busy={fieldBusy}
            shake={fieldShake}
            pulse={fieldPulse}
            hueIndex={nextHue()}
          />
        </div>
      </>
      )}

      {/* One hueIdx for the heading and the button inside. */}
      {page === "security" && (() => {
        const hueIdx = nextHue();
        return (
      <Card title={t("auth.security")} hint={t("auth.passwordHint")} hueIndex={hueIdx}>
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {authEnabled ? t("auth.changePassword") : t("auth.setPassword")}
            </label>
            <RevealInput
              {...revealPwNew}
              id={LOGIN_PASSWORD_FIELD}
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {t("auth.confirmPassword")}
            </label>
            <RevealInput
              {...revealPwConfirm}
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              wrapperClassName="w-full"
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
            {/* The rule, stated before it is broken rather than after. */}
            <span className="text-xs text-carbon-textSub">
              {t("auth.passwordMinHint", minPasswordLen)}
            </span>
          </div>

          <div className="flex items-center gap-3 pt-1">
            <Button
              key={pwSaveShake || 0}
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void handleSetPassword()}
              disabled={pwSaveState === "saving"}
              busy={pwSaveState === "saving"}
              title={pwSaveState === "saving" ? t("auth.saving") : undefined}
              className={pwSaveShake ? "glim-shake" : ""}
              hueIndex={hueIdx}
            />
            {/* Only the pre-flight validation errors render here; the save
                outcome is a toast. */}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-statusFail">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* No sign-out buttons here: a settings card configures and the shell
            operates, and sign-out lives in the sidebar. Changing the password
            rotates the session epoch, which ends every other session (see
            handleSetPassword in internal/api/handlers.go). */}
      </Card>
        );
      })()}

      {/* The second factor has its own card: enrolment takes three steps with a */}
      {/* QR code and recovery codes, and it is a separate decision from having a */}
      {/* password at all. */}
      {page === "security" && (
        <TwoFactorCard
          passwordSet={authEnabled}
          enabled={totpEnabled}
          recoveryLeft={recoveryLeft}
          onChanged={() => {
            void getAuth()
              .then((res) => {
                setTotpEnabled(res.totp ?? false);
                setRecoveryLeft(res.recoveryCodesLeft);
              })
              .catch(() => undefined);
          }}
          hueIndex={nextHue()}
        />
      )}

      {/* A passkey replaces typing the password rather than strengthening it, */}
      {/* and it is not always available, so the card first explains when it is not. */}
      {page === "security" && <PasskeyCard passwordSet={authEnabled} hueIndex={nextHue()} />}

      {page === "pairing" && <PairingPage />}

      {page === "look" && <ThemeCard t={t} hueIndex={nextHue()} />}

      {/* Shape, the corner axis (lib/shape.ts). The segments carry no glyph:
          each segment is drawn at the real radius, so the strip itself is the
          preview, and a scaled-down stand-in beside it read as a weaker shape
          than the one it named. */}
      {page === "look" && (
      <Card title={t("settings.shape")} hint={t("settings.shapeHint")} hueIndex={nextHue()}>
        <Selector
          items={[...SHAPES, ...(leafFound || shape === "leaf" ? (["leaf"] as const) : [])].map((s) => ({
            id: s,
            label: t(`settings.shape.${s}` as TranslationKey),
          }))}
          label={t("settings.shape")}
          select="one"
          active={shape}
          onChange={(id) => {
            const leaf = leafTap(leafClicks.current, id, shape);
            if (leaf) setLeafFound(true);
            const next = (leaf ?? id) as Shape;
            setShapeLocal(next);
            setShape(next);
          }}
          size="lg"
          equalWidth
          hueOffset={HUE_OFFSET.shape}
        />
      </Card>
      )}

      {/* Motion intensity, built like the shape card: lib/motion.ts mirrors
          lib/shape.ts, and the level applies at the app root without a Save. */}
      {page === "look" && (
      <Card title={t("settings.motion")} hint={t("settings.motionHint")} hueIndex={nextHue()}>
        {/* The fourth level, storm, is hidden. It is offered while it is
            chosen, since a picker that hid its current value would misstate
            the interface, and otherwise only while this screen stays open,
            which is why `stormFound` is component state and not storage. */}
        <Selector
          items={[...MOTION_INTENSITIES, ...(stormFound || motion === "storm" ? (["storm"] as const) : [])].map(
            (m) => ({
              id: m,
              label: t(`settings.motion.${m}` as TranslationKey),
            }),
          )}
          label={t("settings.motion")}
          select="one"
          active={motion}
          onChange={(id) => {
            // The gesture first, because it fires on the level already chosen
            // and therefore on a click that changes nothing else.
            const storm = stormTap(stormClicks.current, id, motion);
            if (storm) setStormFound(true);
            const next = (storm ?? id) as MotionIntensity;
            setMotionLocal(next);
            setMotionIntensity(next);
          }}
          size="lg"
          equalWidth
          hueOffset={HUE_OFFSET.motion}
        />
      </Card>
      )}

      {page === "look" && (
      <>
      {/* Control labels (#178), how much of a control's identity is shown.
          One selector per chrome surface rather than one switch, because the
          right answer differs: a rail reduced to glyphs narrows the whole
          page, tabs do not, and action buttons are a density preference.
          Placed straight after Animations: both are per-viewer appearance
          dials kept in this browser rather than server settings, and they
          read as a pair. */}
      <Card title={t("settings.labels")} hint={t("settings.labelsHint")} hueIndex={nextHue()}>
        <div className="flex flex-col gap-4">
          {CONTROL_AXES.map((axis, axisIndex) => (
            <div key={axis} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(`settings.labels.${axis}` as TranslationKey)}
                {/* Layout mounts the rail or the bar, never both, so each of
                    those two rows is dead on the other width and says so.
                    Buttons and tabs need no hint: they answer everywhere. */}
                {axis === "bottombar" && <InfoBubble tip={t("settings.axisBottombarHint")} />}
                {axis === "sidebar" && <InfoBubble tip={t("settings.axisSidebarHint")} />}
              </span>
              <Selector
                items={LABEL_MODES.map((m) => ({
                  id: m,
                  label: t(`settings.labels.mode.${m}` as TranslationKey),
                }))}
                label={t(`settings.labels.${axis}` as TranslationKey)}
                select="one"
                active={labelModes[axis]}
                onChange={(id) => {
                  setLabelMode(axis, id as LabelMode);
                  setLabelModes((prev) => ({ ...prev, [axis]: id as LabelMode }));
                  // Every mounted control re-reads on this, so the page changes
                  // under the selector instead of only after a reload.
                  labelModeChanged();
                }}
                size="lg"
                equalWidth
                // Each row starts one colour further along the palette, so two
                // selectors never repeat the same colour down the page.
                // Offsetting by the row index rather than by the row count
                // keeps the rows adjacent in the palette, so the block still
                // reads as one group rather than unrelated strips. Every other
                // start in the settings tree comes out of the same table.
                hueOffset={HUE_OFFSET.labels + axisIndex}
              />
            </div>
          ))}
        </div>
      </Card>
      </>
      )}

      {/* Accent colour and rainbow mode share one card. Turning the rainbow
          on sets data-rainbow and --rb-0..--rb-7 on <html>, which recolours
          every hue-enabled control; the sidebar is not one of them
          (Sidebar.tsx says why). The two reset badges stay neutral, so a
          reset does not blend into the colours it resets. */}
      {page === "look" && (() => {
      const hueIdx = nextHue();
      // Whether the palette row has anything left to reset, compared
      // case-insensitively like AccentCard's presetsAreDefault: setRainbow()
      // accepts either case, so "#ff8389" typed by hand still counts as the
      // default.
      const paletteIsDefault =
        rainbow.palette.length === RAINBOW.length &&
        rainbow.palette.every((hex, i) => hex.toLowerCase() === RAINBOW[i]?.toLowerCase());
      return (
      <Card title={t("settings.colors")} hueIndex={hueIdx}>
        <AccentCard t={t} rainbowOn={rainbow.on} />
        <div className="flex flex-col gap-3">
          {/* Three toggles rendered together are a list, so each takes a
              rainbow position of its own, counted locally like the Domains
              card's rows. */}
          <ToggleRow
            label={t("settings.rainbow")}
            hint={t("settings.rainbowHint")}
            checked={rainbow.on}
            onChange={rainbowToggled}
            hueIndex={0}
          />

          {/* Everything below depends on the rainbow being on, so while it is
              off none of it is shown: a palette editor under a rainbow that is
              not running offers edits with no effect. The accent row keeps its
              dimming instead, because its value still paints every control
              the rainbow does not reach. */}
          {rainbow.on && (
          <>
          {/* Disco, once found. Shown while it is on as well as while found,
              for the reason the storm's own picker entry is: a switch that
              hid the value it is currently showing would be lying, and
              somebody who reloads with disco running needs a way to stop it.
              Sits first inside the rainbow's sub-controls because it changes
              what the whole set of them does, rather than one more property
              of it. */}
          {(discoFound || disco) && (
            <ToggleRow
              label={t("settings.disco")}
              hint={t("settings.discoHint")}
              checked={disco}
              onChange={(v) => {
                setDisco(v);
                setDiscoLocal(v);
              }}
              hueIndex={0}
            />
          )}
          <ToggleRow
            label={t("settings.rainbowReactive")}
            hint={t("settings.rainbowReactiveHint")}
            checked={rainbow.reactive}
            onChange={(v) => updateRainbow({ reactive: v })}
            hueIndex={1}
          />
          <ToggleRow
            label={t("settings.rainbowRotate")}
            hint={t("settings.rainbowRotateHint")}
            checked={rainbow.rotate}
            onChange={(v) =>
              // Turning rotation on draws a fresh offset immediately, so
              // the switch does something visible instead of silently
              // re-applying whatever rotation the palette already had.
              updateRainbow({
                rotate: v,
                seed: v ? 1 + Math.floor(Math.random() * (RAINBOW.length - 1)) : 0,
              })
            }
            hueIndex={2}
          />

          {/* The same row shape as the accent swatches, because it is the
              same job. setRainbow() and isValidPalette() validate the whole
              palette before it reaches the document (lib/appearance.ts). */}
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm text-carbon-text">{t("settings.rainbowPaletteLabel")}</span>
            <div className="flex items-center gap-2 flex-wrap ms-auto">
            {rainbow.palette.map((hex, i) => (
              <PaletteSwatch
                key={i}
                hex={hex}
                index={i}
                t={t}
                onChange={(v) => {
                  const next = rainbow.palette.slice();
                  next[i] = v;
                  updateRainbow({ palette: next });
                }}
              />
            ))}
            {/* Neutral rather than hue-tinted: beside eight palette swatches, a
                coloured reset would look like a ninth entry in the palette.
                The border matches the swatches' ring so it does not read
                bigger than they do, and it is disabled only when there is
                nothing to reset. The tip names its target, since the card
                holds two reset badges. */}
            <Badge
              as="button"
              shape="square"
              size="icon"
              tone="neutral"
              tip={t("settings.rainbowPaletteReset")}
              onClick={() => updateRainbow({ palette: RAINBOW })}
              disabled={paletteIsDefault}
              className="border-2 border-carbon-border"
            >
              <IconResetArrow />
            </Badge>
            </div>
          </div>
          </>
          )}
        </div>
      </Card>
      );
      })()}

      {page === "apps" && <AppsPage />}

      {page === "system" && <SystemPage />}
      </SettingsStoreContext.Provider>
      </div>
      {confirmDialog}
      </div>
    </div>
  );
}
