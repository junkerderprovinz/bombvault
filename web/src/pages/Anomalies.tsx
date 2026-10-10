// The Anomalies page: the findings as one list, worst first, each with the
// way out of it. Tiles choose which severities the list shows, a selector
// switches between what is open and what was closed lately, and the watched
// items stand below as rings of how far detection has learned them.
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { Button } from "../components/Button";
import { FindingLine, type SettleFindings } from "../components/anomalies/FindingLine";
import { FindingWindow } from "../components/anomalies/FindingWindow";
import { LearningRings } from "../components/anomalies/LearningRings";
import { MonitoringWindow } from "../components/anomalies/MonitoringWindow";
import { IconAnomalies, IconGear } from "../components/navGlyphs";
import { PageTitle } from "../components/PageTitle";
import { Selector } from "../components/Selector";
import type { AnomalyGlobals } from "../components/ItemAnomalySettings";
import { Card } from "./settings/shared";
import {
  acknowledgeAnomalies,
  getAnomalies,
  getSettings,
  markAnomaliesExpected,
  type AnomalySeverity,
  type AnomalyView,
  type Settings,
} from "../lib/api";
import {
  ANOMALY_CHANGED_EVENT,
  anomalyEntryName,
  anomalyErrorText,
  anomalyGroupKey,
  sortClosedAnomalies,
  sortOpenAnomalies,
} from "../lib/anomalies";
import { findingHasCurve } from "../lib/anomalyCurve";
import { hueVars } from "../lib/appearance";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useToast } from "../lib/toast";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useConfirm } from "../lib/useConfirm";

type T = ReturnType<typeof useT>["t"];
type View = "open" | "closed";

const PAGE_SIZE = 500;
const CLOSED_DAYS = 30;
const SETTINGS_PATH = "/settings/integrity#anomalies";

const TILES: { severity: AnomalySeverity | null; label: TranslationKey; glyph: string }[] = [
  { severity: null, label: "filter.all", glyph: "" },
  { severity: "critical", label: "anomaly.severity.critical", glyph: "text-statusFail" },
  { severity: "warning", label: "anomaly.tile.warning", glyph: "text-statusWarn" },
  { severity: "info", label: "anomaly.tile.info", glyph: "text-carbon-textMuted" },
];

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
 * the list arrive together and every later pass refetches. The closed list is
 * only asked for once somebody looks at it.
 */
function useFindings(state: View, wanted = true): FindingList {
  const { summary, loading: summaryLoading } = useAnomalySummary();
  const generation = summary?.generation;
  const [list, setList] = useState<AnomalyView[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (summaryLoading || !wanted) return;
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
  }, [state, generation, summaryLoading, wanted, attempt]);

  return { list, loaded, failed, retry: () => setAttempt((n) => n + 1) };
}

type OpenWindow =
  | { kind: "finding"; id: string; comparing: boolean }
  | { kind: "monitoring"; targetId: string; from?: string };

