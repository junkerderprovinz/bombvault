// The calls that hand out or create key material can be refused outright:
// without a login password, or to the session the phone app opens this
// instance with. The refusal comes as a 403 with a code, which has to reach
// the person.
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
    answers(200, { ok: true, recoveryCode: "K7QF2-M9XRT-4HPLD-W3NVC" });
    const { askKeeper, newDepositCode, decideRelease } = await import("./api");
    expect(await askKeeper("m 1")).toEqual({ ok: true, recoveryCode: "K7QF2-M9XRT-4HPLD-W3NVC" });
    await newDepositCode();
    await decideRelease("rel1", "allow");
    expect(fetchMock.mock.calls.map(([url, init]) => [url, init.method, init.body])).toEqual([
      ["/api/group/members/m%201/roles/keeper", "PUT", undefined],
      ["/api/group/deposit/code", "POST", undefined],
      ["/api/group/deposits/releases/rel1", "POST", JSON.stringify({ decision: "allow" })],
    ]);
    expect(fetchMock.mock.calls[2][1].headers).toEqual({ "Content-Type": "application/json" });
  });

  it.each(["password-not-set", "own-login-needed"])("answers with the refusal when the server refuses it as %s", async (code) => {
    const refused = { ok: false, error: "the server's sentence", code };
    answers(403, refused);
    const { askKeeper, newDepositCode, decideRelease, withdrawKeeper } = await import("./api");
    expect(await askKeeper("m1")).toEqual(refused);
    expect(await newDepositCode()).toEqual(refused);
    expect(await decideRelease("rel1", "allow")).toEqual(refused);
    expect(await withdrawKeeper("m1")).toEqual(refused);
  });

  it("fails as any other call when a refusal says nothing, or the server fails otherwise", async () => {
    const { askKeeper, ApiError } = await import("./api");
    answers(403, undefined);
    await expect(askKeeper("m1")).rejects.toBeInstanceOf(ApiError);
    answers(403, { ok: false, error: "forbidden" });
    await expect(askKeeper("m1")).rejects.toBeInstanceOf(ApiError);
    answers(404, { ok: false, error: "no such request" });
    await expect(askKeeper("m1")).rejects.toMatchObject({ status: 404 });
  });
});
