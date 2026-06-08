import { join } from "node:path";
import type { ContainerInspect } from "../lib/docker";
import type { BackupSummary } from "../lib/restic";
import type { RunRow, FinishRunInput } from "../lib/backup-repo";

// ---------------------------------------------------------------------------
// Dependency interfaces (DI seam — never import real adapters here)
// ---------------------------------------------------------------------------

/** Injected adapter for docker operations. */
export interface DockerDeps {
  /** Stop the container with a timeout in seconds. */
  stopContainer(id: string, timeoutSec: number): Promise<void>;
  /** Start the container by id/name. */
  startContainer(id: string): Promise<void>;
  /** Remove the container by id/name (errors are caught by caller for "no such container"). */
  removeContainer(id: string): Promise<void>;
  /** Pull image, create and start from the captured inspect definition. */
  createAndStartFromDefinition(inspect: ContainerInspect): Promise<unknown>;
  /** Pull the image only (used by restore before stop+remove). */
  pullImage(image: string): Promise<void>;
  /**
   * SEC-106: inspect the live container by id/name and return its Name
   * (dockerode form, e.g. "/plex"), or null if no such container exists.
   * Used by restore to re-verify the live target before the destructive
   * stop/remove step.
   */
  inspectName(id: string): Promise<string | null>;
}

/** Injected adapter for restic operations. */
export interface ResticDeps {
  /** Run a restic backup and return the parsed summary. */
  backup(repo: string, paths: string[], tags: string[], password: string): Promise<BackupSummary>;
  /** Restore a snapshot to a target directory. */
  restore(repo: string, snapshotId: string, targetDir: string, password: string): Promise<void>;
}

/** Injected adapter for Unraid template operations. */
export interface TemplateDeps {
  /** Read the template XML for the given container name from dir; null if absent. */
  readTemplate(dir: string, name: string): string | null;
  /** Write the template XML for the given container name into dir. */
  writeTemplate(dir: string, name: string, xml: string): void;
}

/** Injected adapter for run recording. */
export interface RunRecorder {
  /** Record the start of a run; returns the created RunRow. */
  recordRunStart(targetId: string, kind: "backup" | "restore"): RunRow;
  /** Record the finish of a run (success or failed). */
  recordRunFinish(runId: string, finish: FinishRunInput): RunRow;
}

// ---------------------------------------------------------------------------
// OrchestratorDeps — fully injected bundle passed to each operation
// ---------------------------------------------------------------------------

export interface BackupDeps {
  /** Container name/id (e.g. "plex"). Used as the tag `container:<ref>` and stop/start id. */
  containerRef: string;
  /** Human-readable display name of the container (e.g. "Plex"). Used for template filename. */
  containerName: string;
  /** Local restic repository path. */
  repoPath: string;
  /** Decrypted restic repository password (never stored in this struct after the call). */
  repoPassword: string;
  /** Appdata paths to include in the backup. */
  appdataPaths: string[];
  /** Stop timeout in seconds. */
  stopTimeoutSec: number;
  /** Backup target id (for run recording). */
  targetId: string;
  /** Directory where per-snapshot template copies are written (e.g. `<DATA_DIR>/templates`). */
  snapshotTemplatesDir: string;
  /** Source directory where the live Unraid templates are stored. */
  flashTemplatesDir: string;
  docker: DockerDeps;
  restic: ResticDeps;
  templates: TemplateDeps;
  runs: RunRecorder;
  now?: () => number; // injectable clock (unused in logic, kept for future use)
}

export interface RestoreDeps {
  /** MUST be true — guard against accidental restore. */
  confirmed: boolean;
  /** Container name/id to stop+remove. */
  containerRef: string;
  /** Human-readable display name (for template filename). */
  containerName: string;
  /** Local restic repository path. */
  repoPath: string;
  /** Decrypted restic repository password. */
  repoPassword: string;
  /** Snapshot id to restore. */
  snapshotId: string;
  /** Target directory for `restic restore` (appdata root, e.g. /mnt/user/appdata). */
  restoreTargetDir: string;
  /** Stored template XML to flash back (captured at backup time). */
  templateXml: string;
  /** Directory where the live Unraid templates live. */
  flashTemplatesDir: string;
  /** The inspect data captured at backup time; used to recreate the container. */
  inspect: ContainerInspect;
  /** Backup target id (for run recording). */
  targetId: string;
  docker: DockerDeps;
  restic: ResticDeps;
  templates: TemplateDeps;
  runs: RunRecorder;
}

