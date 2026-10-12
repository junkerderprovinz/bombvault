// A refused call says why in the person's language when the server names a
// code this client knows, and in the server's words otherwise, never as a
// bare status.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "./i18n";

const fetchMock = vi.fn();

function answers(status: number, body: unknown, statusText = "Error") {
  fetchMock.mockImplementation(() =>
    Promise.resolve({
      ok: status >= 200 && status < 300,
      status,
      statusText,
      json: () => (body === undefined ? Promise.reject(new SyntaxError("not JSON")) : Promise.resolve(body)),
    }),
  );
}

beforeEach(() => {
  vi.resetModules();
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("a refused settings change", () => {
  it("tells a phone's session to sign in at the instance itself", async () => {
    answers(403, { ok: false, error: "sign in at this instance's web interface", code: "own-login-needed" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(api.ApiError);
    expect((err as InstanceType<typeof api.ApiError>).message).toBe(en["auth.ownLoginNeeded"]);
    expect((err as InstanceType<typeof api.ApiError>).code).toBe("own-login-needed");
    expect(api.explainedError(err)).toBe(en["auth.ownLoginNeeded"]);
  });

  it("tells a page opened through the relay to go to the instance's own address", async () => {
    const api = await import("./api");
    answers(200, { ok: true, enabled: true, authed: true, relayed: true });
    await api.getAuth();
    expect(api.openedThroughRelay()).toBe(true);
    answers(403, { ok: false, error: "sign in at this instance's web interface", code: "own-login-needed" });
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe(en["auth.relayLoginNeeded"]);
  });

  it("says what the server said for a refusal this client does not know", async () => {
    answers(409, { ok: false, error: "a restore is running", code: "busy" });
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe("a restore is running");
    expect(api.explainedError(err)).toBeUndefined();
  });

  it("falls back to the status when the answer is no JSON", async () => {
    answers(502, undefined, "Bad Gateway");
    const api = await import("./api");
    const err = await api.putSettings({} as Parameters<typeof api.putSettings>[0]).catch((e: unknown) => e);
    expect((err as Error).message).toBe("HTTP 502 Bad Gateway");
  });
});
