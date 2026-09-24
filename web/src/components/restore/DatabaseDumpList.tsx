// The database dumps of one container, under its snapshot list. A dump is a
// full SQL export taken from the running server, so it is the copy to reach for
// when the container's files cannot be used; the row says which server and
// which version it came from, and offers the four things that can be done with
// it.

import { useEffect, useRef, useState } from "react";
import {
  checkDbDumpDownload,
  dbDumpDownloadURL,
  deleteSnapshot,
  importDbDump,
  listDbDumps,
  saveDbDumpTo,
  type DBDumpView,
} from "../../lib/api";
import { Advanced } from "../../lib/advanced";
import { useBackupWatch } from "../../lib/backupWatch";
import { ENGINE_NAMES, importRefusedKey } from "../../lib/dbdump";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { useProgress } from "../../lib/progress";
import { isWarningNote, RunReasonText } from "../../lib/runReason";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { InfoBubble } from "../InfoBubble";
import { SelectField } from "../SelectField";
import { IconDatabase, IconDownload, IconRestore, IconTrash } from "../navGlyphs";
import { RestoreProgress } from "./RestoreProgress";

type T = ReturnType<typeof useT>["t"];

/** Plain SQL or gzipped, kept per viewer so the choice survives the panel. */
type DumpFormat = "sql" | "gz";
const FORMAT_STORAGE_KEY = "bombvault.dbdump.format";

// The browser owns the download once the anchor is clicked, so the button only
// shows that something started. Long enough for a large dump to begin streaming.
const DOWNLOAD_PREPARING_MS = 20_000;

function storedFormat(): DumpFormat {
  try {
    return localStorage.getItem(FORMAT_STORAGE_KEY) === "gz" ? "gz" : "sql";
  } catch {
    return "sql";
  }
}

/** The database names as this language joins a list, each one isolated so a
 *  right-to-left sentence does not reorder it. */
function DatabaseNames({ names, lang }: { names: string[]; lang: string }) {
  const parts = new Intl.ListFormat(lang, { style: "long", type: "conjunction" }).formatToParts(names);
  return (
    <>
      {parts.map((part, i) =>
        part.type === "element" ? <bdi key={i}>{part.value}</bdi> : <span key={i}>{part.value}</span>
      )}
    </>
  );
}

function FormatPicker({
  format,
  onChange,
  t,
}: {
  format: DumpFormat;
  onChange: (next: DumpFormat) => void;
  t: T;
}) {
  return (
    <SelectField
      value={format}
      onChange={onChange}
      label={t("dbdump.format")}
      options={[
        { value: "sql", label: t("dbdump.formatSql") },
        { value: "gz", label: t("dbdump.formatGz") },
      ]}
      className="rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 glim-field-focus-well"
    />
  );
}

