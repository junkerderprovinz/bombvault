import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { anomalyItemPath, anomalyRestorePath } from "./anomalies";
import type { AnomalyView } from "./api";
import {
  ENTRY_KIND,
  ENTRY_KINDS,
  entryKey,
  entryListPath,
  entryPath,
  isEntryKind,
  kindOfDomain,
  legacyLookupName,
  legacyRedirect,
  parseEntryPath,
  progressKey,
  type EntryKind,
} from "./backupEntry";

const SET_ID = "0123456789abcdef0123456789abcdef";
const ZFS_ID = "fedcba9876543210fedcba9876543210";

describe("the kinds", () => {
  it("knows its six kinds and nothing else", () => {
    expect([...ENTRY_KINDS]).toEqual(["container", "vm", "files", "zfs", "flash", "config"]);
    expect(isEntryKind("zfs")).toBe(true);
    expect(isEntryKind("containers")).toBe(false);
  });

  it.each([
    ["container", "container"],
    ["containers", "container"],
    ["vm", "vm"],
    ["vms", "vm"],
    ["files", "files"],
    ["zfs", "zfs"],
    ["flash", "flash"],
    ["config", "config"],
  ])("maps the domain %s to the kind %s", (domain, kind) => {
    expect(kindOfDomain(domain)).toBe(kind);
  });

  it("has no kind for a domain that is not an entry's", () => {
    expect(kindOfDomain("offsite")).toBeUndefined();
    expect(kindOfDomain("")).toBeUndefined();
  });

  it.each([
    ["container", "plex"],
    ["vm", "plex"],
    ["files", SET_ID],
    ["zfs", SET_ID],
    ["flash", "flash"],
    ["config", "config"],
  ] as const)("addresses a %s by %s", (kind, key) => {
    expect(entryKey(kind, { name: "plex", id: SET_ID })).toBe(key);
  });

  it.each([
    ["container", "containers", "containers"],
    ["vm", "vms", "vms"],
    ["files", "files", "files"],
    ["zfs", undefined, undefined],
    ["flash", "flash", undefined],
    ["config", "config", undefined],
  ] as const)("gives %s the timeline domain %s and the placement domain %s", (kind, timeline, placement) => {
    expect(ENTRY_KIND[kind].timelineDomain).toBe(timeline);
    expect(ENTRY_KIND[kind].placementDomain).toBe(placement);
  });
});

describe("progressKey", () => {
  it.each([
    ["container", "plex", "container:plex"],
    ["vm", "Windows 11", "vm:Windows 11"],
    ["files", "Photos", "files:Photos"],
    ["zfs", "tank/appdata", "zfs:tank/appdata"],
    ["flash", "", "flash"],
    ["config", "", "config"],
  ] as const)("reports a %s backup under the key the server uses", (kind, name, key) => {
    expect(progressKey(kind, name)).toBe(key);
  });

  it("keeps a container that is named like another kind apart from it", () => {
    expect(progressKey("container", "flash")).not.toBe(progressKey("flash", ""));
  });
});

describe("the entry's page", () => {
  it.each([
    ["container", "plex", "/backups/container/plex"],
    ["container", "my app#1?x=y&z 50%", "/backups/container/my%20app%231%3Fx%3Dy%26z%2050%25"],
    ["container", "flash", "/backups/container/flash"],
    ["vm", "Büro / Windows 11", "/backups/vm/B%C3%BCro%20%2F%20Windows%2011"],
    ["files", SET_ID, `/backups/files/${SET_ID}`],
    ["zfs", ZFS_ID, `/backups/zfs/${ZFS_ID}`],
    ["flash", "flash", "/backups/flash/flash"],
    ["config", "config", "/backups/config/config"],
  ] as const)("puts the %s %s at %s and reads it back", (kind, key, path) => {
    expect(entryPath(kind, key)).toBe(path);
    expect(parseEntryPath(path)).toEqual({ kind, key });
  });

  it.each([
    "/backups",
    "/backups/container",
    "/backups/container/",
    "/backups/containers/plex",
    "/backups/container/plex/settings",
    "/backups/container/%E0%A4%A",
    "/backups/flash/plex",
    "/containers/container/plex",
  ])("reads no entry out of %s", (path) => {
    expect(parseEntryPath(path)).toBeNull();
  });

  it("names the list, and the list of one kind", () => {
    expect(entryListPath()).toBe("/backups");
    expect(entryListPath("vm")).toBe("/backups?kind=vm");
  });
});

