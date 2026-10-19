import { useEffect, useRef, useState } from "react";
import { importAppdataBackup, scanAppdataBackup, type AppdataBackupArchive } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconSearch } from "../glyphs";
import { IconCheck, IconDownload } from "../navGlyphs";
import { ProgressBar } from "../ProgressBar";
import { Rows, SettingRow } from "../storage/rows";
import { StepBar } from "./StepBar";

/** The progress key the server publishes an import under. */
export const APPDATA_IMPORT_KEY = "import:containers";

const STATUS: Record<AppdataBackupArchive["status"], { key: TranslationKey; tone: "ok" | "neutral" | "warn" }> = {
  new: { key: "recovery.abStatusNew", tone: "ok" },
  imported: { key: "recovery.abStatusImported", tone: "neutral" },
  "no-container": { key: "recovery.abStatusNoContainer", tone: "warn" },
  "not-backed-up": { key: "recovery.abStatusNotBackedUp", tone: "warn" },
  "repo-unavailable": { key: "recovery.abStatusRepoUnavailable", tone: "warn" },
};

const OUTCOME: Record<"done" | "some" | "all", { key: TranslationKey; look: string }> = {
  done: { key: "recovery.abDone", look: "bg-statusOkBg text-statusOk" },
  some: { key: "recovery.abSomeFailed", look: "bg-statusWarnBg text-statusWarn" },
  all: { key: "recovery.abAllFailed", look: "bg-statusFailBgSoft text-statusFail" },
};

/**
 * AppdataWindow turns the archives of the Appdata.Backup plugin into restore
 * points. Looking at a folder writes nothing. The import runs on the server
 * and reports through the shared progress stream, so it carries on when the
 * window is closed.
 */
export function AppdataWindow({ hostMountRoot, onClose }: { hostMountRoot: string; onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [folder, setFolder] = useState("");
  const [archives, setArchives] = useState<AppdataBackupArchive[] | null>(null);
  const [scanning, setScanning] = useState(false);
  const [starting, setStarting] = useState(false);
  const [outcome, setOutcome] = useState<keyof typeof OUTCOME | null>(null);
  const prog = useProgress()[APPDATA_IMPORT_KEY];
  const running = prog?.active === true;
  const wasRunning = useRef(false);
  // How many archives the import set out with, unknown when it was started
  // before the window opened, and how many of them failed.
  const startedWith = useRef<number | null>(null);
  const failed = useRef(0);
  // Counted up by every scan and every change of the folder, so the answer
  // for a folder that was changed since is dropped.
  const scanSeq = useRef(0);

  async function scan(path = folder) {
    if (path.trim() === "") return;
    const seq = ++scanSeq.current;
    setScanning(true);
    try {
      const r = await scanAppdataBackup(path.trim());
      if (seq !== scanSeq.current) return;
      if (r.ok) setArchives(r.archives ?? []);
      else {
        setArchives(null);
        push(r.error ?? t("recovery.abScanFailed"), "fail");
      }
    } catch (err) {
      if (seq === scanSeq.current) push(err instanceof Error ? err.message : t("recovery.abScanFailed"), "fail");
    } finally {
      if (seq === scanSeq.current) setScanning(false);
    }
  }

  // Only the frame that ends an import carries the count of failed archives,
  // and it lingers a moment before the entry goes.
  useEffect(() => {
    if (prog?.finished) failed.current = prog.failed ?? 0;
  }, [prog]);

  // The list is read again once an import ends, so the rows say what it did.
  useEffect(() => {
    if (running) wasRunning.current = true;
    else if (wasRunning.current) {
      wasRunning.current = false;
      const total = startedWith.current;
      setOutcome(failed.current === 0 ? "done" : total !== null && failed.current >= total ? "all" : "some");
      startedWith.current = null;
      failed.current = 0;
      void scan();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running]);

  async function startImport() {
    setStarting(true);
    setOutcome(null);
    try {
      const r = await importAppdataBackup(folder.trim());
      if (r.ok) startedWith.current = r.archives ?? null;
      else push(r.error ?? t("recovery.abImportFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("recovery.abImportFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  function changeFolder(next: string) {
    setFolder(next);
    scanSeq.current++;
    setScanning(false);
  }

  const fresh = (archives ?? []).filter((a) => a.status === "new").length;
  const listed = archives !== null;

  return (
    <InstanceWindow
      title={t("recovery.abTitle")}
      hint={t("recovery.abIntro")}
      onClose={onClose}
      footer={
        !listed ? (
          <>
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
            <Button
              label={t("recovery.abScan")}
              labelKey="recovery.abScan"
              glyph={<IconSearch />}
              tone="accent"
              onClick={() => void scan()}
              disabled={folder.trim() === "" || scanning}
              busy={scanning}
            />
          </>
        ) : (
          <>
            <Button
              label={t("common.back")}
              labelKey="common.back"
              glyph={<IconBack />}
              tone="subtle"
              onClick={() => setArchives(null)}
              disabled={running}
            />
            {outcome === "done" && fresh === 0 ? (
              <Button label={t("common.done")} labelKey="common.done" glyph={<IconCheck />} tone="accent" onClick={onClose} />
            ) : (
              <Button
                label={t("recovery.abImport")}
                labelKey="recovery.abImport"
                glyph={<IconDownload />}
                tone="accent"
                onClick={() => void startImport()}
                disabled={fresh === 0 || running || starting}
                busy={running || starting}
                title={running ? t("recovery.abImporting") : undefined}
              />
            )}
          </>
        )
      }
    >
      <StepBar
        steps={[t("appdata.stepFolder"), t("recovery.abStepImport")]}
        current={listed ? 2 : 1}
        reach={listed && !running ? 2 : listed ? 0 : 1}
        label={t("recovery.abTitle")}
        onGo={(n) => {
          if (n === 1) setArchives(null);
        }}
      />

      {!listed && (
        <FolderBrowser
          label={t("recovery.abFolder")}
          value={folder}
          hostMountRoot={hostMountRoot}
          onChange={changeFolder}
          hint={t("recovery.abFolderHint")}
        />
      )}

      {listed && (
        <>
          <p className="text-sm text-carbon-text">
            {archives.length === 0
              ? t("recovery.abNothing")
              : t("appdata.found", archives.length).replace("{fresh}", String(fresh))}
          </p>
          {prog && <ProgressBar percent={prog.percent} active={prog.active} inline label={running ? t("recovery.abImporting") : undefined} />}
          {outcome && <p className={`rounded-card px-3 py-2.5 text-sm leading-relaxed ${OUTCOME[outcome].look}`}>{t(OUTCOME[outcome].key)}</p>}
          {archives.length > 0 && (
            <Rows as="ul">
              {archives.map((a) => (
                <SettingRow
                  key={`${a.folder}/${a.file}`}
                  as="li"
                  label={a.container}
                  name={
                    <bdi dir="ltr" className="min-w-0 wrap-break-word">
                      {a.container}
                    </bdi>
                  }
                  note={
                    <span className="tabular-nums">
                      {new Date(a.time * 1000).toLocaleString()} · {humanBytes(a.size)}
                    </span>
                  }
                >
                  <Badge tone={STATUS[a.status].tone} size="small">
                    {t(STATUS[a.status].key)}
                  </Badge>
                </SettingRow>
              ))}
            </Rows>
          )}
        </>
      )}
    </InstanceWindow>
  );
}
