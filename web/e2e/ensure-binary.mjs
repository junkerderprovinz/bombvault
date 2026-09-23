// ---------------------------------------------------------------------------
// test:e2e's build-first step.
//
// `playwright test` boots the webServer from a prebuilt binary at the repo
// root (see playwright.config.ts's `binary`) and never builds anything, so a
// missing binary dies as an opaque "Process from config.webServer exited
// early" and a stale one silently serves yesterday's SPA. Both builds run
// here, in CI's order: the SPA first, then the server, because the binary
// embeds web/dist (web/embed.go's `//go:embed all:dist`). Skipping the first
// is the stale-SPA case, and it looks green.
//
// The output name duplicates playwright.config.ts's platform math (win32
// boots bombvault.exe, everything else bombvault) because a static
// package.json script cannot pick a per-platform -o argument. Keep the two
// in sync: the config's `binary` is the only consumer of what this builds.
//
// Runs from package.json as `node e2e/ensure-binary.mjs` (cwd = web/), so
// the repo root is one directory up; the build itself runs there, exactly
// like CI's step.
// ---------------------------------------------------------------------------
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const output = process.platform === "win32" ? "bombvault.exe" : "bombvault";

const web = spawnSync("npm", ["run", "build"], {
  cwd: join(repoRoot, "web"),
  stdio: "inherit",
  shell: process.platform === "win32",
});

if (web.status !== 0) {
  console.error(
    "test:e2e: building the SPA failed (npm run build in web/). The binary " +
      "embeds web/dist, so the gate would boot against a stale or empty " +
      "frontend. Fix the build before running the e2e gate."
  );
  process.exit(web.status ?? 1);
}

const result = spawnSync("go", ["build", "-o", output, "./cmd/bombvault"], {
  cwd: repoRoot,
  stdio: "inherit",
});

if (result.error?.code === "ENOENT") {
  console.error(
    "test:e2e: `go` was not found on PATH. The Playwright webServer boots the " +
      "compiled Go binary (it serves the embedded SPA and the real API), so " +
      "the server under test must be built first. Install Go 1.25+ and retry."
  );
  process.exit(1);
}

if (result.status !== 0) {
  console.error(
    `test:e2e: building the server under test failed (go build -o ${output} ` +
      "./cmd/bombvault). Fix the build before running the e2e gate; the " +
      "webServer cannot boot without the binary."
  );
  process.exit(result.status ?? 1);
}
