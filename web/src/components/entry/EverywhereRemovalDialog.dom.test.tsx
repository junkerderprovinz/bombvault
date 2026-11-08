// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { ItemRef } from "../../lib/api";
import { everywhereRemoval, renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () =>
  (await import("../../lib/placement.testsupport")).createPlacementApi()
);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { EverywhereRemovalDialog } = await import("./EverywhereRemovalDialog");

const item: ItemRef = { domain: "containers", key: "vaultwarden" };

function open(shown?: { item: ItemRef; name: string }) {
  const onDone = vi.fn();
  const onClose = vi.fn();
  renderWithProviders(
    <EverywhereRemovalDialog item={shown?.item ?? item} name={shown?.name ?? "vaultwarden"} onDone={onDone} onClose={onClose} />
  );
  return { onDone, onClose };
}

async function typeAndDelete(name = "vaultwarden") {
  fireEvent.change(await screen.findByLabelText(`Type ${name} to confirm`), { target: { value: name } });
  fireEvent.click(screen.getByRole("button", { name: "Delete everywhere" }));
}

describe("EverywhereRemovalDialog", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("lists every place with its count", async () => {
    open();
    expect(await screen.findByText("Delete every backup of vaultwarden at every place listed below? The entry and its settings stay.")).toBeTruthy();
    expect(screen.getByText("NAS Keller: 14")).toBeTruthy();
    expect(screen.getByText("B2: 12")).toBeTruthy();
  });

  it("keeps the delete locked until the name is typed", async () => {
    const { onDone } = open();
    const button = (await screen.findByRole("button", { name: "Delete everywhere" })) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.className).not.toContain("bg-accent");
    fireEvent.change(screen.getByLabelText("Type vaultwarden to confirm"), { target: { value: "vault" } });
    expect(button.disabled).toBe(true);
    await typeAndDelete();
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("deleteEverywhere")).toEqual([[item, "vaultwarden", ["local", "offsite:t-b2"]]]);
    expect(await screen.findByText("Backups of vaultwarden deleted: 26")).toBeTruthy();
  });

  it("asks for the name the server compares, not the one on the card", async () => {
    const vm: ItemRef = { domain: "vms", key: "win10-gaming" };
    fake.reply("getEverywhereRemoval", { ok: true, ...everywhereRemoval({ name: "win10-gaming" }) });
    const { onDone } = open({ item: vm, name: "Windows 10" });
    await typeAndDelete("win10-gaming");
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("deleteEverywhere")[0]).toEqual([vm, "win10-gaming", ["local", "offsite:t-b2"]]);
  });

  it("says which place keeps its backups because it is append-only", async () => {
    fake.reply("getEverywhereRemoval", {
      ok: true,
      ...everywhereRemoval({
        places: [{ place: "offsite:t-b2", label: "B2", count: 12, appendOnly: true, unreadable: false }],
      }),
    });
    fake.reply("deleteEverywhere", { ok: true, results: [{ place: "offsite:t-b2", label: "B2", deleted: 0, kept: "append-only" }] });
    open();
    expect(await screen.findByText("B2: 12, stays because it is append-only")).toBeTruthy();
    await typeAndDelete();
    expect(await screen.findByText("B2 is append-only and kept its backups.")).toBeTruthy();
  });

  it("names the section's own folder by the section's name", async () => {
    fake.reply("getEverywhereRemoval", {
      ok: true,
      ...everywhereRemoval({ places: [{ place: "local", label: "", count: 3, appendOnly: false, unreadable: false }] }),
    });
    open();
    expect(await screen.findByText(/^.+: 3$/)).toBeTruthy();
    expect(screen.queryByText("local: 3")).toBeNull();
  });

  it("shows the changed list in the same window and asks again", async () => {
    const grown = everywhereRemoval({
      places: [
        ...everywhereRemoval().places,
        { place: "offsite:t-hz", label: "Hetzner", count: 5, appendOnly: false, unreadable: false },
      ],
    });
    fake.reply("deleteEverywhere", { ok: false, code: "places-changed", error: "changed", preview: grown }, { ok: true, results: [] });
    const { onDone } = open();
    await typeAndDelete();
    expect(await screen.findByText("Hetzner: 5")).toBeTruthy();
    await typeAndDelete();
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("deleteEverywhere")[1]).toEqual([item, "vaultwarden", ["local", "offsite:t-b2", "offsite:t-hz"]]);
  });

  it("closes with a note when there is nothing to delete", async () => {
    fake.reply("getEverywhereRemoval", { ok: true, ...everywhereRemoval({ places: [] }) });
    const { onClose } = open();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(await screen.findByText("vaultwarden has no backups anywhere.")).toBeTruthy();
  });

  it("closes with the server's own words when it refuses the delete", async () => {
    fake.reply("deleteEverywhere", { ok: false, error: "B2 could not be reached" });
    const { onClose } = open();
    await typeAndDelete();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(await screen.findByText("B2 could not be reached")).toBeTruthy();
  });
});