function DumpRow({
  dump,
  containerName,
  source,
  canImport,
  importStops,
  hostMountRoot,
  defaultFolder,
  format,
  onFormat,
  onChanged,
  flagged,
  t,
}: {
  dump: DBDumpView;
  containerName: string;
  source: string;
  canImport: boolean;
  importStops: string[];
  hostMountRoot: string;
  defaultFolder: string;
  format: DumpFormat;
  onFormat: (next: DumpFormat) => void;
  onChanged: () => void;
  /** An open finding says this dump lost most of the database. */
  flagged: boolean;
  t: T;
}) {
  const { lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [preparing, setPreparing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [shake, setShake] = useState(0);
  const [saveOpen, setSaveOpen] = useState(false);
  const [folder, setFolder] = useState(defaultFolder);
  const [savedPath, setSavedPath] = useState("");

  const progressMap = useProgress();
  const prog = progressMap[`container:${containerName}`];

  const saveCancelled = useRef(false);
  const { state: saveState, fire: fireSave, isPending: saving } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "dbdumpsave",
    start: async () => {
      const path = folder.trim();
      const res = await saveDbDumpTo(containerName, dump.id, path, format === "gz", source);
      if (res.ok) setSavedPath(res.target ?? path);
      return res;
    },
    matchRun: (r) => r.domain === "container" && r.target === containerName,
    cancelledRef: saveCancelled,
  });

  const importCancelled = useRef(false);
  const { state: importState, fire: fireImport, isPending: importing } = useBackupWatch({
    progressKey: `container:${containerName}`,
    kind: "dbimport",
    start: async () => {
      const res = await importDbDump(containerName, dump.id, source);
      if (res.ok) return res;
      const key = importRefusedKey(res.code, dump.engine);
      if (!key) return res;
      setShake((n) => n + 1);
      return {
        ...res,
        error: t(key).replace("{server}", res.server ?? "").replace("{dump}", res.dump ?? ""),
      };
    },
    matchRun: (r) => r.domain === "container" && r.target === containerName,
    cancelledRef: importCancelled,
  });

  const importNote = importState.phase === "success" ? importState.note : undefined;
  const engineName = dump.engine ? ENGINE_NAMES[dump.engine] : "";
  // Without the engine the import has no tool to pick and the server refuses
  // it, so the row offers everything else and says only that this is a database.
  const importable = canImport && dump.engine !== "" && !dump.damaged;

  async function handleDownload() {
    try {
      const res = await checkDbDumpDownload(containerName, dump.id, source);
      if (!res.ok) {
        push(`${t("dbdump.downloadRefused")}: ${res.error ?? ""}`.trim(), "fail");
        setShake((n) => n + 1);
        return;
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("dbdump.downloadRefused"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setPreparing(true);
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = dbDumpDownloadURL(containerName, dump.id, source, format === "gz");
    a.download = `${containerName}-${dump.id.slice(0, 8)}.sql`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  async function handleImport() {
    const question = dump.version
      ? t("dbdump.importConfirm").replace("{version}", dump.version)
      : `${t("dbdump.importConfirmNoVersion")} ${t("dbdump.importNoVersion")}`;
    let asked = question.replace("{engine}", engineName).replace("{container}", containerName);
    if (importStops.length > 0) {
      asked += " " + t("dbdump.importStops", importStops.length).replace("{apps}", importStops.join(", "));
    }
    if (!(await confirm(asked, { confirmKey: "dbdump.import" }))) return;
    void fireImport();
  }

  async function handleDelete() {
    if (!(await confirm(t("dbdump.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("containers", dump.id, source);
      if (res.ok) onChanged();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm flex-wrap">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">
          {dump.id.slice(0, 8)}
        </span>
        <span className="text-carbon-textMuted text-xs">{new Date(dump.time).toLocaleString()}</span>
        <Badge tone={dump.engine ? "neutral" : "muted"} size="small">
          {engineName || t("dbdump.engineUnknown")}
        </Badge>
        {engineName && dump.version && (
          <span className="text-xs text-carbon-textSub">
            {t("dbdump.versionLabel").replace("{engine}", engineName).replace("{version}", dump.version)}
          </span>
        )}
        {flagged && (
          <Badge tone="fail" size="small">
            {t("anomaly.snapshotFlagged")}
          </Badge>
        )}
        <span className="text-xs text-carbon-textMuted flex-1">{humanBytes(dump.bytes)}</span>

        {dump.damaged ? (
          <span className="flex items-center gap-1.5">
            <Badge tone="warn" size="small">
              {t("dbdump.damagedBadge")}
            </Badge>
            <InfoBubble tip={t("dbdump.damagedHint")} />
          </span>
        ) : (
          <>
            <Advanced>
              <FormatPicker format={format} onChange={onFormat} t={t} />
            </Advanced>
            <Button
              label={t("dbdump.download")}
              labelKey="dbdump.download"
              glyph={<IconDownload />}
              tone="accent"
              onClick={() => void handleDownload()}
              disabled={preparing}
              busy={preparing}
              className="shrink-0"
            />
            <Advanced>
              <Button
                label={t("dbdump.saveToFolder")}
                labelKey="dbdump.saveToFolder"
                glyph={<IconRestore />}
                tone="accent"
                onClick={() => setSaveOpen((p) => !p)}
                disabled={saving}
                busy={saving}
                title={saving ? t("dbdump.busySaving") : undefined}
                className="shrink-0"
              />
            </Advanced>
            {importable && (
              <Button
                key={shake}
                label={t("dbdump.import")}
                labelKey="dbdump.import"
                glyph={<IconDatabase />}
                tone="accent"
                onClick={() => void handleImport()}
                disabled={importing}
                busy={importing}
                title={importing ? t("dbdump.busyImporting") : undefined}
                className={`shrink-0${shake ? " glim-shake" : ""}`}
              />
            )}
          </>
        )}

        <Button
          label={t("snapshots.delete")}
          labelKey="snapshots.delete"
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting}
          className="shrink-0"
        />
      </div>

      {dump.databases.length > 0 && (
        <p className="text-caption text-carbon-textMuted">
          {t("dbdump.databasesLine").split("{names}")[0]}
          <DatabaseNames names={dump.databases} lang={lang} />
          {t("dbdump.databasesLine").split("{names}")[1]}
        </p>
      )}
      {dump.image && (
        <p dir="ltr" className="hidden sm:block text-caption text-carbon-textMuted text-start font-mono">
          {dump.image}
        </p>
      )}

      {saveOpen && !dump.damaged && (
        <div className="mt-1 rounded-card bg-carbon-background p-2 flex flex-col gap-1.5">
          <p className="text-caption text-carbon-textMuted">{t("dbdump.saveHint")}</p>
          <FormatPicker format={format} onChange={onFormat} t={t} />
          <FolderBrowser
            label={t("restore.targetPath")}
            value={folder}
            hostMountRoot={hostMountRoot}
            onChange={setFolder}
          />
          <Button
            label={t("common.confirm")}
            labelKey="common.confirm"
            tone="accent"
            onClick={() => void fireSave()}
            disabled={!folder.trim() || saving}
            busy={saving}
            title={saving ? t("dbdump.busySaving") : undefined}
            className="self-start"
          />
          <RestoreProgress
            state={saveState}
            isPending={saving}
            prog={prog}
            cancelKey={`container:${containerName}`}
            inPlace={false}
            name={containerName}
            cancelledRef={saveCancelled}
            cancelConfirm={t("dbdump.saveCancelHint")}
            successMessage={t("dbdump.savedTo").replace("{path}", savedPath)}
            showStartedHint={false}
            t={t}
          />
        </div>
      )}

      {(importing || importState.phase !== "idle") && (
        <RestoreProgress
          state={importState}
          isPending={importing}
          prog={prog}
          cancelKey={`container:${containerName}`}
          inPlace
          name={containerName}
          cancelledRef={importCancelled}
          cancellable={false}
          successMessage={importNote ? <RunReasonText reason={importNote} t={t} /> : t("dbdump.importDone")}
          successWarn={isWarningNote(importNote)}
          showStartedHint={false}
          t={t}
        />
      )}
      {confirmDialog}
    </div>
  );
}

export function DatabaseDumpList({
  containerName,
  source,
  recognised,
  canImport,
  importStops = [],
  hostMountRoot,
  defaultFolder,
  reloadTick,
  onDumps,
  t,
}: {
  containerName: string;
  source: string;
  /** The container is a database BombVault dumps, so an empty list is news. */
  recognised: boolean;
  /** Installed and running: the import needs the server it writes into. */
  canImport: boolean;
  /** The running apps the import stops while it runs, from the container's
   *  stop list. */
  importStops?: string[];
  hostMountRoot: string;
  defaultFolder: string;
  /** Bumped by the panel around it when something changed the repository. */
  reloadTick: number;
  /** Hands the loaded dumps to the panel, whose snapshot rows mark the one
   *  taken in the same backup. */
  onDumps?: (dumps: DBDumpView[]) => void;
  t: T;
}) {
  const [dumps, setDumps] = useState<DBDumpView[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [ownTick, setOwnTick] = useState(0);
  const [format, setFormat] = useState<DumpFormat>(storedFormat);
  const { flagged } = useOpenAnomalies();
  const onDumpsRef = useRef(onDumps);
  onDumpsRef.current = onDumps;

  useEffect(() => {
    let current = true;
    const fail = () => {
      if (!current) return;
      setDumps([]);
      onDumpsRef.current?.([]);
      setFailed(true);
    };
    setLoading(true);
    setFailed(false);
    listDbDumps(containerName, source)
      .then((res) => {
        if (!current) return;
        if (res.ok) {
          setDumps(res.dumps ?? []);
          onDumpsRef.current?.(res.dumps ?? []);
        } else fail();
      })
      .catch(fail)
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
  }, [containerName, source, reloadTick, ownTick]);

  function pickFormat(next: DumpFormat) {
    setFormat(next);
    try {
      localStorage.setItem(FORMAT_STORAGE_KEY, next);
    } catch {
      // Without storage the choice lasts as long as the panel is open.
    }
  }

  // A container nobody dumps says nothing about dumps, even while the list is
  // still loading.
  if (!recognised && dumps.length === 0) return null;
  if (loading) return null;

  return (
    <div className="py-2 border-t border-carbon-border">
      <div className="flex items-center gap-1.5 pb-1">
        <span className="text-xs font-medium text-carbon-textSub">{t("dbdump.listTitle")}</span>
        <InfoBubble tip={t("dbdump.listHint")} />
      </div>
      {failed && <p className="text-xs text-statusFail">{t("dbdump.loadFailed")}</p>}
      {!failed && dumps.length === 0 && (
        <p className="text-xs text-carbon-textMuted">{t("dbdump.none")}</p>
      )}
      {dumps.map((dump) => (
        <DumpRow
          key={dump.id}
          dump={dump}
          containerName={containerName}
          source={source}
          canImport={canImport}
          importStops={importStops}
          hostMountRoot={hostMountRoot}
          defaultFolder={defaultFolder}
          format={format}
          onFormat={pickFormat}
          onChanged={() => setOwnTick((n) => n + 1)}
          flagged={flagged.has(dump.id)}
          t={t}
        />
      ))}
    </div>
  );
}
