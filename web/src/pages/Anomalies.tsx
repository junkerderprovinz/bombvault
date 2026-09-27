// The Anomalies page: one card per item with open findings, because the
// question a reader brings is which of their things is in trouble and what to
// press. Items without findings keep their monitoring settings in one card at
// the end, and what was closed lately sits in a row below that.
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconDisclosure } from "../components/IconDisclosure";
import { FindingLine, RetentionNote, type FindingAction } from "../components/anomalies/FindingLine";
import { ItemMonitoring } from "../components/anomalies/ItemMonitoring";
import type { AnomalyGlobals } from "../components/ItemAnomalySettings";
import { Card } from "./settings/shared";
import {
  acknowledgeAnomalies,
  getAnomalies,
  getSettings,
  markAnomaliesExpected,
  type AnomalyItem,
  type AnomalySeverity,
  type AnomalyView,
  type Settings,
} from "../lib/api";
import {
  ANOMALY_CHANGED_EVENT,
  ANOMALY_SEVERITY_LABEL,
  anomalyDomainsLabel,
  anomalyErrorText,
  anomalyGroupKey,
  anomalyLearningText,
  anomalyRestorePath,
  anomalySeverityTone,
  groupAnomalies,
  type AnomalyGroup,
} from "../lib/anomalies";
import { hueVars } from "../lib/appearance";
import { useT, type TranslationKey } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { formatTs } from "../lib/reltime";
import { useToast } from "../lib/toast";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useConfirm } from "../lib/useConfirm";

type T = ReturnType<typeof useT>["t"];

const PAGE_SIZE = 500;
const CLOSED_DAYS = 30;
const SEVERITIES: AnomalySeverity[] = ["critical", "warning", "info"];

const TILE_LABEL: Record<AnomalySeverity, TranslationKey> = {
  critical: "anomaly.severity.critical",
  warning: "anomaly.tile.warning",
  info: "anomaly.tile.info",
};

const NUMBER_TONE: Record<AnomalySeverity, string> = {
  critical: "text-statusFail",
  warning: "text-statusWarn",
  info: "text-carbon-text",
};

interface FindingList {
  list: AnomalyView[];
  loaded: boolean;
  failed: boolean;
  retry: () => void;
}

/**
 * useFindings reads every finding in one state, following the cursor to the
 * end: the counts on the page are drawn from this list, and a list cut short
 * would show an item as quiet. It waits for the summary, so its generation and
 * the list arrive together and every later pass refetches.
 */
