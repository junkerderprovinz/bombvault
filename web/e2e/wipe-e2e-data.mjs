// ---------------------------------------------------------------------------
// Playwright webServer pre-command — the fresh-DB half of the harness
// guarantee. playwright.config.ts composes `node web/e2e/wipe-e2e-data.mjs &&
// <binary>` so this runs immediately before the binary boots, in the
// webServer's cwd (the repo root).
//
// Why a pre-command and not globalSetup: Playwright starts the webServer
// BEFORE globalSetup runs (the server is a plugin in the global-setup task
// list), so a wipe there would land after the binary has already created and
// opened its SQLite file — an EBUSY failure on Windows, and on POSIX a
// silent no-op where the server keeps writing to the unlinked inode. Before
// the process exists is the only moment the wipe is both possible and
// complete.
//
// Why the wipe at all: .playwright-data/ is gitignored and otherwise never
// cleaned, so the harness DB persisted across runs and the specs' fresh-DB
// assumptions (auth disabled, every domain gate off, bar = 4 slots, sheet =
// Recovery-only) silently degraded into "fresh only after a manual delete" —
// the narrow-viewport backstop observed a de/fr bv-lang display pref
// surviving on it into later runs.
//
// Fails LOUDLY rather than booting a poisoned DB: a stale bombvault.exe from
// a previous run (the known Windows teardown hang) still holds the SQLite
// file and the delete will fail. Kill it and re-run — see the error below.
// ---------------------------------------------------------------------------
import { rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// web/e2e/ -> repo root: the same root the config's DATA_DIR
// ("./.playwright-data") resolves against via the webServer's cwd.
const dataDir = join(dirname(fileURLToPath(import.meta.url)), "..", "..", ".playwright-data");

try {
  rmSync(dataDir, { recursive: true, force: true });
} catch (err) {
  console.error(
    `[wipe-e2e-data] could not remove ${dataDir} — a previous run's ` +
      "bombvault.exe is probably still holding it (the known Windows teardown " +
    'hang). Kill it ("taskkill /IM bombvault.exe /F" on Windows) and re-run. ' +
      `Cause: ${err}`,
  );
  process.exit(1);
}
