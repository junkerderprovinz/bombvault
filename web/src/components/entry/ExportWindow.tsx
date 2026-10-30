import { useEffect, useState, type ReactNode } from "react";
import {
  cancelBackup,
  exportBackup,
  exportContainer,
  exportVM,
  listItemRuns,
  previewBackupExport,
  type ExportPreview,
  type ExportRequest,
  type TimelineRow,
} from "../../lib/api";
import { actionMark, rowTime, type Entry } from "../../lib/entry";
import { humanBytes } from "../../lib/forecast";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { anyActive, busyPhraseKey, useProgress } from "../../lib/progress";
import { formatRecent } from "../../lib/reltime";
import { newestId, sourceOfPlace } from "../../lib/timeline";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { PageSentence } from "../../pages/settings/SidebarTile";
import { Button } from "../Button";
import { CopyBlock } from "../CopyBlock";
import { IconConfirm, IconExport } from "../glyphs";
import { ProgressBar } from "../ProgressBar";
import { Selector } from "../Selector";
import { Rows, SettingRow } from "../storage/rows";
import type { TimelineState } from "./BackupPlaces";
import { Note } from "./Note";

type Form = ExportRequest["form"];

const FORMS: { id: Form; label: string }[] = [
  { id: "tar", label: "tar" },
  { id: "zip", label: "zip" },
  { id: "repository", label: "restic" },
];

const FORM_HINT: Record<Form, TranslationKey> = {
  tar: "entry.export.tarHint",
  zip: "entry.export.zipHint",
  repository: "entry.export.resticHint",
};

type Phase = "choose" | "starting" | "running" | "done" | "failed" | "cancelled";

function SubHead({ children }: { children: ReactNode }) {
  return <h3 className="text-sm font-semibold text-carbon-text">{children}</h3>;
}

/** Where an export ended, as the path to copy. A single file is named, and
 *  several are found in their folder. */
function writtenTo(preview: ExportPreview): string {
  return preview.files.length === 1 ? `${preview.folder}/${preview.files[0]}` : preview.folder;
}

/**
 * ExportWindow writes one stored backup, or every backup of the entry, into
 * the export folder on the server. Before anything is written the server
 * names the folder and the files, and for a repository of its own the key it
 * opens with. The window then follows the run to its end.
 */
