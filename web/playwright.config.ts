// ---------------------------------------------------------------------------
// Playwright harness: the responsive-regression gate for the mobile shell
// and the Dashboard's phone layout.
//
// Why the compiled Go binary and not `vite preview` or MSW: the binary serves
// the embedded SPA and the real API (web/embed.go), `vite preview` serves zero
// API, and MSW would be a new runtime dependency, the harness adds none (the
// exact-pinned @playwright/test devDependency is the single sanctioned
// addition, so Renovate moves the npm package and the browser binaries in one
// lockstep commit).
//
// Browser engines: Chromium + WebKit. Safari is half the mobile audience;
// headless WebKit catches layout divergence early.
//
// Server readiness follows the CI Docker boot smoke precedent
// (.github/workflows/build.yml "Run container + health check"): fresh APP_KEY,
// HTTP_ONLY=true, poll /api/health until it answers, but via webServer.url
// polling, not hand-rolled shell loops.
//
// Auth: a fresh DATA_DIR database ships with auth disabled
// (auth_password_hash empty: internal/store/settings.go), so specs assert
// chrome/layout only. No login bypass, no seeded password, no auto-login
// helper: the harness relies on exactly the default a fresh instance has.
// HTTP_ONLY=true replaces any TLS-bypass flag, plain HTTP, nothing to
// ignore; nothing TLS-related leaks into app code.
//
// testMatch is an explicit allow-list of exactly the spec files the harness
// runs: it must never silently pick up a spec whose subject matter the tree
// does not carry (its assertions would fail against a tree that lacks the
// surfaces they guard). Specs join this list as their subject matter lands.
// ---------------------------------------------------------------------------
import { defineConfig, devices } from "@playwright/test";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Repo root: where `go build ./cmd/bombvault` lands the binary. Node-24-safe
// URL math, no import.meta.dirname dependency.
const repoRoot = fileURLToPath(new URL("..", import.meta.url));
// Absolute path on purpose: webServer.command runs inside `cwd` below, so a
// "../bombvault"-style relative path would climb out of the repo. Absolute
// makes the resolution unambiguous regardless of shell semantics.
const binary = join(repoRoot, process.platform === "win32" ? "bombvault.exe" : "bombvault");
// The webServer command is a wipe-then-boot pipeline, and the wipe has to run
// here rather than in globalSetup: Playwright starts the webServer before
// globalSetup (the server is a plugin in the global-setup task list), so a
// globalSetup-based wipe would land after the binary has already booted and
// opened its SQLite file. The pre-command is the only hook that runs before
// the process exists: which is what makes the "fresh empty-state DB per
// run" guarantee below true rather than aspirational. `&&` keeps it strict:
// a failed wipe (a stale bombvault.exe still holding the DB, the known
// Windows teardown hang) aborts the run instead of booting a poisoned DB.
// Runs through the shell on every platform (Playwright launches `command`
// with shell: true). The node interpreter is spliced in absolutely
// (process.execPath: the exact binary running Playwright, JSON-quoted)
// rather than relied on from PATH: spawned shells on Windows can lack `node`
// on PATH (the same reason this repo never relies on npm/npx shims in
// spawned shells: observed live: "'node' n'est pas reconnu").
const node = JSON.stringify(process.execPath);
const command = `${node} web/e2e/wipe-e2e-data.mjs && "${binary}"`;

// Shrink guards, both load-time (maintainer #244 round 3, test item T5). The
// testMatch allow-list and the project list can each narrow SILENTLY: a
// renamed spec file leaves an allow-list entry that matches zero files and
// Playwright exits 0 having run nothing from it, and a renamed project drops
// its half of every spec the same way. Both are checked here, when the
// config loads, so the gate can only get smaller loudly.
const testMatch = [
  "mobile-shell.spec.ts",
  "home-trigger.spec.ts",
  "desktop-untouched.spec.ts",
  "narrow-viewport.spec.ts",
];

for (const spec of testMatch) {
  if (!existsSync(join(repoRoot, "web", "e2e", spec))) {
    throw new Error(
      `playwright.config.ts: testMatch entry "${spec}" matches no file in ` +
        "web/e2e. A spec was renamed or deleted without updating the " +
        "allow-list, and a stale entry silently runs nothing (exit 0). " +
        "Fix the entry or remove it deliberately."
    );
  }
}

