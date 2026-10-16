import { describe, expect, it } from "vitest";
import type { StorageLocation, StorageLocationSection } from "./api";
import { en } from "./i18n";
import {
  canFollow,
  credentialKind,
  deviating,
  distinctNames,
  fillOf,
  keepCounts,
  localName,
  namedLocal,
  rcloneRemote,
  retentionKeep,
  sectionDomains,
} from "./storageLocations";

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

const t = ((key: string) => en[key as keyof typeof en] ?? key) as never;

function folder(name: string, ...domains: StorageLocationSection["domain"][]): StorageLocation {
  return location({
    id: `path:${name}`,
    object: "path",
    kind: "local",
    backend: "local",
    name,
    where: `user/${name}`,
    sections: domains.map((domain) => section({ domain, use: "home" })),
  });
}

describe("localName", () => {
  it("calls the only folder on this host Local, as the storage pages do", () => {
    const list = namedLocal(t, [folder("bombvault", "files", "containers"), location()]);
    expect(localName(t, list, "files")).toBe(en["placement.segLocal"]);
  });

  it("names each of several folders after itself", () => {
    const list = namedLocal(t, [folder("bombvault", "containers"), folder("cache", "files")]);
    expect(localName(t, list, "files")).toBe("cache");
    expect(localName(t, list, "containers")).toBe("bombvault");
  });

  it("names a section's remote folder after itself", () => {
    const remote = location({ id: "path:box", object: "path", name: "bv", sections: [section({ domain: "files", use: "home" })] });
    expect(localName(t, namedLocal(t, [folder("bombvault", "containers"), remote]), "files")).toBe("bv");
  });

  it("says Local until the storage locations are read", () => {
    expect(localName(t, [], "files")).toBe(en["placement.segLocal"]);
  });
});

describe("distinctNames", () => {
  const names = (list: StorageLocation[]) => distinctNames(list).map((l) => l.name);

  it("adds the server or the folder to places that share a name", () => {
    const rest = location({ id: "target:t1", name: "Primary", where: "rest:http://192.168.20.185:8000/rc-b-files" });
    const folder = location({ id: "target:t2", name: "Primary", where: "user/offsite/config" });
    expect(names([rest, folder, location()])).toEqual(["Primary · 192.168.20.185", "Primary · config", "NAS"]);
  });

  it("falls back to the whole address where the short part is the same", () => {
    const a = location({ id: "target:a", name: "Box", where: "rest:https://box:8000/a" });
    const b = location({ id: "target:b", name: "box", where: "rest:https://box:8000/b" });
    expect(names([a, b])).toEqual(["Box · rest:https://box:8000/a", "box · rest:https://box:8000/b"]);
  });

  it("leaves a name nobody else has alone", () => {
    const list = [location(), location({ id: "target:t1", name: "B2", where: "b2:bucket:files" })];
    expect(distinctNames(list)).toEqual(list);
  });
});

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

  it("lets the copy a domain's off-site settings describe follow only for its keep rules", () => {
    const primary = section({ targetId: "t1", primary: true });
    expect(canFollow(location(), primary, "retention")).toBe(true);
    expect(canFollow(location(), primary, "compression")).toBe(false);
    expect(canFollow(location(), primary)).toBe(false);
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
