/**
 * discoverAll's skip handling.
 *
 * The server half of "discovery reports what it could not read" landed a round
 * before the client half, and the client half then landed without a test. Both
 * things this pins were real: the union has to survive a domain that FAILED (a
 * pass searches the named repositories first, so a failure in the domain's own
 * repository leaves real results behind), and the same named repository is
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
      // The first domain fails AFTER the named repositories yielded something.
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
    // The partial rebuild is real and must not be thrown away with the error.
    expect(res.containers + res.vms + res.files).toBe(4);
    expect(res.skipped).toEqual(["backups (it was there before and is not reachable now)"]);
  });

  // What to SAY and what to FLAG are two answers. A repository switched off on
  // purpose belongs in the sentence and must not colour a pill, or retiring one
  // share leaves the Recovery readability step amber for good and swallows the
  // save-success toast behind it. The server draws that line (repoSkip.Note) and
  // sends it as its own field; the union here has to preserve it in both
  // directions.
  it("keeps a deliberate skip out of the flag while keeping it in the list", async () => {
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
