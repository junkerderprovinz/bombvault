// The backend sends reason codes; this file is where they become sentences.
// The two lists live in Go, so the test reads them from there rather than
// repeating them: a code added on one side and forgotten on the other fails
// here instead of rendering "Unknown state (snapdir-disabled)" to a user.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { en } from "./i18n";
import {
  ZFS_CODE_KEY,
  ZFS_CODE_VARS,
  ZFS_FIX_KEY,
  ZFS_MEMBER_KEY,
  zfsCodeSentence,
  zfsRunReasonCode,
} from "./zfsCodes";

const REPO = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const CODES_GO = readFileSync(join(REPO, "internal", "zfs", "codes.go"), "utf8");

/** The quoted entries of one `var <name> = []string{ … }` block. */
function goList(name: string): string[] {
  const start = CODES_GO.indexOf(`var ${name} = []string{`);
  expect(start, `${name} not found in internal/zfs/codes.go`).toBeGreaterThanOrEqual(0);
  const end = CODES_GO.indexOf("\n}", start);
  return [...CODES_GO.slice(start, end).matchAll(/"([^"]+)"/g)].map((m) => m[1]);
}

/** Unique {placeholder} names in a translation value. */
function placeholders(value: string): string[] {
  return [...new Set(value.match(/\{([a-zA-Z0-9_]+)\}/g) ?? [])].map((p) => p.slice(1, -1)).sort();
}

describe("every backend reason code has a sentence", () => {
  it("covers exactly the codes the backend sends", () => {
    expect(Object.keys(ZFS_CODE_KEY).sort()).toEqual(goList("AllCodes").sort());
  });

  it("labels exactly the member outcomes plus the new mark", () => {
    expect(Object.keys(ZFS_MEMBER_KEY).sort()).toEqual([...goList("MemberOutcomes"), "new"].sort());
  });

  it("points every sentence and fix at a key the tables carry", () => {
    const missing = [...Object.values(ZFS_CODE_KEY), ...Object.values(ZFS_FIX_KEY)].filter(
      (key) => !(key in en),
    );
    expect(missing).toEqual([]);
  });
});

describe("every placeholder has a source", () => {
  it.each(Object.entries(ZFS_CODE_KEY))("%s names the field that fills it", (code, key) => {
    const want = placeholders(en[key]);
    const got = Object.keys(ZFS_CODE_VARS[code as keyof typeof ZFS_CODE_VARS] ?? {}).sort();
    expect(got).toEqual(want);
  });

  it("fills the placeholders it was given", () => {
    const t = (key: string) => en[key as keyof typeof en];
    expect(zfsCodeSentence(t, "ssh-unreachable", { target: "root@192.168.1.10" })).toContain(
      "root@192.168.1.10",
    );
    expect(zfsCodeSentence(t, "leftover-snapshots", { leftoverCount: 3 })).toContain("3");
    expect(zfsCodeSentence(t, "not-visible", { hostMountpoint: "/mnt/cache/appdata" })).toContain(
      "/mnt/cache/appdata",
    );
    expect(zfsCodeSentence(t, "name-too-long", { max: 255, names: ["a", "b"] })).toContain("a, b");
  });

  it("names a code it does not know instead of dropping it", () => {
    const t = (key: string) => en[key as keyof typeof en];
    expect(zfsCodeSentence(t, "pool-on-fire", {})).toContain("pool-on-fire");
  });
});

describe("a run's error", () => {
  it("reads the code at the head of what the orchestrator wrote", () => {
    expect(zfsRunReasonCode("pre-snapshot-failed: hook exited 1: about to fail")).toEqual({
      code: "pre-snapshot-failed",
      detail: "hook exited 1: about to fail",
    });
    expect(zfsRunReasonCode("snapshot-failed")).toEqual({ code: "snapshot-failed", detail: "" });
  });

  it("reads the code at the end of a refusal before the run began", () => {
    expect(zfsRunReasonCode("zfs backup: tank/app: no dataset is readable [nothing-readable]")).toEqual({
      code: "nothing-readable",
      detail: "",
    });
  });

  it("leaves an error without a code of its own to the run reasons", () => {
    expect(zfsRunReasonCode("cancelled by the user")).toBeNull();
    expect(zfsRunReasonCode("restic backup failed: Fatal: wrong password")).toBeNull();
    expect(zfsRunReasonCode("stopped by the stall guard after 1 hour without progress while reading tank/app")).toBeNull();
  });
});
