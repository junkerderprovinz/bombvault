import { join, resolve, sep } from "node:path";

// Pure restore-input validators. Kept dependency-free (no "use server", no DB,
// no Next.js) so they can be unit-tested in isolation. The destructive restore
// path consumes these BEFORE doing any work.

/** A plain restic snapshot id: lowercase hex, 8–64 chars. */
const SNAPSHOT_ID_RE = /^[0-9a-f]{8,64}$/;

/**
 * SEC-103/104: true iff `snapshotId` is a plain restic hex id. A snapshotId is
 * used both in restic argv and in a template filename, so a non-hex value could
 * enable arg injection (SEC-103) or path traversal (SEC-104). Validate first.
 */
export function isValidSnapshotId(snapshotId: string): boolean {
  return SNAPSHOT_ID_RE.test(snapshotId);
}

/** Throws "invalid snapshot id" unless `snapshotId` is a plain restic hex id. */
export function assertValidSnapshotId(snapshotId: string): void {
  if (!isValidSnapshotId(snapshotId)) {
    throw new Error("invalid snapshot id");
  }
}

/**
 * SEC-104: build the snapshot-template path and assert it stays inside
 * `snapshotTemplatesDir` (defence-in-depth on top of the hex validation — the
 * container_name is also interpolated into the filename). Returns the resolved,
 * verified absolute path; throws "invalid snapshot template path" on traversal.
 */
export function resolveSnapshotTemplatePath(
  snapshotTemplatesDir: string,
  snapshotId: string,
  containerName: string,
): string {
  const fileName = `${snapshotId}-my-${containerName}.xml`;
  const templatePath = join(snapshotTemplatesDir, fileName);
  if (!resolve(templatePath).startsWith(resolve(snapshotTemplatesDir) + sep)) {
    throw new Error("invalid snapshot template path");
  }
  return templatePath;
}
