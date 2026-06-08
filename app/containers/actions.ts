"use server";

import { revalidatePath } from "next/cache";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import { createRepo } from "../../lib/backup-repo";
import { createDockerClient, inspectContainer } from "../../lib/docker";
import { resolveAppdataPaths } from "../../lib/appdata";
import { backupContainer, makeBackupDeps } from "../../server/orchestrator";
import { getTranslator } from "../../lib/i18n/server";
import { join } from "node:path";

/**
 * Server action: trigger an immediate backup for the named container.
 *
 * Flow:
 *  1. Load (or create) a backup_target for this container.
 *  2. Resolve the single configured destination — throw a translated error if none.
 *  3. Build orchestrator deps via makeBackupDeps.
 *  4. Run backupContainer (stop → backup → start — always-restart is in the orchestrator).
 *  5. Revalidate /containers so the last-backup timestamp refreshes.
 *
 * Throws on any failure (the form button surfaces errors via Next.js error boundary).
 */
export async function backupNowAction(containerName: string): Promise<void> {
  const { t } = await getTranslator();
  const cfg = getConfig();
  const repo = createRepo(getDb(), cfg.APP_KEY);

  // ── Resolve or create the backup_target ───────────────────────────────────
  // Look for an existing target by container_name.
  // We query by scanning — backup_target has no unique index on container_name,
  // but in P1 each container maps to at most one target.
  const db = getDb();
  const existingTarget = db
    .prepare<[string], { id: string; destination_id: string; appdata_paths: string }>(
      "SELECT id, destination_id, appdata_paths FROM backup_target WHERE container_name = ? LIMIT 1",
    )
    .get(containerName);

  let targetId: string;
  let destinationId: string;
  let appdataPaths: string[];

  if (existingTarget) {
    targetId = existingTarget.id;
    destinationId = existingTarget.destination_id;
    appdataPaths = JSON.parse(existingTarget.appdata_paths) as string[];
  } else {
    // No target yet — need a destination to attach.
    const destinations = repo.listDestinations();
    if (destinations.length === 0) {
      throw new Error(t("containers.noDestination"));
    }
    const destination = destinations[0]!;
    destinationId = destination.id;

    // Inspect the container to resolve its appdata paths.
    const docker = createDockerClient();
    const inspect = await inspectContainer(docker, containerName);
    appdataPaths = resolveAppdataPaths(
      { Name: inspect.Name, Mounts: inspect.Mounts },
      cfg.APPDATA_DIR,
    );

    const targetRow = repo.createTarget({
      containerRef: containerName,
      appdataPaths,
      destinationId,
    });
    targetId = targetRow.id;
  }

  // ── Fetch destination details ─────────────────────────────────────────────
  const destRow = repo.getDestinationRow(destinationId);
  if (!destRow) {
    throw new Error(t("containers.noDestination"));
  }
  const repoPassword = repo.getDestinationPassword(destinationId);

  // ── Build a run recorder bound to the real DB ─────────────────────────────
  const runs: Parameters<typeof makeBackupDeps>[0]["runs"] = {
    recordRunStart: (tid, kind) => repo.createRun({ targetId: tid, kind }),
    recordRunFinish: (runId, finish) => repo.finishRun(runId, finish),
  };

  // ── Wire up deps and run ──────────────────────────────────────────────────
  const snapshotTemplatesDir = join(cfg.DATA_DIR, "templates");
  const deps = await makeBackupDeps({
    containerRef: containerName,
    containerName,
    repoPath: destRow.repo_path,
    repoPassword,
    appdataPaths,
    stopTimeoutSec: 30,
    targetId,
    snapshotTemplatesDir,
    flashTemplatesDir: cfg.FLASH_TEMPLATES_DIR,
    runs,
  });

  await backupContainer(deps);

  revalidatePath("/containers");
}
