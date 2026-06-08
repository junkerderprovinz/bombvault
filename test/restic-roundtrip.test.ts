import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { backup, restore, snapshots, initRepo } from "../lib/restic";

// Skip the whole suite gracefully when the restic binary is not installed.
// CI installs restic and runs this for real; the Dockerfile bundles it.
let skip: string | false = false;
try {
  execFileSync("restic", ["version"], { stdio: "ignore" });
} catch {
  skip = "restic binary not installed";
}

const PW = "roundtrip-test-password";
const KNOWN_CONTENT = "hello from the roundtrip test\n";
const FILE_NAME = "hello.txt";
const TAGS = ["container:roundtrip-test", "p1"];

test(
  "restic roundtrip: backup → snapshot listed → wipe → restore → byte-identical content",
  { skip },
  async () => {
    // Set up temp dirs: a dedicated parent to anchor the absolute path nesting.
    const base = mkdtempSync(join(tmpdir(), "bv-roundtrip-"));
    const repoDir = join(base, "repo");
    const srcDir = join(base, "src");
    const targetDir = join(base, "target");

    mkdirSync(repoDir, { recursive: true });
    mkdirSync(srcDir, { recursive: true });
    mkdirSync(targetDir, { recursive: true });

    // Write a known file into the source directory.
    writeFileSync(join(srcDir, FILE_NAME), KNOWN_CONTENT, "utf8");

    // 1. Initialise the repo.
    await initRepo(repoDir, PW);

    // 2. Backup the source directory with tags.
    const summary = await backup(repoDir, [srcDir], TAGS, PW);
    assert.match(
      summary.snapshotId,
      /^[0-9a-f]{8,}$/,
      `expected a hex snapshot id, got "${summary.snapshotId}"`,
    );
    assert.ok(typeof summary.bytesAdded === "number", "bytesAdded should be a number");

    // 3. Assert the snapshot is listed and carries our tags.
    const snaps = await snapshots(repoDir, PW);
    assert.equal(snaps.length, 1, "exactly one snapshot expected after first backup");
    const snap = snaps[0];
    assert.ok(
      snap.id.startsWith(summary.snapshotId) || summary.snapshotId.startsWith(snap.short_id),
      `snapshot id mismatch: listed=${snap.id} summary=${summary.snapshotId}`,
    );
    for (const tag of TAGS) {
      assert.ok(
        snap.tags?.includes(tag),
        `expected snapshot to carry tag "${tag}", got ${JSON.stringify(snap.tags)}`,
      );
    }

    // 4. Wipe the source directory so there is nothing left to confuse the restore.
    rmSync(srcDir, { recursive: true, force: true });

    // 5. Restore to targetDir.
    await restore(repoDir, summary.snapshotId, targetDir, PW);

    // 6. restic nests the absolute source path under --target.
    //    srcDir was e.g. /tmp/bv-roundtrip-xxx/src, so restic recreates
    //    <targetDir>/tmp/bv-roundtrip-xxx/src/hello.txt (the full absolute path).
    //    On Windows the drive letter becomes e.g. <targetDir>/C/tmp/.../hello.txt
    //    but CI runs on Linux so we handle the POSIX case.
    //
    //    Strip the leading separator from srcDir to build the nested path.
    const srcDirRelative = srcDir.replace(/^[/\\]+/, "");
    const restoredFile = join(targetDir, srcDirRelative, FILE_NAME);

    const restoredContent = readFileSync(restoredFile, "utf8");
    assert.equal(
      restoredContent,
      KNOWN_CONTENT,
      `restored content should be byte-identical to the original`,
    );
  },
);