// ---------------------------------------------------------------------------
// backupContainer
// ---------------------------------------------------------------------------

/**
 * Orchestrate a container backup:
 *   recordRunStart
 *   → try {
 *       stopContainer(timeout)
 *       → resticBackup(repoPath, password, appdataPaths, tags)
 *       → readTemplate + persist snapshot copy
 *     } finally {
 *       startContainer()   ← ALWAYS, even on failure
 *     }
 *   → recordRunFinish(success|failed)
 *   → re-throw on failure
 *
 * The container is GUARANTEED to be restarted even if the backup or template
 * operations throw. Failures are recorded before re-throwing.
 */
export async function backupContainer(deps: BackupDeps): Promise<BackupSummary> {
  const run = deps.runs.recordRunStart(deps.targetId, "backup");
  const tags = [`container:${deps.containerRef}`, "p1"];

  let summary: BackupSummary | undefined;
  let backupError: unknown;

  try {
    await deps.docker.stopContainer(deps.containerRef, deps.stopTimeoutSec);

    summary = await deps.restic.backup(
      deps.repoPath,
      deps.appdataPaths,
      tags,
      deps.repoPassword,
    );

    // Capture and persist the live Unraid template alongside the snapshot.
    const xml = deps.templates.readTemplate(deps.flashTemplatesDir, deps.containerName);
    if (xml !== null) {
      const fileName = `${summary.snapshotId}-my-${deps.containerName}.xml`;
      const destPath = join(deps.snapshotTemplatesDir, fileName);
      // write to a flat dir — templateFileName would add another prefix; write directly
      deps.templates.writeTemplate(deps.snapshotTemplatesDir, `${summary.snapshotId}-my-${deps.containerName}`, xml);
      void destPath; // used only for documentation; writeTemplate builds the path itself
    }
  } catch (err) {
    backupError = err;
  } finally {
    // Always restart — even if backup threw.
    await deps.docker.startContainer(deps.containerRef);
  }

  if (backupError !== undefined) {
    const msg = backupError instanceof Error ? backupError.message : String(backupError);
    deps.runs.recordRunFinish(run.id, { status: "failed", error: msg });
    throw backupError;
  }

  deps.runs.recordRunFinish(run.id, {
    status: "success",
    snapshotId: summary!.snapshotId,
    bytes: summary!.bytesAdded,
  });

  return summary!;
}

// ---------------------------------------------------------------------------
// restoreContainer
// ---------------------------------------------------------------------------

/**
 * Orchestrate a container restore:
 *   guard confirmed === true
 *   → recordRunStart
 *   → pullImage
 *   → stopContainer (ignore "no such container")
 *   → removeContainer (ignore "no such container")
 *   → resticRestore(snapshotId, restoreTargetDir)
 *   → writeTemplate (flash the stored template back)
 *   → createAndStartFromDefinition
 *   → recordRunFinish(success)
 *
 * Throws without recording if `confirmed` is false.
 */
export async function restoreContainer(deps: RestoreDeps): Promise<void> {
  if (!deps.confirmed) {
    throw new Error("restore requires confirmed: true");
  }

  const run = deps.runs.recordRunStart(deps.targetId, "restore");

  try {
    // SEC-106: re-verify the live container matches the target BEFORE the
    // destructive stop/remove. If a container by this ref exists, its name must
    // match the expected target name; abort on mismatch (wrong-target hazard).
    // A missing container is acceptable (a fresh restore recreates it).
    const liveName = await deps.docker.inspectName(deps.containerRef);
    if (liveName !== null) {
      const normalize = (n: string) => (n.startsWith("/") ? n.slice(1) : n);
      if (normalize(liveName) !== normalize(deps.containerName)) {
        throw new Error(
          `restore aborted: live container "${normalize(liveName)}" does not match target "${normalize(deps.containerName)}"`,
        );
      }
    }

    // Pull the image before touching the running container.
    await deps.docker.pullImage(deps.inspect.Config.Image);

    // Stop the existing container — ignore errors (may already be stopped or absent).
    try {
      await deps.docker.stopContainer(deps.containerRef, 30);
    } catch {
      // "no such container" or already stopped — acceptable
    }

    // Remove the existing container — ignore errors (may not exist).
    try {
      await deps.docker.removeContainer(deps.containerRef);
    } catch {
      // "no such container" — acceptable
    }

    // Restore appdata from the snapshot.
    await deps.restic.restore(
      deps.repoPath,
      deps.snapshotId,
      deps.restoreTargetDir,
      deps.repoPassword,
    );

    // Flash the captured template back to the flash drive.
    deps.templates.writeTemplate(deps.flashTemplatesDir, deps.containerName, deps.templateXml);

    // Recreate and start the container from the captured definition.
    await deps.docker.createAndStartFromDefinition(deps.inspect);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    deps.runs.recordRunFinish(run.id, { status: "failed", error: msg });
    throw err;
  }

  deps.runs.recordRunFinish(run.id, { status: "success" });
}

