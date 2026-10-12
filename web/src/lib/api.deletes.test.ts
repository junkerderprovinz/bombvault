// Deleting backups for good, and a paired phone's restore into a folder that
// holds files, can be refused over the login. The refusal comes as a 403 with
// a code, which has to reach the page instead of a bare status.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

function answers(status: number, body: unknown) {
  fetchMock.mockImplementation(() =>
    Promise.resolve({
      ok: status >= 200 && status < 300,
      status,
      statusText: status === 403 ? "Forbidden" : "",
      json: () => Promise.resolve(body),
    }),
  );
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const item = { domain: "containers" as const, key: "nginx" };

describe("a call that deletes backups for good or restores for a paired phone", () => {
  it.each(["password-not-set", "own-login-needed"])("answers with the refusal when the server refuses it as %s", async (code) => {
    const refused = { ok: false, error: "the server's sentence", code };
    answers(403, refused);
    const api = await import("./api");
    const calls = [
      api.deleteBackups("nginx", "local"),
      api.deleteBackupsVM("win11", "local"),
      api.deleteFileSetBackups("set1"),
      api.deleteBackupsZFSDataset("z1"),
      api.deleteZFSDataset("z1", true),
      api.deleteZFSSafetySnapshot("z1", "tank/apps", "bombvault-safety-1"),
      api.deleteSnapshot("containers", "a1a1a1a1", "local"),
      api.pruneDomain("containers", "local"),
      api.deleteTimelineRow("containers", "nginx", "a1a1a1a1", []),
      api.deleteAtTarget(item, "t-b2", [], ""),
      api.restore("nginx", "a1a1a1a1", true, "local"),
      api.restoreVM("win11", "a1a1a1a1", true, "local"),
      api.restoreContainerFiles("nginx", "a1a1a1a1", ["/config"], "user/restore", true, "local"),
      api.restoreContainerToPath("nginx", "a1a1a1a1", "user/restore", "local"),
      api.restoreFileSet("set1", "a1a1a1a1", true, "user/restore", "local"),
      api.restoreFileSetFiles("set1", "a1a1a1a1", ["/a"], "user/restore", true, "local"),
      api.restoreZFS("z1", { stamp: "s1", dataset: "", wholeTree: true, paths: [], targetPath: "user/restore", newDataset: "", confirm: true }, "local"),
    ];
    for (const answer of await Promise.all(calls)) expect(answer).toEqual(refused);
  });

  it("fails as any other call on a 403 without such a code", async () => {
    answers(403, { ok: false, error: "forbidden" });
    const { deleteBackups, ApiError } = await import("./api");
    await expect(deleteBackups("nginx")).rejects.toBeInstanceOf(ApiError);
  });
});
