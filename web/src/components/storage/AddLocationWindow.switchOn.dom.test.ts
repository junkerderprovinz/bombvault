// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../../lib/i18n";
import { defaultRow, placementOptions } from "../../lib/placement.testsupport";

const destinationForDomain = vi.fn();
const previewPlacementDefault = vi.fn();
const putPlacementDefault = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  destinationForDomain: (...a: unknown[]) => destinationForDomain(...a),
  previewPlacementDefault: (...a: unknown[]) => previewPlacementDefault(...a),
  putPlacementDefault: (...a: unknown[]) => putPlacementDefault(...a),
}));

const { switchOnFor } = await import("./AddLocationWindow");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as Parameters<typeof switchOnFor>[4];

beforeEach(() => {
  destinationForDomain.mockReset().mockResolvedValue({ ok: true, target: { id: "t-new" } });
  previewPlacementDefault.mockReset().mockResolvedValue({ ok: true, impact: { targets: [] } });
  putPlacementDefault.mockReset().mockResolvedValue({ ok: true });
});

describe("switching a new location on for a section", () => {
  it("answers nothing once the section's default takes the new place", async () => {
    expect(await switchOnFor("dest-1", "containers", defaultRow(), placementOptions(), t, "en")).toBeNull();
    expect(putPlacementDefault).toHaveBeenCalled();
  });

  it("names why the default refused the new place", async () => {
    putPlacementDefault.mockResolvedValue({ ok: false, code: "stale", error: "the default changed" });
    const why = await switchOnFor("dest-1", "containers", defaultRow(), placementOptions(), t, "en");
    expect(why).toBe(en["placementCode.stale"]);
  });

  it("names why the preview of the change was refused", async () => {
    previewPlacementDefault.mockResolvedValue({ ok: false, code: "domain-busy", error: "a backup is running" });
    const why = await switchOnFor("dest-1", "vms", defaultRow({ domain: "vms" }), placementOptions({ domain: "vms" }), t, "en");
    expect(why).toBe(en["placementCode.domainBusy"]);
    expect(putPlacementDefault).not.toHaveBeenCalled();
  });

  it("passes on what the server said for a section without placement", async () => {
    destinationForDomain.mockResolvedValue({ ok: false, error: "the folder is inside another location" });
    expect(await switchOnFor("dest-1", "flash", undefined, null, t, "en")).toBe("the folder is inside another location");
  });
});
