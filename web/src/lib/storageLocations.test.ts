import { describe, expect, it } from "vitest";
import type { StorageLocation, StorageLocationSection } from "./api";
import { canFollow, credentialKind, deviating, fillOf, keepCounts, rcloneRemote, retentionKeep, sectionDomains } from "./storageLocations";

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(over: Partial<StorageLocationSection> = {}): StorageLocationSection {
  return {
    domain: "containers",
    use: "copy",
    where: "",
    enabled: true,
    immutable: false,
    retention: KEEP,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "destination:d1",
    object: "destination",
    kind: "offsite",
    provider: "",
    backend: "rest",
    name: "NAS",
    where: "rest:https://nas/bombvault",
    enabled: true,
    offPremises: true,
    sections: [],
    protection: { immutable: false, testable: true },
    capacity: {},
    ...over,
  };
}

describe("fillOf", () => {
  it("reads the used share of a measured volume", () => {
    expect(fillOf({ usedBytes: 25, totalBytes: 100 })).toEqual({ state: "measured", share: 0.25, warn: false });
  });

  it("rounds the forecast to whole weeks and warns when the volume fills soon", () => {
    expect(fillOf({ usedBytes: 90, totalBytes: 100, weeksToFull: 2.6 })).toEqual({ state: "measured", share: 0.9, weeks: 3, warn: true });
    expect(fillOf({ usedBytes: 10, totalBytes: 100, weeksToFull: 33.2 })).toMatchObject({ weeks: 33, warn: false });
  });

  it("never forecasts less than a week or names a week count past a year", () => {
    expect(fillOf({ usedBytes: 99, totalBytes: 100, weeksToFull: 0.2 })).toMatchObject({ weeks: 1 });
    expect(fillOf({ usedBytes: 1, totalBytes: 100, weeksToFull: 400 })).toMatchObject({ weeks: 53 });
  });

  it("tells a backend that reports no room from one nobody asked yet", () => {
    expect(fillOf({ unsupported: true, storedBytes: 5 })).toEqual({ state: "unsupported" });
    expect(fillOf({ storedBytes: 5 })).toEqual({ state: "unmeasured" });
  });

  it("keeps the share inside the vessel when the figures disagree", () => {
    expect(fillOf({ usedBytes: 150, totalBytes: 100 })).toMatchObject({ share: 1 });
  });
});

describe("the sections of a location", () => {
  it("names each domain once, however many sections it has", () => {
    const loc = location({
      sections: [section({ domain: "vms" }), section({ domain: "vms", use: "home", targetId: "t1" }), section({ domain: "flash" })],
    });
    expect(sectionDomains(loc)).toEqual(["vms", "flash"]);
  });

  it("lists the sections that hold a setting themselves", () => {
    const own = section({ domain: "flash", own: ["retention", "limits"] });
    const loc = location({ sections: [section(), own] });
    expect(deviating(loc, "retention")).toEqual([own]);
    expect(deviating(loc, "compression")).toEqual([]);
  });

  it("leaves out a direct repository, which carries its copy's settings", () => {
    const copy = section({ targetId: "t1", own: ["retention"] });
    const home = section({ use: "home", targetId: "t1", repoId: "r1", own: ["retention"] });
    expect(deviating(location({ sections: [copy, home] }), "retention")).toEqual([copy]);
  });
});

describe("canFollow", () => {
  it("lets a target made from a destination follow it again", () => {
    expect(canFollow(location(), section({ targetId: "t1" }))).toBe(true);
  });

  it("refuses the copy a domain's off-site settings describe", () => {
    expect(canFollow(location(), section({ targetId: "t1", primary: true }))).toBe(false);
  });

  it("lets a domain with a local rule of its own go back to the shared one", () => {
    const path = location({ id: "path:abc", object: "path", kind: "local", backend: "local" });
    expect(canFollow(path, section({ use: "home" }))).toBe(true);
  });

  it("has nothing to follow on a target that is a location by itself", () => {
    const target = location({ id: "target:t1", object: "target" });
    expect(canFollow(target, section({ targetId: "t1" }))).toBe(false);
  });
});

describe("keep counts", () => {
  it("round-trips between the server's fields and the plan's order", () => {
    expect(keepCounts(KEEP)).toEqual([0, 7, 4, 6, 1]);
    expect(retentionKeep(keepCounts(KEEP))).toEqual(KEEP);
  });
});

describe("addresses", () => {
  it("finds the rclone remote an address goes through", () => {
    expect(rcloneRemote("rclone:storagebox:bombvault/containers")).toBe("storagebox");
    expect(rcloneRemote("s3:https://host/bucket")).toBeNull();
  });

  it("knows which backends sign in with stored credentials", () => {
    expect(credentialKind("s3")).toBe("s3");
    expect(credentialKind("rest")).toBe("rest");
    expect(credentialKind("rclone")).toBeNull();
    expect(credentialKind("local")).toBeNull();
  });
});
