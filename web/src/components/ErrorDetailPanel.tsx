// ---------------------------------------------------------------------------
// ErrorDetailPanel (#126) — the modal opened by clicking the dashboard's error
// count. It lists the failed backup runs behind that count, GROUPED by their
// (normalized) error message so one fault hitting many targets reads as a single
// row with a count + the affected target names, and lets the user acknowledge
// ("Resolve") a group — or every failure at once ("Mark all resolved") — which
// dismisses them from the count without touching SQLite by hand.
//
// It fetches its own run list (listRuns) and, after any acknowledge, refetches
// and fires onChanged so the parent can refresh the headline error count.
// ---------------------------------------------------------------------------

import { useEffect, useMemo, useRef, useState } from "react";
import { listRuns, ackRuns } from "../lib/api";
import type { Run } from "../lib/api";
import { useT } from "../lib/i18n";
import { formatTs, relativeTime } from "../lib/reltime";
import { Badge } from "./Badge";

// The domain <select> reuses ActivityLog's PLURAL vocabulary
// (containers/vms/…), but a Run carries the SINGULAR domain tag
// (container/vm/…, with flash/config/files identical in both) — so a filter
// selection must be translated to the run shape before comparing (#126 note).
const DOMAIN_SELECT_TO_RUN: Record<string, string> = {
  containers: "container",
  vms: "vm",
  flash: "flash",
  config: "config",
  files: "files",
};

interface ErrorGroup {
  key: string; // normalized (trimmed) error message — the group identity
  message: string; // display text (may be empty → rendered as "—")
  ids: string[]; // the run ids in this group (the acknowledge targets)
  targets: string[]; // unique affected target names (run.target, never the UUID)
  domains: string[]; // unique singular domains present in the group
  latest: number; // newest startedAt across the group (unix seconds)
  count: number; // number of failed runs in the group
}

