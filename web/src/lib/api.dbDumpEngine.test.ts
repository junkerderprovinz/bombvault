// A lookalike's dump is switched on by naming its engine. The row keeps any
// opt-out from a time the container was recognised by its image, and that
// opt-out would stop the dump the card shows as on.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({
    ok: true,
    status: 200,
    headers: { get: () => "application/json" },
    json: () => Promise.resolve({ ok: true }),
    text: () => Promise.resolve('{"ok":true}'),
  });
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

function sentBody(): unknown {
  return JSON.parse(fetchMock.mock.calls[0][1].body as string);
}

describe("setDbDumpEngine", () => {
  it("lifts an earlier opt-out together with the engine", async () => {
    const { setDbDumpEngine } = await import("./api");
    await setDbDumpEngine("pg", "mysql");
    expect(sentBody()).toEqual({ dbDumpEngine: "mysql", dbDumpOff: false });
  });

  it("clears only the engine when the dump is switched off", async () => {
    const { setDbDumpEngine } = await import("./api");
    await setDbDumpEngine("pg", "");
    expect(sentBody()).toEqual({ dbDumpEngine: "" });
  });
});
