// What an entry of one kind has to say under its head, before any card: a row
// of facts and the notes that ask for something.

import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { sweepZFSDataset, type FileSetView, type ZFSDatasetView } from "../../lib/api";
import type { Entry, EntryTab } from "../../lib/entry";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence, zfsFixKey } from "../../lib/zfsCodes";
import { Button } from "../Button";
import { IconFilter, IconInfo, IconStop } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconFolder, IconRecovery, IconTrash } from "../navGlyphs";
import { failText } from "../zfs/failText";
import { RED_CODES } from "../zfs/ZFSDatasetRow";
import { zfsMemberActionable } from "../zfs/ZFSMemberList";
import { Note } from "./Note";

function Fact({ glyph, children }: { glyph: ReactNode; children: ReactNode }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5 [&_svg]:h-3.5 [&_svg]:w-3.5 [&_svg]:flex-none">
      {glyph}
      <span className="min-w-0 wrap-anywhere">{children}</span>
    </span>
  );
}

function Facts({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs text-carbon-textSub">{children}</div>;
}

function ZFSHead({ dataset, onChanged }: { dataset: ZFSDatasetView; onChanged: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [sweeping, setSweeping] = useState(false);
  const skipped = dataset.members.filter(zfsMemberActionable);
  const code = dataset.lastCheckCode;
  const fixKey = zfsFixKey(code);

  async function sweep() {
    setSweeping(true);
    try {
      const res = await sweepZFSDataset(dataset.id);
      const remaining = res.remaining ?? 0;
      if (res.ok && remaining === 0) push(t("zfs.sweepDone"), "success");
      else push(t("zfs.sweepRemaining", remaining), "warn");
      onChanged();
    } catch (err) {
      push(failText(t, err), "fail");
    } finally {
      setSweeping(false);
    }
  }

  return (
    <>
      <Facts>
        <Fact glyph={<IconFolder />}>
          <span dir="ltr" className="font-mono text-start">
            {dataset.hostMountpoint}
          </span>
        </Fact>
        {dataset.stopContainers.length > 0 && (
          <Fact glyph={<IconStop />}>
            {t("zfs.consistencyChips").replace("{names}", dataset.stopContainers.join(", "))}
          </Fact>
        )}
        <Fact glyph={<IconCheck />}>
          {code !== "" && dataset.lastCheckAt > 0
            ? t("zfs.checkedAt").replace("{when}", relativeTime(t, dataset.lastCheckAt))
            : t("zfs.notChecked")}
        </Fact>
        {dataset.excludes.length > 0 && (
          <Fact glyph={<IconFilter />}>{t("zfs.excludesCount", dataset.excludes.length)}</Fact>
        )}
        {skipped.length > 0 && (
          <Fact glyph={<IconInfo />}>
            <span className="inline-flex items-center gap-1.5">
              {t("zfs.skippedCount", skipped.length)}
              <InfoBubble tip={skipped.map((m) => m.dataset).join(", ")} />
            </span>
          </Fact>
        )}
      </Facts>
      {code !== "" && code !== "ok" && (
        <Note tone={RED_CODES.has(code) ? "fail" : "warn"} hint={fixKey ? tLtr(t, fixKey) : undefined}>
          {zfsCodeSentence(t, code, { hostMountpoint: dataset.hostMountpoint, leftoverCount: dataset.leftoverCount })}
        </Note>
      )}
      {dataset.restartPending.length > 0 && (
        <Note tone="warn" hint={t("zfs.restartPendingHint")}>
          {t("zfs.restartPending").replace("{names}", dataset.restartPending.join(", "))}
        </Note>
      )}
      {dataset.leftoverCount > 0 && (
        <Note
          tone="warn"
          hint={t("zfs.leftoversHint")}
          action={
            <Button
              label={t("zfs.removeLeftovers")}
              labelKey="zfs.removeLeftovers"
              glyph={<IconTrash />}
              onClick={() => void sweep()}
              disabled={sweeping}
              busy={sweeping}
            />
          }
        >
          {t("zfs.leftovers", dataset.leftoverCount)}
        </Note>
      )}
    </>
  );
}

function FilesHead({ set }: { set: FileSetView }) {
  const { t } = useT();
  return (
    <>
      {set.excludes.length > 0 && (
        <Facts>
          <Fact glyph={<IconFilter />}>{t("files.excludesCount").replace("{n}", String(set.excludes.length))}</Fact>
        </Facts>
      )}
      {set.path === "" ? (
        <Note hint={t("files.noPathHint")}>{t("files.noPath")}</Note>
      ) : (
        !set.pathExists && (
          <Note tone="warn" hint={t("entry.files.missingHint")}>
            {t("files.pathMissing")}
          </Note>
        )
      )}
    </>
  );
}

/**
 * KindHead stands between the head and the tabs. Flash and the self-backup
 * only speak on the Backups tab, where their note explains how a backup of
 * theirs comes back.
 */
export function KindHead({ entry, tab, onChanged }: { entry: Entry; tab: EntryTab; onChanged: () => void }) {
  const { t } = useT();
  const navigate = useNavigate();
  const { detail } = entry;
  switch (detail.kind) {
    case "zfs":
      return <ZFSHead dataset={detail.dataset} onChanged={onChanged} />;
    case "files":
      return <FilesHead set={detail.set} />;
    case "flash":
      return tab === "backups" ? <Note>{t("entry.flash.note")}</Note> : null;
    case "config":
      return tab === "backups" ? (
        <Note
          action={
            <Button
              label={t("entry.config.toRecovery")}
              labelKey="entry.config.toRecovery"
              glyph={<IconRecovery />}
              onClick={() => navigate("/recovery")}
            />
          }
        >
          {t("entry.config.note")}
        </Note>
      ) : null;
    default:
      return null;
  }
}