describe("links to a kind's own page", () => {
  const sets: Record<string, string> = { Photos: SET_ID, "tank/app data": ZFS_ID };
  const lookup = (name: string) => sets[name];

  it.each([
    ["container", "/backups?kind=container"],
    ["vm", "/backups?kind=vm"],
    ["files", "/backups?kind=files"],
    ["zfs", "/backups?kind=zfs"],
    ["flash", "/backups/flash/flash"],
    ["config", "/backups/config/config"],
  ] as const)("sends the bare %s page to %s", (kind, target) => {
    expect(legacyRedirect(kind, "")).toBe(target);
    expect(legacyLookupName(kind, "")).toBe("");
  });

  it("sends a container link to the container's page and keeps the restore request", () => {
    expect(legacyRedirect("container", "?restore=abc123&at=1700000000&item=my+app%231&dump=1")).toBe(
      "/backups/container/my%20app%231?restore=abc123&at=1700000000&dump=1",
    );
    expect(legacyLookupName("container", "?item=plex")).toBe("");
  });

  it("sends a VM link to the page of the libvirt name", () => {
    expect(legacyRedirect("vm", "?restore=abc123&at=1700000000&item=Windows%2011")).toBe(
      "/backups/vm/Windows%2011?restore=abc123&at=1700000000",
    );
  });

  it("looks a folder set up by the name its link carries", () => {
    const search = "?restore=abc123&at=1700000000&item=Photos";
    expect(legacyLookupName("files", search)).toBe("Photos");
    expect(legacyRedirect("files", search, lookup)).toBe(`/backups/files/${SET_ID}?restore=abc123&at=1700000000`);
  });

  it("looks a ZFS item up by its dataset and keeps the member the request is for", () => {
    const search = "?restore=abc123&at=1700000000&item=tank%2Fapp+data&dataset=tank%2Fapp+data%2Fdb";
    expect(legacyLookupName("zfs", search)).toBe("tank/app data");
    expect(legacyRedirect("zfs", search, lookup)).toBe(
      `/backups/zfs/${ZFS_ID}?restore=abc123&at=1700000000&dataset=tank%2Fapp+data%2Fdb`,
    );
  });

  it.each(["files", "zfs"] as const)("sends a %s link whose entry is gone to the list of the kind", (kind) => {
    expect(legacyRedirect(kind, "?restore=abc123&at=1700000000&item=Gone", lookup)).toBe(`/backups?kind=${kind}`);
    expect(legacyRedirect(kind, "?restore=abc123&at=1700000000&item=Photos")).toBe(`/backups?kind=${kind}`);
  });

  it.each(["flash", "config"] as const)("keeps the restore request of a %s link", (kind) => {
    expect(legacyRedirect(kind, "?restore=abc123&at=1700000000")).toBe(
      `/backups/${kind}/${kind}?restore=abc123&at=1700000000`,
    );
  });

  it("carries a parameter it does not know along to the entry", () => {
    expect(legacyRedirect("container", "?item=plex&scope=item%3Aabc")).toBe(
      "/backups/container/plex?scope=item%3Aabc",
    );
  });
});