export function Anomalies() {
  const { t } = useT();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const { items, byTarget, error: itemsFailed, loading: itemsLoading, retry: retryItems } = useAnomalyItems();
  const [view, setView] = useState<View>("open");
  const [chosen, setChosen] = useState<AnomalySeverity[] | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [shownWindow, setShownWindow] = useState<OpenWindow | null>(null);
  const open = useFindings("open");
  const closed = useFindings("closed", view === "closed");

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok && res.settings) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  const globals: AnomalyGlobals = {
    sensitivity: settings?.anomalySensitivity ?? "balanced",
    notifyMin: settings?.anomalyNotifyMin ?? "critical",
  };
  const enabled = settings?.anomalyEnabled ?? true;

  const current = view === "open" ? open : closed;
  const sorted = useMemo(
    () => (view === "open" ? sortOpenAnomalies(current.list) : sortClosedAnomalies(current.list)),
    [view, current.list]
  );
  const shown = chosen ? sorted.filter((a) => chosen.includes(a.severity)) : sorted;

  // A link names one item. Its only finding opens, several are brought into
  // view, and an item with none shows its monitoring. It acts once, so
  // findings arriving later do not pull the page back.
  const scope = params.get("scope") ?? "";
  const focusKey = scope.startsWith("item:") ? scope.slice("item:".length) : "";
  const focused = useRef(false);
  useEffect(() => {
    if (!focusKey || focused.current || !open.loaded) return;
    const own = sortOpenAnomalies(open.list).filter((a) => anomalyGroupKey(a) === focusKey);
    if (own.length === 0) {
      if (itemsLoading) return;
      focused.current = true;
      if (byTarget.has(focusKey)) setShownWindow({ kind: "monitoring", targetId: focusKey });
      return;
    }
    focused.current = true;
    document.getElementById(`finding-${own[0].id}`)?.scrollIntoView({ block: "center" });
    if (own.length === 1) setShownWindow({ kind: "finding", id: own[0].id, comparing: false });
  }, [focusKey, open.loaded, open.list, itemsLoading, byTarget]);

  function choose(severity: AnomalySeverity | null) {
    if (!severity) return setChosen(null);
    setChosen((prev) => {
      const next = prev?.includes(severity) ? prev.filter((s) => s !== severity) : [...(prev ?? []), severity];
      return next.length === 0 || next.length === 3 ? null : next;
    });
  }

  const settle: SettleFindings = async (list, how) => {
    const confirmKey = how === "expected" ? "anomaly.action.expected" : "anomaly.action.acknowledge";
    if (list.some((a) => a.retentionHeld)) {
      const message = t("anomaly.releaseConfirm").replace("{name}", anomalyEntryName(list[0], t));
      if (!(await confirm(message, { confirmKey }))) return false;
    }
    try {
      const call = how === "expected" ? markAnomaliesExpected : acknowledgeAnomalies;
      const res = await call(list.map((a) => a.id));
      if (!res.ok) {
        push(anomalyErrorText(res.code, t), "fail");
        return false;
      }
    } catch {
      push(anomalyErrorText(undefined, t), "fail");
      return false;
    }
    window.dispatchEvent(new Event(ANOMALY_CHANGED_EVENT));
    push(
      how === "expected"
        ? t("anomaly.markedExpected")
        : list.length > 1
          ? t("anomaly.toast.acknowledgedAll", list.length)
          : t("anomaly.state.acknowledged")
    );
    return true;
  };

  const windowFinding =
    shownWindow?.kind === "finding" ? [...open.list, ...closed.list].find((a) => a.id === shownWindow.id) : undefined;
  const monitored = shownWindow?.kind === "monitoring" ? byTarget.get(shownWindow.targetId) : undefined;

  // The curve in an item's monitoring follows the finding the window was
  // opened from, otherwise the item's most pressing one.
  function curveFinding(targetId: string, from?: string): AnomalyView | undefined {
    const own = sortOpenAnomalies(open.list).filter((a) => a.targetId === targetId && findingHasCurve(a));
    return own.find((a) => a.id === from) ?? own[0];
  }

  const empty: TranslationKey =
    view === "closed" ? "anomaly.emptyClosed" : open.list.length > 0 ? "anomaly.emptySeverity" : "anomaly.emptyOpen";
  const seen = new Set<string>();

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("anomaly.title")}</PageTitle>
      {settings && !settings.anomalyEnabled && (
        <div className="flex flex-wrap items-center gap-3 rounded-card bg-accentSoft py-3 pe-3 ps-4 text-sm text-carbon-text">
          <span className="flex-none text-accentText">
            <IconAnomalies />
          </span>
          <span className="min-w-[13rem] flex-1">{t("anomaly.offPage")}</span>
          <Button
            label={t("anomaly.openSettings")}
            labelKey="anomaly.openSettings"
            glyph={<IconGear />}
            onClick={() => navigate(SETTINGS_PATH)}
          />
        </div>
      )}

      {current.loaded && !current.failed && (
        <SeverityTiles t={t} list={current.list} view={view} chosen={chosen} onChoose={choose} />
      )}

      <div className="flex md:justify-end">
        <Selector
          items={[
            { id: "open", label: t("anomaly.state.open") },
            { id: "closed", label: t("anomaly.closedRecent") },
          ]}
          label={t("anomaly.view")}
          select="one"
          active={view}
          onChange={(id) => setView(id as View)}
          variant="well"
          inline
        />
      </div>

      <Card title={t(view === "open" ? "anomaly.list.open" : "anomaly.closedRecent")} hueIndex={TILES.length}>
        {current.failed ? (
          <div className="flex flex-col items-start gap-2">
            <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
            <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={current.retry} />
          </div>
        ) : !current.loaded ? (
          <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
        ) : shown.length === 0 ? (
          <p className="px-2 py-9 text-center text-sm text-carbon-textMuted">{t(empty)}</p>
        ) : (
          <ul className="flex flex-col gap-2.5">
            {shown.map((a, i) => {
              const key = anomalyGroupKey(a);
              const first = !seen.has(key);
              seen.add(key);
              const siblings = first && view === "open" ? shown.filter((b) => anomalyGroupKey(b) === key) : [];
              return (
                <FindingLine
                  key={a.id}
                  a={a}
                  t={t}
                  closed={view === "closed"}
                  lead={i === 0}
                  siblings={siblings.length > 1 ? siblings : undefined}
                  onSettle={settle}
                  onDetails={(comparing) => setShownWindow({ kind: "finding", id: a.id, comparing })}
                />
              );
            })}
          </ul>
        )}
      </Card>

      {view === "open" &&
        !chosen &&
        (itemsFailed ? (
          <Card title={t("anomaly.learn.title")} hint={t("anomaly.learn.hint")} hueIndex={TILES.length + 1}>
            {/* A listing that never arrived says nothing about what is watched,
                so it must not look like an empty one. */}
            <div className="flex flex-col items-start gap-2">
              <p className="text-sm text-statusWarn">{t("anomaly.loadFailed")}</p>
              <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={retryItems} />
            </div>
          </Card>
        ) : (
          !itemsLoading &&
          items.length > 0 && (
            <Card title={t("anomaly.learn.title")} hint={t("anomaly.learn.hint")} hueIndex={TILES.length + 1}>
              <LearningRings
                items={items}
                t={t}
                onOpen={(item) => setShownWindow({ kind: "monitoring", targetId: item.targetId })}
              />
            </Card>
          )
        ))}

      <SensitivityHint t={t} />

      {windowFinding && shownWindow?.kind === "finding" && (
        <FindingWindow
          a={windowFinding}
          t={t}
          comparing={shownWindow.comparing}
          onCompare={() => setShownWindow({ ...shownWindow, comparing: true })}
          onMonitoring={
            byTarget.has(windowFinding.targetId)
              ? () => setShownWindow({ kind: "monitoring", targetId: windowFinding.targetId, from: windowFinding.id })
              : undefined
          }
          onSettle={(list, how) => {
            setShownWindow(null);
            return settle(list, how);
          }}
          onClose={() => setShownWindow(null)}
        />
      )}
      {monitored && shownWindow?.kind === "monitoring" && (
        <MonitoringWindow
          item={monitored}
          t={t}
          globals={globals}
          enabled={enabled}
          finding={curveFinding(monitored.targetId, shownWindow.from)}
          onBack={
            shownWindow.from
              ? () => setShownWindow({ kind: "finding", id: shownWindow.from!, comparing: false })
              : undefined
          }
          onClose={() => setShownWindow(null)}
        />
      )}
      {confirmDialog}
    </div>
  );
}

