import { describe, expect, it } from "vitest";
import { countText, en, type TranslationKey } from "../../../lib/i18n";
import {
  allowLine,
  cloneCommand,
  DEFAULT_KEEP,
  keepCounts,
  keepSummary,
  takeoverCommand,
  targetRoot,
  withOwnCount,
} from "./replicaModel";

const t = (key: TranslationKey, n?: number) => countText(en[key], "en", n);

describe("replica keep rules", () => {
  it("start a new replica at a month of dailies and weeklies", () => {
    expect(keepCounts(DEFAULT_KEEP)).toEqual([0, 7, 3, 0, 0]);
    expect(keepSummary(t, DEFAULT_KEEP)).toBe("7 daily, 3 weekly");
  });

  it("read a preset's numbers rather than the stored own ones", () => {
    expect(keepCounts({ preset: "long", own: [0, 7, 3, 0, 0] })).toEqual([0, 14, 8, 12, 3]);
    expect(keepSummary(t, { preset: "balanced", own: [0, 0, 0, 0, 0] })).toBe("7 daily, 4 weekly, 6 monthly, 1 yearly");
  });

  it("clamp an own number to a whole count of at least zero", () => {
    expect(withOwnCount([0, 7, 3, 0, 0], 1, "-4")).toEqual([0, 0, 3, 0, 0]);
    expect(withOwnCount([0, 7, 3, 0, 0], 4, "2.7")).toEqual([0, 7, 3, 0, 2]);
    expect(withOwnCount([0, 7, 3, 0, 0], 0, "")).toEqual([0, 7, 3, 0, 0]);
  });
});

describe("target commands", () => {
  const path = "backup/bombvault-replica/tower/cache/appdata";

  it("find the root above the server folder", () => {
    expect(targetRoot(path, "cache/appdata")).toBe("backup/bombvault-replica");
    expect(targetRoot("tank/r/tower/cache", "cache")).toBe("tank/r");
  });

  it("clone a snapshot beside the replica, not inside it", () => {
    expect(cloneCommand(path, "cache/appdata", "bombvault-replica-20261006014100")).toBe(
      "zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata",
    );
  });

  it("make the replica writable and mounted for a takeover", () => {
    expect(takeoverCommand(path)).toBe(
      `zfs inherit -r readonly ${path} && zfs inherit -r canmount ${path} && zfs mount -a`,
    );
  });

  it("delegate only what a receive needs to a user other than root", () => {
    expect(allowLine("bv")).toBe("zfs allow bv receive,create,mount,rollback,destroy,userprop <pool>");
  });
});
