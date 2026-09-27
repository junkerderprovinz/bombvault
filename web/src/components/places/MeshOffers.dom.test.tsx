// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { en } from "../../lib/i18n";
import { PLACES_CHANGED } from "../../lib/places";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  listMeshOffers: () =>
    Promise.resolve({
      ok: true,
      offers: [
        { id: "o1", from: "DXP480T", suggestedDomain: "vms", repo: "rest:http://192.0.2.5:8000/bv/vms", restUser: "bv", status: "pending", receivedAt: 1_758_170_400 },
        { id: "o0", from: "Old box", suggestedDomain: "flash", repo: "rest:http://192.0.2.6:8000/bv/flash", restUser: "bv", status: "accepted", receivedAt: 1_758_000_000 },
      ],
    }),
}));

const { MeshOffers } = await import("./MeshOffers");

describe("MeshOffers", () => {
  beforeEach(() => fake.reset());
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("lists the open offers only", async () => {
    renderWithProviders(<MeshOffers onAccepted={vi.fn()} />);
    expect(await screen.findByText("DXP480T")).toBeTruthy();
    expect(screen.queryByText("Old box")).toBeNull();
    expect(screen.getByText(en["places.offers.title"])).toBeTruthy();
  });

  it("renders an offer without a warning from React", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    renderWithProviders(<MeshOffers onAccepted={vi.fn()} />);
    await screen.findByText("DXP480T");
    expect(error).not.toHaveBeenCalled();
  });

  it("hands the place an accepted offer made to the window", async () => {
    fake.reply("acceptMeshOffer", { ok: true, place: { id: "p9", name: "mesh: DXP480T" } });
    const changed = vi.fn();
    window.addEventListener(PLACES_CHANGED, changed);
    const onAccepted = vi.fn();
    renderWithProviders(<MeshOffers onAccepted={onAccepted} />);
    fireEvent.click(await screen.findByRole("button", { name: en["fleet.mesh.accept"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(onAccepted).toHaveBeenCalledWith({ id: "p9", name: "mesh: DXP480T" }));
    expect(fake.callsTo("acceptMeshOffer")).toEqual([["o1", "vms", undefined]]);
    expect(changed).toHaveBeenCalled();
    window.removeEventListener(PLACES_CHANGED, changed);
  });

  it("says why an accept was refused in the reader's language", async () => {
    fake.reply("acceptMeshOffer", { ok: false, code: "exclusion-unsaved", error: "the exclusion was not saved" });
    renderWithProviders(<MeshOffers onAccepted={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: en["fleet.mesh.accept"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText(en["placementCode.exclusionUnsaved"])).toBeTruthy();
    expect(screen.queryByText("the exclusion was not saved")).toBeNull();
  });
});