function useFindings(state: "open" | "closed"): FindingList {
  const { summary, loading: summaryLoading } = useAnomalySummary();
  const generation = summary?.generation;
  const [list, setList] = useState<AnomalyView[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (summaryLoading) return;
    let active = true;
    const since = state === "closed" ? Math.floor(Date.now() / 1000) - CLOSED_DAYS * 86400 : undefined;
    (async () => {
      const all: AnomalyView[] = [];
      let cursor = "";
      do {
        const res = await getAnomalies({ state, since, limit: PAGE_SIZE, cursor: cursor || undefined });
        if (!res.ok) throw new Error("the findings were refused");
        all.push(...res.anomalies);
        cursor = res.nextCursor;
      } while (cursor);
      return all;
    })()
      .then((all) => {
        if (!active) return;
        setList(all);
        setFailed(false);
        setLoaded(true);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [state, generation, summaryLoading, attempt]);

  return { list, loaded, failed, retry: () => setAttempt((n) => n + 1) };
}

async function settle(call: (ids: string[]) => ReturnType<typeof acknowledgeAnomalies>, ids: string[]) {
  const res = await call(ids);
  if (res.ok) window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
  return res;
}

const acknowledgeOne: FindingAction = (a) => settle(acknowledgeAnomalies, [a.id]);
const expectOne: FindingAction = (a) => settle(markAnomaliesExpected, [a.id]);

export function Anomalies() {
  const { t } = useT();
  const [params] = useSearchParams();
  const { items, byTarget, error: itemsFailed, loading: itemsLoading, retry: retryItems } = useAnomalyItems();
  const open = useFindings("open");
  const closed = useFindings("closed");
  const [settings, setSettings] = useState<Settings | null>(null);
  const [hidden, setHidden] = useState<Set<AnomalySeverity>>(new Set());

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  const scope = params.get("scope") ?? "";
  const focusKey = scope.startsWith("item:") ? scope.slice("item:".length) : "";
  const globals: AnomalyGlobals = {
    sensitivity: settings?.anomalySensitivity ?? "balanced",
    notifyMin: settings?.anomalyNotifyMin ?? "critical",
  };
  const enabled = settings?.anomalyEnabled ?? true;

  const counts = useMemo(() => {
    const out: Record<AnomalySeverity, number> = { critical: 0, warning: 0, info: 0 };
    for (const a of open.list) out[a.severity] += 1;
    return out;
  }, [open.list]);
  const groups = useMemo(
    () => groupAnomalies(open.list.filter((a) => !hidden.has(a.severity))),
    [open.list, hidden]
  );
  const loud = useMemo(() => new Set(open.list.map(anomalyGroupKey)), [open.list]);
  const quiet = items.filter((item) => !loud.has(item.targetId));

  function toggle(severity: AnomalySeverity) {
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(severity)) next.delete(severity);
      else next.add(severity);
      return next;
    });
  }

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("anomaly.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("anomaly.pageSubtitle")}</p>
        {settings && !settings.anomalyEnabled && (
          <p className="mt-1 text-sm text-statusWarn">{t("anomaly.offPage")}</p>
        )}
      </div>

      {open.failed ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={open.retry} />
        </div>
      ) : !open.loaded ? (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      ) : (
        <>
          <SeverityTiles t={t} counts={counts} hidden={hidden} onToggle={toggle} />
          {open.list.length === 0 ? (
            <p className="text-sm text-carbon-textSub">{t("anomaly.emptyOpen")}</p>
          ) : groups.length === 0 ? (
            <p className="text-sm text-carbon-textSub">{t("anomaly.emptyHidden")}</p>
          ) : (
            <div className="grid gap-6 md:gap-10 lg:grid-cols-2 lg:gap-x-6">
              {groups.map((g, i) => (
                <FindingCard
                  key={g.key}
                  t={t}
                  group={g}
                  item={byTarget.get(g.key)}
                  globals={globals}
                  enabled={enabled}
                  hueIndex={i}
                  focused={focusKey === g.key}
                />
              ))}
            </div>
          )}
        </>
      )}

      {itemsFailed ? (
        <Card title={t("anomaly.quiet")} hint={t("anomaly.items.hint")} hueIndex={groups.length}>
          {/* A listing that never arrived says nothing about what is watched,
              so it must not look like an empty one. */}
          <div className="flex flex-col items-start gap-2">
            <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
            <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retryItems} />
          </div>
        </Card>
      ) : (
        !itemsLoading &&
        open.loaded &&
        quiet.length > 0 && (
          <QuietCard
            t={t}
            items={quiet}
            globals={globals}
            enabled={enabled}
            hueIndex={groups.length}
            focusKey={focusKey}
          />
        )
      )}

      <ClosedRow t={t} closed={closed} />
    </div>
  );
}

/**
 * SeverityTiles count the open findings by severity and double as the only
 * filter: a press hides that severity's findings and a second one brings them
 * back. The count is a status readout in its fixed hue; the tile itself is a
 * control, so it keeps neutral chrome and takes its colour-engine position
 * like any other member of a set.
 */
