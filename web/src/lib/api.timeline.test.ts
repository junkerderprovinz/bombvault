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

describe("timeline calls", () => {
  it("reads a timeline and one place of it", async () => {
    const { getTimeline, getTimelinePlace } = await import("./api");
    await getTimeline("vms", "win 11");
    await getTimelinePlace("flash", "flash", "offsite:t1");
    expect(urls()).toEqual([
      "/api/items/vms/win%2011/timeline",
      "/api/items/flash/flash/timeline?place=offsite%3At1",
    ]);
  });

  it("names each place of a delete preview and none for a whole row", async () => {
    const { getTimelineDeletePreview } = await import("./api");
    await getTimelineDeletePreview("containers", "nginx", "a1a1a1a1", ["local", "offsite:t1"]);
    await getTimelineDeletePreview("containers", "nginx", "a1a1a1a1");
    expect(urls()).toEqual([
      "/api/items/containers/nginx/timeline/a1a1a1a1/delete?place=local&place=offsite%3At1",
      "/api/items/containers/nginx/timeline/a1a1a1a1/delete",
    ]);
  });

  it("deletes with the confirmed places in the body", async () => {
    const { deleteTimelineRow } = await import("./api");
    const places = [{ place: "local", label: "", snapshotIds: ["a1a1a1a1"] }];
    await deleteTimelineRow("containers", "nginx", "a1a1a1a1", places);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/items/containers/nginx/timeline/a1a1a1a1");
    expect(init.method).toBe("DELETE");
    expect(JSON.parse(init.body)).toEqual({ places });
  });

  it("asks for the project folder at a source and sends its source with a stack restore", async () => {
    const { getStackDir, restoreStack } = await import("./api");
    await getStackDir("immich", "offsite:t1");
    await getStackDir("immich", "local");
    await restoreStack("immich", true, true, "offsite:t1", "local");
    await restoreStack("immich", true, true);
    expect(urls().slice(0, 2)).toEqual(["/api/stacks/immich/dir?source=offsite%3At1", "/api/stacks/immich/dir"]);
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({ startAfter: true, confirm: true, stackDirSource: "local" });
    expect(JSON.parse(fetchMock.mock.calls[3][1].body)).toEqual({ startAfter: true, confirm: true });
  });
});
