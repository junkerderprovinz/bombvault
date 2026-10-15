import { describe, expect, it } from "vitest";
import { snapshotStamp } from "./reltime";

describe("snapshotStamp", () => {
  it("reads the digits at the end of a snapshot name as UTC, whatever the local zone", () => {
    expect(snapshotStamp("tank/vms/testvm@bombvault-before-restore-20261015201434")).toBe(1792095274);
    expect(snapshotStamp("tank/appdata@bombvault-prerestore-20261015201434")).toBe(1792095274);
  });

  it("finds no time in a name without a stamp", () => {
    expect(snapshotStamp("tank/appdata@manual")).toBeNull();
    expect(snapshotStamp("tank/vms/testvm@bombvault-before-restore-")).toBeNull();
  });
});
