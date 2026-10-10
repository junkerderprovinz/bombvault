import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Destination, OffsiteTarget, Settings, StorageLocation, StorageLocationSection } from "./api";

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    getSettings: vi.fn(),
    putSettings: vi.fn(),
    updateDestination: vi.fn(),
    updateOffsiteTarget: vi.fn(),
    updateRepo: vi.fn(),
  };
});

const api = await import("./api");
const getSettings = vi.mocked(api.getSettings);
const putSettings = vi.mocked(api.putSettings);
const updateDestination = vi.mocked(api.updateDestination);
const updateOffsiteTarget = vi.mocked(api.updateOffsiteTarget);
const updateRepo = vi.mocked(api.updateRepo);

const { followLocation, saveLocation, settingsOwned, storageClassOf, writable } = await import("./storageWrites");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 3, keepYearly: 0 };

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
    backend: "s3",
    name: "Bucket",
    where: "s3:https://host/bucket",
    enabled: true,
    offPremises: true,
    sections: [],
    protection: { immutable: true, testable: false },
    capacity: {},
    ...over,
  };
}

function target(over: Partial<OffsiteTarget> = {}): OffsiteTarget {
  return {
    id: "t1",
    domain: "containers",
    name: "Bucket",
    repo: "s3:https://host/bucket/containers",
    credsRef: "",
    storageClass: "STANDARD_IA",
    immutable: false,
    schedule: "",
    retentionKeepLast: 0,
    retentionKeepDaily: 14,
    retentionKeepWeekly: 8,
    retentionKeepMonthly: 12,
    retentionKeepYearly: 3,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: 1,
    sortOrder: 1,
    ...over,
  };
}

const DESTINATION = { id: "d1", name: "Bucket", storageClass: "GLACIER_IR", immutable: true } as Destination;

beforeEach(() => {
  vi.clearAllMocks();
  updateDestination.mockResolvedValue({ ok: true });
  updateOffsiteTarget.mockResolvedValue({ ok: true });
  updateRepo.mockResolvedValue({ ok: true });
  putSettings.mockResolvedValue({ ok: true });
});

describe("writable", () => {
  it("lets a destination change everything it carries, the storage class on S3 only", () => {
    const can = writable(location(), { destination: DESTINATION, targets: [] });
    expect([...can].sort()).toEqual(
      ["compression", "enabled", "immutable", "limitDownload", "limitUpload", "name", "offPremises", "retention", "storageClass"].sort()
    );
    expect(writable(location({ backend: "rclone" }), { destination: DESTINATION, targets: [] }).has("storageClass")).toBe(false);
  });

  it("changes nothing on a destination whose record did not load", () => {
    expect(writable(location(), { targets: [] }).size).toBe(0);
  });

  it("lets a target of its own pick its credentials", () => {
    const loc = location({ id: "target:t1", object: "target", sections: [section({ targetId: "t1" })] });
    const can = writable(loc, { targets: [target()] });
    expect(can.has("credsRef")).toBe(true);
    expect(can.has("retention")).toBe(true);
    expect(can.has("offPremises")).toBe(false);
  });

  it("changes nothing on the copy a domain's off-site settings describe", () => {
    const loc = location({ id: "target:t1", object: "target", sections: [section({ targetId: "t1", primary: true })] });
    expect(settingsOwned(loc)).toBe(true);
    expect(writable(loc, { targets: [target()] }).size).toBe(0);
  });

  it("lets the local folder change the shared rule and nothing else", () => {
    const loc = location({ id: "path:abc", object: "path", kind: "local", backend: "local" });
    expect([...writable(loc, { targets: [] })]).toEqual(["retention"]);
  });

  it("gives a named repository no rule of its own", () => {
    const loc = location({ id: "repo:r1", object: "repo", kind: "local", backend: "local" });
    const can = writable(loc, { targets: [] });
    expect(can.has("retention")).toBe(false);
    expect(can.has("immutable")).toBe(true);
    expect(can.has("credsRef")).toBe(false);
  });
});

describe("saveLocation", () => {
  it("sends a destination what its route wants back along with the change", async () => {
    await saveLocation(location(), { destination: DESTINATION, targets: [] }, { retention: KEEP });
    expect(updateDestination).toHaveBeenCalledWith("d1", {
      name: "Bucket",
      storageClass: "GLACIER_IR",
      immutable: true,
      retention: KEEP,
    });
  });

  it("writes a target back whole, with a rule as its five columns", async () => {
    const loc = location({ id: "target:t1", object: "target" });
    await saveLocation(loc, { targets: [target()] }, { retention: KEEP, enabled: false });
    expect(updateOffsiteTarget).toHaveBeenCalledWith(
      "t1",
      expect.objectContaining({
        repo: "s3:https://host/bucket/containers",
        storageClass: "STANDARD_IA",
        enabled: false,
        retentionKeepDaily: 7,
        retentionKeepMonthly: 3,
        retentionKeepYearly: 0,
      })
    );
    expect(updateOffsiteTarget.mock.calls[0][1]).not.toHaveProperty("retention");
  });

  it("patches a named repository with the change alone", async () => {
    await saveLocation(location({ id: "repo:r1", object: "repo" }), { targets: [] }, { immutable: true });
    expect(updateRepo).toHaveBeenCalledWith("r1", { immutable: true });
  });

  it("writes the local folder's rule into the settings as they stand", async () => {
    getSettings.mockResolvedValue({ ok: true, settings: { retentionKeepDaily: 14, containersPath: "/a" } as Settings, hostMountRoot: "", platform: "" });
    await saveLocation(location({ id: "path:abc", object: "path" }), { targets: [] }, { retention: KEEP });
    expect(putSettings).toHaveBeenCalledWith(
      expect.objectContaining({ containersPath: "/a", retentionKeepDaily: 7, retentionKeepWeekly: 4, retentionKeepMonthly: 3 })
    );
  });

  it("writes nothing when the settings cannot be read", async () => {
    getSettings.mockResolvedValue({ ok: false, error: "locked" } as Awaited<ReturnType<typeof api.getSettings>>);
    const res = await saveLocation(location({ id: "path:abc", object: "path" }), { targets: [] }, { retention: KEEP });
    expect(res.ok).toBe(false);
    expect(putSettings).not.toHaveBeenCalled();
  });
});

describe("followLocation", () => {
  it("asks the target's route to take one setting from the destination again", async () => {
    const flash = target({ id: "t2", domain: "flash" });
    await followLocation(location(), { destination: DESTINATION, targets: [target(), flash] }, section({ domain: "flash", targetId: "t2" }), "compression");
    expect(updateOffsiteTarget).toHaveBeenCalledWith("t2", flash, undefined, ["compression"]);
  });

  it("drops a domain's own local rule and keeps the others", async () => {
    getSettings.mockResolvedValue({
      ok: true,
      settings: { ownRetention: { vms: KEEP, flash: KEEP } } as unknown as Settings,
      hostMountRoot: "",
      platform: "",
    });
    const loc = location({ id: "path:abc", object: "path" });
    await followLocation(loc, { targets: [] }, section({ domain: "vms", use: "home" }), "retention");
    expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ ownRetention: { flash: KEEP } }));
  });
});

describe("storageClassOf", () => {
  it("reads the class from the record behind the location", () => {
    expect(storageClassOf(location(), { destination: DESTINATION, targets: [] })).toBe("GLACIER_IR");
    expect(storageClassOf(location({ id: "target:t1", object: "target" }), { targets: [target()] })).toBe("STANDARD_IA");
    expect(storageClassOf(location({ id: "repo:r1", object: "repo" }), { targets: [] })).toBeUndefined();
  });
});
