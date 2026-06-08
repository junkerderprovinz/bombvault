"use server";

import { revalidatePath } from "next/cache";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import { createRepo } from "../../lib/backup-repo";
import { createDockerClient, inspectContainer } from "../../lib/docker";
import { resolveAppdataPaths } from "../../lib/appdata";
import { backupContainer, makeBackupDeps } from "../../server/orchestrator";
import { initRepo } from "../../lib/restic";
import { deriveResticPassword } from "../../lib/restic-key";
import { getTranslator } from "../../lib/i18n/server";
import { join } from "node:path";

export interface BackupActionState {
  ok: boolean;
  /** Success message (when ok). */
  message?: string;
  /** Error message (when !ok) — shown inline, never thrown to the error boundary. */
  error?: string;
}

/**
 * Server action (useActionState-compatible): back up the container named in the
 * `container` form field — with one click, no setup.
 *
 * Simplified UX (see [[bombvault-backup-ux-simple]]):
 *  - The destination is the template-mounted containers repo (cfg.CONTAINERS_REPO);
 *    the user never picks a path.
 *  - The restic password is derived from APP_KEY (deriveResticPassword); the user
 *    never types a password. Backups are still encrypted.
 *  - The repo is auto-initialised on first use (idempotent).
 *
 * Errors are returned as state (never thrown) so the page surfaces them inline
 * instead of crashing via the Next.js error boundary.
 */
export async function backupNowAction(
  _prev: BackupActionState,
  formData: FormData,
): Promise<BackupActionState> {
  const { t } = await getTranslator();
  const containerName = String(formData.get("container") ?? "").trim();
  if (!containerName) {
    return { ok: false, error: "missing container name" };
  }

  try {
    const cfg = getConfig();
    const db = getDb();
    const repo = createRepo(db, cfg.APP_KEY);

    const repoPath = cfg.CONTAINERS_REPO;

    // Auto-managed internal destination (one per repo path — not user-facing).
    // The restic password is derived from APP_KEY when the destination is first
    // created; we then always read it back from the destination so a backup uses
    // the EXACT password the restore path (getDestinationPassword) will use later
    // — no drift between backup and restore.
    const destination =
      repo.findDestinationByRepoPath(repoPath) ??
      repo.createDestination({
        name: "containers",
        repoPath,
        password: deriveResticPassword(cfg.APP_KEY),
      });
    const password = repo.getDestinationPassword(destination.id);

    // Auto-initialise the restic repo (idempotent: tolerate "already initialized").
    try {
      await initRepo(repoPath, password);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (!/already initialized|already exists/i.test(msg)) throw err;
    }

    // Resolve or create the backup_target for this container.
    const existing = repo.findTargetByContainer(containerName);
    let targetId: string;
    let appdataPaths: string[];
    if (existing) {
      targetId = existing.id;
      appdataPaths = JSON.parse(existing.appdata_paths) as string[];
    } else {
      const docker = createDockerClient();
      const inspect = await inspectContainer(docker, containerName);
      appdataPaths = resolveAppdataPaths(
        { Name: inspect.Name, Mounts: inspect.Mounts },
        cfg.APPDATA_DIR,
      );
      const created = repo.createTarget({
        containerRef: containerName,
        appdataPaths,
        destinationId: destination.id,
      });
      targetId = created.id;
    }

    // Run recorder bound to the real DB.
    const runs: Parameters<typeof makeBackupDeps>[0]["runs"] = {
      recordRunStart: (tid, kind) => repo.createRun({ targetId: tid, kind }),
      recordRunFinish: (runId, finish) => repo.finishRun(runId, finish),
    };

    const snapshotTemplatesDir = join(cfg.DATA_DIR, "templates");
    const deps = await makeBackupDeps({
      containerRef: containerName,
      containerName,
      repoPath,
      repoPassword: password,
      appdataPaths,
      stopTimeoutSec: 30,
      targetId,
      snapshotTemplatesDir,
      flashTemplatesDir: cfg.FLASH_TEMPLATES_DIR,
      runs,
    });

    await backupContainer(deps);

    revalidatePath("/containers");
    return { ok: true, message: t("containers.backupStarted") };
  } catch (err) {
    // Log the full error server-side; the restic adapter already scrubs paths
    // and secrets from its message (SEC-006). Surface a short message inline.
    const msg = err instanceof Error ? err.message : String(err);
    // eslint-disable-next-line no-console
    console.error("backupNowAction failed:", err);
    return { ok: false, error: msg };
  }
}
