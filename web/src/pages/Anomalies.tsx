// The Anomalies page. Two tabs answer two different questions: Findings is
// "what did the history turn up", Items is "what does BombVault know about
// each thing it backs up, and how closely should it watch". They share a page
// because a finding is only actionable next to the item's usual figures.
import { useCallback, useEffect, useId, useMemo, useState, type CSSProperties } from "react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom";

import { AnomalyRow } from "../components/AnomalyRow";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { ItemAnomalySettings, type AnomalyGlobals } from "../components/ItemAnomalySettings";
import { LabelledSelect } from "../components/SelectField";
import { Selector } from "../components/Selector";
import { Card } from "./settings/shared";
import {
  acknowledgeAnomalies,
  forgetAnomalyExpectation,
  getAnomalies,
  getSettings,
  markAnomaliesExpected,
  type AnomalyActionResult,
  type AnomalyFilter,
  type AnomalyItem,
  type AnomalySeriesInfo,
  type AnomalySeverity,
  type AnomalyView,
  type Settings,
} from "../lib/api";
import {
  ANOMALY_CHANGED_EVENT,
  ANOMALY_DETECTOR_LABEL,
  ANOMALY_DOMAIN_LABEL,
  ANOMALY_FAMILY_LABEL,
  ANOMALY_SEVERITY_LABEL,
  anomalyErrorText,
  anomalyItemLabel,
  anomalyLearningText,
  anomalySeverityTone,
  itemOpenCounts,
  worstSeverity,
} from "../lib/anomalies";
import { humanBytes } from "../lib/forecast";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { formatDuration, formatTs } from "../lib/reltime";
import { useToast } from "../lib/toast";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useConfirm } from "../lib/useConfirm";

type T = ReturnType<typeof useT>["t"];

const TABS = ["findings", "items"] as const;
type AnomalyTab = (typeof TABS)[number];

const PAGE_SIZE = 200;
// The acknowledge and expected endpoints take a bounded list, so a selection
// over that size goes in several calls.
const BULK_CHUNK = 500;
const NOTE_MAX = 500;
const FILTER_STORAGE_KEY = "bv-anomalies-filter";

type StateFilter = "open" | "closed" | "all";
type PeriodFilter = "7" | "30" | "90" | "all";

interface Filters {
  state: StateFilter;
  period: PeriodFilter;
  severity: string;
  detector: string;
  domain: string;
}

const DEFAULT_FILTERS: Filters = {
  state: "open",
  period: "30",
  severity: "",
  detector: "",
  domain: "",
};

const STATE_LABEL: Record<StateFilter, TranslationKey> = {
  open: "anomaly.filter.stateOpen",
  closed: "anomaly.filter.stateClosed",
  all: "anomaly.filter.any",
};

const PERIOD_LABEL: Record<PeriodFilter, TranslationKey> = {
  "7": "anomaly.filter.period7",
  "30": "anomaly.filter.period30",
  "90": "anomaly.filter.period90",
  all: "anomaly.filter.periodAll",
};

// A finding of an item carries the singular domain, one of a whole domain
// the plural, so each choice sends both spellings.
const DOMAIN_FILTERS: { value: string; key: TranslationKey }[] = [
  { value: "container,containers", key: "dashboard.domainContainers" },
  { value: "vm,vms", key: "dashboard.domainVMs" },
  { value: "flash", key: "dashboard.domainFlash" },
  { value: "files", key: "dashboard.domainFiles" },
  { value: "zfs", key: "dashboard.domainZFS" },
  { value: "config", key: "dashboard.domainConfig" },
];

function isTab(value: string): value is AnomalyTab {
  return (TABS as readonly string[]).includes(value);
}

