// The page of one backup entry, whatever its kind: what is backed up, how it
// is doing, and under three tabs its backups, its settings and the checks
// that say whether the backups are any good.
import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";

import { entryName } from "../components/backups/kinds";
import { Button } from "../components/Button";
import type { OpenBackup } from "../components/entry/BackupList";
import { BackupsTab } from "../components/entry/BackupsTab";
import { EntryCorner, EntryMenu } from "../components/entry/EntryActions";
import { EntryHead } from "../components/entry/EntryHead";
import { KindHead } from "../components/entry/KindHead";
import { Note } from "../components/entry/Note";
import { SettingsTab } from "../components/entry/SettingsTab";
import { VerifyTab } from "../components/entry/VerifyTab";
import { IconBack } from "../components/glyphs";
import { IconDatabase, IconGear, IconTabIntegrity } from "../components/navGlyphs";
import { PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { RenameTakeoverRow } from "../components/RenameTakeoverRow";
import { Selector } from "../components/Selector";
import type { PlacementView, Run, TimelineRow } from "../lib/api";
import { ENTRY_KIND, ENTRY_LIST_PATH, entryListPath, isEntryKind, type EntryKind } from "../lib/backupEntry";
import { isGone, itemProgressKey } from "../lib/backupList";
import { ENTRY_TABS, entryTabPath, parseEntryTab, type Entry, type EntryTab } from "../lib/entry";
import { useT, type TranslationKey } from "../lib/i18n";
import { readRestoreRequest } from "../lib/restoreRequest";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useProgress } from "../lib/progress";
import { RunReasonText } from "../lib/runReason";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useEntry } from "../lib/useEntry";
import { useTimeline } from "../lib/useTimeline";
import { hueCounter } from "./settings/shared";
import { containerTakeover, vmTakeover } from "../lib/useTakeOver";

const TAB_LABEL: Record<EntryTab, TranslationKey> = {
  backups: "backups.title",
  settings: "nav.settings",
  verify: "entry.tab.verify",
};

const TAB_GLYPH: Record<EntryTab, () => React.ReactNode> = {
  backups: IconDatabase,
  settings: IconGear,
  verify: IconTabIntegrity,
};

function Crumb() {
  const { t } = useT();
  return (
    <Link
      to={ENTRY_LIST_PATH}
      className="inline-flex items-center gap-1.5 self-start rounded-control text-sm text-carbon-textSub hover:text-carbon-text"
    >
      <IconBack />
      {t("backups.title")}
    </Link>
  );
}

/** The look-alike a rename leaves behind, offered for takeover. Containers
 *  and VMs are the kinds the host can rename under BombVault. */
function TakeoverNote({ entry, onDone }: { entry: Entry; onDone: () => void }) {
  const { t } = useT();
  const { detail } = entry;
  if (isGone(entry.item)) return null;
  if (detail.kind === "container" && !detail.container.self && detail.container.renameFrom) {
    const c = detail.container;
    return (
      <RenameTakeoverRow
        key={c.renameFrom}
        from={c.renameFrom ?? ""}
        reason={c.renameReason ?? ""}
        entry={{ name: c.name, displayName: c.name, api: containerTakeover }}
        onDone={onDone}
        t={t}
      />
    );
  }
  if (detail.kind === "vm" && detail.vm.renameFrom) {
    const vm = detail.vm;
    return (
      <RenameTakeoverRow
        key={vm.renameFrom}
        from={vm.renameFrom ?? ""}
        reason={vm.renameReason ?? ""}
        entry={{ name: vm.libvirtName, displayName: vm.name, api: vmTakeover }}
        onDone={onDone}
        t={t}
      />
    );
  }
  return null;
}

