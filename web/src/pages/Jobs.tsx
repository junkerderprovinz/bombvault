import { useEffect, useState } from "react";
import { getSettings, listContainers } from "../lib/api";
import type { Settings, Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { NavLink } from "react-router-dom";

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
// Domain section — Containers
// ---------------------------------------------------------------------------

function ContainersSection({
  settings,
  containers,
  t,
}: {
  settings: Settings;
  containers: Container[];
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.containersSchedule;
  const status = scheduleStatus(schedule);
  const included = containers.filter((c) => c.installed && c.includeInSchedule);

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
// Domain section — VMs
// ---------------------------------------------------------------------------

function VMsSection({
  settings,
  t,
}: {
  settings: Settings;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.vmsSchedule;
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
      <p className="text-sm text-carbon-textMuted">{t("jobs.noVMs")}</p>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Domain section — Flash
// ---------------------------------------------------------------------------

function FlashSection({
  settings,
  t,
}: {
  settings: Settings;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.flashSchedule;
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
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([getSettings(), listContainers()])
      .then(([settingsRes, containersRes]) => {
        if (!active) return;
        if (settingsRes.ok) setSettings(settingsRes.settings);
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

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold text-carbon-text">
            {t("jobs.title")}
          </h1>
          <p className="mt-1 text-sm text-carbon-textSub">{t("jobs.subtitle")}</p>
        </div>
        {/* Link to Settings */}
        <NavLink
          to="/settings"
          className="text-xs text-carbon-textSub hover:text-carbon-text transition-colors shrink-0 mt-1"
        >
          → {t("jobs.configureInSettings")}
        </NavLink>
      </div>

      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {error && (
        <p className="text-sm text-[#ff8389]">{error}</p>
      )}

      {!loading && !error && settings && (
        <>
          <ContainersSection settings={settings} containers={containers} t={t} />
          <VMsSection settings={settings} t={t} />
          <FlashSection settings={settings} t={t} />
        </>
      )}
    </div>
  );
}
