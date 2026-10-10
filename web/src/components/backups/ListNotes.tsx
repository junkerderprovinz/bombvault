// The notes that can stand between the tiles and the list: a batch under way,
// the database dumps explained once, and a ZFS connection that is down.

import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import type { BackupItem, ZFSConnectionResult } from "../../lib/api";
import { entryTarget } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { zfsCodeSentence, zfsFixKey } from "../../lib/zfsCodes";
import { Button } from "../Button";
import { IconEye } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck } from "../navGlyphs";

type T = (key: TranslationKey, n?: number) => string;

const TONE = {
  neutral: "bg-carbon-surface3 text-carbon-text",
  ok: "bg-statusOkBg text-statusOk",
  warn: "bg-statusWarnBg text-statusWarn",
  fail: "bg-statusFailBg text-statusFail",
};

export function ListNote({
  tone = "neutral",
  testId,
  children,
}: {
  tone?: keyof typeof TONE;
  testId?: string;
  children: ReactNode;
}) {
  return (
    <div data-testid={testId} className={`flex flex-wrap items-center gap-3 rounded-control px-3 py-2.5 text-sm ${TONE[tone]}`}>
      {children}
    </div>
  );
}

/**
 * BatchNote is the progress of several backups or restores started together.
 * The counts are there when this page started them and knows how many there
 * are.
 */
export function BatchNote({
  label,
  percent,
  done,
  of,
  t,
}: {
  label: string;
  percent?: number;
  done?: number;
  of?: number;
  t: T;
}) {
  return (
    <ListNote testId="batch-note">
      <span
        className="inline-block h-3 w-3 flex-none animate-spin rounded-full border-2"
        style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
      />
      <span className="min-w-0 flex-1">{label}</span>
      {percent !== undefined && <b className="tabular-nums">{Math.round(percent)} %</b>}
      {done !== undefined && of !== undefined && (
        <span className="tabular-nums text-carbon-textMuted">
          {t("progress.count.items", of).replace("{done}", String(done))}
        </span>
      )}
    </ListNote>
  );
}

/** DumpIntro says once that recognised databases are dumped before each
 *  backup, and leads to the first of them. */
export function DumpIntro({ databases, onDismiss, t }: { databases: BackupItem[]; onDismiss: () => void; t: T }) {
  const first = databases[0];
  const [before, after] = t("dbdump.introNotice", databases.length).split("{name}");
  return (
    <ListNote testId="dump-intro">
      <p className="min-w-[14rem] flex-1">
        {before}
        <Link className="text-accentText underline hover:no-underline" to={entryTarget(first)}>
          <bdi>{first.name}</bdi>
        </Link>
        {after}
      </p>
      <Button label={t("dbdump.introDismiss")} labelKey={null} glyph={<IconCheck />} onClick={onDismiss} />
    </ListNote>
  );
}

/** ZFSDownNote stands above the ZFS entries while zfs commands do not reach
 *  the server, because every one of them fails until that is fixed. */
export function ZFSDownNote({ result, onView, t }: { result: ZFSConnectionResult; onView: () => void; t: T }) {
  const fix = zfsFixKey(result.code);
  return (
    <ListNote tone="fail" testId="zfs-down">
      <span className="flex min-w-[14rem] flex-1 items-center gap-1.5">
        {zfsCodeSentence(t, result.code, { target: result.target, uriTarget: result.uriTarget })}
        {fix && <InfoBubble tip={tLtr(t, fix)} onAccent />}
      </span>
      <Button label={t("backups.zfs.viewConnection")} labelKey={null} glyph={<IconEye />} onClick={onView} />
    </ListNote>
  );
}
