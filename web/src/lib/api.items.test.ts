import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(() =>
    Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) })
  );
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const urls = () => fetchMock.mock.calls.map((c) => c[0]);

describe("item calls", () => {
  it("lists the items of every kind", async () => {
    const { listItems } = await import("./api");
    await listItems();
    expect(urls()).toEqual(["/api/items"]);
  });

  it("asks for the runs of one item by kind and key, with a limit when given", async () => {
    const { listItemRuns } = await import("./api");
    await listItemRuns("vm", "Windows 11");
    await listItemRuns("container", "plex", 50);
    expect(urls()).toEqual([
      "/api/runs?itemKind=vm&itemKey=Windows+11",
      "/api/runs?itemKind=container&itemKey=plex&limit=50",
    ]);
  });

  it("starts a batch of VMs by their libvirt names", async () => {
    const { backupVMsAll } = await import("./api");
    await backupVMsAll(["win11", "1_debian"]);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/vms/backup-all");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ names: ["win11", "1_debian"] });
  });

  it("leaves the unfiltered run list as it was", async () => {
    const { listRuns } = await import("./api");
    await listRuns();
    await listRuns("r1");
    expect(urls()).toEqual(["/api/runs", "/api/runs?run=r1"]);
  });
});
