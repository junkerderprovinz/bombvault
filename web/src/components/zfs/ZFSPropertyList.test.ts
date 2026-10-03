import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { ZFS_CREATE_ONLY, ZFS_NOT_APPLIED, zfsPropertyFate } from "./ZFSPropertyList";

const REPO = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "..");
const PROPS_GO = readFileSync(join(REPO, "internal", "zfs", "props.go"), "utf8");

/** The quoted keys of one `var <name> = map[string]… { … }` block. */
function goKeys(name: string): string[] {
  const start = PROPS_GO.indexOf(`var ${name} = map[string]`);
  expect(start, `${name} not found in internal/zfs/props.go`).toBeGreaterThanOrEqual(0);
  const end = PROPS_GO.indexOf("\n}", start);
  return [...PROPS_GO.slice(start, end).matchAll(/^\s+"([^"]+)":/gm)].map((m) => m[1]).sort();
}

describe("the page says what the server does with each property", () => {
  it("knows the same creation-time properties as the server", () => {
    expect([...ZFS_CREATE_ONLY].sort()).toEqual(goKeys("createOnlyDefaults"));
  });

  it("knows the same properties the server never sets", () => {
    expect([...ZFS_NOT_APPLIED].sort()).toEqual(goKeys("notApplied"));
  });

  it("passes a creation-time property to a new dataset but not to an existing one", () => {
    expect(zfsPropertyFate("casesensitivity", "new")).toBe("applied");
    expect(zfsPropertyFate("casesensitivity", "existing")).toBe("createOnly");
    expect(zfsPropertyFate("mountpoint", "new")).toBe("notApplied");
    expect(zfsPropertyFate("compression", "existing")).toBe("applied");
  });
});
