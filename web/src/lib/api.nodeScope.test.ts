import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { inNode } from "./nodeScope";

const fetchMock = vi.fn();
const ROUTES = ["GET /api/containers", "POST /api/containers/{name}/backup", "POST /api/containers/{name}/restore"];

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation((url: string) =>
    Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve(url === "/api/nodes/routes" ? { ok: true, routes: ROUTES } : { ok: true }),
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const asked = () => fetchMock.mock.calls.map(([url]) => url as string).filter((url) => url !== "/api/nodes/routes");

describe("requests in a node's scope", () => {
  it("go to the node for the routes it takes", async () => {
    const { backupNow, listContainers } = await import("./api");
    await inNode("n1", () => listContainers());
    await inNode("n1", () => backupNow("plex"));
    expect(asked()).toEqual(["/api/nodes/n1/api/containers", "/api/nodes/n1/api/containers/plex/backup"]);
  });

  it("stay at home for every other route", async () => {
    const { getSettings, listItems } = await import("./api");
    await inNode("n1", () => getSettings());
    await inNode("n1", () => listItems());
    expect(asked()).toEqual(["/api/settings", "/api/items"]);
  });

  it("leave requests outside the scope alone", async () => {
    const { listContainers } = await import("./api");
    await listContainers();
    expect(asked()).toEqual(["/api/containers"]);
  });

  it("keep their method and body on the way to the node", async () => {
    const { restore } = await import("./api");
    await inNode("n2", () => restore("plex", "abc", true));
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/nodes/n2/api/containers/plex/restore");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toMatchObject({ snapshotId: "abc", confirm: true });
  });

  it("fail rather than reach home when the node's routes cannot be read", async () => {
    vi.resetModules();
    fetchMock.mockImplementation((url: string) =>
      url === "/api/nodes/routes"
        ? Promise.resolve({ ok: false, status: 503, json: () => Promise.resolve({ ok: false }) })
        : Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) }),
    );
    const fresh = await import("./api");
    const scope = await import("./nodeScope");
    await expect(scope.inNode("n1", () => fresh.listContainers())).rejects.toThrow();
    expect(asked()).toEqual([]);
  });
});
