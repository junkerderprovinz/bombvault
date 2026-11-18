// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  openedAsPairedPhone: () => true,
}));

const { readNavPins, setNavPin } = await import("./navPins");

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response("{}"))));
});

afterEach(() => {
  localStorage.clear();
  vi.unstubAllGlobals();
});

describe("the pins on a paired phone", () => {
  it("leave Snapshots out even where the person pinned it", () => {
    const settings = { receiverEnabled: false, fleetEnabled: false, pullEnabled: false, zfsEnabled: true };
    expect(readNavPins(settings).snapshots).toBe(false);
    setNavPin("snapshots", true);
    expect(readNavPins(settings).snapshots).toBe(false);
  });
});
