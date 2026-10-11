// Every kind that can run a backup can stop one. The server registers a cancel
// under the key the run reports its progress with, and POST /api/backup/cancel
// takes that key. A backup shows in two places, the row of the list and the
// head of the entry's page, and each is correct on its own terms, so only a
// look across both sees one that lacks the control or sends another key.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import type { BackupItem } from "./api";
import { itemProgressKey } from "./backupList";

const COMPONENTS = join(dirname(fileURLToPath(import.meta.url)), "..", "components");

const PLACES = ["backups/BackupRow.tsx", "entry/EntryHead.tsx"];

function item(kind: BackupItem["kind"], key: string, name: string): BackupItem {
  return { kind, key, name } as BackupItem;
}

describe("the key a running backup is stopped by", () => {
  // The keys are the server's own, from registerBackupCancel.
  it.each([
    { entry: item("container", "immich", "immich"), key: "container:immich" },
    { entry: item("vm", "win11", "Windows 11"), key: "vm:win11" },
    { entry: item("files", "a1b2c3", "Photos"), key: "files:Photos" },
    { entry: item("zfs", "0f1e2d", "tank/appdata"), key: "zfs:tank/appdata" },
    { entry: item("flash", "flash", "flash"), key: "flash" },
    { entry: item("config", "config", "config"), key: "config" },
  ])("is $key for a $entry.kind entry, as the server registered it", ({ entry, key }) => {
    expect(itemProgressKey(entry)).toBe(key);
  });
});

describe("the cancel control", () => {
  it.each(PLACES)("%s offers it, with the key of the entry's run", (file) => {
    const src = readFileSync(join(COMPONENTS, file), "utf8");
    expect(src, `${file} has no BackupCancelButton, so a backup shown there cannot be stopped from there`).toContain(
      "<BackupCancelButton"
    );
    expect(
      src,
      `${file} renders the cancel button with a key other than itemProgressKey(item). With any other key the\n` +
        `POST answers cancelled:false and the button does nothing.`
    ).toContain("cancelKey={itemProgressKey(item)}");
  });

  it.each(PLACES)("%s does not offer it during a restore", (file) => {
    const src = readFileSync(join(COMPONENTS, file), "utf8");
    const at = src.indexOf("<BackupCancelButton");
    expect(at, `${file} has no BackupCancelButton`).toBeGreaterThan(-1);
    const guard = src.slice(Math.max(0, at - 400), at);
    expect(
      guard,
      `${file} does not gate its cancel button on phase !== "restore". A restore has its own cancel with its\n` +
        `own warning about a half-restored target.`
    ).toMatch(/phase\s*!==\s*"restore"/);
    expect(
      guard,
      `${file} does not gate its cancel button on progress.active, so the last frame of a finished run leaves\n` +
        `a button that can only answer "nothing to cancel".`
    ).toMatch(/\.active/);
  });
});
