import { execFile } from "node:child_process";

// Thin typed wrapper around the restic CLI. P0 implements only version(),
// initRepo() and snapshots(); backup/restore/check/forget arrive in later phases.
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

function run(args: string[], password?: string): Promise<RunResult> {
  return new Promise((resolve, reject) => {
    const env = { ...process.env };
    if (password !== undefined) env.RESTIC_PASSWORD = password;
    execFile("restic", args, { env, maxBuffer: 64 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err) {
        reject(new Error(`restic ${args.join(" ")} failed: ${stderr || err.message}`));
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
