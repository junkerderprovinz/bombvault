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

describe("deleting at one target", () => {
  it("asks for the preview of one item at one target", async () => {
    const { getOffsiteRemoval } = await import("./api");
    await getOffsiteRemoval({ domain: "vms", key: "win 11" }, "t1");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/items/vms/win%2011/offsite/t1/removal");
  });

  it("sends the confirmed snapshots and the typed name", async () => {
    const { deleteAtTarget } = await import("./api");
    await deleteAtTarget({ domain: "containers", key: "vaultwarden" }, "t1", ["b9"], "vaultwarden");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/items/containers/vaultwarden/offsite/t1/removal");
    expect(init.method).toBe("DELETE");
    expect(JSON.parse(init.body)).toEqual({ onlyThere: ["b9"], typedName: "vaultwarden" });
  });

  it("passes a source to the bulk deletes of containers and file sets", async () => {
    const { deleteBackups, deleteFileSetBackups } = await import("./api");
    await deleteBackups("nginx", "offsite:t1");
    await deleteFileSetBackups("s1");
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual([
      "/api/containers/nginx/backups?source=offsite%3At1",
      "/api/files/sets/s1/backups",
    ]);
  });
});
