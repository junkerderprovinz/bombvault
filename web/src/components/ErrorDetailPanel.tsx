// ErrorDetailPanel is the modal behind the dashboard's error count. It groups
// the failed runs by kind and error message, so one fault across many targets
// reads as one row while a failed database dump never reads as the container's
// backup, and lets the user acknowledge a group or every failure at once.
// After an acknowledge it reloads and calls onChanged so the parent can
// refresh its count.

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { listRuns, ackRuns } from "../lib/api";
import type { Run } from "../lib/api";
import { useT } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import { LOG_FILTER_DOMAINS, LOG_FILTER_KINDS } from "../lib/activityLog";
import type { LogFilterDomain, LogFilterKind } from "../lib/activityLog";
import { SelectField } from "./SelectField";
import { remedyKey } from "../lib/dbdump";
import { runKindLabel } from "../lib/runKind";
import { RunReasonText } from "../lib/runReason";
import { formatTs, relativeTime } from "../lib/reltime";
import { InfoBubble } from "./InfoBubble";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { IconClose } from "./Sidebar";

// The domain filter uses the Activity Log's plural values (containers, vms),
// while a Run carries the singular tag (container, vm). flash, config, files
// and everything are spelled the same in both.
const DOMAIN_SELECT_TO_RUN: Record<string, string> = {
  containers: "container",
  vms: "vm",
  flash: "flash",
  config: "config",
  files: "files",
  everything: "everything",
};

interface ErrorGroup {
  key: string; // kind and trimmed error message, the group identity
  kind: string; // the run kind every member of the group has
  message: string; // display text, may be empty
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
  const [filterDomain, setFilterDomain] = useState<LogFilterDomain>("all");
  const [filterType, setFilterType] = useState<LogFilterKind>("all");

  const load = () => {
    setLoading(true);
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
      })
      .catch(() => {
        /* keep the last known runs */
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
  }, []);

  useEffect(() => {
    closeRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Label for a run's singular domain, using the Activity Log's translations.
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
      case "everything":
        return t("activityLog.domainEverything");
      default:
        return d;
    }
  };

  // Unacknowledged failures that pass the filters, grouped by trimmed error
  // message, newest group first.
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
      // A kind never contains a space, so the pair cannot be read two ways.
      const key = `${r.kind} ${message}`;
      let g = byMsg.get(key);
      if (!g) {
        g = { key, kind: r.kind, message, ids: [], targets: [], domains: [], latest: 0, count: 0 };
        byMsg.set(key, g);
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
        /* leave the list as it is */
      })
      .finally(() => setBusy(false));
  };

  // Portalled to <body>: Dashboard sits inside .glim-page-enter, whose
  // animation leaves a transform behind, and that makes the wrapper the
  // containing block for `position: fixed`, leaving the sidebar clickable.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="errorpanel-title"
        className="glim-modal-card relative flex max-h-[85vh] w-full max-w-3xl flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-carbon-border px-5 py-4">
          <h2 id="errorpanel-title" className="flex items-center">
            <Badge tone="heading" size="heading" wrap>{t("errorPanel.title")}</Badge>
          </h2>
          <div className="flex items-center gap-2">
            <Button
              label={t("errorPanel.resolveAll")}
              labelKey="errorPanel.resolveAll"
              tone="neutral"
              onClick={() => acknowledge({ all: true })}
              disabled={busy || groups.length === 0}
            />
            <Button
              ref={closeRef}
              label={t("common.close")}
              labelKey="common.close"
              glyph={<IconClose />}
              tone="neutral"
              onClick={onClose}
              className="shrink-0"
            />
          </div>
        </div>

        {/* The same text, domain and type filters as the Activity Log. */}
        <div className="flex flex-wrap items-center gap-2 border-b border-carbon-border px-5 py-3">
          <input
            type="text"
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            placeholder={t("errorPanel.filterPlaceholder")}
            aria-label={t("errorPanel.filterPlaceholder")}
            className="flex-1 min-w-[10rem] rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text placeholder:text-carbon-textMuted glim-field-focus"
          />
          <SelectField
            value={filterDomain}
            onChange={(v) => setFilterDomain(v)}
            label={t("activityLog.filterAllDomains")}
            options={LOG_FILTER_DOMAINS.map((o) => ({ value: o.value, label: t(o.key as TranslationKey) }))}
            className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
          />
          <SelectField
            value={filterType}
            onChange={(v) => setFilterType(v)}
            label={t("activityLog.filterAllTypes")}
            options={LOG_FILTER_KINDS.map((o) => ({ value: o.value, label: t(o.key as TranslationKey) }))}
            className="rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text glim-field-focus"
          />
        </div>

        {/* One row per distinct error message. */}
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
          {!loading && groups.length === 0 && (
            <p className="text-sm text-carbon-textMuted">{t("errorPanel.empty")}</p>
          )}
          {!loading && groups.length > 0 && (
            <div className="divide-y divide-carbon-border">
              {groups.map((g) => {
                // Only a dump failure has advice of ours; everything else in
                // here is a message from restic, rclone or Docker.
                const remedy = g.kind === "dbdump" ? remedyKey(g.message) : null;
                return (
                <div key={g.key} className="flex flex-col gap-1.5 py-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-start gap-2">
                      <span className="mt-1 h-2 w-2 shrink-0 rounded-full bg-statusFailSolid" />
                      <p className="min-w-0 wrap-break-word text-sm text-statusFail">
                        {g.message ? (
                          <>
                            {runKindLabel(t, g.kind)}: <RunReasonText reason={g.message} t={t} />
                          </>
                        ) : (
                          runKindLabel(t, g.kind)
                        )}
                      </p>
                      {remedy && <InfoBubble tip={t(remedy)} />}
                    </div>
                    {/* Both badges take size="large" so they are the same height,
                        although one renders a span and the other a button. */}
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
                  <div className="flex flex-col gap-0.5 ps-4 text-xs text-carbon-textMuted">
                    <span className="wrap-break-word">
                      <span className="text-carbon-textSub">{t("errorPanel.affected")}: </span>
                      {g.targets.join(", ")}
                      {g.domains.length > 0 ? ` · ${g.domains.map((d) => domainLabel(d)).join(", ")}` : ""}
                    </span>
                    <span title={formatTs(g.latest)}>{relativeTime(t, g.latest)}</span>
                  </div>
                </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body
  );
}
