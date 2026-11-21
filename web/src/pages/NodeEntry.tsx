// The page of an entry on a node: what it is, its backups, and a restore from
// any of them. Everything on it is read and started through the node, which
// answers only for the routes it lets home reach.
import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useParams } from "react-router-dom";

import { BackButton } from "../components/BackButton";
import { Badge } from "../components/Badge";
import { stopAcknowledged } from "../components/BackupButton";
import { KindGlyph, kindHue } from "../components/backups/kinds";
import { Button } from "../components/Button";
import { IconBackupNow, IconRestore } from "../components/navGlyphs";
import { PageTitle } from "../components/PageTitle";
import { Rows, SettingRow } from "../components/storage/rows";
import {
  backupFileSet,
  backupNow,
  fileSetSnapshots,
  getContainer,
  listFileSets,
  listSnapshots,
  restore,
  restoreFileSet,
  type BackupItem,
  type Snapshot,
} from "../lib/api";
import { hueVars } from "../lib/appearance";
import { entryListPath } from "../lib/backupEntry";
import { dateLocale } from "../lib/dateLocale";
import { useT } from "../lib/i18n";
import { containerItem, fileSetItem, parseNodeEntryPath, type NodeEntryKind } from "../lib/nodeEntries";
import { inNode } from "../lib/nodeScope";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { formatRecent } from "../lib/reltime";
import { secretsSentence } from "../lib/secretsRefusal";
import { useToast } from "../lib/toast";
import { useConfirm } from "../lib/useConfirm";
import { useNodes } from "../lib/useNodes";
import { useOwnLogin } from "../lib/useOwnLogin";
import { Card } from "./settings/shared";

async function readEntry(node: string, kind: NodeEntryKind, key: string): Promise<BackupItem | null> {
  if (kind === "container") {
    const res = await inNode(node, () => getContainer(key));
    return res.container ? containerItem(res.container) : null;
  }
  const res = await inNode(node, () => listFileSets());
  const set = res.fileSets.find((f) => f.id === key);
  return set ? fileSetItem(set) : null;
}

function readSnapshots(node: string, kind: NodeEntryKind, key: string): Promise<Snapshot[]> {
  return inNode(node, () => (kind === "container" ? listSnapshots(key) : fileSetSnapshots(key))).then((res) =>
    [...(res.snapshots ?? [])].sort((a, b) => b.time.localeCompare(a.time)),
  );
}

function when(snapshot: Snapshot): string {
  return new Date(snapshot.time).toLocaleString(dateLocale(), { dateStyle: "medium", timeStyle: "short" });
}

export function NodeEntryPage() {
  const params = useParams();
  const entry = parseNodeEntryPath(params.node, params.kind, params.key);
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const { nodes } = useNodes();
  const [item, setItem] = useState<BackupItem | null>(null);
  const [snapshots, setSnapshots] = useState<Snapshot[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [missing, setMissing] = useState(false);
  const [starting, setStarting] = useState(false);

  const node = entry?.node ?? "";
  const kind = entry?.kind ?? "container";
  const key = entry?.key ?? "";
  const nodeName = nodes.find((n) => n.id === node)?.name ?? "";

  const load = useCallback(() => {
    if (!node) return;
    readEntry(node, kind, key)
      .then((found) => {
        setItem(found);
        setMissing(found === null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t("backups.loadFailed")));
    readSnapshots(node, kind, key)
      .then(setSnapshots)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t("backups.loadFailed")));
  }, [node, kind, key, t]);

  useEffect(load, [load]);

  const name = item?.name ?? key;

  async function backUp() {
    if (kind === "container" && !(await stopAcknowledged(confirm, t))) return;
    setStarting(true);
    try {
      const res = await inNode(node, () => (kind === "container" ? backupNow(key) : backupFileSet(key)));
      if (res.ok) push(t("nodes.backupStarted").replace("{name}", () => name), "success");
      else push(res.error ?? t("containers.backupStartFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("containers.backupStartFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  async function restoreFrom(snapshot: Snapshot) {
    const ask = t("nodes.restoreAsk")
      .replace("{name}", () => name)
      .replace("{time}", () => when(snapshot));
    if (!(await confirm(ask, { confirmKey: "snapshots.restore" }))) return;
    try {
      const res = await inNode(node, () => (kind === "container" ? restore(key, snapshot.id, true) : restoreFileSet(key, snapshot.id, true)));
      if (res.ok) {
        push(t("nodes.restoreStarted").replace("{name}", () => name), "success");
        return;
      }
      if (ownLogin.handled(res)) return;
      push(secretsSentence(t, res.code) ?? res.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const facts = [
    nodeName && t("nodes.onNode").replace("{name}", () => nodeName),
    item?.state,
    item?.image,
    item && (item.lastBackup > 0 ? formatRecent(item.lastBackup, lang) : t("containers.never")),
  ].filter(Boolean);

  if (!entry) return <div className={PAGE_SHELL_RESPONSIVE} />;

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{name}</PageTitle>
      {confirmDialog}
      {ownLogin.dialog}
      <div style={hueVars(kindHue(kind)) as CSSProperties} className="glim-hue flex flex-wrap items-center gap-x-4 gap-y-3">
        <span className="flex flex-none items-center gap-2">
          <BackButton to={entryListPath(kind)} label={t("backups.title")} />
          <span className="glim-row-glyph flex h-10 w-10 flex-none items-center justify-center rounded-control bg-carbon-surface2 [&_svg]:h-5 [&_svg]:w-5">
            <KindGlyph kind={kind} />
          </span>
        </span>
        <div className="flex min-w-0 flex-1 basis-64 flex-col gap-0.5">
          <span className="min-w-0 text-status font-semibold text-carbon-text wrap-anywhere">{name}</span>
          <span className="text-sm text-carbon-textMuted wrap-anywhere">{facts.join(" · ")}</span>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2 max-md:justify-start">
          {item && !item.included && <Badge tone="neutral">{t("schedule.paused")}</Badge>}
          <Button
            label={t("containers.backupNow")}
            labelKey="containers.backupNow"
            glyph={<IconBackupNow />}
            tone="accent"
            busy={starting}
            disabled={starting || missing || item?.installed === false}
            onClick={() => void backUp()}
          />
        </div>
      </div>

      {error && <p className="text-sm text-statusWarn wrap-break-word">{error}</p>}
      {missing && <p className="text-sm text-statusWarn">{t("nodes.entryMissing")}</p>}

      <Card title={t("backups.title")} hueIndex={0}>
        {!snapshots && !error && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {snapshots && snapshots.length === 0 && <p className="text-sm text-carbon-textMuted">{t("nodes.noSnapshots")}</p>}
        {snapshots && snapshots.length > 0 && (
          <Rows as="ul" label={t("backups.title")}>
            {snapshots.map((s) => (
              <SettingRow
                key={s.id}
                as="li"
                label={when(s)}
                note={
                  <bdi dir="ltr" className="font-mono text-xs">
                    {s.id.slice(0, 8)}
                  </bdi>
                }
              >
                <Button label={t("snapshots.restore")} labelKey="snapshots.restore" glyph={<IconRestore />} tone="neutral" onClick={() => void restoreFrom(s)} />
              </SettingRow>
            ))}
          </Rows>
        )}
      </Card>
    </div>
  );
}