function Loaded({
  entry,
  runs,
  reload,
  setPlacement,
}: {
  entry: Entry;
  runs: Run[];
  reload: () => void;
  setPlacement: (placement: PlacementView) => void;
}) {
  const { t } = useT();
  const location = useLocation();
  const navigate = useNavigate();
  const { item, detail } = entry;
  const tab = parseEntryTab(location.search);
  const name = entryName(item, t);
  const progress = useProgress()[itemProgressKey(item)];
  const anomaly = useAnomalyItems().find(item.kind, item.key);
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const gone = isGone(item);
  const self = detail.kind === "container" && detail.container.self === true;
  const lastFailure =
    item.lastRunStatus === "failed" ? runs.find((run) => run.kind === "backup" && run.status === "failed") : undefined;
  // A finding's restore link is read once. It stays in the address, and the
  // backup it opened must not open again with every change of tab.
  const [request] = useState(() => readRestoreRequest(location.search));
  const nextHue = hueCounter();

  // Read here and not in the tab, so the head can open the newest backup and
  // a change of tab reads nothing again.
  const domain = ENTRY_KIND[item.kind].timelineDomain;
  const read = useTimeline(domain, item.key, !self);
  const timeline = { ...read, domain };
  const reloadTimeline = read.reload;
  const [opened, setOpened] = useState<OpenBackup | null>(
    request.snapshot && !request.dump ? { key: request.snapshot } : null
  );

  const active = progress?.active === true;
  const wasActive = useRef(false);
  useEffect(() => {
    if (wasActive.current && !active) reloadTimeline();
    wasActive.current = active;
  }, [active, reloadTimeline]);

  const tabPath = (next: EntryTab) => entryTabPath(item.kind, item.key, next, location.search);
  const newest: TimelineRow | undefined = read.rows[0];
  const openNewest = newest
    ? () => {
        if (tab !== "backups") navigate(tabPath("backups"), { replace: true });
        setOpened({ key: newest.key });
      }
    : undefined;

  return (
    <>
      <PageTitle>{name}</PageTitle>
      <div className="flex flex-col gap-3">
        <Crumb />
        <EntryHead
          entry={entry}
          name={name}
          progress={progress}
          anomaly={anomaly}
          anomalyEnabled={anomalyEnabled}
          actions={
            <EntryMenu
              entry={entry}
              name={name}
              timeline={timeline}
              onChanged={reload}
              onRemoved={() => navigate(entryListPath(item.kind), { replace: true })}
              onOpenNewest={openNewest}
            />
          }
        />
        {self && <Note>{t("backups.selfNote")}</Note>}
        {!gone && item.lastRunStatus === "failed" && (
          <Note tone="fail">
            {lastFailure?.error && (
              <>
                <RunReasonText reason={lastFailure.error} t={t} />{" "}
              </>
            )}
            {t("entry.retryNote")}
          </Note>
        )}
        <TakeoverNote entry={entry} onDone={reload} />
        {gone && <Note>{t(item.kind === "vm" ? "entry.gone.vm" : "entry.gone.container")}</Note>}
        {!self && <KindHead entry={entry} tab={tab} onChanged={reload} />}
        {!self && (
          <div data-new-way="entry-tabs" className="mt-2 max-w-xl">
            <Selector
              items={ENTRY_TABS.map((id) => {
                const Glyph = TAB_GLYPH[id];
                return { id, label: t(TAB_LABEL[id]), icon: <Glyph /> };
              })}
              label={name}
              select="one"
              size="lg"
              fit
              hueOffset={0}
              active={tab}
              // Replaced, so Back leaves the entry instead of walking its tabs.
              onChange={(id) => navigate(tabPath(id as EntryTab), { replace: true })}
            />
          </div>
        )}
      </div>

      {!self && tab === "backups" && (
        <BackupsTab
          entry={entry}
          name={name}
          runs={runs}
          timeline={timeline}
          opened={opened}
          onOpen={setOpened}
          request={request}
          nextHue={nextHue}
        />
      )}

      {!self && tab === "settings" && (
        <SettingsTab
          entry={entry}
          name={name}
          anomaly={anomaly}
          onChanged={reload}
          onPlacement={setPlacement}
          nextHue={nextHue}
        />
      )}

      {!self && tab === "verify" && (
        <VerifyTab
          entry={entry}
          runs={runs}
          anomaly={anomaly}
          anomalyEnabled={anomalyEnabled}
          onRestore={(run) => {
            navigate(tabPath("backups"), { replace: true });
            setOpened({ key: run.snapshotId });
          }}
          nextHue={nextHue}
        />
      )}

      {!self && (
        <PageActions>
          <EntryCorner entry={entry} onChanged={reload} onRestoreNewest={openNewest} />
        </PageActions>
      )}
    </>
  );
}

function EntryView({ kind, entryKey }: { kind: EntryKind; entryKey: string }) {
  const { t } = useT();
  const { entry, runs, status, reload, setPlacement } = useEntry(kind, entryKey);

  if (entry) return <Loaded entry={entry} runs={runs} reload={() => void reload()} setPlacement={setPlacement} />;
  return (
    <>
      <PageTitle>{t("backups.title")}</PageTitle>
      <Crumb />
      {status === "missing" ? (
        <p className="text-sm text-carbon-textSub">{t("entry.missing")}</p>
      ) : status === "failed" ? (
        <div className="flex flex-col items-start gap-2">
          <p className="text-sm text-statusWarn">{t("backups.loadFailed")}</p>
          <Button label={t("anomaly.retry")} labelKey="anomaly.retry" onClick={() => void reload()} />
        </div>
      ) : (
        <p className="text-sm text-carbon-textSub">{t("dashboard.checking")}</p>
      )}
    </>
  );
}

export function BackupEntryPage() {
  const { t } = useT();
  const { kind = "", id = "" } = useParams();
  const known = isEntryKind(kind) && (ENTRY_KIND[kind].keyedBy !== "kind" || id === kind);
  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      {known ? (
        // Keyed on the entry, so one entry's drafts and open windows never show on another.
        <EntryView key={`${kind}:${id}`} kind={kind} entryKey={id} />
      ) : (
        <>
          <PageTitle>{t("backups.title")}</PageTitle>
          <Crumb />
          <p className="text-sm text-carbon-textSub">{t("entry.missing")}</p>
        </>
      )}
    </div>
  );
}
