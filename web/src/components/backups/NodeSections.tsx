// A node's containers and folder sets stand under this instance's own, one
// section per node. Each is read through the node when it is opened, so a
// node that is away costs the page nothing until someone looks. Folder sets
// are added and edited here in the same window as this instance's own.

import { useCallback, useEffect, useState, type MouseEvent } from "react";
import { useNavigate } from "react-router-dom";

import {
  backupFileSet,
  backupNow,
  browse,
  listContainers,
  listFileSets,
  type BackupItem,
  type FileSetView,
  type NodeView,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { containerItem, fileSetItem, nodeEntryPath, type NodeEntryKind } from "../../lib/nodeEntries";
import { inNode } from "../../lib/nodeScope";
import { isAdopted } from "../../lib/useNodes";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { Badge } from "../Badge";
import { stopAcknowledged } from "../BackupButton";
import { Button } from "../Button";
import { IconEye, IconEyeOff } from "../glyphs";
import { IconAdd, IconBackupNow, IconPencil } from "../navGlyphs";
import { FileSetDialog } from "../files/FileSetDialog";
import { Card } from "../../pages/settings/shared";
import { ActionMenu, type MenuPlace } from "./ActionMenu";
import { BackupRow, LIST_COLUMNS, LIST_SPAN } from "./BackupRow";
import { entryName } from "./kinds";

// Whether a section is open is kept per browser, so a node someone never
// looks at stays shut and is not asked.
const OPEN_KEY = "bv-node-sections";

function readOpen(): Record<string, boolean> {
  try {
    return JSON.parse(localStorage.getItem(OPEN_KEY) ?? "{}") as Record<string, boolean>;
  } catch {
    return {};
  }
}

function keepOpen(node: string, open: boolean) {
  try {
    localStorage.setItem(OPEN_KEY, JSON.stringify({ ...readOpen(), [node]: open }));
  } catch {
    // The section opens as it did before next time.
  }
}

/** How often an open section is read again, so a backup started from it shows its end. */
const REREAD_MS = 15000;

async function readEntries(node: string, kinds: NodeEntryKind[]): Promise<BackupItem[]> {
  const lists = await Promise.all(
    kinds.map((kind) =>
      kind === "container"
        ? inNode(node, () => listContainers()).then((res) => res.containers.filter((c) => !c.self).map(containerItem))
        : inNode(node, () => listFileSets()).then((res) => res.fileSets.map(fileSetItem)),
    ),
  );
  return lists.flat();
}

function NodeSection({ node, kinds, hueIndex }: { node: NodeView; kinds: NodeEntryKind[]; hueIndex: number }) {
  const { t, lang } = useT();
  const navigate = useNavigate();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [open, setOpen] = useState(() => readOpen()[node.id] ?? true);
  const [items, setItems] = useState<BackupItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ place: MenuPlace; item: BackupItem } | null>(null);
  const [setWindow, setSetWindow] = useState<{ set: FileSetView | null; root: string } | null>(null);
  const kindsKey = kinds.join(",");

  const load = useCallback(() => {
    readEntries(node.id, kindsKey.split(",") as NodeEntryKind[])
      .then((next) => {
        setItems(next);
        setError(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t("backups.loadFailed")));
  }, [node.id, kindsKey, t]);

  useEffect(() => {
    if (!open) return;
    load();
    const timer = window.setInterval(load, REREAD_MS);
    return () => window.clearInterval(timer);
  }, [open, load]);

  function toggle() {
    keepOpen(node.id, !open);
    setOpen(!open);
  }

  async function backUp(item: BackupItem) {
    if (item.kind === "container" && !(await stopAcknowledged(confirm, t))) return;
    const name = entryName(item, t);
    try {
      const res = await inNode(node.id, () => (item.kind === "container" ? backupNow(item.key) : backupFileSet(item.key)));
      if (res.ok) push(t("nodes.backupStarted").replace("{name}", () => name), "success");
      else push(res.error ?? t("containers.backupStartFailed"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("containers.backupStartFailed"), "fail");
    }
  }

  // The window shows paths under the node's own mount, which only the node
  // knows, and edits the set as the node has it now.
  async function openSet(id: string | null) {
    try {
      const [place, sets] = await Promise.all([
        inNode(node.id, () => browse("")),
        id === null ? null : inNode(node.id, () => listFileSets()),
      ]);
      if (!place.ok || place.root === undefined) {
        push(place.error ?? t("folder.browseFailed"), "fail");
        return;
      }
      const set = sets?.fileSets.find((f) => f.id === id) ?? null;
      if (id !== null && set === null) {
        push(t("backups.loadFailed"), "fail");
        return;
      }
      setSetWindow({ set, root: place.root });
    } catch (err) {
      push(err instanceof Error ? err.message : t("folder.browseFailed"), "fail");
    }
  }

  const shown = items ?? [];
  const toggleLabel = t("nodes.sectionToggle").replace("{name}", () => node.name);

  return (
    <section data-node-section={node.id} aria-label={node.name}>
      <Card title={node.name} hint={t("nodes.tip")} hueIndex={hueIndex}>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone={node.online ? "ok" : "neutral"}>{t(node.online ? "instances.connected" : "instances.inactive")}</Badge>
          {items && <span className="text-xs text-carbon-textMuted tabular-nums">{shown.length}</span>}
          <div className="ms-auto flex min-w-0 flex-wrap items-center gap-2">
            {kinds.includes("files") && (
              <Button label={t("files.addSet")} labelKey="files.addSet" glyph={<IconAdd />} tone="subtle" onClick={() => void openSet(null)} />
            )}
            <Button
              label={toggleLabel}
              labelKey={null}
              glyph={open ? <IconEyeOff /> : <IconEye />}
              tone="subtle"
              ariaExpanded={open}
              ariaControls={`bv-node-entries-${node.id}`}
              onClick={toggle}
            />
          </div>
        </div>
        {open && (
          <div id={`bv-node-entries-${node.id}`} className="@container glim-content-fade">
            {error && <p className="px-1.5 py-2 text-sm text-statusWarn wrap-break-word">{error}</p>}
            {!error && !items && <p className="px-1.5 py-2 text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
            {!error && items && shown.length === 0 && <p className="px-1.5 py-2 text-sm text-carbon-textMuted">{t("nodes.sectionEmpty")}</p>}
            {!error && shown.length > 0 && (
              <div className={LIST_COLUMNS}>
                <ul className={LIST_SPAN}>
                  {shown.map((item) => (
                    <BackupRow
                      key={`${item.kind}:${item.key}`}
                      item={item}
                      name={entryName(item, t)}
                      tones={[]}
                      progress={undefined}
                      anomaly={undefined}
                      anomalyEnabled={false}
                      scheduleSentence={() => null}
                      picked={false}
                      showIp={false}
                      onClick={() => false}
                      onMenu={(e: MouseEvent) => setMenu({ place: { x: e.clientX, y: e.clientY }, item })}
                      onLongPress={() => undefined}
                      to={nodeEntryPath(node.id, item.kind as NodeEntryKind, item.key)}
                      t={t}
                      lang={lang}
                    />
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Card>
      {confirmDialog}
      {menu && (
        <ActionMenu
          label={entryName(menu.item, t)}
          actions={[
            { id: "backup", label: t("containers.backupNow"), glyph: <IconBackupNow />, onSelect: () => void backUp(menu.item) },
            ...(menu.item.kind === "files"
              ? [{ id: "edit", label: t("common.edit"), glyph: <IconPencil />, onSelect: () => void openSet(menu.item.key) }]
              : []),
            {
              id: "open",
              label: t("instances.open"),
              glyph: <IconEye />,
              onSelect: () => navigate(nodeEntryPath(node.id, menu.item.kind as NodeEntryKind, menu.item.key)),
            },
          ]}
          place={menu.place}
          onClose={() => setMenu(null)}
        />
      )}
      {setWindow && (
        <FileSetDialog
          initial={setWindow.set}
          presetSeed={null}
          hostMountRoot={setWindow.root}
          node={node}
          t={t}
          onClose={() => setSetWindow(null)}
          onSaved={() => {
            setSetWindow(null);
            load();
          }}
        />
      )}
    </section>
  );
}

/** NodeSections are the sections of this instance's adopted nodes for the
 *  kinds the list shows. */
export function NodeSections({ nodes, kinds, firstHue }: { nodes: NodeView[]; kinds: NodeEntryKind[]; firstHue: number }) {
  const adopted = nodes.filter(isAdopted);
  if (kinds.length === 0) return null;
  return (
    <>
      {adopted.map((node, i) => (
        <NodeSection key={node.id} node={node} kinds={kinds} hueIndex={firstHue + i} />
      ))}
    </>
  );
}