function SeverityTiles({
  t,
  counts,
  hidden,
  onToggle,
}: {
  t: T;
  counts: Record<AnomalySeverity, number>;
  hidden: Set<AnomalySeverity>;
  onToggle: (severity: AnomalySeverity) => void;
}) {
  return (
    <div role="group" aria-label={t("anomaly.filter.severity")} className="grid grid-cols-3 gap-2 sm:gap-4">
      {SEVERITIES.map((severity, i) => {
        const shown = !hidden.has(severity);
        return (
          <button
            key={severity}
            type="button"
            aria-pressed={shown}
            onClick={() => onToggle(severity)}
            style={hueVars(i) as CSSProperties}
            className={`glim-hue glim-tint${shown ? " glim-active" : ""} flex min-w-0 flex-col items-start gap-0.5 rounded-card bg-carbon-surface px-3 py-2.5 text-start hover:bg-carbon-surface2 sm:px-4 sm:py-3`}
          >
            <span
              className={`glim-num text-2xl font-semibold leading-tight ${shown ? NUMBER_TONE[severity] : "text-carbon-textMuted"}`}
            >
              {counts[severity].toLocaleString()}
            </span>
            <span className={`min-w-0 text-sm wrap-anywhere ${shown ? "text-carbon-text" : "text-carbon-textMuted"}`}>
              {t(TILE_LABEL[severity])}
            </span>
            <span aria-hidden="true" className="min-w-0 text-xs text-carbon-textSub wrap-anywhere">
              {t(shown ? "anomaly.tile.shown" : "anomaly.tile.hidden")}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function cardTitle(g: AnomalyGroup, item: AnomalyItem | undefined, t: T): string {
  const a = g.findings[0];
  if (a.scopeKind === "domain") return t("anomaly.detector.integrity");
  if (a.scopeKind === "volume") return t("anomaly.detector.capacity");
  return item?.name || a.name || anomalyDomainsLabel(a.domain, t);
}

/** What kind of thing the card is about, ahead of its count. */
function cardKind(g: AnomalyGroup, t: T): string {
  const a = g.findings[0];
  const domains = anomalyDomainsLabel(a.domain, t);
  if (a.scopeKind === "volume") return t("anomaly.card.volume").replace("{domains}", domains);
  if (a.scopeKind === "domain") {
    const offsite = a.metric === "drill_dr" || a.details.source === "offsite";
    return `${domains} · ${a.targetName || t(offsite ? "source.offsite" : "source.local")}`;
  }
  return domains;
}

function FindingCard({
  t,
  group,
  item,
  globals,
  enabled,
  hueIndex,
  focused,
}: {
  t: T;
  group: AnomalyGroup;
  item?: AnomalyItem;
  globals: AnomalyGlobals;
  enabled: boolean;
  hueIndex: number;
  focused: boolean;
}) {
  const navigate = useNavigate();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const ref = useRef<HTMLElement>(null);
  const [openIds, setOpenIds] = useState<Set<string>>(
    () => new Set(focused ? group.findings.map((a) => a.id) : [])
  );
  const [monitoring, setMonitoring] = useState(false);
  const [busy, setBusy] = useState(false);
  const monitoringId = `monitoring-${group.key}`;

  useEffect(() => {
    if (!focused) return;
    setOpenIds(new Set(group.findings.map((a) => a.id)));
    ref.current?.scrollIntoView({ block: "start" });
    // The link names the card once; findings arriving later must not pull
    // the page back to it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focused]);

  const title = cardTitle(group, item, t);
  const findings = group.findings;
  const restore = findings.find((a) => a.severity === "critical" && anomalyRestorePath(a));
  const heldAndClosed = findings.some((a) => a.retentionHeld && !openIds.has(a.id));

  function toggleLine(id: string) {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function acknowledgeAll() {
    if (findings.some((a) => a.retentionHeld)) {
      const message = t("anomaly.releaseConfirm").replace("{name}", title);
      if (!(await confirm(message, { confirmKey: "anomaly.action.acknowledge" }))) return;
    }
    setBusy(true);
    try {
      const res = await settle(acknowledgeAnomalies, findings.map((a) => a.id));
      if (!res.ok) push(anomalyErrorText(res.code, t), "fail");
    } catch {
      push(anomalyErrorText(undefined, t), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      ref={ref}
      aria-label={title}
      style={hueVars(hueIndex) as CSSProperties}
      className={`glim-hue glim-notch-card relative flex scroll-mt-10 flex-col gap-1 rounded-card p-5 pt-6 ${
        group.worst === "critical" ? "bg-statusFailBgSoft" : "bg-carbon-surface"
      }${findings.length > 1 ? " lg:col-span-2" : ""}`}
    >
      {/* Two heading badges in the flow of one positioned h2, as in StepCard,
          so the pair stays centred on the card's top edge when a long name
          wraps. */}
      <h2 className="absolute top-0 z-10 flex max-w-[calc(100%-2.5rem)] -translate-y-1/2 items-center gap-1.5">
        <Badge tone="heading" size="heading" inFlow wrap hueIndex={hueIndex} className="min-w-0">
          {/* An item name is an identifier, so it keeps its own case. */}
          <span className="min-w-0 normal-case tracking-normal wrap-anywhere">{title}</span>
        </Badge>
        <Badge tone={anomalySeverityTone(group.worst)} size="heading" className="shrink-0 shadow-[var(--elevation)]">
          {t(ANOMALY_SEVERITY_LABEL[group.worst])}
        </Badge>
      </h2>

      <p className="text-xs text-carbon-textSub wrap-anywhere">
        {cardKind(group, t)} · {t("anomaly.card.open").replace("{n}", findings.length.toLocaleString())}
      </p>

      <div className="flex flex-col divide-y divide-carbon-border/60">
        {findings.map((a) => (
          <FindingLine
            key={a.id}
            a={a}
            t={t}
            open={openIds.has(a.id)}
            onToggle={() => toggleLine(a.id)}
            restoreIsPrimary={a === restore}
            onAcknowledge={acknowledgeOne}
            onExpected={expectOne}
          />
        ))}
      </div>

      {heldAndClosed && <RetentionNote t={t} />}

      <div className="mt-2 flex flex-wrap items-center gap-3">
        {item && (
          <button
            type="button"
            onClick={() => setMonitoring(!monitoring)}
            aria-expanded={monitoring}
            aria-controls={monitoring ? monitoringId : undefined}
            className="-mx-1 flex min-h-11 items-center gap-1.5 rounded-control px-1 text-xs text-carbon-textSub hover:text-carbon-text"
          >
            {t("anomaly.card.monitoring")}
            <IconDisclosure open={monitoring} />
          </button>
        )}
        <div className="ms-auto">
          {restore?.lastGood ? (
            <Button
              label={t("anomaly.action.restoreLastGood").replace("{date}", isolateLtr(formatTs(restore.lastGood.at)))}
              labelKey="anomaly.action.restoreLastGood"
              tone="accent"
              onClick={() => navigate(anomalyRestorePath(restore)!)}
              className="glim-btn-wrap"
            />
          ) : findings.length === 1 ? (
            <Button
              label={t("anomaly.action.acknowledge")}
              labelKey="anomaly.action.acknowledge"
              onClick={() => void acknowledgeAll()}
              disabled={busy}
              hint={t("anomaly.acknowledgeHint")}
              className="glim-btn-wrap"
            />
          ) : (
            <Button
              label={t("anomaly.action.acknowledgeAll").replace("{n}", findings.length.toLocaleString())}
              labelKey="anomaly.action.acknowledgeAll"
              onClick={() => void acknowledgeAll()}
              disabled={busy}
              hint={t("anomaly.acknowledgeHint")}
              className="glim-btn-wrap"
            />
          )}
        </div>
      </div>

      {item && monitoring && (
        <div className="mt-2">
          <ItemMonitoring id={monitoringId} t={t} item={item} globals={globals} enabled={enabled} />
        </div>
      )}
      {confirmDialog}
    </section>
  );
}

function QuietCard({
  t,
  items,
  globals,
  enabled,
  hueIndex,
  focusKey,
}: {
  t: T;
  items: AnomalyItem[];
  globals: AnomalyGlobals;
  enabled: boolean;
  hueIndex: number;
  focusKey: string;
}) {
  return (
    <Card title={`${t("anomaly.quiet")} · ${items.length.toLocaleString()}`} hint={t("anomaly.items.hint")} hueIndex={hueIndex}>
      <ul className="grid gap-x-8 md:grid-cols-2">
        {items.map((item) => (
          <QuietRow
            key={item.targetId}
            t={t}
            item={item}
            globals={globals}
            enabled={enabled}
            focused={focusKey === item.targetId}
          />
        ))}
      </ul>
    </Card>
  );
}

function QuietRow({
  t,
  item,
  globals,
  enabled,
  focused,
}: {
  t: T;
  item: AnomalyItem;
  globals: AnomalyGlobals;
  enabled: boolean;
  focused: boolean;
}) {
  const ref = useRef<HTMLLIElement>(null);
  const [open, setOpen] = useState(focused);
  const panelId = `monitoring-${item.targetId}`;
  const { samples, needed, noData } = item.learning;
  // An item with nothing to back up says so in its panel; the chip is for
  // the count only.
  const learning = enabled && item.scheduled && !noData && samples < needed;

  useEffect(() => {
    if (!focused) return;
    setOpen(true);
    ref.current?.scrollIntoView({ block: "center" });
  }, [focused]);

  return (
    <li ref={ref} className="flex min-w-0 flex-col">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        className="-mx-2 flex min-h-11 items-center gap-2 rounded-control px-2 py-1.5 text-start hover:bg-carbon-hover"
      >
        <span className="min-w-0 flex-1 text-sm text-carbon-text wrap-anywhere">
          {item.name || anomalyDomainsLabel(item.domain, t)}
        </span>
        {!item.scheduled && (
          <Badge tone="neutral" size="small" className="shrink-0">
            {t("anomaly.items.notScheduled")}
          </Badge>
        )}
        {learning && (
          <Badge tone="neutral" size="small" className="shrink-0">
            {anomalyLearningText(t, samples, needed, noData)}
          </Badge>
        )}
        <span className="shrink-0 text-xs text-carbon-textSub max-sm:hidden">
          {anomalyDomainsLabel(item.domain, t)}
        </span>
        <span className="shrink-0 text-carbon-textSub">
          <IconDisclosure open={open} />
        </span>
      </button>
      {open && (
        <div className="pb-3 pt-1">
          <ItemMonitoring id={panelId} t={t} item={item} globals={globals} enabled={enabled} />
        </div>
      )}
    </li>
  );
}

function ClosedRow({ t, closed }: { t: T; closed: FindingList }) {
  const [open, setOpen] = useState(false);
  const [openIds, setOpenIds] = useState<Set<string>>(new Set());

  function toggleLine(id: string) {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <section className="flex flex-col rounded-card bg-carbon-surface px-5 py-1">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        className="-mx-2 flex min-h-12 items-center gap-3 rounded-control px-2 text-start hover:bg-carbon-hover"
      >
        <span className="text-carbon-textSub">
          <IconDisclosure open={open} />
        </span>
        <span className="min-w-0 flex-1 text-sm text-carbon-text">{t("anomaly.closedRecent")}</span>
        {closed.loaded && (
          <span className="glim-num shrink-0 text-sm text-carbon-textSub">{closed.list.length.toLocaleString()}</span>
        )}
      </button>
      {open && (
        <div className="flex flex-col divide-y divide-carbon-border/60 pb-2 glim-content-fade">
          {closed.failed ? (
            <div className="flex flex-col items-start gap-2 py-2">
              <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
              <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={closed.retry} />
            </div>
          ) : !closed.loaded ? (
            <p className="py-2 text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
          ) : closed.list.length === 0 ? (
            <p className="py-2 text-sm text-carbon-textSub">{t("anomaly.emptyClosed")}</p>
          ) : (
            closed.list.map((a) => (
              <FindingLine
                key={a.id}
                a={a}
                t={t}
                closed
                open={openIds.has(a.id)}
                onToggle={() => toggleLine(a.id)}
              />
            ))
          )}
        </div>
      )}
    </section>
  );
}

