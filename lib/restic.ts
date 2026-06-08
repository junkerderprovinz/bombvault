import { execFile } from "node:child_process";

// Thin typed wrapper around the restic CLI. P0 implements only version(),
// initRepo() and snapshots(); backup/restore/stats arrive in P1.
// The repo password is passed via RESTIC_PASSWORD in the child env (never argv,
// so it stays out of the process list). --json is parsed where restic emits it.

export interface ResticSnapshot {
  id: string;
  short_id: string;
  time: string;
  paths: string[];
  hostname: string;
  tags?: string[];
}

interface RunResult {
  stdout: string;
  stderr: string;
}

// Extract the restic subcommand from an argv for a sanitized error message.
// Skips leading flags (and the value after the only flag we pass with one, -r),
// returning the first bare token; falls back to "command".
function subcommandOf(args: string[]): string {
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "-r") {
      i++; // skip the repo path value
      continue;
    }
    if (a.startsWith("-")) continue;
    return a;
  }
  return "command";
}

function run(args: string[], password?: string): Promise<RunResult> {
  return new Promise((resolve, reject) => {
    const env = { ...process.env };
    if (password !== undefined) env.RESTIC_PASSWORD = password;
    execFile("restic", args, { env, maxBuffer: 64 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err) {
        // SEC-006: never interpolate the full argv (which may carry a repo path,
        // host detail or secret) into a thrown/propagated error. Log the details
        // server-side; throw a message scoped to the subcommand only. The
        // subcommand is the first non-flag, non-flag-value token (e.g. for
        // ["-r", localPath, "init"] it is "init").
        const subcommand = subcommandOf(args);
        // eslint-disable-next-line no-console
        console.error(`restic ${args.join(" ")} failed: ${stderr || err.message}`);
        reject(new Error(`restic ${subcommand} failed`));
        return;
      }
      resolve({ stdout, stderr });
    });
  });
}

/** Returns true when the restic binary is callable; false otherwise. */
export async function resticAvailable(): Promise<boolean> {
  try {
    await run(["version"]);
    return true;
  } catch {
    return false;
  }
}

/** restic version, e.g. "0.17.3". Parsed from `restic version` text output. */
export async function version(): Promise<string> {
  const { stdout } = await run(["version"]);
  const m = stdout.match(/restic\s+(\d+\.\d+\.\d+)/i);
  if (m) return m[1];
  if (stdout.trim() === "") throw new Error("restic version: unexpected output");
  return stdout.trim();
}

/** Initialise a local restic repository at `localPath`. */
export async function initRepo(localPath: string, password: string): Promise<void> {
  await run(["-r", localPath, "init"], password);
}

/** Parse restic `snapshots --json` stdout safely. Exported for unit testing. */
export function parseSnapshotsJson(stdout: string): ResticSnapshot[] {
  const s = stdout.trim();
  return s ? (JSON.parse(s) as ResticSnapshot[]) : [];
}

/** List snapshots in `repo` as parsed JSON. */
export async function snapshots(repo: string, password: string): Promise<ResticSnapshot[]> {
  const { stdout } = await run(["-r", repo, "snapshots", "--json"], password);
  return parseSnapshotsJson(stdout);
}

// ---------------------------------------------------------------------------
// P1 — backup / restore / stats
// ---------------------------------------------------------------------------

export interface BackupSummary {
  snapshotId: string;
  bytesAdded: number;
  totalBytesProcessed: number;
}

/**
 * Build the argv for `restic backup --json`.
 * Pure — no I/O, no password in argv.
 */
export function buildBackupArgs(
  repo: string,
  paths: string[],
  tags: string[],
): string[] {
  const tagArgs: string[] = [];
  for (const tag of tags) {
    tagArgs.push("--tag", tag);
  }
  // SEC-103: `--` (end-of-options) before the positional paths so a path that
  // begins with `-` can never be reinterpreted as a restic flag (arg injection).
  return ["-r", repo, "backup", "--json", ...tagArgs, "--", ...paths];
}

/**
 * Build the argv for `restic restore --target <dir> -- <snapshotId>`.
 * Pure — no I/O, no password in argv.
 */
export function buildRestoreArgs(
  repo: string,
  snapshotId: string,
  targetDir: string,
): string[] {
  // SEC-103: `--` (end-of-options) before the positional snapshotId so a value
  // beginning with `-` can never be reinterpreted as a restic flag. The
  // snapshotId is additionally hex-validated at the action layer (SEC-103/104).
  return ["-r", repo, "restore", "--target", targetDir, "--", snapshotId];
}

/**
 * Parse a multi-line `restic backup --json` stdout stream.
 * Each line is tried independently (malformed lines are skipped).
 * Picks the line with `message_type === "summary"` and maps its fields.
 * Throws if no summary line is found.
 */
export function parseBackupSummary(stdout: string): BackupSummary {
  for (const line of stdout.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    let obj: Record<string, unknown>;
    try {
      obj = JSON.parse(trimmed) as Record<string, unknown>;
    } catch {
      continue;
    }
    if (obj["message_type"] !== "summary") continue;
    return {
      snapshotId: obj["snapshot_id"] as string,
      bytesAdded: obj["data_added"] as number,
      totalBytesProcessed: obj["total_bytes_processed"] as number,
    };
  }
  throw new Error("restic backup: no summary line found in output");
}

/** Run a restic backup and return the parsed summary. */
export async function backup(
  repo: string,
  paths: string[],
  tags: string[],
  password: string,
): Promise<BackupSummary> {
  const args = buildBackupArgs(repo, paths, tags);
  const { stdout } = await run(args, password);
  return parseBackupSummary(stdout);
}

/** Restore a snapshot to a local directory. */
export async function restore(
  repo: string,
  snapshotId: string,
  targetDir: string,
  password: string,
): Promise<void> {
  const args = buildRestoreArgs(repo, snapshotId, targetDir);
  await run(args, password);
}

/** Return raw stats JSON string for `repo`. */
export async function stats(repo: string, password: string): Promise<string> {
  const { stdout } = await run(["-r", repo, "stats", "--json"], password);
  return stdout;
}
