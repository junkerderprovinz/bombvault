import { useEffect, useState } from "react";
import { listContainers, getSettings, putSettings } from "../lib/api";
import type { Container, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { BackupButton } from "../components/BackupButton";
import { RestorePanel } from "../components/RestorePanel";
import { IncludeToggle } from "../components/IncludeToggle";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTs(unix: number | null | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

// ---------------------------------------------------------------------------
// State chip
// ---------------------------------------------------------------------------

function StateChip({ state }: { state: string }) {
  const lower = state.toLowerCase();
  const cls =
    lower === "running"
      ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
      : lower === "exited" || lower === "stopped"
      ? "bg-[#3a1c1c] text-[#ff8389] border border-[#5a2a2a]"
      : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border";
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls}`}>
      {state}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Schedule editor card
// ---------------------------------------------------------------------------

type SchedulePreset = "off" | "daily" | "weekly" | "cron";

function parsePreset(value: string): SchedulePreset {
  if (value === "off" || value === "") return "off";
  if (/^daily\s+\d{1,2}:\d{2}$/.test(value)) return "daily";
  if (/^weekly\s+\w+\s+\d{1,2}:\d{2}$/.test(value)) return "weekly";
  return "cron";
}

const DAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

interface ScheduleCardProps {
  t: ReturnType<typeof useT>["t"];
}

function ScheduleCard({ t }: ScheduleCardProps) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [preset, setPreset] = useState<SchedulePreset>("off");
  const [dailyTime, setDailyTime] = useState("02:00");
  const [weeklyDay, setWeeklyDay] = useState("Mon");
  const [weeklyTime, setWeeklyTime] = useState("02:00");
  const [cronExpr, setCronExpr] = useState("");
  const [saveState, setSaveState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (!res.ok) return;
        setSettings(res.settings);
        const sched = res.settings.containersSchedule ?? "off";
        const p = parsePreset(sched);
        setPreset(p);
        if (p === "daily") {
          const m = /^daily\s+(\d{1,2}:\d{2})$/.exec(sched);
          if (m) setDailyTime(m[1]);
        } else if (p === "weekly") {
          const m = /^weekly\s+(\w+)\s+(\d{1,2}:\d{2})$/.exec(sched);
          if (m) { setWeeklyDay(m[1]); setWeeklyTime(m[2]); }
        } else if (p === "cron") {
          setCronExpr(sched);
        }
      })
      .catch(() => {/* non-fatal */});
  }, []);

  function buildValue(): string {
    if (preset === "off") return "off";
    if (preset === "daily") return `daily ${dailyTime}`;
    if (preset === "weekly") return `weekly ${weeklyDay} ${weeklyTime}`;
    return cronExpr.trim() || "off";
  }

  async function handleSave() {
    if (!settings) return;
    setSaveState("saving");
    setSaveError(null);
    const updated: Settings = { ...settings, containersSchedule: buildValue() };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        setSettings(updated);
        setSaveState("saved");
        setTimeout(() => setSaveState("idle"), 3000);
      } else {
        setSaveError(res.error ?? "Save failed");
        setSaveState("error");
      }
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Save failed");
      setSaveState("error");
    }
  }

  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("containers.schedule")}
        </h2>
      </div>

      {/* Preset selector */}
      <div className="flex flex-wrap gap-2">
        {(["off", "daily", "weekly", "cron"] as SchedulePreset[]).map((p) => (
          <button
            key={p}
            onClick={() => setPreset(p)}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              preset === p
                ? "bg-carbon-surface3 text-carbon-text"
                : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
            }`}
          >
            {p === "off" ? t("settings.scheduleOff") : p}
          </button>
        ))}
      </div>

      {/* Fields */}
      {preset === "daily" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textSub w-16">Time</label>
          <input
            type="time"
            value={dailyTime}
            onChange={(e) => setDailyTime(e.target.value)}
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          />
        </div>
      )}

      {preset === "weekly" && (
        <div className="flex items-center gap-3 flex-wrap">
          <label className="text-xs text-carbon-textSub w-16">Day</label>
          <select
            value={weeklyDay}
            onChange={(e) => setWeeklyDay(e.target.value)}
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          >
            {DAYS.map((d) => <option key={d}>{d}</option>)}
          </select>
          <label className="text-xs text-carbon-textSub">at</label>
          <input
            type="time"
            value={weeklyTime}
            onChange={(e) => setWeeklyTime(e.target.value)}
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          />
        </div>
      )}

      {preset === "cron" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textSub w-16">Cron</label>
          <input
            type="text"
            value={cronExpr}
            onChange={(e) => setCronExpr(e.target.value)}
            placeholder="30 2 * * *"
            spellCheck={false}
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff] w-44"
          />
          <span className="text-xs text-carbon-textMuted">standard 5-field cron</span>
        </div>
      )}

      {/* Preview */}
      <p className="text-xs text-carbon-textMuted">
        Value: <span className="font-mono text-carbon-textSub">{buildValue()}</span>
      </p>

      {/* Save */}
      <div className="flex items-center gap-3">
        <button
          onClick={() => void handleSave()}
          disabled={saveState === "saving" || !settings}
          className="inline-flex items-center gap-2 rounded-lg bg-carbon-surface3 px-4 py-1.5 text-xs font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
        >
          {saveState === "saving" ? (
            <>
              <span className="h-3 w-3 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
              Saving…
            </>
          ) : (
            t("settings.save")
          )}
        </button>
        {saveState === "saved" && (
          <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>
        )}
        {saveState === "error" && saveError && (
          <span className="text-xs text-[#ff8389]">{saveError}</span>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Container row
// ---------------------------------------------------------------------------

function ContainerRow({
  container,
  t,
}: {
  container: Container;
  t: ReturnType<typeof useT>["t"];
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-4 flex flex-col gap-3">
      {/* Top row */}
      <div className="flex items-start gap-3 flex-wrap">
        {/* Name + image */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate">
              {container.name}
            </span>
            <StateChip state={container.state} />
          </div>
          <p className="text-xs text-carbon-textMuted mt-0.5 truncate">{container.image}</p>
        </div>

        {/* Last backup */}
        <div className="text-right shrink-0">
          <p className="text-xs text-carbon-textMuted">{t("containers.lastBackup")}</p>
          <p className="text-xs text-carbon-textSub">
            {container.lastBackup ? formatTs(container.lastBackup) : t("containers.never")}
          </p>
        </div>
      </div>

      {/* Actions row */}
      <div className="flex items-start gap-4 flex-wrap">
        {/* Include toggle */}
        <label className="flex items-center gap-2 cursor-pointer">
          <IncludeToggle name={container.name} initial={container.includeInSchedule} />
          <span className="text-xs text-carbon-textSub">
            {t("containers.includeInSchedule")}
          </span>
        </label>

        {/* Backup button */}
        <BackupButton name={container.name} t={t} />
      </div>

      {/* Snapshots / Restore disclosure */}
      <RestorePanel name={container.name} t={t} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Containers page
// ---------------------------------------------------------------------------

export function Containers() {
  const { t } = useT();
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listContainers()
      .then((res) => {
        if (res.ok) setContainers(res.containers ?? []);
        else setError("Failed to load containers");
      })
      .catch(() => setError("Failed to load containers"))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="flex flex-col gap-6 max-w-5xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("containers.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          Manage container backups, schedules, and restores.
        </p>
      </div>

      {/* Schedule card */}
      <ScheduleCard t={t} />

      {/* Container list */}
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}
      {!loading && !error && containers.length === 0 && (
        <div className="bg-carbon-surface rounded-card border border-carbon-border p-6 text-center">
          <p className="text-sm text-carbon-textMuted">
            No containers found. Is Docker running?
          </p>
        </div>
      )}
      {!loading && containers.length > 0 && (
        <div className="flex flex-col gap-3">
          {containers.map((c) => (
            <ContainerRow key={c.name} container={c} t={t} />
          ))}
        </div>
      )}
    </div>
  );
}