// The exact project roster the specs branch on (narrow-viewport.spec.ts
// reads the names; desktop-untouched rides the desktop pair). A missing name
// halves the gate; an unexpected one runs specs against a context nobody
// wrote assertions for. Both fail the load.
const REQUIRED_PROJECTS = ["desktop-1280", "desktop-768", "mobile-iphone", "mobile-android"];

// The harness port is env-overridable (E2E_PORT) but defaults to the app's
// own 3000. The override exists because 3000 is a popular port: a foreign
// container on this host (a sibling project's dev instance, hit live once)
// must not be killed to run this project's gate, and reuseExistingServer must
// stay false, so the only clean path is another port. The throwaway harness
// boots its own binary either way, no reuse, no foreign state, every
// fresh-DB guarantee below intact.
const e2ePort = process.env.E2E_PORT ?? "3000";

const projects = [
  {
    name: "desktop-1280",
    use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 720 } },
  },
  {
    // 768 wide = the 48rem breakpoint boundary the mobile shell switches at.
    name: "desktop-768",
    use: { ...devices["Desktop Chrome"], viewport: { width: 768, height: 900 } },
  },
  {
    // iPhone 13 descriptor: WebKit, 390×844, isMobile + hasTouch.
    name: "mobile-iphone",
    use: devices["iPhone 13"],
  },
  {
    // Pixel 5 descriptor (Chromium) with the viewport dropped to 360×800 so
    // the narrowest supported width is exercised; only the viewport key is
    // overridden: isMobile/hasTouch/deviceScaleFactor survive the spread.
    name: "mobile-android",
    use: { ...devices["Pixel 5"], viewport: { width: 360, height: 800 } },
  },
];

{
  const actual = projects.map((p) => p.name).sort().join("|");
  const expected = [...REQUIRED_PROJECTS].sort().join("|");
  if (actual !== expected) {
    throw new Error(
      `playwright.config.ts: the project roster is [${actual}] but the specs ` +
        `branch on [${expected}]. A renamed or removed project silently drops ` +
        "its half of every spec; an added one runs them against a context " +
        "nobody wrote assertions for. Align the roster with REQUIRED_PROJECTS."
    );
  }
}

export default defineConfig({
  // A `.only` that survives locally fails the CI run instead of silently
  // shrinking the gate: on CI an only'd run reports "unexpected focused
  // test" and exits non-zero. Local runs stay free to focus.
  forbidOnly: !!process.env.CI,
  // The dot reporter keeps the console readable; the HTML report is written
  // but never opened, so a red CI run uploads real evidence: lint.yml's
  // failure() step picks up web/playwright-report/ (the HTML reporter's
  // default output dir) together with test-results/, where the per-test
  // trace and screenshot configured below land.
  reporter: [["dot"], ["html", { open: "never" }]],
  testDir: "./e2e",
  testMatch,
  use: {
    baseURL: `http://127.0.0.1:${e2ePort}`,
    // Failure evidence, written only where it matters (on the failing test),
    // so green runs produce none of it: the trace replays the run in the
    // HTML report, the screenshot shows what the page actually looked like.
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command,
    cwd: repoRoot,
    url: `http://127.0.0.1:${e2ePort}/api/health`,
    timeout: 120_000,
    // never reuse whatever already listens on the harness port: a running dev
    // BombVault (real settings, enabled domains, a password) would silently
    // take the run's place: APP_KEY/DATA_DIR/HTTP_ONLY below would not be
    // applied, and every fresh-DB assertion (auth off, bar = Dashboard,
    // Recovery, Containers + More, sheet = Settings + the view toggle,
    // sign-out absent) would be evaluated against foreign
    // state. A loud "URL is already used" failure beats a green run against
    // the wrong server: kill the holder and re-run. (Reuse would also skip
    // `command` entirely, silently dropping the fresh-DB wipe, one more
    // reason reuse and this harness do not mix.)
    reuseExistingServer: false,
    env: {
      // Dev-only throwaway constant: 64 lowercase zeros satisfy config.go's
      // [0-9a-f]{64} APP_KEY check. never a real secret; the data dir it
      // unlocks is gitignored scratch.
      APP_KEY: "0".repeat(64),
      // Fresh empty-state DB per run, not per project: Playwright starts
      // one webServer per run and all four projects share it. The dir is
      // wiped by the pre-command before every boot (see `command`);
      // writability is the only fail-fast boot check
      // (cmd/bombvault/main.go ensureDataDirWritable).
      DATA_DIR: "./.playwright-data",
      // Plain HTTP instead of any TLS-bypass flag.
      HTTP_ONLY: "true",
      PORT: e2ePort,
    },
  },
  projects,
});