// ---------------------------------------------------------------------------
// makeOrchestratorDeps — wires real adapters (never imported in tests)
// ---------------------------------------------------------------------------

// Kept out of the DI seam so unit tests never touch real adapters.
// Import real adapters only inside this function to preserve the seam.

export async function makeBackupDeps(
  overrides: Pick<
    BackupDeps,
    | "containerRef"
    | "containerName"
    | "repoPath"
    | "repoPassword"
    | "appdataPaths"
    | "stopTimeoutSec"
    | "targetId"
    | "snapshotTemplatesDir"
    | "flashTemplatesDir"
  > & { runs: RunRecorder },
): Promise<BackupDeps> {
  const [
    { stopContainer, startContainer },
    { backup },
    { readTemplate, writeTemplate },
  ] = await Promise.all([
    import("../lib/docker"),
    import("../lib/restic"),
    import("../lib/unraid-template"),
  ]);
  const { createDockerClient } = await import("../lib/docker");
  const docker = createDockerClient();

  return {
    ...overrides,
    docker: {
      stopContainer: (id, t) => stopContainer(docker, id, t),
      startContainer: (id) => startContainer(docker, id),
      removeContainer: async () => { /* not used in backup */ },
      createAndStartFromDefinition: async () => { /* not used in backup */ },
      pullImage: async () => { /* not used in backup */ },
      inspectName: async () => null, /* not used in backup */
    },
    restic: {
      backup: (repo, paths, tags, password) => backup(repo, paths, tags, password),
      restore: async () => { /* not used in backup */ },
    },
    templates: { readTemplate, writeTemplate },
  };
}

export async function makeRestoreDeps(
  overrides: Pick<
    RestoreDeps,
    | "confirmed"
    | "containerRef"
    | "containerName"
    | "repoPath"
    | "repoPassword"
    | "snapshotId"
    | "restoreTargetDir"
    | "templateXml"
    | "flashTemplatesDir"
    | "inspect"
    | "targetId"
  > & { runs: RunRecorder },
): Promise<RestoreDeps> {
  const [
    { stopContainer, startContainer, removeContainer, createAndStartFromDefinition, pullImage, inspectContainer, createDockerClient },
    { restore },
    { readTemplate, writeTemplate },
  ] = await Promise.all([
    import("../lib/docker"),
    import("../lib/restic"),
    import("../lib/unraid-template"),
  ]);
  const docker = createDockerClient();

  return {
    ...overrides,
    docker: {
      stopContainer: (id, t) => stopContainer(docker, id, t),
      startContainer: (id) => startContainer(docker, id),
      removeContainer: (id) => removeContainer(docker, id),
      createAndStartFromDefinition: (inspect) => createAndStartFromDefinition(docker, inspect),
      pullImage: (image) => pullImage(docker, image),
      // SEC-106: re-verify the live container before the destructive step.
      // A missing container surfaces as null (dockerode throws "no such
      // container"); any other inspect error propagates and aborts the restore.
      inspectName: async (id) => {
        try {
          const info = await inspectContainer(docker, id);
          return info.Name ?? null;
        } catch (err) {
          const msg = err instanceof Error ? err.message : String(err);
          if (/no such container/i.test(msg)) return null;
          throw err;
        }
      },
    },
    restic: {
      backup: async () => { throw new Error("not used in restore"); },
      restore: (repo, snapshotId, targetDir, password) => restore(repo, snapshotId, targetDir, password),
    },
    templates: { readTemplate, writeTemplate },
  };
}
