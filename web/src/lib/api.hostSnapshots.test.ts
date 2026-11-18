// The server reads a snapshot's name and nothing else of it, and refuses a
// body with fields it does not know. The page holds whole snapshots, so every
// call has to send the name alone.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { HostSnapshot } from "./api";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(() => Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) }));
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const snap: HostSnapshot = {
  kind: "btrfs",
  fs: "/mnt/cache",
  source: "appdata",
  name: ".bombvault-snapshots/appdata/before-update",
  label: "before-update",
  createdAt: 1,
  usedBytes: 1,
  referencedBytes: 1,
  newSinceBytes: -1,
  holds: [],
  clones: [],
  owner: "",
  readOnly: true,
  caps: { rename: true, hold: false, delete: true, estimate: true },
};
const ref = { kind: "btrfs", fs: "/mnt/cache", source: "appdata", name: ".bombvault-snapshots/appdata/before-update" };

function body(): Record<string, unknown> {
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  return JSON.parse(String(init.body)) as Record<string, unknown>;
}

describe("the calls that change a snapshot", () => {
  it("send its name and nothing else of it", async () => {
    const api = await import("./api");
    const calls: [() => Promise<unknown>, (b: Record<string, unknown>) => unknown][] = [
      [() => api.renameHostSnapshot(snap, "kept"), (b) => b.ref],
      [() => api.holdHostSnapshot(snap, true), (b) => b.ref],
      [() => api.whereHostSnapshot(snap), (b) => b.ref],
      [() => api.freeHostSnapshot(snap), (b) => b.ref],
      [() => api.deleteHostSnapshots([snap]), (b) => (b.refs as unknown[])[0]],
      [() => api.estimateHostSnapshots([snap]), (b) => (b.refs as unknown[])[0]],
    ];
    for (const [call, pick] of calls) {
      fetchMock.mockClear();
      await call();
      expect(pick(body())).toEqual(ref);
    }
  });
});
