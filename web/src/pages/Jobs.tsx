import { useEffect, useState } from "react";
import { getSettings, putSettings, listContainers } from "../lib/api";
import type { Settings, Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { CadenceBuilder } from "../components/CadenceBuilder";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Convert a cadence string to a human-readable label. */
function cadenceLabel(raw: string, t: ReturnType<typeof useT>["t"]): string {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return t("jobs.notScheduled");

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return t("jobs.cadenceDaily").replace("{time}", dailyM[1]);

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) return t("jobs.cadenceWeekly").replace("{days}", weeklyM[1]).replace("{time}", weeklyM[2]);

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) return t("jobs.cadenceEveryN").replace("{n}", everyNM[1]).replace("{time}", everyNM[2]);

  return s;
}

// ---------------------------------------------------------------------------
// Status badge for cadence / schedule state
// ---------------------------------------------------------------------------

type ScheduleStatus = "active" | "paused" | "off";

function scheduleStatus(schedule: string): ScheduleStatus {
  if (!schedule || schedule === "off") return "off";
  return "active";
}

function ScheduleBadge({
  status,
  label,
}: {
  status: ScheduleStatus;
  label: string;
}) {
  const cls: Record<ScheduleStatus, string> = {
    active: "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    paused: "bg-[#2a2a1c] text-[#f1c21b] border border-[#4a4a2a]",
    off:    "bg-carbon-surface2 text-carbon-textSub border border-carbon-border",
  };
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls[status]}`}
    >
      {label}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

function Card({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {title}
      </h2>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Save bar (mirrors Settings.tsx) — one button persists every domain schedule.
// ---------------------------------------------------------------------------

type SaveState = "idle" | "saving" | "saved" | "error";

function SaveBar({
  state,
  error,
  onSave,
  t,
}: {
  state: SaveState;
  error: string | null;
  onSave: () => void;
  t: ReturnType<typeof useT>["t"];
}) {
  return (
    <div className="flex items-center gap-3 pt-1">
      <button
        onClick={onSave}
        disabled={state === "saving"}
        className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {state === "saving" ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.saving")}
          </>
        ) : (
          t("settings.save")
        )}
      </button>
      {state === "saved" && (
        <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>
      )}
      {state === "error" && error && (
        <span className="text-sm text-[#ff8389]">{error}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Domain section — Containers (editable schedule + included-containers list)
// ---------------------------------------------------------------------------

function ContainersSection({
  settings,
  containers,
  onChange,
  t,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.containersSchedule;
  const status = scheduleStatus(schedule);
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    <Card title={t("jobs.containersSection")}>
      {/* Cadence row */}
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>

      {/* Editable cadence builder */}
      <div className="rounded-lg bg-carbon-surface2 border border-carbon-border p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
        />
      </div>

      {/* Member list */}
      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div
              key={c.name}
              className="flex items-center gap-3 py-2 text-sm"
            >
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  c.state.toLowerCase() === "running"
                    ? "bg-[#6fdc8c]"
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
          ))}
        </div>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Domain section — VMs (editable schedule)
// ---------------------------------------------------------------------------

function VMsSection({
  settings,
  syncSchedules,
  onChange,
  t,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const status = scheduleStatus(schedule);

  return (
    <Card title={t("jobs.vmsSection")}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.vmIncludeHint")}</p>
        )}
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Domain section — Flash (editable schedule)
// ---------------------------------------------------------------------------

function FlashSection({
  settings,
  syncSchedules,
  onChange,
  t,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.flashSchedule;
  const status = scheduleStatus(schedule);

  return (
    <Card title={t("jobs.flashSection")}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.flashNotImplemented")}</p>
        )}
      </div>
      <div className="flex items-center gap-3 py-2 text-sm border-t border-carbon-border">
        <div className="w-2 h-2 rounded-full bg-carbon-surface3 shrink-0" />
        <span className="font-medium text-carbon-text flex-1">{t("jobs.flashRow")}</span>
        <span className="text-xs text-carbon-textMuted italic">{t("jobs.flashPlanned")}</span>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Jobs page
// ---------------------------------------------------------------------------

export function Jobs() {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(null);
  // savedSettings is the server's last-confirmed state. Saving merges only the
  // schedule fields onto THIS baseline (not the live, possibly-edited settings),
  // so saving never silently commits an unrelated change made elsewhere.
  const [savedSettings, setSavedSettings] = useState<Settings | null>(null);
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // "Use the Containers schedule for VMs and Flash too" checkbox.
  const [syncSchedules, setSyncSchedules] = useState(false);

  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([getSettings(), listContainers()])
      .then(([settingsRes, containersRes]) => {
        if (!active) return;
        if (settingsRes.ok) {
          setSettings(settingsRes.settings);
          setSavedSettings(settingsRes.settings);
          // Detect if the schedules are already in sync, so the checkbox reflects it.
          const s = settingsRes.settings;
          if (
            s.vmsSchedule === s.containersSchedule &&
            s.flashSchedule === s.containersSchedule &&
            s.containersSchedule !== "off" &&
            s.containersSchedule !== ""
          ) {
            setSyncSchedules(true);
          }
        }
        if (containersRes.ok) setContainers(containersRes.containers ?? []);
        if (!settingsRes.ok) setError("Failed to load settings");
      })
      .catch(() => {
        if (active) setError("Failed to load data");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  // While "sync" is on, mirror the Containers cadence onto VMs + Flash in live
  // state too — not just in the save patch. Otherwise unchecking sync would snap
  // the VM/Flash editors back to their stale pre-sync values instead of the
  // synced one the user was just looking at. The equality guard stops re-renders
  // from looping.
  useEffect(() => {
    if (!syncSchedules) return;
    setSettings((prev) => {
      if (!prev) return prev;
      if (
        prev.vmsSchedule === prev.containersSchedule &&
        prev.flashSchedule === prev.containersSchedule
      ) {
        return prev;
      }
      return { ...prev, vmsSchedule: prev.containersSchedule, flashSchedule: prev.containersSchedule };
    });
  }, [syncSchedules, settings?.containersSchedule]);

  // Build the schedule patch (used by the single Save button).
  function buildSchedulePatch(): Partial<Settings> {
    if (!settings) return {};
    const patch: Partial<Settings> = {
      containersSchedule: settings.containersSchedule,
    };
    if (syncSchedules) {
      patch.vmsSchedule = settings.containersSchedule;
      patch.flashSchedule = settings.containersSchedule;
    } else {
      patch.vmsSchedule = settings.vmsSchedule;
      patch.flashSchedule = settings.flashSchedule;
    }
    return patch;
  }

  async function handleSave() {
    const base = savedSettings ?? settings;
    if (!base) return;
    setSaveState("saving");
    setSaveError(null);
    const patch = buildSchedulePatch();
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        setSavedSettings(updated);
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        setSaveState("saved");
        setTimeout(() => setSaveState("idle"), 3000);
      } else {
        setSaveError(res.error ?? t("settings.error"));
        setSaveState("error");
      }
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : t("settings.error"));
      setSaveState("error");
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("jobs.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("jobs.subtitle")}</p>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}

      {!loading && !error && settings && (
        <>
          <ContainersSection
            settings={settings}
            containers={containers}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, containersSchedule: v } : prev))
            }
            t={t}
          />

          {/* Sync checkbox — applies the Containers schedule to VMs + Flash too. */}
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={syncSchedules}
              onChange={(e) => setSyncSchedules(e.target.checked)}
              className="h-4 w-4 rounded border-carbon-border bg-carbon-surface2 accent-[#6fdc8c]"
            />
            <span className="text-sm text-carbon-text">{t("jobs.syncSchedules")}</span>
          </label>

          <VMsSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, vmsSchedule: v } : prev))
            }
            t={t}
          />
          <FlashSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, flashSchedule: v } : prev))
            }
            t={t}
          />

          {/* One Save persists all three domain schedules. */}
          <SaveBar state={saveState} error={saveError} onSave={() => void handleSave()} t={t} />
        </>
      )}
    </div>
  );
}
