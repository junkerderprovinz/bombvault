// The calls that hand out or create key material can be refused outright, as
// the recovery kit is on an instance without a login password. The refusal
// comes as a 403 with a sentence, which has to reach the person.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

function answers(status: number, body: unknown) {
  fetchMock.mockImplementation(() =>
    Promise.resolve({
      ok: status >= 200 && status < 300,
      status,
      statusText: status === 403 ? "Forbidden" : "",
      json: () => (body === undefined ? Promise.reject(new Error("no body")) : Promise.resolve(body)),
    }),
  );
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("a call that hands out or creates key material", () => {
  it("goes through as any other while the server does not refuse it", async () => {
    answers(200, { ok: true, code: "K7QF2-M9XRT-4HPLD-W3NVC" });
    const { askKeeper, newDepositCode, decideRelease } = await import("./api");
    expect(await askKeeper("m 1")).toEqual({ ok: true, code: "K7QF2-M9XRT-4HPLD-W3NVC" });
    await newDepositCode();
    await decideRelease("rel1", "allow");
    expect(fetchMock.mock.calls.map(([url, init]) => [url, init.method, init.body])).toEqual([
      ["/api/group/members/m%201/roles/keeper", "PUT", undefined],
      ["/api/group/deposit/code", "POST", undefined],
      ["/api/group/deposits/releases/rel1", "POST", JSON.stringify({ decision: "allow" })],
    ]);
    expect(fetchMock.mock.calls[2][1].headers).toEqual({ "Content-Type": "application/json" });
  });

  it("answers with the server's sentence when the server refuses it outright", async () => {
    answers(403, { ok: false, error: "set a login password before making a key deposit" });
    const { askKeeper, newDepositCode, decideRelease } = await import("./api");
    const refused = { ok: false, error: "set a login password before making a key deposit" };
    expect(await askKeeper("m1")).toEqual(refused);
    expect(await newDepositCode()).toEqual(refused);
    expect(await decideRelease("rel1", "allow")).toEqual(refused);
  });

  it("fails as any other call when a refusal says nothing, or the server fails otherwise", async () => {
    const { askKeeper, ApiError } = await import("./api");
    answers(403, undefined);
    await expect(askKeeper("m1")).rejects.toBeInstanceOf(ApiError);
    answers(404, { ok: false, error: "no such request" });
    await expect(askKeeper("m1")).rejects.toMatchObject({ status: 404 });
  });
});
