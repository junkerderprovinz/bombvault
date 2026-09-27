// ---------------------------------------------------------------------------
// Playwright webServer pre-command: the fresh-DB half of the harness
// guarantee. playwright.config.ts composes `node web/e2e/wipe-e2e-data.mjs &&
// <binary>` so this runs immediately before the binary boots, in the
// webServer's cwd (the repo root).
//
// Why a pre-command and not globalSetup: Playwright starts the webServer
// before globalSetup runs (the server is a plugin in the global-setup task
// list), so a wipe there would land after the binary has already created and
// opened its SQLite file: an EBUSY failure on Windows, and on POSIX a
// silent no-op where the server keeps writing to the unlinked inode. Before
// the process exists is the only moment the wipe is both possible and
// complete.
//
// Why the wipe at all: .playwright-data/ is gitignored and otherwise never
// cleaned, so the harness DB persisted across runs and the specs' fresh-DB
// assumptions (auth disabled, every domain gate off, bar = Dashboard,
// Recovery, Containers + More, sheet = Settings + the view toggle) silently
// degraded into "fresh only after a manual delete",
// the narrow-viewport backstop observed a de/fr bv-lang display pref
// surviving on it into later runs.
//
// Fails loudly rather than booting a poisoned DB: a previous run's harness
// binary (the known Windows teardown hang) still holds the SQLite file and
// the delete will fail. Kill it and re-run; the error below names the
// candidate(s) by port, never by image name.
// ---------------------------------------------------------------------------
import { execFileSync } from "node:child_process";
import { rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// web/e2e/ -> repo root: the same root the config's DATA_DIR
// ("./.playwright-data") resolves against via the webServer's cwd.
const dataDir = join(dirname(fileURLToPath(import.meta.url)), "..", "..", ".playwright-data");

// The harness port: the same default and override playwright.config.ts uses
// (the webServer boots the binary with PORT from this same value). It is the
// only safe handle this script has: the process listening on it is the
// throwaway harness instance, while a name-based sweep would name the
// developer's own running BombVault just as readily as the stale one.
const harnessPort = process.env.E2E_PORT ?? "3000";

// The PIDs listening on the harness port, resolved by the OS. netstat needs
// no admin (-ano: numeric, all sockets, owning PID); lsof -ti prints bare
// PIDs and exits non-zero on no match, which is the same "none listening"
// as an empty list. netstat resolves through System32 absolutely: a spawned
// process on Windows does not go through PATHEXT the way a shell does, and
// a bare "netstat" failed with ENOENT live in an environment whose inherited
// PATH was thin (the same reason playwright.config.ts splices in the node
// interpreter absolutely).
const systemRoot = process.env.SystemRoot ?? "C:\\Windows";
const netstat = join(systemRoot, "System32", "netstat.exe");

function harnessPortPids() {
  try {
    if (process.platform === "win32") {
      const out = execFileSync(netstat, ["-ano"], { encoding: "utf8" });
      const listener = new RegExp(`^\\s*TCP\\s+\\S+:${harnessPort}\\s+\\S+\\s+LISTENING\\s+(\\d+)`, "gm");
      return [...new Set([...out.matchAll(listener)].map((m) => Number(m[1])))].filter(
        (n) => Number.isFinite(n) && n > 0
      );
    }
    const out = execFileSync("lsof", ["-ti", `:${harnessPort}`], { encoding: "utf8" });
    return [...new Set(out.split("\n").map(Number))].filter((n) => Number.isFinite(n) && n > 0);
  } catch {
    return [];
  }
}

try {
  rmSync(dataDir, { recursive: true, force: true });
} catch (err) {
  const pids = harnessPortPids();
  const who =
    pids.length > 0
      ? `PID(s) ${pids.join(", ")} (listening on the harness port ${harnessPort})`
      : `whatever listens on the harness port ${harnessPort} (find it by port, not by name)`;
  console.error(
    `[wipe-e2e-data] could not remove ${dataDir}: a previous run's ` +
      "harness binary is probably still holding it (the known Windows teardown " +
      `hang). Kill ${who}, by PID only, and re-run. Never kill by image name: ` +
      "an image-name kill takes down a real BombVault instance of yours just " +
      'as happily as the stale harness one. Windows: "taskkill /PID <pid> /F"; ' +
      `POSIX: "kill <pid>". Cause: ${err}`,
  );
  process.exit(1);
}
