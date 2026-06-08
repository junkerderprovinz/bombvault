"use server";

import { revalidatePath } from "next/cache";
import { getDb } from "../../../server/db";
import { getConfig } from "../../../lib/config";
import { createRepo } from "../../../lib/backup-repo";
import { inspectContainer, createDockerClient } from "../../../lib/docker";
import type { ContainerInspect } from "../../../lib/docker";
import { restoreContainer, makeRestoreDeps } from "../../../server/orchestrator";
import { join } from "node:path";
import { readFileSync } from "node:fs";

/**
 * Server action: restore a container from a given snapshot.
 *
 * @param targetId   - The backup_target id.
 * @param snapshotId - The restic snapshot id to restore from.
 * @param confirmed  - MUST be exactly `true`. The action throws immediately if
 *                     this is false (the two-step confirm form enforces this in
 *                     the UI layer, and the orchestrator enforces it at the
 *                     operation layer).
 *
 * Throws on any failure (surfaces via Next.js error boundary).
 */
export async function restoreAction(
  targetId: string,
  snapshotId: string,
  confirmed: boolean,
): Promise<void> {
  // Guard: never overwrite without an explicit user confirmation.
  if (confirmed !== true) {
    throw new Error("restore requires confirmed: true");
  }

  const cfg = getConfig();
  const repo = createRepo(getDb(), cfg.APP_KEY);

  // ── Resolve target + destination ─────────────────────────────────────────
  const targetRow = repo.getTarget(targetId);
  if (!targetRow) throw new Error(`backup target not found: ${targetId}`);

  const parsed = repo.parseTarget(targetRow);
  const destRow = repo.getDestinationRow(parsed.destination_id);
  if (!destRow) throw new Error(`destination not found: ${parsed.destination_id}`);

  const repoPassword = repo.getDestinationPassword(parsed.destination_id);

  // ── Load the snapshot template copy (captured at backup time) ────────────
  // Snapshot templates are stored as <DATA_DIR>/templates/<snapshotId>-my-<Name>.xml
  const snapshotTemplatesDir = join(cfg.DATA_DIR, "templates");
  const templateFileName = `${snapshotId}-my-${parsed.container_name}.xml`;
  const templatePath = join(snapshotTemplatesDir, templateFileName);

  let templateXml: string;
  try {
    templateXml = readFileSync(templatePath, "utf-8");
  } catch {
    throw new Error(`snapshot template not found at ${templateFileName}`);
  }

  // ── Inspect the current container (to get image + create options) ─────────
  // inspectContainer returns dockerode's ContainerInspectInfo; we narrow it to
  // our ContainerInspect shape (the orchestrator only touches the fields we map).
  const docker = createDockerClient();
  const rawInspect = await inspectContainer(docker, parsed.container_name);
  const inspect: ContainerInspect = {
    Id: rawInspect.Id,
    Name: rawInspect.Name,
    Image: rawInspect.Image ?? rawInspect.Config?.Image ?? "",
    Config: {
      Image: rawInspect.Config?.Image ?? "",
      Env: rawInspect.Config?.Env ?? null,
      Cmd: rawInspect.Config?.Cmd ?? null,
    },
    HostConfig: {
      Binds: rawInspect.HostConfig?.Binds ?? null,
      PortBindings:
        (rawInspect.HostConfig?.PortBindings as ContainerInspect["HostConfig"]["PortBindings"]) ??
        null,
      RestartPolicy: rawInspect.HostConfig?.RestartPolicy ?? null,
    },
    Mounts: (rawInspect.Mounts ?? []).map((m) => ({
      Type: m.Type ?? "",
      Source: m.Source ?? "",
      Destination: m.Destination,
    })),
  };

  // ── Build run recorder ────────────────────────────────────────────────────
  const runs: Parameters<typeof makeRestoreDeps>[0]["runs"] = {
    recordRunStart: (tid, kind) => repo.createRun({ targetId: tid, kind }),
    recordRunFinish: (runId, finish) => repo.finishRun(runId, finish),
  };

  // ── Wire deps + run ───────────────────────────────────────────────────────
  const deps = await makeRestoreDeps({
    confirmed: true,
    containerRef: parsed.container_name,
    containerName: parsed.container_name,
    repoPath: destRow.repo_path,
    repoPassword,
    snapshotId,
    restoreTargetDir: cfg.APPDATA_DIR,
    templateXml,
    flashTemplatesDir: cfg.FLASH_TEMPLATES_DIR,
    inspect,
    targetId,
    runs,
  });

  await restoreContainer(deps);

  revalidatePath("/containers");
  revalidatePath(`/containers/snapshots?targetId=${targetId}`);
}