/**
 * SeverityTiles count the findings of the chosen view and choose which of
 * them the list shows. All shows everything; a severity adds or removes its
 * own findings, so two can be shown together. The glyph is a status readout
 * in its fixed hue; the tile itself is a control and takes its colour-engine
 * position like any other member of a set.
 */
function SeverityTiles({
  t,
  list,
  view,
  chosen,
  onChoose,
}: {
  t: T;
  list: AnomalyView[];
  view: View;
  chosen: AnomalySeverity[] | null;
  onChoose: (severity: AnomalySeverity | null) => void;
}) {
  return (
    <div
      role="group"
      aria-label={t("anomaly.filter.severity")}
      className="grid grid-cols-2 gap-2 rounded-card bg-carbon-surface p-3 md:grid-cols-4"
    >
      {TILES.map(({ severity, label, glyph }, i) => {
        const on = severity ? !!chosen?.includes(severity) : !chosen;
        const count = list.filter((a) => !severity || a.severity === severity).length;
        return (
          <button
            key={label}
            type="button"
            aria-pressed={on}
            onClick={() => onChoose(severity)}
            style={hueVars(i) as CSSProperties}
            className={`glim-hue${severity ? "" : " glim-hue-icon"}${
              on ? " glim-active bg-accent text-accentContrast" : " bg-carbon-surface2 text-carbon-text hover:bg-carbon-surface3"
            } grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-2.5 rounded-card px-3 py-2 text-start`}
          >
            <span className={`row-span-2 [&>svg]:h-[22px] [&>svg]:w-[22px] ${on ? "" : glyph}`}>
              <IconAnomalies />
            </span>
            <span className="truncate font-semibold">{t(label)}</span>
            <span className="truncate text-xs tabular-nums opacity-75">
              {t(view === "open" ? "anomaly.tile.open" : "anomaly.tile.closed", count)}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function SensitivityHint({ t }: { t: T }) {
  const [before, after] = t("anomaly.sensitivityWhere").split("{link}");
  return (
    <p className="text-xs text-carbon-textMuted">
      {before}
      <Link to={SETTINGS_PATH} className="underline hover:text-carbon-text">
        {t("nav.settings")} › {t("settings.tab.integrity")}
      </Link>
      {after}
    </p>
  );
}