export function ExportWindow({
  entry,
  name,
  row,
  timeline,
  onClose,
}: {
  entry: Entry;
  name: string;
  row: TimelineRow;
  timeline: TimelineState;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { item } = entry;
  const mark = actionMark(row, timeline.places);
  const source = sourceOfPlace(mark.place);
  const snapshotId = newestId(mark);
  const progressMap = useProgress();
  const other = anyActive(progressMap);

  const [form, setForm] = useState<Form>("tar");
  const [every, setEvery] = useState(false);
  const [preview, setPreview] = useState<ExportPreview | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [phase, setPhase] = useState<Phase>("choose");
  const wholeEntry = form === "repository" && every;

  useEffect(() => {
    let current = true;
    setPreview(null);
    setProblem(null);
    previewBackupExport(
      timeline.domain,
      item.key,
      { form, snapshotId: wholeEntry ? undefined : snapshotId },
      source
    )
      .then((res) => {
        if (!current) return;
        if (res.ok && res.export) setPreview(res.export);
        else setProblem(placementErrorText(t, lang, res, "entry.export.failed"));
      })
      .catch((err: unknown) => {
        if (current) setProblem(err instanceof Error ? err.message : t("entry.export.failed"));
      });
    return () => {
      current = false;
    };
  }, [timeline.domain, item.key, form, wholeEntry, snapshotId, source, t, lang]);

  const progress = preview ? progressMap[preview.key] : undefined;
  const reporting = phase === "running" && progress !== undefined;
  // A run whose last frame was missed shows as progress that was there and
  // is gone.
  const [seen, setSeen] = useState(false);
  useEffect(() => {
    if (reporting) setSeen(true);
  }, [reporting]);
  const ended = phase === "running" && (progress?.finished === true || (seen && !progress));
  const succeeded = progress?.finished === true && progress.percent >= 100;

  useEffect(() => {
    if (!ended) return;
    if (succeeded) {
      setPhase("done");
      return;
    }
    let current = true;
    // The frame that ends a run says only that it did not work. Why is on
    // the run the export recorded.
    listItemRuns(item.kind, item.key, 10)
      .then((res) => {
        if (!current) return;
        const run = (res.runs ?? []).find((r) => r.kind === "export");
        if (run?.status === "success") {
          setPhase("done");
          return;
        }
        if (run?.status === "cancelled") {
          setPhase("cancelled");
          return;
        }
        setProblem(run?.error || t("entry.export.failed"));
        setPhase("failed");
      })
      .catch(() => {
        if (!current) return;
        setProblem(t("entry.export.failed"));
        setPhase("failed");
      });
    return () => {
      current = false;
    };
  }, [ended, succeeded, item.kind, item.key, t]);

  async function start() {
    setPhase("starting");
    setProblem(null);
    setSeen(false);
    try {
      const res = await exportBackup(
        timeline.domain,
        item.key,
        { form, snapshotId: wholeEntry ? undefined : snapshotId },
        source
      );
      if (res.ok && res.started && res.export) {
        setPreview(res.export);
        setPhase("running");
        return;
      }
      const busy = (res.error ?? "").toLowerCase().includes("already running");
      setProblem(busy ? t("entry.export.busy") : placementErrorText(t, lang, res, "entry.export.failed"));
    } catch (err) {
      setProblem(err instanceof Error ? err.message : t("entry.export.failed"));
    }
    setPhase("choose");
  }

  const choosing = phase === "choose" || phase === "starting";
  const when = formatRecent(rowTime(row), lang);
  let running = "";
  if (progress?.snapshotTotal && progress.snapshotTotal > 1 && progress.snapshotIndex) {
    running = t("overview.head.copyStep")
      .replace("{index}", String(progress.snapshotIndex))
      .replace("{total}", String(progress.snapshotTotal));
  } else if (progress?.bytes) {
    running = t("entry.export.written").replace("{size}", humanBytes(progress.bytes));
  }

  let footer: ReactNode;
  if (choosing) {
    footer = (
      <>
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
        <Button
          label={t("entry.export.action")}
          labelKey="entry.export.action"
          glyph={<IconExport />}
          tone="accent"
          disabled={preview === null || phase === "starting" || other.active}
          busy={phase === "starting"}
          title={other.active ? t(busyPhraseKey(other.phase, other.stage)) : undefined}
          onClick={() => void start()}
        />
      </>
    );
  } else if (phase === "running") {
    footer = (
      <>
        <Button
          label={t("entry.export.cancel")}
          labelKey="entry.export.cancel"
          tone="subtle"
          onClick={() => preview && void cancelBackup(preview.key).catch(() => undefined)}
        />
        <Button label={t("common.close")} labelKey="common.close" tone="subtle" onClick={onClose} />
      </>
    );
  } else {
    footer = (
      <Button label={t("common.done")} labelKey="common.done" glyph={<IconConfirm />} tone="accent" onClick={onClose} />
    );
  }

  return (
    <InstanceWindow
      title={t("entry.export.title").replace("{name}", () => name)}
      facts={wholeEntry ? t("entry.export.every") : t("entry.export.stored").replace("{when}", when)}
      onClose={onClose}
      footer={footer}
    >
      <Rows>
        <SettingRow label={t("entry.export.format")} hint={t("entry.export.formatHint")}>
          <Selector
            items={FORMS.map((f) => ({ id: f.id, label: f.label, disabled: !choosing }))}
            label={t("entry.export.format")}
            select="one"
            variant="well"
            inline
            active={form}
            onChange={(id) => setForm(id as Form)}
          />
        </SettingRow>
        {form === "repository" && (
          <SettingRow label={t("backups.title")}>
            <Selector
              items={[
                { id: "one", label: t("entry.export.scopeOne"), disabled: !choosing },
                { id: "every", label: t("entry.export.scopeEvery"), disabled: !choosing },
              ]}
              label={t("backups.title")}
              select="one"
              variant="well"
              inline
              active={every ? "every" : "one"}
              onChange={(id) => setEvery(id === "every")}
            />
          </SettingRow>
        )}
      </Rows>

      {preview && (
        <div data-testid="export-preview" className="flex flex-col gap-2">
          <SubHead>{t("entry.export.folder")}</SubHead>
          <CopyBlock text={preview.folder} />
          <p className="text-xs text-carbon-textMuted wrap-anywhere">
            {t("entry.export.files").replace("{files}", () => preview.files.join(", "))}
            {preview.backups !== undefined && ` ${t("entry.places.count", preview.backups)}.`}
            {preview.bytes !== undefined && ` ${t("entry.export.size").replace("{size}", humanBytes(preview.bytes))}`}{" "}
            {t(wholeEntry ? "entry.export.resticEveryHint" : FORM_HINT[form])} {t("entry.export.where")}
          </p>
          {preview.segments && <p className="text-xs text-carbon-textMuted">{t("entry.export.segments")}</p>}
          {preview.encryption === "age" && <p className="text-xs text-carbon-textMuted">{t("entry.export.keyAge")}</p>}
          {form === "repository" && preview.encryption === "none" && (
            <p className="text-xs text-carbon-textMuted">{t("entry.export.keyNone")}</p>
          )}
          {preview.encryption === "password" && (
            <>
              <p className="text-xs text-carbon-textMuted">{t("entry.export.keyPassword")}</p>
              <PageSentence text="tile.crypt.kitLine" page="activityLog.typeRestore" to="/settings/restore" />
            </>
          )}
          {preview.replaces.length > 0 && (
            <Note tone="warn">{t("entry.export.replaces").replace("{files}", () => preview.replaces.join(", "))}</Note>
          )}
        </div>
      )}

      {phase === "running" && (
        <div className="flex flex-col gap-1.5">
          <ProgressBar percent={progress?.percent ?? 0} active inline label={running || undefined} />
          <p className="text-xs text-carbon-textMuted">{t("entry.export.background")}</p>
        </div>
      )}
      {phase === "done" && preview && (
        <div data-testid="export-result" className="flex flex-col gap-2">
          <SubHead>{t("export.exportedTo")}</SubHead>
          <CopyBlock text={writtenTo(preview)} />
        </div>
      )}
      {phase === "cancelled" && <Note>{t("entry.export.cancelled")}</Note>}
      {problem !== null && (
        <div role="alert">
          <Note tone="fail">{problem}</Note>
        </div>
      )}
    </InstanceWindow>
  );
}

/**
 * LiveExportWindow writes a container or VM as it stands on the host into the
 * export folder: its folders or disk images as a tar and its definition
 * beside it. The server answers once the files are written.
 */
export function LiveExportWindow({ entry, name, onClose }: { entry: Entry; name: string; onClose: () => void }) {
  const { t } = useT();
  const { item, settings } = entry;
  const [busy, setBusy] = useState(false);
  const [folder, setFolder] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const sealed = settings.exportEncryptEnabled ? ".age" : "";
  const files = [`${item.key}.tar.gz${sealed}`, `${item.key}.xml${sealed}`];

  async function start() {
    setBusy(true);
    setProblem(null);
    try {
      const res = await (item.kind === "vm" ? exportVM(item.key) : exportContainer(item.key));
      if (res.ok && res.path) setFolder(res.path);
      else setProblem(res.error ?? t("entry.export.failed"));
    } catch (err) {
      setProblem(err instanceof Error ? err.message : t("entry.export.failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("entry.export.title").replace("{name}", () => name)}
      facts={t(item.kind === "vm" ? "entry.export.liveVM" : "entry.export.liveContainer")}
      onClose={onClose}
      footer={
        folder === null ? (
          <>
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
            <Button
              label={t("entry.export.action")}
              labelKey="entry.export.action"
              glyph={<IconExport />}
              tone="accent"
              disabled={busy}
              busy={busy}
              onClick={() => void start()}
            />
          </>
        ) : (
          <Button label={t("common.done")} labelKey="common.done" glyph={<IconConfirm />} tone="accent" onClick={onClose} />
        )
      }
    >
      <p className="text-xs text-carbon-textMuted wrap-anywhere">
        {t("entry.export.files").replace("{files}", () => files.join(", "))} {t("entry.export.tarHint")}{" "}
        {t("entry.export.where")}
      </p>
      {sealed !== "" && <p className="text-xs text-carbon-textMuted">{t("entry.export.keyAge")}</p>}
      {folder !== null && (
        <div data-testid="export-result" className="flex flex-col gap-2">
          <SubHead>{t("export.exportedTo")}</SubHead>
          <CopyBlock text={folder} />
        </div>
      )}
      {problem !== null && (
        <div role="alert">
          <Note tone="fail">{problem}</Note>
        </div>
      )}
    </InstanceWindow>
  );
}
