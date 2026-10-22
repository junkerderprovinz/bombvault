// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  runDrill: vi.fn(),
}));

const api = await import("../../lib/api");
const runDrill = vi.mocked(api.runDrill);
const { useTrialRestore } = await import("./useTrialRestore");

afterEach(() => runDrill.mockReset());

async function start() {
  const hook = renderHook(() => useTrialRestore("failed"));
  await act(async () => hook.result.current.start("containers"));
  return hook.result.current;
}

describe("useTrialRestore", () => {
  it("reports a refusal under its section", async () => {
    runDrill.mockRejectedValue(new api.ApiError(409, "busy"));
    expect((await start()).refused).toEqual({ containers: "busy" });
  });

  it("takes an answer a proxy gave up on for no refusal, since the server carries on", async () => {
    runDrill.mockRejectedValue(new api.ApiError(504, "gateway timeout"));
    expect((await start()).refused).toEqual({});
  });
});
