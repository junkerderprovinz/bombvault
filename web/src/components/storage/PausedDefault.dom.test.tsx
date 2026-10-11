// @vitest-environment jsdom
/**
 * A section whose off-site copies wait for its default to be confirmed.
 *
 * Nothing is confirmed before the window said what the next run copies, and a
 * name in the backups without an entry here is left out only when its switch
 * says so.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { accentButtons, renderWithProviders, targetPreview } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { PausedDefault } = await import("./PausedDefault");

function stagePreview() {
  fake.reply("getConfirmPreview", {
    ok: true,
    paused: true,
    targets: [{ targetId: "t-b2", name: "B2", preview: targetPreview() }],
    unmatched: [{ identity: "container:plex", snapshots: 12 }],
  });
}

describe("a paused default", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("names the section and says in its bubble what waits", () => {
    renderWithProviders(<PausedDefault domain="containers" />);

    expect(screen.getByText(/Containers: Off-site paused after a rebuild/)).toBeTruthy();
    expect(screen.getByLabelText(/Nothing from Containers is copied off-site until this default is confirmed/)).toBeTruthy();
  });

  it("shows what the next run copies before anything is confirmed", async () => {
    stagePreview();
    renderWithProviders(<PausedDefault domain="containers" />);

    fireEvent.click(screen.getByRole("button", { name: "Confirm default" }));
    const dialog = await screen.findByRole("dialog");

    expect(dialog.textContent).toContain("At the next run, this is copied:");
    expect(dialog.textContent).toContain("Items and project folders: 15");
    expect(dialog.textContent).toContain("Names in the backups without an entry here. Ticked ones are left out:");
    expect(accentButtons(dialog)).toEqual(["Confirm default"]);
    expect(fake.callsTo("confirmPlacementDefault")).toEqual([]);
  });

  it("confirms with the names that were switched to stay out", async () => {
    stagePreview();
    renderWithProviders(<PausedDefault domain="containers" />);
    fireEvent.click(screen.getByRole("button", { name: "Confirm default" }));
    const dialog = await screen.findByRole("dialog");

    fireEvent.click(within(dialog).getByRole("switch", { name: "plex, snapshots: 12" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm default" }));

    await waitFor(() => expect(fake.callsTo("confirmPlacementDefault")).toEqual([["containers", ["container:plex"]]]));
    expect(await screen.findByText("Default confirmed. Off-site copies resume at the next run.")).toBeTruthy();
  });

  it("confirms nothing when the window is cancelled", async () => {
    stagePreview();
    renderWithProviders(<PausedDefault domain="containers" />);
    fireEvent.click(screen.getByRole("button", { name: "Confirm default" }));
    const dialog = await screen.findByRole("dialog");

    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(fake.callsTo("confirmPlacementDefault")).toEqual([]);
  });

  it("says so when the server cannot be reached", async () => {
    fake.reply("getConfirmPreview", new Error("Failed to fetch"));
    renderWithProviders(<PausedDefault domain="containers" />);

    fireEvent.click(screen.getByRole("button", { name: "Confirm default" }));

    expect(await screen.findByText("Failed to fetch")).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
