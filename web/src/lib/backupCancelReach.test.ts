// Every domain that can run a backup can stop one. The server registers a
// cancel for containers, VMs, folder sets, flash and config, and
// POST /api/backup/cancel accepts all five keys, so every page's card needs the
// button. Each page is correct on its own terms, so only a scan across the
// files can see one that lacks it, as pageHeading.test.ts does for headings.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const PAGES = join(dirname(fileURLToPath(import.meta.url)), "..", "pages");

// page -> the exact cancel key that page's card must send, as it appears in the
// source. The keys are the server's own, from registerBackupCancel.
const DOMAINS: { file: string; key: string }[] = [
  { file: "Containers.tsx", key: "`container:${container.name}`" },
  { file: "VMs.tsx", key: "`vm:${vm.libvirtName}`" },
  { file: "Files.tsx", key: "`files:${set.name}`" },
  { file: "Flash.tsx", key: '"flash"' },
  { file: "Config.tsx", key: '"config"' },
];

describe("the cancel control", () => {
  it.each(DOMAINS)("$file offers it, with the server's own key", ({ file, key }) => {
    const src = readFileSync(join(PAGES, file), "utf8");
    expect(
      src,
      `${file} has no BackupCancelButton. The server registers a cancel for this domain and the\n` +
        `endpoint accepts its key, so a running backup here can be stopped - there is just no way\n` +
        `to ask for it. Four of the five pages were in this state for a whole release.`,
    ).toContain("<BackupCancelButton");
    expect(
      src,
      `${file} renders the cancel button with a key other than ${key}. It must be the exact key\n` +
        `the backend registered the run under, or the POST answers cancelled:false and the button\n` +
        `silently does nothing - which is the original #200 defect, in a new place.`,
    ).toContain(`cancelKey={${key}}`);
  });

  it.each(DOMAINS)("$file does not offer it during a restore", ({ file }) => {
    const src = readFileSync(join(PAGES, file), "utf8");
    const at = src.indexOf("<BackupCancelButton");
    expect(at, `${file} has no BackupCancelButton`).toBeGreaterThan(-1);
    // The guard sits immediately above the button; take the preceding few lines.
    const guard = src.slice(Math.max(0, at - 400), at);
    expect(
      guard,
      `${file} does not gate its cancel button on phase !== "restore". A restore has its own\n` +
        `cancel with its own warning about a half-restored target, and two cancels on one card\n` +
        `that mean different things is worse than none.`,
    ).toMatch(/phase\s*!==\s*"restore"/);
    expect(
      guard,
      `${file} does not gate its cancel button on progress.active, so a finished run's last\n` +
        `frame leaves a button that can only ever answer "nothing to cancel".`,
    ).toMatch(/\.active/);
  });
});
