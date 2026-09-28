// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { en } from "../lib/i18n";
import { renderWithProviders, targetPreview } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const host = vi.hoisted(() => ({ zfsEnabled: false }));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listFleetPeers: () => Promise.resolve({ ok: true, peers: [] }),
  listMeshOffers: () =>
    Promise.resolve({
      ok: true,
      offers: [
        {
          id: "o1",
          from: "DXP480T",
          suggestedDomain: "vms",
          repo: "rest:http://192.0.2.5:8000/vms",
          restUser: "bv",
          status: "pending",
          receivedAt: 1_758_170_400,
        },
      ],
    }),
  getSettings: () =>
    Promise.resolve({ ok: true, settings: { fleetEnabled: true, zfsEnabled: host.zfsEnabled }, hostMountRoot: "/host/user", platform: "unraid" }),
}));

const { Fleet } = await import("./Fleet");

describe("accepting a mesh offer", () => {
  beforeEach(() => {
    fake.reset();
    host.zfsEnabled = false;
  });
  afterEach(cleanup);

  it("asks what the new target receives and sends the chosen exclusions along", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "vm:win11", skip: ["t-b2"] }] }),
    });
    renderWithProviders(<Fleet />);
    fireEvent.click(await screen.findByRole("button", { name: "Accept" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("At its first run DXP480T receives every item not set to Local.");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await waitFor(() =>
      expect(fake.callsTo("acceptMeshOffer")).toEqual([["o1", "vms", { identities: ["vm:win11"], default: false }]])
    );
    expect(fake.callsTo("getNewTargetPreview")).toEqual([["vms", "rest:http://192.0.2.5:8000/vms", undefined]]);
  });

  it("locks Accept while the question is still being prepared", async () => {
    const answer = fake.hold("getNewTargetPreview");
    renderWithProviders(<Fleet />);
    const button = await screen.findByRole("button", { name: "Accept" });

    fireEvent.click(button);

    await waitFor(() => expect(fake.callsTo("getNewTargetPreview")).toHaveLength(1));
    expect(button.hasAttribute("disabled")).toBe(true);

    answer();
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(button.hasAttribute("disabled")).toBe(false));
  });

  it("words a placement refusal of the accept in the reader's language", async () => {
    fake.reply("acceptMeshOffer", { ok: false, code: "copy-rule-taken", error: "copy rule taken" });
    renderWithProviders(<Fleet />);
    fireEvent.click(await screen.findByRole("button", { name: "Accept" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText("That name already has copy rules of its own.")).toBeTruthy();
    expect(screen.queryByText("copy rule taken")).toBeNull();
  });

  it("offers ZFS datasets to apply the offer to only while ZFS is on", async () => {
    const offered = async () => {
      renderWithProviders(<Fleet />);
      const field = await screen.findByRole("combobox", { name: en["fleet.mesh.applyTo"] });
      await act(async () => {});
      fireEvent.click(field);
      return screen.getAllByRole("option").map((o) => o.textContent);
    };
    expect(await offered()).not.toContain(en["nav.zfs"]);
    cleanup();
    host.zfsEnabled = true;
    expect(await offered()).toContain(en["nav.zfs"]);
  });

  it("accepts nothing when the question is cancelled", async () => {
    renderWithProviders(<Fleet />);
    fireEvent.click(await screen.findByRole("button", { name: "Accept" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(fake.callsTo("acceptMeshOffer")).toEqual([]);
  });
});
