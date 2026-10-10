import { useEffect, useState } from "react";
import type { CSSProperties } from "react";
import { getStats } from "../../lib/api";
import type { DomainStatus, RepoStat, StorageForecast } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { buildForecastLine, humanBytes, type ResolveForecast } from "../../lib/forecast";
import type { TranslationKey, useT } from "../../lib/i18n";
import { NO_VALUE, relativeTime } from "../../lib/reltime";
import { InfoBubble } from "../../components/InfoBubble";
import { MobileSectionLabel } from "../../components/mobile/MobileSectionLabel";
import { Card } from "./Card";
import { WorstRpoHealthLine } from "./WorstRpoHealthLine";

// Sparkline is a hand-drawn SVG trend line; the app carries no charting library.
function Sparkline({
  values,
  width = 120,
  height = 28,
}: {
  values: number[];
  width?: number;
  height?: number;
}) {
  // Need at least two points to draw a line.
  if (!values || values.length < 2) return null;

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  const pad = 2; // keep the stroke off the edges
  const usableH = height - pad * 2;
  const usableW = width - pad * 2;
  const step = usableW / (values.length - 1);

  const points = values
    .map((v, i) => {
      const x = pad + i * step;
      // Flat line when all values are equal (avoid divide-by-zero).
      const y = span === 0 ? height / 2 : pad + usableH - ((v - min) / span) * usableH;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    // The design language gives a hand-drawn chart one colour source, the
    // accent, never rainbow or status hues. text-accentText instead of the
    // flat accent: the gold measures 1.61:1 against this card in the light
    // theme, well under the 3:1 minimum for non-text marks.
    <span className="text-accentText shrink-0">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        className="block"
        aria-hidden="true"
      >
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      </svg>
    </span>
  );
}

type StorageDomain = "containers" | "vms" | "flash" | "files" | "zfs";

interface DomainStats {
  domain: StorageDomain;
  stats: RepoStat[];
  latest: RepoStat | null;
  /** Growth + time-to-full forecast riding the same /api/stats response. */
  forecast: StorageForecast | null;
}

export function StorageCard({
  t,
  hueIndex,
  dense,
  domains,
  statusLoading,
  statusFailed,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** True = the phone glance block; false = the desktop history card. */
  dense: boolean;
  /** Extended domain status (off-site configuration + last replication),
   *  read by the phone face's health + replication lines. */
  domains: DomainStatus[];
  /** True until the page's /api/status load settles (the phone face's health
   *  row gates on it; the desktop card has nothing to gate on it). */
  statusLoading: boolean;
  /** True when that load refused or failed; the phone off-site line then
   *  reports the failed read instead of reading the empty list as "no
   *  off-site copy". */
  statusFailed: boolean;
}) {
  const [data, setData] = useState<DomainStats[] | null>(null);
  const [loading, setLoading] = useState(true);

  // Resolves a translation key and its {placeholder} params for the forecast
  // line. ActivityLog injects the same seam, so buildForecastLine stays pure
  // and testable without an I18nProvider.
  const resolveForecast: ResolveForecast = (key, params) => {
    let s = t(key as TranslationKey);
    if (params) {
      for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
    }
    return s;
  };

  useEffect(() => {
    let active = true;
    const statsDomains: StorageDomain[] = ["containers", "vms", "flash", "files", "zfs"];
    Promise.all(statsDomains.map((d) => getStats(d, "local", 90)))
      .then((results) => {
        if (!active) return;
        setData(
          results.map((res, i) => ({
            domain: statsDomains[i],
            stats: res.ok ? (res.stats ?? []) : [],
            latest: res.ok ? (res.latest ?? null) : null,
            forecast: res.ok ? (res.forecast ?? null) : null,
          }))
        );
      })
      .catch(() => {/* non-fatal */})
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const domainLabel = (d: StorageDomain): string => {
    switch (d) {
      case "containers":
        return t("dashboard.domainContainers");
      case "vms":
        return t("dashboard.domainVMs");
      case "flash":
        return t("dashboard.domainFlash");
      case "files":
        return t("dashboard.domainFiles");
      case "zfs":
        return t("dashboard.domainZFS");
    }
  };

  const anyData = !!data && data.some((d) => d.latest != null);

  // Repo totals for the phone face, from this component's one stats fetch.
  // The phone face is the !isDesktop complement of the desktop grid, so the
  // two never fetch side by side.
  const totals = (() => {
    if (!data) return null;
    let rawSize = 0;
    let restoreSize = 0;
    let snapshots = 0;
    let any = false;
    for (const d of data) {
      if (d.latest) {
        any = true;
        rawSize += d.latest.rawSize;
        restoreSize += d.latest.restoreSize;
        snapshots += d.latest.snapshots;
      }
    }
    return any ? { rawSize, restoreSize, snapshots } : null;
  })();
  const totalsDedup =
    totals && totals.rawSize > 0 && totals.restoreSize > 0
      ? `${(totals.restoreSize / totals.rawSize).toFixed(1)}x`
      : NO_VALUE;

  if (dense) {
    // Off-site copy age: the most recent replication across the configured
    // domains, in the OffsiteIndicator line language (↗ + relative age,
    // text-statusOffsite; the token is text-only by design), never a fifth
    // status hue. With a repo configured but nothing replicated yet, the
    // replication line's own "not replicated yet" says so; with none
    // configured, the protection card's existing "No off-site copy".
    const configured = domains.filter((d) => d.offsiteConfigured);
    const newestReplication = configured.reduce<DomainStatus | null>(
      (newest, d) =>
        d.lastReplicationAt > 0 && (newest === null || d.lastReplicationAt > newest.lastReplicationAt)
          ? d
          : newest,
      null
    );
    return (
      <section className="relative glim-notch-card glim-hue" style={hueVars(hueIndex ?? 0) as CSSProperties}>
        <MobileSectionLabel t={t} labelKey="dashboard.storageTitle" />
        <div className="flex flex-col gap-2 rounded-card bg-carbon-surface p-4 pt-5">
          <div className="flex flex-wrap items-center gap-2">
            <WorstRpoHealthLine t={t} domains={domains} loading={statusLoading} />
          </div>
          <p className="text-xs text-carbon-textMuted">
            {loading
              ? t("dashboard.checking")
              : totals
                ? `${humanBytes(totals.rawSize)} · ${t("dashboard.dedup")} ${totalsDedup} · ${totals.snapshots} ${t("dashboard.snapshotsLabel")}`
                : t("dashboard.noStats")}
          </p>
          <p className="text-xs">
            {/* Three states, in order: still checking, could not check, and
                only then the answer itself. An empty list before the first
                answer (or after a failed read) must not read as "no off-site
                copy", which is a claim about the user's setup, not about the
                request. */}
            {statusLoading ? (
              <span className="text-carbon-textMuted">{t("dashboard.checking")}</span>
            ) : statusFailed ? (
              <span className="text-carbon-textMuted">{t("dashboard.statusLoadFailed")}</span>
            ) : newestReplication ? (
              <span className="font-semibold text-statusOffsite">
                ↗ {relativeTime(t, newestReplication.lastReplicationAt)}
              </span>
            ) : configured.length > 0 ? (
              <span className="text-carbon-textMuted">{t("ransomware.replicationNever")}</span>
            ) : (
              <span className="text-carbon-textMuted">{t("dashboard.noOffsite")}</span>
            )}
          </p>
        </div>
      </section>
    );
  }

  return (
    <Card title={t("dashboard.storageTitle")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && !anyData && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noStats")}</p>
      )}
      {!loading && anyData && data && (
        // Rows separated by shade (soft tiles), never divider lines.
        <div className="flex flex-col gap-1 glim-content-fade">
          {data.map((d) => {
            const has = d.latest != null;
            const dedup =
              d.latest && d.latest.restoreSize > 0 && d.latest.rawSize > 0
                ? `${(d.latest.restoreSize / d.latest.rawSize).toFixed(1)}x`
                : NO_VALUE;
            // Compact per-domain forecast line (growth/week + time-to-full +
            // free space) from the same /api/stats response. It is null when
            // the backend could determine nothing, and then no line renders.
            const forecastLine = buildForecastLine(d.forecast, resolveForecast);
            return (
              <div key={d.domain} className="flex flex-col gap-0.5 rounded-control bg-carbon-surface2 px-2 py-2.5 min-w-0">
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm min-w-0">
                  <span
                    className={`font-medium w-28 shrink-0 truncate ${
                      has ? "text-carbon-text" : "text-carbon-textMuted"
                    }`}
                  >
                    {domainLabel(d.domain)}
                  </span>
                  {has && d.latest ? (
                    <>
                      <span className="text-carbon-text tabular-nums w-20 shrink-0 text-end">
                        {humanBytes(d.latest.rawSize)}
                      </span>
                      <span className="text-carbon-textMuted text-xs shrink-0 w-24 truncate">
                        {t("dashboard.dedup")} {dedup}
                      </span>
                      <span className="text-carbon-textMuted text-xs shrink-0 w-24 truncate">
                        {d.latest.snapshots} {t("dashboard.snapshotsLabel")}
                      </span>
                      <span className="ms-auto shrink-0">
                        <Sparkline values={d.stats.map((s) => s.rawSize)} />
                      </span>
                    </>
                  ) : (
                    <span className="text-xs text-carbon-textMuted flex-1">
                      {t("dashboard.noStats")}
                    </span>
                  )}
                </div>
                {forecastLine && (
                  <p className="ps-1 text-xs text-carbon-textSub wrap-break-word">
                    {forecastLine.growth}
                    {forecastLine.growth && (forecastLine.projection || forecastLine.free) ? " · " : ""}
                    {forecastLine.projection && (
                      /* Near-term projection (< 8 weeks to full) flips to the
                         existing warn text token; otherwise it stays muted. */
                      <span className={forecastLine.warn ? "text-statusWarn" : undefined}>
                        {forecastLine.projection}
                      </span>
                    )}
                    {forecastLine.projection && forecastLine.free ? " · " : ""}
                    {forecastLine.free}
                    {forecastLine.freeUnknown && (
                      <>
                        {forecastLine.growth ? " · " : ""}
                        <span className="inline-flex items-center gap-1 align-middle">
                          {forecastLine.freeUnknown}
                          <InfoBubble tip={forecastLine.freeUnknownTip ?? ""} />
                        </span>
                      </>
                    )}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}