describe("the links out of a finding", () => {
  function finding(over: Partial<AnomalyView>): AnomalyView {
    return {
      domain: "containers",
      name: "plex",
      targetId: "t1",
      scopeKind: "item",
      part: "",
      lastGood: { runId: "r1", snapshotId: "abc123", at: 1700000000 },
      ...over,
    } as AnomalyView;
  }
  const ids: Record<string, string> = { Photos: SET_ID, "tank/appdata": ZFS_ID };

  it.each(["container", "containers", "vm", "vms", "files", "zfs", "flash", "config"])(
    "knows the page a %s finding links to as that kind's own page",
    (domain) => {
      const kind = kindOfDomain(domain)!;
      expect(anomalyItemPath(finding({ domain }))).toBe(ENTRY_KIND[kind].legacyPath);
    },
  );

  it.each<[string, Partial<AnomalyView>, EntryKind, string]>([
    ["container", { domain: "containers" }, "container", "/backups/container/plex?restore=abc123&at=1700000000"],
    [
      "database dump",
      { domain: "container", scopeKind: "dump" },
      "container",
      "/backups/container/plex?restore=abc123&at=1700000000&dump=1",
    ],
    ["VM", { domain: "vms", name: "win11" }, "vm", "/backups/vm/win11?restore=abc123&at=1700000000"],
    [
      "folder set",
      { domain: "files", name: "Photos" },
      "files",
      `/backups/files/${SET_ID}?restore=abc123&at=1700000000`,
    ],
    [
      "ZFS member",
      { domain: "zfs", name: "tank/appdata", scopeKind: "zfsds", part: "tank/appdata/db" },
      "zfs",
      `/backups/zfs/${ZFS_ID}?restore=abc123&at=1700000000&dataset=tank%2Fappdata%2Fdb`,
    ],
    ["flash", { domain: "flash", name: "" }, "flash", "/backups/flash/flash?restore=abc123&at=1700000000"],
    ["config", { domain: "config", name: "" }, "config", "/backups/config/config?restore=abc123&at=1700000000"],
  ])("redirects the restore link of a %s finding to the entry", (_, over, kind, target) => {
    const [path, search] = anomalyRestorePath(finding(over))!.split("?");
    expect(path).toBe(ENTRY_KIND[kind].legacyPath);
    expect(legacyRedirect(kind, `?${search}`, (name) => ids[name])).toBe(target);
  });

  it("finds a folder set or ZFS item by the id a finding carries, without a lookup", () => {
    const a = finding({ domain: "files", name: "Photos", targetId: SET_ID });
    expect(entryPath("files", entryKey("files", { name: a.name, id: a.targetId }))).toBe(`/backups/files/${SET_ID}`);
  });
});

describe("the calls of a kind", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([
    ["container", "my app", "/api/containers/my%20app/backup"],
    ["vm", "win11", "/api/vms/win11/backup"],
    ["files", SET_ID, `/api/files/sets/${SET_ID}/backup`],
    ["zfs", ZFS_ID, `/api/zfs/datasets/${ZFS_ID}/backup`],
    ["flash", "flash", "/api/flash/backup"],
    ["config", "config", "/api/config/backup"],
  ] as const)("starts a %s backup", async (kind, key, url) => {
    await ENTRY_KIND[kind].backup(key);
    expect(fetchMock).toHaveBeenCalledWith(url, expect.objectContaining({ method: "POST" }));
  });

  it.each([
    ["container", "plex", "/api/containers/plex", { includeInSchedule: false }],
    ["vm", "win11", "/api/vms/win11", { includeInSchedule: false }],
    ["files", SET_ID, `/api/files/sets/${SET_ID}`, { enabled: false }],
    ["zfs", ZFS_ID, `/api/zfs/datasets/${ZFS_ID}`, { enabled: false }],
  ] as const)("pauses a %s entry", async (kind, key, url, body) => {
    await ENTRY_KIND[kind].setIncluded!(key, false);
    const [calledUrl, init] = fetchMock.mock.calls[0];
    expect(calledUrl).toBe(url);
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual(body);
  });

  it.each(["flash", "config"] as const)("offers no pause for %s", (kind) => {
    expect(ENTRY_KIND[kind].setIncluded).toBeUndefined();
  });
});
