// The page of one backup entry, whatever its kind: what is backed up, how it
// is doing, and under three tabs its backups, its settings and the checks
// that say whether the backups are any good.
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";

import { entryName } from "../components/backups/kinds";
import { Button } from "../components/Button";
import { EntryCorner, EntryHeadActions } from "../components/entry/EntryActions";
import { EntryHead } from "../components/entry/EntryHead";
import { KindHead } from "../components/entry/KindHead";
import { Note } from "../components/entry/Note";
import { IconBack } from "../components/glyphs";
import { IconDatabase, IconGear, IconTabIntegrity } from "../components/navGlyphs";
import { PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { RenameTakeoverRow } from "../components/RenameTakeoverRow";
import { Selector } from "../components/Selector";
import type { Run } from "../lib/api";
import { ENTRY_KIND, ENTRY_LIST_PATH, entryListPath, isEntryKind, type EntryKind } from "../lib/backupEntry";
import { isGone, itemProgressKey } from "../lib/backupList";
import { ENTRY_TABS, entryTabPath, parseEntryTab, type Entry, type EntryTab } from "../lib/entry";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { useProgress } from "../lib/progress";
import { RunReasonText } from "../lib/runReason";
import { useAnomalyItems, useAnomalySummary } from "../lib/useAnomalies";
import { useEntry } from "../lib/useEntry";
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
}: {
  entry: Entry;
  runs: Run[];
  reload: () => void;
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
  const lastFailure = item.lastRunStatus === "failed" ? runs.find((run) => run.kind === "backup" && run.status === "failed") : undefined;

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
            <EntryHeadActions entry={entry} onChanged={reload} onRemoved={() => navigate(entryListPath(item.kind), { replace: true })} />
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
          <div className="mt-2 flex">
            <Selector
              items={ENTRY_TABS.map((id) => {
                const Glyph = TAB_GLYPH[id];
                return { id, label: t(TAB_LABEL[id]), icon: <Glyph /> };
              })}
              label={name}
              select="one"
              size="lg"
              inline
              active={tab}
              // Replaced, so Back leaves the entry instead of walking its tabs.
              onChange={(id) =>
                navigate(entryTabPath(item.kind, item.key, id as EntryTab, location.search), { replace: true })
              }
            />
          </div>
        )}
      </div>

      {!self && (
        <PageActions>
          <EntryCorner entry={entry} onChanged={reload} />
        </PageActions>
      )}
    </>
  );
}

function EntryView({ kind, entryKey }: { kind: EntryKind; entryKey: string }) {
  const { t } = useT();
  const { entry, runs, status, reload } = useEntry(kind, entryKey);

  if (entry) return <Loaded entry={entry} runs={runs} reload={() => void reload()} />;
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