function loadFilters(): Filters {
  try {
    const raw = localStorage.getItem(FILTER_STORAGE_KEY);
    if (!raw) return DEFAULT_FILTERS;
    const stored = JSON.parse(raw) as Partial<Filters>;
    return {
      state: stored.state && stored.state in STATE_LABEL ? stored.state : DEFAULT_FILTERS.state,
      period: stored.period && stored.period in PERIOD_LABEL ? stored.period : DEFAULT_FILTERS.period,
      severity: typeof stored.severity === "string" ? stored.severity : "",
      detector: typeof stored.detector === "string" ? stored.detector : "",
      domain: typeof stored.domain === "string" ? stored.domain : "",
    };
  } catch {
    return DEFAULT_FILTERS;
  }
}

function storeFilters(filters: Filters) {
  try {
    localStorage.setItem(FILTER_STORAGE_KEY, JSON.stringify(filters));
  } catch {
    /* a browser that refuses storage still filters, it just forgets */
  }
}

function periodSince(period: PeriodFilter): number {
  if (period === "all") return 0;
  return Math.floor(Date.now() / 1000) - Number(period) * 86400;
}

export function Anomalies() {
  const { t } = useT();
  const location = useLocation();
  const navigate = useNavigate();
  const { items, byTarget, error: itemsFailed, loading: itemsLoading, retry } = useAnomalyItems();
  const [settings, setSettings] = useState<Settings | null>(null);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  const hash = location.hash.replace(/^#/, "");
  const tab: AnomalyTab = isTab(hash) ? hash : "findings";

  function choose(next: AnomalyTab) {
    navigate({ search: location.search, hash: `#${next}` }, { replace: true });
  }

  return (
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("anomaly.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("anomaly.pageSubtitle")}</p>
      </div>

      {settings && !settings.anomalyEnabled && (
        <p className="text-sm text-statusWarn">{t("anomaly.offPage")}</p>
      )}

      <div className="inline-flex self-start max-w-full">
        <Selector
          items={[
            { id: "findings", label: t("anomaly.tab.findings") },
            { id: "items", label: t("anomaly.tab.items") },
          ]}
          label={t("anomaly.title")}
          select="one"
          active={tab}
          onChange={(id) => {
            if (isTab(id)) choose(id);
          }}
          size="lg"
          equalWidth
        />
      </div>

      {/* Keyed on the tab so the slide replays on every switch, as on
          Instances and in Settings. */}
      <div
        key={tab}
        className="glim-tab-slide flex flex-col gap-10"
        style={{ "--tab-dir": tab === "items" ? 1 : -1 } as CSSProperties}
      >
        {tab === "findings" ? (
          <FindingsTab t={t} byTarget={byTarget} />
        ) : (
          <ItemsTab
            t={t}
            items={items}
            settings={settings}
            failed={itemsFailed}
            loading={itemsLoading}
            onRetry={retry}
          />
        )}
      </div>
    </div>
  );
}

function FindingsTab({ t, byTarget }: { t: T; byTarget: Map<string, AnomalyItem> }) {
  const { summary, loading: summaryLoading } = useAnomalySummary();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filters, setFilters] = useState<Filters>(loadFilters);
  const [rows, setRows] = useState<AnomalyView[]>([]);
  const [cursor, setCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const countId = useId();

  const scope = searchParams.get("scope") ?? "";
  const generation = summary?.generation;

  const query: AnomalyFilter = useMemo(
    () => ({
      state: filters.state,
      severity: filters.severity || undefined,
      detector: filters.detector || undefined,
      domain: filters.domain || undefined,
      scope: scope || undefined,
      since: filters.state === "open" ? undefined : periodSince(filters.period),
      limit: PAGE_SIZE,
    }),
    [filters, scope]
  );

  // The first pass waits for the summary, so its generation and this listing
  // arrive together instead of costing two requests on every mount.
  useEffect(() => {
    if (summaryLoading) return;
    let active = true;
    setLoading(true);
    getAnomalies(query)
      .then((res) => {
        if (!active) return;
        if (!res.ok) {
          setFailed(true);
        } else {
          setRows(res.anomalies);
          setCursor(res.nextCursor);
          setSelected(new Set());
          setFailed(false);
        }
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setFailed(true);
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [query, attempt, generation, summaryLoading]);

  function change(patch: Partial<Filters>) {
    setFilters((prev) => {
      const next = { ...prev, ...patch };
      storeFilters(next);
      return next;
    });
  }

  function clearScope() {
    const next = new URLSearchParams(searchParams);
    next.delete("scope");
    setSearchParams(next, { replace: true });
  }

  async function loadMore() {
    const res = await getAnomalies({ ...query, cursor });
    if (!res.ok) {
      push(anomalyErrorText(res.code, t), "fail");
      return;
    }
    setRows((prev) => [...prev, ...res.anomalies]);
    setCursor(res.nextCursor);
  }

  const openRows = rows.filter((a) => a.state === "open");
  const chosen = rows.filter((a) => selected.has(a.id));
  const allSelected = openRows.length > 0 && openRows.every((a) => selected.has(a.id));

  function toggleRow(id: string, on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function toggleAll() {
    setSelected(allSelected ? new Set() : new Set(openRows.map((a) => a.id)));
  }

  async function bulk(
    call: (ids: string[], note?: string) => Promise<AnomalyActionResult>,
    confirmKey: TranslationKey
  ) {
    setBusy(true);
    try {
      const held = chosen.filter((a) => a.retentionHeld);
      if (held.length > 0) {
        const names = held.map((a) => anomalyItemLabel(a, t)).join(", ");
        if (!(await confirm(t("anomaly.releaseConfirmMany").replace("{names}", names), { confirmKey }))) {
          return;
        }
      }
      const ids = chosen.map((a) => a.id);
      let changed = 0;
      let skipped = 0;
      for (let at = 0; at < ids.length; at += BULK_CHUNK) {
        const res = await call(ids.slice(at, at + BULK_CHUNK), note);
        if (!res.ok) {
          push(anomalyErrorText(res.code, t), "fail");
          return;
        }
        changed += res.changed;
        skipped += res.skipped;
      }
      push(
        t("anomaly.bulk.done")
          .replace("{changed}", changed.toLocaleString())
          .replace("{skipped}", skipped.toLocaleString())
      );
      window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
      setSelected(new Set());
      setNote("");
    } finally {
      setBusy(false);
    }
  }

  const single = useCallback(
    (call: (ids: string[]) => Promise<AnomalyActionResult>) => async (a: AnomalyView) => {
      const res = await call([a.id]);
      if (res.ok) window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
      return res;
    },
    []
  );

  const narrowed = Boolean(filters.severity || filters.detector || filters.domain || scope);
  const scopeTarget = scope.startsWith("item:") ? scope.slice("item:".length) : "";
  const scopeName =
    byTarget.get(scopeTarget)?.name ?? rows.find((a) => a.targetId === scopeTarget)?.name ?? scopeTarget;

  const severityOptions = [
    { value: "", label: t("anomaly.filter.any") },
    ...(["critical", "warning", "info"] as AnomalySeverity[]).map((s) => ({
      value: s as string,
      label: t(ANOMALY_SEVERITY_LABEL[s]),
    })),
  ];
  const detectorOptions = [
    { value: "", label: t("anomaly.filter.any") },
    ...Object.entries(ANOMALY_DETECTOR_LABEL).map(([value, key]) => ({ value, label: t(key) })),
  ];
  const domainOptions = [
    { value: "", label: t("anomaly.filter.any") },
    ...DOMAIN_FILTERS.map((d) => ({ value: d.value, label: t(d.key) })),
  ];

  return (
    <Card title={t("anomaly.tab.findings")} hueIndex={0}>
      <div className="flex flex-wrap items-end gap-3">
        <LabelledSelect
          label={t("anomaly.filter.state")}
          value={filters.state}
          onChange={(state: StateFilter) => change({ state })}
          options={(["open", "closed", "all"] as StateFilter[]).map((s) => ({
            value: s,
            label: t(STATE_LABEL[s]),
          }))}
        />
        {filters.state !== "open" && (
          <LabelledSelect
            label={t("anomaly.filter.period")}
            value={filters.period}
            onChange={(period: PeriodFilter) => change({ period })}
            options={(["7", "30", "90", "all"] as PeriodFilter[]).map((p) => ({
              value: p,
              label: t(PERIOD_LABEL[p]),
            }))}
          />
        )}
        <LabelledSelect
          label={t("anomaly.filter.severity")}
          value={filters.severity}
          onChange={(severity: string) => change({ severity })}
          options={severityOptions}
        />
        <LabelledSelect
          label={t("anomaly.filter.detector")}
          value={filters.detector}
          onChange={(detector: string) => change({ detector })}
          options={detectorOptions}
        />
        <LabelledSelect
          label={t("common.domain")}
          value={filters.domain}
          onChange={(domain: string) => change({ domain })}
          options={domainOptions}
        />
        {scopeTarget && (
          <span className="inline-flex items-center gap-1 rounded-control bg-carbon-surface2 px-2 py-1 text-xs text-carbon-text">
            {t("anomaly.filter.itemChip").replace("{name}", scopeName)}
            <Button
              label={t("anomaly.filter.removeChip")}
              labelKey="anomaly.filter.removeChip"
              variant="chip"
              onClick={clearScope}
            />
          </span>
        )}
      </div>

      {failed ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
          <Button
            label={t("anomaly.retry")}
            labelKey="anomaly.retry"
            onClick={() => setAttempt((n) => n + 1)}
          />
        </div>
      ) : loading && rows.length === 0 ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-carbon-textSub">
          {t(
            narrowed
              ? "anomaly.emptyFiltered"
              : filters.state === "closed"
                ? "anomaly.emptyClosed"
                : filters.state === "all"
                  ? "anomaly.emptyAny"
                  : "anomaly.emptyOpen"
          )}
        </p>
      ) : (
        <div className="flex flex-col gap-3">
          {openRows.length > 0 && (
            <div
              role="group"
              aria-labelledby={countId}
              className="flex flex-wrap items-center gap-3 rounded-card bg-carbon-surface2 px-3 py-2"
            >
              <label className="flex cursor-pointer items-center gap-2 text-xs text-carbon-textSub">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  className="h-4 w-4 cursor-pointer"
                  style={{ accentColor: "var(--accent)" }}
                />
                {t("anomaly.bulk.selectAll")}
              </label>
              <span id={countId} className="text-xs text-carbon-textSub">
                {t("anomaly.bulk.selected").replace("{n}", selected.size.toLocaleString())}
              </span>
              {cursor && <span className="text-xs text-carbon-textMuted">{t("anomaly.bulk.loadedOnly")}</span>}
              <input
                type="text"
                value={note}
                maxLength={NOTE_MAX}
                onChange={(e) => setNote(e.target.value)}
                placeholder={t("anomaly.notePlaceholder")}
                aria-label={t("anomaly.notePlaceholder")}
                className="min-w-40 flex-1 rounded-control bg-carbon-surface px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              />
              <Button
                label={t("anomaly.bulk.clearSelection")}
                labelKey="anomaly.bulk.clearSelection"
                onClick={() => setSelected(new Set())}
                disabled={busy || selected.size === 0}
              />
              <Button
                label={t("anomaly.action.expected")}
                labelKey="anomaly.action.expected"
                onClick={() => void bulk(markAnomaliesExpected, "anomaly.action.expected")}
                disabled={busy || !chosen.some((a) => a.expectable)}
              />
              <Button
                label={t("anomaly.action.acknowledge")}
                labelKey="anomaly.action.acknowledge"
                tone="accent"
                onClick={() => void bulk(acknowledgeAnomalies, "anomaly.action.acknowledge")}
                disabled={busy || selected.size === 0}
              />
            </div>
          )}

          {rows.map((a) => (
            <AnomalyRow
              key={a.id}
              a={a}
              t={t}
              selectable
              selected={selected.has(a.id)}
              onSelect={toggleRow}
              onAcknowledge={single(acknowledgeAnomalies)}
              onExpected={single(markAnomaliesExpected)}
            />
          ))}

          {cursor && (
            <div className="self-start">
              <Button label={t("anomaly.loadMore")} labelKey="anomaly.loadMore" onClick={() => void loadMore()} />
            </div>
          )}
        </div>
      )}
      {confirmDialog}
    </Card>
  );
}

/** The figures of a series that has one rule set rather than all of them. */
function SeriesLine({ t, label, series }: { t: T; label: string; series: AnomalySeriesInfo }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-3 ps-4 text-xs text-carbon-textSub">
      <span className="text-carbon-text">{label}</span>
      <span>{anomalyLearningText(t, series.learning.samples, series.learning.needed, false)}</span>
      {series.typical.sourceBytes !== null && series.typical.resticMs !== null && (
        <span>
          {t("anomaly.items.typicalSize")
            .replace("{size}", humanBytes(series.typical.sourceBytes))
            .replace("{duration}", formatDuration(series.typical.resticMs / 1000))}
        </span>
      )}
      {series.retentionHeld && <span className="text-statusWarn">{t("anomaly.retentionPaused")}</span>}
    </div>
  );
}

function ItemsTab({
  t,
  items,
  settings,
  failed,
  loading,
  onRetry,
}: {
  t: T;
  items: AnomalyItem[];
  settings: Settings | null;
  failed: boolean;
  loading: boolean;
  onRetry: () => void;
}) {
  const globals: AnomalyGlobals = {
    sensitivity: settings?.anomalySensitivity ?? "balanced",
    notifyMin: settings?.anomalyNotifyMin ?? "critical",
  };
  const enabled = settings?.anomalyEnabled ?? true;

  if (failed || loading || items.length === 0) {
    return (
      <Card title={t("anomaly.tab.items")} hint={t("anomaly.items.hint")} hueIndex={0}>
        {failed ? (
          // A listing that never arrived says nothing about what is watched,
          // so it must not borrow the empty list's wording.
          <div className="flex flex-col items-start gap-2">
            <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
            <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={onRetry} />
          </div>
        ) : (
          <p className="text-sm text-carbon-textSub">
            {t(loading ? "dashboard.checking" : "anomaly.items.empty")}
          </p>
        )}
      </Card>
    );
  }

  const domains: string[] = [];
  for (const item of items) if (!domains.includes(item.domain)) domains.push(item.domain);

  return (
    <Card title={t("anomaly.tab.items")} hint={t("anomaly.items.hint")} hueIndex={0}>
      {domains.map((domain) => (
        <div key={domain} className="flex flex-col gap-3">
          <h3 className="text-sm font-medium text-carbon-text">
            {t(ANOMALY_DOMAIN_LABEL[domain] ?? "repos.title")}
          </h3>
          {items
            .filter((item) => item.domain === domain)
            .map((item) => (
              <ItemRow
                key={item.targetId}
                t={t}
                item={item}
                globals={globals}
                enabled={enabled}
              />
            ))}
        </div>
      ))}
    </Card>
  );
}

function ItemRow({
  t,
  item,
  globals,
  enabled,
}: {
  t: T;
  item: AnomalyItem;
  globals: AnomalyGlobals;
  enabled: boolean;
}) {
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [forgotten, setForgotten] = useState<string[]>([]);

  async function forget(family: string, scopeKind: string, part: string) {
    if (!(await confirm(t("anomaly.expectation.forgetConfirm"), { confirmKey: "anomaly.expectation.forget" }))) {
      return;
    }
    const res = await forgetAnomalyExpectation(item.targetId, family, scopeKind, part);
    if (!res.ok) {
      push(anomalyErrorText(res.code, t), "fail");
      return;
    }
    setForgotten((prev) => [...prev, `${scopeKind}:${part}:${family}`]);
    window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
  }

  const counts = itemOpenCounts(item);
  const openCount = counts.critical + counts.warning + counts.info;
  const worst = worstSeverity(counts);
  const typical = item.typical;

  return (
    <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 px-3 py-2">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="text-sm text-carbon-text">{item.name}</span>
        {!item.scheduled ? (
          <span className="text-xs text-carbon-textSub">{t("anomaly.items.notScheduled")}</span>
        ) : (
          <span className="inline-flex items-center gap-1 text-xs text-carbon-textSub">
            {anomalyLearningText(t, item.learning.samples, item.learning.needed, item.learning.noData)}
            <InfoBubble
              tip={t("anomaly.items.learningDetail")
                .replace("{newData}", item.learning.newData.toLocaleString())
                .replace("{source}", item.learning.source.toLocaleString())
                .replace("{duration}", item.learning.duration.toLocaleString())
                .replace("{needed}", item.learning.needed.toLocaleString())}
            />
          </span>
        )}
        {typical.sourceBytes !== null && typical.resticMs !== null && (
          <span className="text-xs text-carbon-textSub">
            {typical.newDataBytes !== null
              ? t("anomaly.items.typical")
                  .replace("{size}", humanBytes(typical.sourceBytes))
                  .replace("{newData}", humanBytes(typical.newDataBytes))
                  .replace("{duration}", formatDuration(typical.resticMs / 1000))
              : t("anomaly.items.typicalSize")
                  .replace("{size}", humanBytes(typical.sourceBytes))
                  .replace("{duration}", formatDuration(typical.resticMs / 1000))}
          </span>
        )}
        {openCount > 0 && worst && (
          <Link
            to={`/anomalies?scope=item:${encodeURIComponent(item.targetId)}#findings`}
            aria-label={t("anomaly.itemBadgeAria")
              .replace("{name}", item.name)
              .replace("{n}", openCount.toLocaleString())}
          >
            <Badge tone={anomalySeverityTone(worst)} size="small" shape="pill">
              {openCount}
            </Badge>
          </Link>
        )}
        {item.retentionHeld && <span className="text-xs text-statusWarn">{t("anomaly.retentionPaused")}</span>}
      </div>

      {item.dump && <SeriesLine t={t} label={t("anomaly.items.dumpSeries")} series={item.dump} />}
      {item.datasets.map((series) => (
        <SeriesLine key={series.part} t={t} label={series.part} series={series} />
      ))}

      {item.selectionSince > 0 && (
        <p className="text-xs text-carbon-textSub">
          {t("anomaly.expectation.selectionSince").replace("{date}", formatTs(item.selectionSince))}
        </p>
      )}

      {item.expectations
        .filter((e) => !forgotten.includes(`${e.scopeKind}:${e.part}:${e.family}`))
        .map((e) => (
          <div
            key={`${e.scopeKind}:${e.part}:${e.family}`}
            className="flex flex-wrap items-center gap-2 text-xs text-carbon-textSub"
          >
            {/* An expectation of a dump or a dataset says which series it
                belongs to, or it would read as the item's own. */}
            {e.scopeKind === "zfsds" && (
              <span dir="ltr" className="font-mono text-carbon-text text-start">
                {e.part}
              </span>
            )}
            {e.scopeKind === "dump" && <span className="text-carbon-text">{t("anomaly.items.dumpSeries")}</span>}
            <span>
              {e.ceiling > 0
                ? t("anomaly.expectation.ceiling")
                    .replace("{family}", t(ANOMALY_FAMILY_LABEL[e.family] ?? "anomaly.family.newData"))
                    .replace("{bytes}", humanBytes(e.ceiling))
                : t("anomaly.expectation.since")
                    .replace("{family}", t(ANOMALY_FAMILY_LABEL[e.family] ?? "anomaly.family.newData"))
                    .replace("{date}", formatTs(e.sinceAt))}
            </span>
            <Button
              label={t("anomaly.expectation.forget")}
              labelKey="anomaly.expectation.forget"
              onClick={() => void forget(e.family, e.scopeKind, e.part)}
            />
          </div>
        ))}

      <ItemAnomalySettings item={item} enabled={enabled} globals={globals} t={t} />
      {confirmDialog}
    </div>
  );
}
