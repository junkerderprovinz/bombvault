// One restore as the window describes it, turned into what the server takes:
// the check that runs before it and the call that starts it. Each kind has a
// route per target, and a combination no route takes has no request.

import {
  restore,
  restoreContainerFiles,
  restoreContainerToPath,
  restoreFileSet,
  restoreFileSetFiles,
  restoreVM,
  restoreZFS,
  type OkEnvelope,
  type RestoreCheckRequest,
  type RestoreOptions,
  type ZFSRestoreAck,
  type ZFSRestoreRequest,
} from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import type { RestoreWhat, RestoreWhere } from "../../lib/restorePlan";

/** What only a ZFS restore asks for. */
export interface ZFSChoice {
  stamp: string;
  /** "" restores every dataset of the backup. */
  dataset: string;
  newDataset: string;
  applyProperties: boolean;
  safetySnapshot: boolean;
  safetyOffConfirm: boolean;
  stopContainers: boolean;
}

export interface RestoreChoice {
  kind: EntryKind;
  /** What the kind's routes take: a name, an id, or the kind itself. */
  id: string;
  snapshotId: string;
  source: string;
  what: RestoreWhat;
  where: RestoreWhere;
  paths: string[];
  folder: string;
  leaveStopped: boolean;
  withoutRuntime: boolean;
  backupFirst: boolean;
  zfs?: ZFSChoice;
}

export type StartAnswer = OkEnvelope & { started?: boolean; code?: string; target?: string } & Partial<ZFSRestoreAck>;

function options(c: RestoreChoice): RestoreOptions {
  if (c.where === "beside") return { target: "beside" };
  return c.where === "replace" && c.backupFirst ? { backupFirst: true } : {};
}

function zfsRequest(c: RestoreChoice, zfs: ZFSChoice): ZFSRestoreRequest {
  return {
    stamp: zfs.stamp,
    dataset: zfs.dataset,
    wholeTree: zfs.dataset === "",
    paths: c.what === "files" ? c.paths : [],
    targetPath: c.where === "folder" ? c.folder : "",
    newDataset: c.where === "newDataset" ? zfs.newDataset : "",
    confirm: true,
  };
}

/** checkRequest is the check of the restore, or null while a choice it needs
 *  is still open. */
export function checkRequest(c: RestoreChoice): RestoreCheckRequest | null {
  const { id: name, snapshotId, source } = c;
  if (c.what === "files" && c.paths.length === 0) return null;
  if (c.where === "folder" && c.folder === "") return null;
  const targetPath = c.where === "folder" ? c.folder : "";
  switch (c.kind) {
    case "container":
      if (c.what === "files") return { kind: "containerFiles", name, snapshotId, source, paths: c.paths, targetPath };
      if (c.where === "folder") return { kind: "containerTo", name, snapshotId, source, targetPath };
      return { kind: "container", name, snapshotId, source, ...options(c) };
    case "vm":
      return { kind: "vm", name, snapshotId, source, ...options(c) };
    case "files":
      if (c.what === "files") return { kind: "fileSetFiles", name, snapshotId, source, paths: c.paths, targetPath };
      return { kind: "fileSet", name, snapshotId, source, targetPath, ...options(c) };
    case "zfs":
      if (!c.zfs || c.zfs.stamp === "") return null;
      if (c.where === "newDataset" && c.zfs.newDataset === "") return null;
      return { kind: "zfs", name, source, zfs: zfsRequest(c, c.zfs) };
    case "flash":
      return { kind: "flash", snapshotId, source };
    default:
      return null;
  }
}

/** startRestore fires the restore the choice describes. The download of a
 *  flash backup is a link and has no start. */
export function startRestore(c: RestoreChoice): Promise<StartAnswer> {
  const { id, snapshotId, source } = c;
  const targetPath = c.where === "folder" ? c.folder : "";
  const stopped = c.where === "replace" && c.leaveStopped;
  switch (c.kind) {
    case "container":
      if (c.what === "files") return restoreContainerFiles(id, snapshotId, c.paths, targetPath, true, source);
      if (c.where === "folder") return restoreContainerToPath(id, snapshotId, targetPath, source);
      return restore(id, snapshotId, true, source, stopped, c.withoutRuntime, options(c));
    case "vm":
      return restoreVM(id, snapshotId, true, source, stopped, options(c));
    case "files":
      if (c.what === "files") return restoreFileSetFiles(id, snapshotId, c.paths, targetPath, true, source);
      return restoreFileSet(id, snapshotId, true, targetPath, source, options(c));
    case "zfs": {
      if (!c.zfs) break;
      const inPlace = c.where === "replace";
      return restoreZFS(
        id,
        {
          ...zfsRequest(c, c.zfs),
          applyProperties: inPlace && c.zfs.applyProperties,
          safetySnapshot: inPlace && c.zfs.safetySnapshot,
          safetyOffConfirm: inPlace && c.zfs.safetyOffConfirm,
          stopContainers: inPlace && c.zfs.stopContainers,
        },
        source
      );
    }
  }
  return Promise.resolve({ ok: false });
}
