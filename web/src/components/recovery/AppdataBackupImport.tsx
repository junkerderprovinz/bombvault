import { useEffect, useRef, useState } from "react";
import { importAppdataBackup, scanAppdataBackup, type AppdataBackupArchive } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import type { TranslationKey, useT } from "../../lib/i18n";
import { useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { InfoBubble } from "../InfoBubble";
import { ProgressBar } from "../ProgressBar";
import { IconDownload } from "../Sidebar";
import { StepCard } from "./StepCard";

type T = ReturnType<typeof useT>["t"];

/** The progress key the server publishes an import under. */
export const APPDATA_IMPORT_KEY = "import:containers";

const STATUS: Record<AppdataBackupArchive["status"], { key: TranslationKey; tone: "ok" | "neutral" | "warn" }> = {
  new: { key: "recovery.abStatusNew", tone: "ok" },
  imported: { key: "recovery.abStatusImported", tone: "neutral" },
  "no-container": { key: "recovery.abStatusNoContainer", tone: "warn" },
  "not-backed-up": { key: "recovery.abStatusNotBackedUp", tone: "warn" },
  "repo-unavailable": { key: "recovery.abStatusRepoUnavailable", tone: "warn" },
};

// AppdataBackupImport turns the archives of the Appdata.Backup plugin into
// restore points. Looking at a folder writes nothing; the import runs on the
// server and reports through the shared progress stream.
export function AppdataBackupImport({ hostMountRoot, nextHue, t }: { hostMountRoot: string; nextHue: () => number; t: T }) {
  const { push } = useToast();
  const [folder, setFolder] = useState("");
  const [archives, setArchives] = useState<AppdataBackupArchive[] | null>(null);
  const [scanning, setScanning] = useState(false);
  const [starting, setStarting] = useState(false);
  const [shake, setShake] = useState(0);
  const prog = useProgress()[APPDATA_IMPORT_KEY];
  const running = prog?.active === true;
  const wasRunning = useRef(false);
  // How many archives the import set out with, unknown when it was started
  // before this page was opened, and how many of them failed.
  const startedWith = useRef<number | null>(null);
  const failed = useRef(0);
  // Bumped by every scan and every change of the folder, so an answer for a
  // folder the user has since changed is dropped.
  const scanSeq = useRef(0);
  const hues = { heading: nextHue(), folder: nextHue(), list: nextHue() };

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
        setShake((n) => n + 1);
      }
    } catch (err) {
      if (seq !== scanSeq.current) return;
      push(err instanceof Error ? err.message : t("recovery.abScanFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      if (seq === scanSeq.current) setScanning(false);
    }
  }

  // The frame that ends an import lingers a moment before the entry goes, and
  // only it carries the count of failed archives.
  useEffect(() => {
    if (prog?.finished) failed.current = prog.failed ?? 0;
  }, [prog]);

  // The list is read again once an import ends, so the rows say what it did.
  useEffect(() => {
    if (running) wasRunning.current = true;
    else if (wasRunning.current) {
      wasRunning.current = false;
      const total = startedWith.current;
      if (failed.current === 0) push(t("recovery.abDone"), "success");
      else if (total !== null && failed.current >= total) push(t("recovery.abAllFailed"), "fail");
      else push(t("recovery.abSomeFailed"), "warn");
      startedWith.current = null;
      failed.current = 0;
      void scan();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running]);

  async function startImport() {
    setStarting(true);
    try {
      const r = await importAppdataBackup(folder.trim());
      if (r.ok) startedWith.current = r.archives ?? null;
      else {
        push(r.error ?? t("recovery.abImportFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("recovery.abImportFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setStarting(false);
    }
  }

  const hasNew = (archives ?? []).some((a) => a.status === "new");

  return (
    <div className="flex flex-col gap-10 border-t border-carbon-border pt-10">
      <h2 className="relative flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hues.heading}>
          {t("recovery.abTitle")}
          <InfoBubble tip={t("recovery.abIntro")} onAccent />
        </Badge>
      </h2>

      <StepCard n={1} title={t("recovery.abStepFolder")} state={archives === null ? "idle" : "ok"} hueIndex={hues.folder}>
        <FolderBrowser
          label={t("recovery.abFolder")}
          value={folder}
          hostMountRoot={hostMountRoot}
          onChange={(v) => {
            setFolder(v);
            setArchives(null);
            scanSeq.current++;
            setScanning(false);
          }}
          hint={t("recovery.abFolderHint")}
        />
        <div className="flex justify-end">
          <Button
            key={shake}
            label={t("recovery.abScan")}
            labelKey="recovery.abScan"
            tone="accent"
            onClick={() => void scan()}
            disabled={folder.trim() === "" || scanning}
            busy={scanning}
            className={shake ? "glim-shake" : ""}
          />
        </div>
      </StepCard>

      {archives !== null && (
        <StepCard n={2} title={t("recovery.abStepImport")} state={hasNew ? "idle" : "ok"} hueIndex={hues.list}>
          {archives.length === 0 ? (
            <p className="text-xs text-carbon-textMuted">{t("recovery.abNothing")}</p>
          ) : (
            <ul className="flex flex-col rounded-card bg-carbon-background px-3 py-1">
              {archives.map((a) => (
                <li key={`${a.folder}/${a.file}`} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-carbon-border py-1.5 text-xs last:border-0">
                  <span className="w-full shrink-0 text-carbon-textMuted sm:w-36">{new Date(a.time * 1000).toLocaleString()}</span>
                  <span dir="ltr" className="min-w-0 flex-1 truncate text-start font-mono text-carbon-text">{a.container}</span>
                  <span className="text-carbon-textSub">{humanBytes(a.size)}</span>
                  <Badge tone={STATUS[a.status].tone} size="small">
                    {t(STATUS[a.status].key)}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
          <div className="flex items-center justify-end gap-3">
            {running && <span className="text-xs text-carbon-textSub">{t("recovery.abImporting")}</span>}
            <Button
              label={t("recovery.abImport")}
              labelKey="recovery.abImport"
              glyph={<IconDownload />}
              tone="accent"
              onClick={() => void startImport()}
              disabled={!hasNew || running || starting}
              busy={running || starting}
            />
          </div>
          {prog && <ProgressBar percent={prog.percent} active={prog.active} inline />}
        </StepCard>
      )}
    </div>
  );
}
