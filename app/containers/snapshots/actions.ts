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
import { assertValidSnapshotId, resolveSnapshotTemplatePath } from "./validate";

/**
 * Server action: restore a container from a given snapshot.
 *
 * @param targetId   - The backup_target id (bound via the form action).
 * @param snapshotId - The restic snapshot id to restore from (bound via the
 *                     form action). Must be a hex id of 8–64 chars (SEC-103/104).
 * @param formData   - The submitted form data. The restore only proceeds when
 *                     the required `confirm` checkbox was actually ticked
 *                     (`confirm === "on"`); otherwise the action throws. This is
 *                     a guard against accidental/scripted-without-confirm
 *                     destructive restores — it is NOT an authentication layer.
 *
 * Throws on any failure (surfaces via Next.js error boundary).
 */
export async function restoreAction(
  targetId: string,
  snapshotId: string,
  formData: FormData,
): Promise<void> {
  // The confirm checkbox (name="confirm", required) submits the value "on" only
  // when ticked. Derive the real confirmation from the submitted form, never a
  // hardcoded constant.
  const confirmed = formData.get("confirm") === "on";

  // Guard: never overwrite without an explicit user confirmation.
  if (confirmed !== true) {
    throw new Error("restore requires confirmed: true");
  }

  // SEC-103/104: snapshotId is used in argv and in a template filename. Reject
  // anything that is not a plain restic hex id BEFORE doing any work — this
  // blocks both restic arg injection and path traversal at the source.
  assertValidSnapshotId(snapshotId);

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
  // SEC-104: build + verify the template path stays inside the templates dir
  // (defence-in-depth on top of the hex validation above).
  const snapshotTemplatesDir = join(cfg.DATA_DIR, "templates");
  const templatePath = resolveSnapshotTemplatePath(
    snapshotTemplatesDir,
    snapshotId,
    parsed.container_name,
  );

  let templateXml: string;
  try {
    templateXml = readFileSync(templatePath, "utf-8");
  } catch {
    throw new Error("snapshot template not found");
  }

  // ── Inspect the current container (to get image + create options) ─────────
  // inspectContainer returns dockerode's ContainerInspectInfo; we narrow it to
  // our ContainerInspect shape (the orchestrator only touches the fields we map).
  const docker = createDockerClient();
  const rawInspect = await inspectContainer(docker, parsed.container_name);
  const rawHost = rawInspect.HostConfig;
  const inspect: ContainerInspect = {
    Id: rawInspect.Id,
    Name: rawInspect.Name,
    Image: rawInspect.Image ?? rawInspect.Config?.Image ?? "",
    Config: {
      Image: rawInspect.Config?.Image ?? "",
      Env: rawInspect.Config?.Env ?? null,
      Cmd: rawInspect.Config?.Cmd ?? null,
      // SEC-105: preserve the process user.
      User: rawInspect.Config?.User ?? null,
    },
    HostConfig: {
      Binds: rawHost?.Binds ?? null,
      PortBindings:
        (rawHost?.PortBindings as ContainerInspect["HostConfig"]["PortBindings"]) ??
        null,
      RestartPolicy: rawHost?.RestartPolicy ?? null,
      // SEC-105: preserve security-relevant fields so the recreated container
      // does not silently run with more privilege than the original.
      CapAdd: rawHost?.CapAdd ?? null,
      CapDrop: rawHost?.CapDrop ?? null,
      Privileged: rawHost?.Privileged ?? null,
      SecurityOpt: rawHost?.SecurityOpt ?? null,
      ReadonlyRootfs: rawHost?.ReadonlyRootfs ?? null,
      NetworkMode: rawHost?.NetworkMode ?? null,
      Devices:
        (rawHost?.Devices as ContainerInspect["HostConfig"]["Devices"]) ?? null,
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
  // SEC-102: restic backs up ABSOLUTE paths (e.g. /sources/appdata/<name> under
  // APPDATA_DIR). Restoring with `--target <APPDATA_DIR>` would double-nest the
  // absolute path under the target (…/appdata/sources/appdata/<name>) and leave
  // the container pointing at the un-restored origin. We MUST restore to `/` so
  // the backed-up absolute paths land back at their origin.
  // NOTE: the appdata mount must therefore be writable (rw) for restore to
  // succeed — the original files at APPDATA_DIR are overwritten in place.
  const deps = await makeRestoreDeps({
    confirmed,
    containerRef: parsed.container_name,
    containerName: parsed.container_name,
    repoPath: destRow.repo_path,
    repoPassword,
    snapshotId,
    restoreTargetDir: "/",
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
