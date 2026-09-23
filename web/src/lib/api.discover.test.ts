/**
 * discoverAll's skip handling. The union has to survive a domain that failed
 * (a pass searches the named repositories first, so a failure in the domain's
 * own repository leaves real results behind), and each named repository is
 * searched by all three domains, so an unmounted share must be named once.
 */
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

function answer(body: unknown) {
  return Promise.resolve({
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  });
}

describe("discoverAll", () => {
  it("names a shared repository once, not once per domain", async () => {
    const { discoverAll } = await import("./api");
    fetchMock.mockImplementation(() =>
      answer({ ok: true, discovered: 1, skipped: ["Cold (switched off)"] })
    );
    const res = await discoverAll();
    expect(res.skipped).toEqual(["Cold (switched off)"]);
    expect(res.containers + res.vms + res.files).toBe(3);
  });

  it("keeps the counts and the skips of a domain that failed", async () => {
    const { discoverAll } = await import("./api");
    let n = 0;
    fetchMock.mockImplementation(() => {
      n += 1;
      // The first domain fails after the named repositories yielded something.
      return n === 1
        ? answer({
            ok: false,
            error: "wrong password or no key found",
            discovered: 2,
            skipped: ["backups (it was there before and is not reachable now)"],
          })
        : answer({ ok: true, discovered: 1, skipped: [] });
    });
    const res = await discoverAll();
    expect(res.error).toMatch(/wrong password/);
    // The partial rebuild is real and stays despite the error.
    expect(res.containers + res.vms + res.files).toBe(4);
    expect(res.skipped).toEqual(["backups (it was there before and is not reachable now)"]);
  });

  // A repository switched off on purpose belongs in the sentence but must not
  // colour a pill, or retiring one share leaves the Recovery readability step
  // amber for good and hides the save-success toast behind it. The server
  // decides (repoSkip.Note) and sends its own field, and the union keeps it
  // in both directions.
  it("keeps an intentional skip in the list but out of the flag", async () => {
    const { discoverAll } = await import("./api");
    fetchMock.mockImplementation(() =>
      answer({ ok: true, discovered: 1, skipped: ["Cold (switched off)"], skippedNeedsAction: false })
    );
    const res = await discoverAll();
    expect(res.skipped).toEqual(["Cold (switched off)"]);
    expect(res.skippedNeedsAction).toBe(false);
  });

  it("raises the flag when any one domain hit something actionable", async () => {
    const { discoverAll } = await import("./api");
    let n = 0;
    fetchMock.mockImplementation(() => {
      n += 1;
      return n === 2
        ? answer({
            ok: true,
            discovered: 1,
            skipped: ["Cold (it was there before and is not reachable now)"],
            skippedNeedsAction: true,
          })
        : answer({ ok: true, discovered: 1, skipped: [], skippedNeedsAction: false });
    });
    const res = await discoverAll();
    expect(res.skippedNeedsAction).toBe(true);
  });

  it("names the paused domains in order and merges what the three passes left open or found direct", async () => {
    const { discoverAll } = await import("./api");
    const replies = [
      { ok: true, discovered: 1, paused: true, leftOpen: ["nginx"], directRepos: [{ repoId: "r1", name: "B2 old", targets: [{ id: "t1", name: "B2" }] }] },
      { ok: true, discovered: 0, paused: false, leftOpen: [], directRepos: [{ repoId: "r1", name: "B2 old", targets: [{ id: "t2", name: "B2 VMs" }] }] },
      { ok: true, discovered: 2, paused: true, leftOpen: ["nginx", "Photos"], directRepos: [] },
    ];
    let n = 0;
    fetchMock.mockImplementation(() => answer(replies[n++]));
    const res = await discoverAll();
    expect(res.paused).toEqual(["containers", "files"]);
    expect(res.leftOpen).toEqual(["nginx", "Photos"]);
    expect(res.directRepos).toEqual([
      { repoId: "r1", name: "B2 old", targets: [{ id: "t1", name: "B2" }, { id: "t2", name: "B2 VMs" }] },
    ]);
  });
});