export function ErrorDetailPanel({
  onClose,
  onChanged,
}: {
  onClose: () => void;
  /** Called after a successful acknowledge so the parent can refetch its count. */
  onChanged?: () => void;
}) {
  const { t } = useT();
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const closeRef = useRef<HTMLButtonElement>(null);

  const [filterText, setFilterText] = useState("");
  const [filterDomain, setFilterDomain] = useState("all");
  const [filterType, setFilterType] = useState("all");

  const load = () => {
    setLoading(true);
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
      })
      .catch(() => {
        /* non-fatal — keep the last known runs */
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
  }, []);

  // Focus the close button on open + dismiss on Escape (mirrors WhatsNewDialog).
  useEffect(() => {
    closeRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Translate a singular run.domain to a human label, reusing the Activity Log's
  // already-translated domain keys.
  const domainLabel = (d: string): string => {
    switch (d) {
      case "container":
        return t("activityLog.domainContainers");
      case "vm":
        return t("activityLog.domainVMs");
      case "flash":
        return t("activityLog.domainFlash");
      case "config":
        return t("activityLog.domainConfig");
      case "files":
        return t("activityLog.domainFiles");
      default:
        return d;
    }
  };

  // Only UNACKNOWLEDGED failures, narrowed by the filter bar, grouped by their
  // trimmed error message; groups sorted newest-failure-first.
  const groups = useMemo<ErrorGroup[]>(() => {
    const wantDomain = filterDomain === "all" ? null : (DOMAIN_SELECT_TO_RUN[filterDomain] ?? filterDomain);
    const text = filterText.trim().toLowerCase();
    const byMsg = new Map<string, ErrorGroup>();
    for (const r of runs) {
      if (r.status !== "failed") continue;
      if (r.acknowledged) continue;
      if (filterType !== "all" && r.kind !== filterType) continue;
      if (wantDomain !== null && r.domain !== wantDomain) continue;
      const message = (r.error ?? "").trim();
      if (text) {
        const hay = `${message} ${r.target} ${domainLabel(r.domain)}`.toLowerCase();
        if (!hay.includes(text)) continue;
      }
      let g = byMsg.get(message);
      if (!g) {
        g = { key: message, message, ids: [], targets: [], domains: [], latest: 0, count: 0 };
        byMsg.set(message, g);
      }
      g.ids.push(r.id);
      g.count++;
      if (r.target && !g.targets.includes(r.target)) g.targets.push(r.target);
      if (r.domain && !g.domains.includes(r.domain)) g.domains.push(r.domain);
      if (r.startedAt > g.latest) g.latest = r.startedAt;
    }
    return Array.from(byMsg.values()).sort((a, b) => b.latest - a.latest);
    // domainLabel is stable across renders for a fixed language; t drives it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runs, filterText, filterDomain, filterType, t]);

  const countLabel = (n: number) => t("errorPanel.count").replace("{count}", String(n));

  const acknowledge = (body: { ids?: string[]; all?: boolean }) => {
    setBusy(true);
    ackRuns(body)
      .then(() => {
        load();
        onChanged?.();
      })
      .catch(() => {
        /* non-fatal — leave the list as-is */
      })
      .finally(() => setBusy(false));
  };

  return (
    <div
      className="bv-modal-backdrop fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="errorpanel-title"
        className="bv-modal-card flex max-h-[85vh] w-full max-w-3xl flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 border-b border-carbon-border px-5 py-4">
          <h2 id="errorpanel-title" className="text-lg font-semibold text-carbon-text">
            {t("errorPanel.title")}
          </h2>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => acknowledge({ all: true })}
              disabled={busy || groups.length === 0}
              className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("errorPanel.resolveAll")}
            </button>
            <button
              ref={closeRef}
              type="button"
              onClick={onClose}
              aria-label={t("common.close")}
              className="shrink-0 rounded-control p-1 text-carbon-textMuted hover:bg-carbon-hover hover:text-carbon-text"
            >
              <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        </div>

        {/* Filter bar — mirrors the Activity Log's text + domain + type filters. */}
        <div className="flex flex-wrap items-center gap-2 border-b border-carbon-border px-5 py-3">
          <input
            type="text"
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            placeholder={t("errorPanel.filterPlaceholder")}
            aria-label={t("errorPanel.filterPlaceholder")}
            className="flex-1 min-w-[10rem] rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text placeholder:text-carbon-textMuted focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
          />
          <select
            value={filterDomain}
            onChange={(e) => setFilterDomain(e.target.value)}
            className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
          >
            <option value="all">{t("activityLog.filterAllDomains")}</option>
            <option value="containers">{t("activityLog.domainContainers")}</option>
            <option value="vms">{t("activityLog.domainVMs")}</option>
            <option value="flash">{t("activityLog.domainFlash")}</option>
            <option value="config">{t("activityLog.domainConfig")}</option>
            <option value="files">{t("activityLog.domainFiles")}</option>
          </select>
          <select
            value={filterType}
            onChange={(e) => setFilterType(e.target.value)}
            className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
          >
            <option value="all">{t("activityLog.filterAllTypes")}</option>
            <option value="backup">{t("activityLog.typeBackup")}</option>
            <option value="restore">{t("activityLog.typeRestore")}</option>
            <option value="prune">{t("activityLog.typePrune")}</option>
            <option value="verify">{t("activityLog.typeVerify")}</option>
            <option value="offsite">{t("activityLog.typeOffsite")}</option>
            <option value="drill">{t("activityLog.jobDrill")}</option>
            <option value="drdrill">{t("run.kindDRDrill")}</option>
            <option value="tamper">{t("activityLog.jobTamper")}</option>
            <option value="export">{t("activityLog.typeExport")}</option>
          </select>
        </div>

        {/* Body (scrolls) — one row per distinct error message. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
          {!loading && groups.length === 0 && (
            <p className="text-sm text-carbon-textMuted">{t("errorPanel.empty")}</p>
          )}
          {!loading && groups.length > 0 && (
            <div className="divide-y divide-carbon-border">
              {groups.map((g) => (
                <div key={g.key || "(none)"} className="flex flex-col gap-1.5 py-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-start gap-2">
                      <span className="mt-1 h-2 w-2 shrink-0 rounded-full bg-statusFailSolid" />
                      <p className="min-w-0 wrap-break-word text-sm text-statusFail">{g.message || "—"}</p>
                    </div>
                    {/* Count badge + Resolve button share one stage (size="large")
                        so their heights are pixel-identical regardless of the
                        <span> vs <button> element underneath — see Badge.tsx's
                        file header for why that isn't automatic. */}
                    <div className="flex shrink-0 items-center gap-2">
                      <Badge tone="fail" shape="pill" size="large" className="tabular-nums">
                        {countLabel(g.count)}
                      </Badge>
                      <Badge
                        as="button"
                        tone="neutral"
                        size="large"
                        onClick={() => acknowledge({ ids: g.ids })}
                        disabled={busy}
                      >
                        {t("errorPanel.resolve")}
                      </Badge>
                    </div>
                  </div>
                  <div className="flex flex-col gap-0.5 pl-4 text-xs text-carbon-textMuted">
                    <span className="wrap-break-word">
                      <span className="text-carbon-textSub">{t("errorPanel.affected")}: </span>
                      {g.targets.join(", ")}
                      {g.domains.length > 0 ? ` · ${g.domains.map((d) => domainLabel(d)).join(", ")}` : ""}
                    </span>
                    <span title={formatTs(g.latest)}>{relativeTime(t, g.latest)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
