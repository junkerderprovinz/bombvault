import { useEffect, useState, type ReactNode } from "react";
import {
  browse,
  getFileSetPreset,
  listFileSets,
  zfsHostDatasets,
  type FileSetPresetResponse,
  type ZFSHostResult,
} from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import type { AddedKind } from "../../lib/backupList";
import { useT } from "../../lib/i18n";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconBack } from "../glyphs";
import { ZFSAddForm } from "../zfs/ZFSAddDialog";
import { FolderAddForm, type FoundFolder } from "./FolderAddForm";
import { KindGlyph } from "./kinds";

// Unraid keeps its user shares in this folder of the host mount. Elsewhere it
// does not exist and nothing is offered.
const SHARES = "user";

function Choice({
  kind,
  title,
  line,
  disabled,
  onChoose,
}: {
  kind: AddedKind;
  title: string;
  line: string;
  disabled: boolean;
  onChoose: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onChoose}
      className="flex items-start gap-3 rounded-control bg-carbon-surface2 p-4 text-start transition-colors hover:bg-carbon-surface3 disabled:opacity-55 disabled:hover:bg-carbon-surface2"
    >
      <span className="flex h-11 w-11 flex-none items-center justify-center rounded-control bg-accent text-accentContrast [&_svg]:h-[22px] [&_svg]:w-[22px]">
        <KindGlyph kind={kind} />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-sm font-semibold text-carbon-text">{title}</span>
        <span className="text-xs text-carbon-textSub wrap-anywhere">{line}</span>
      </span>
    </button>
  );
}

/**
 * AddWindow adds what BombVault cannot find by itself: a folder or a ZFS
 * dataset. Opened from the tile of one of the two it starts in that form,
 * otherwise with the choice between them.
 */
export function AddWindow({
  start,
  enabled,
  hostMountRoot,
  hostRoot,
  onClose,
  onAdded,
}: {
  start: AddedKind | "choose";
  enabled: Partial<Record<EntryKind, boolean>>;
  hostMountRoot: string;
  /** What the host calls the folder mounted at hostMountRoot. */
  hostRoot: string;
  onClose: () => void;
  onAdded: () => void;
}) {
  const { t } = useT();
  const [kind, setKind] = useState<AddedKind | null>(start === "choose" ? null : start);
  const [shares, setShares] = useState<FoundFolder[]>([]);
  const [taken, setTaken] = useState<ReadonlySet<string>>(new Set());
  const [preset, setPreset] = useState<FileSetPresetResponse | null>(null);
  const [host, setHost] = useState<ZFSHostResult | "loading" | "down">("loading");

  // None of these holds the window up: a failed read leaves its line or its
  // list out.
  useEffect(() => {
    browse(SHARES)
      .then((res) => setShares(res.ok ? (res.dirs ?? []) : []))
      .catch(() => undefined);
    listFileSets()
      .then((res) => setTaken(new Set((res.fileSets ?? []).map((set) => set.path))))
      .catch(() => undefined);
    getFileSetPreset()
      .then((res) => setPreset(res.ok ? res : null))
      .catch(() => undefined);
  }, []);

  const zfsOn = !!enabled.zfs;
  useEffect(() => {
    if (!zfsOn) return;
    zfsHostDatasets()
      .then((res) => setHost(res.ok && res.available ? res : "down"))
      .catch(() => setHost("down"));
  }, [zfsOn]);

  const found: FoundFolder[] =
    shares.length > 0 ? [...shares, { name: t("backups.add.allShares"), path: SHARES }] : [];

  const sectionOff = (name: string) =>
    t("backups.add.sectionOff")
      .replace("{kind}", name)
      .replace("{where}", `${t("nav.settings")} › ${t("settings.tab.general")}`);

  let folderLine = t("backups.add.folderAny").replace("{root}", hostRoot);
  if (!enabled.files) folderLine = sectionOff(t("nav.files"));
  else if (shares.length > 0) {
    const backedUp = shares.filter((share) => taken.has(share.path)).length;
    folderLine = t("backups.add.folderFound", shares.length).replace("{m}", String(backedUp));
  }

  let datasetLine = t("zfs.add.loading");
  if (!zfsOn) datasetLine = sectionOff(t("nav.zfs"));
  else if (host === "down") datasetLine = t("backups.add.datasetDown");
  else if (host !== "loading") datasetLine = t("zfs.notInItem", host.notInItem);

  const chosen = (label: string) => (
    <div className="flex justify-end">
      <Badge tone="neutral">{label}</Badge>
    </div>
  );
  const back = <Button label={t("backups.add.other")} labelKey={null} glyph={<IconBack />} tone="subtle" onClick={() => setKind(null)} />;
  const cancel = <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />;
  const frame = (title: string, hint: string, body: ReactNode, footer: ReactNode, closable = true) => (
    <InstanceWindow title={title} hint={hint} footer={footer} onClose={closable ? onClose : () => undefined}>
      {body}
    </InstanceWindow>
  );

  if (kind === "files") {
    return (
      <FolderAddForm
        found={found}
        foundUnder={SHARES}
        taken={taken}
        preset={preset}
        hostMountRoot={hostMountRoot}
        hostRoot={hostRoot}
        onClose={onClose}
        onAdded={onAdded}
      >
        {({ body, cancel: formCancel, forward }) =>
          frame(
            t("backups.add.title"),
            t("backups.add.hint"),
            <>
              {chosen(t("storage.folder"))}
              {body}
            </>,
            <>
              {formCancel}
              {back}
              {forward}
            </>
          )
        }
      </FolderAddForm>
    );
  }

  if (kind === "zfs") {
    return (
      <ZFSAddForm onClose={onClose} onAdded={onAdded}>
        {({ body, cancel: formCancel, forward, closable }) =>
          frame(
            t("zfs.add.title"),
            t("zfs.add.hint"),
            <>
              {closable && chosen(t("backups.add.zfsDataset"))}
              {body}
            </>,
            <>
              {formCancel}
              {closable && back}
              {forward}
            </>,
            closable
          )
        }
      </ZFSAddForm>
    );
  }

  return frame(
    t("backups.add.title"),
    t("backups.add.hint"),
    <div className="grid gap-3 md:grid-cols-2">
      <Choice
        kind="files"
        title={t("backups.add.folder")}
        line={folderLine}
        disabled={!enabled.files}
        onChoose={() => setKind("files")}
      />
      <Choice
        kind="zfs"
        title={t("backups.add.dataset")}
        line={datasetLine}
        disabled={!zfsOn}
        onChoose={() => setKind("zfs")}
      />
    </div>,
    cancel
  );
}
