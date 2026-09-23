// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithProviders, timelineMark, timelineRow } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () =>
  (await import("../../lib/placement.testsupport")).createPlacementApi()
);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { TimelineDeleteDialog } = await import("./TimelineDeleteDialog");

const row = timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"));
const atHome = { place: "local", label: "", snapshotIds: ["a1a1a1a1"] };

function open(places: string[] = ["local"]) {
  const onDone = vi.fn();
  const onClose = vi.fn();
  renderWithProviders(
    <TimelineDeleteDialog domain="containers" itemKey="nginx" row={row} places={places} onDone={onDone} onClose={onClose} />
  );
  return { onDone, onClose };
}

describe("TimelineDeleteDialog", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("names the places that still hold the backup and deletes what the preview named", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [atHome],
      others: [{ place: "offsite:t-b2", label: "B2", state: "holds" }],
    });
    const { onDone } = open();
    expect(await screen.findByText("Delete this backup here: Unraid?")).toBeTruthy();
    expect(screen.getByText("Still held by: B2")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(fake.callsTo("deleteTimelineRow")).toEqual([["containers", "nginx", "a1a1a1a1", [atHome]]]);
  });

  it("says the space waits on a prune and that the delete is final", async () => {
    fake.reply("getTimelineDeletePreview", { ok: true, delete: [atHome], others: [] });
    open();
    expect(await screen.findByText("The space is not reclaimed until a prune runs.")).toBeTruthy();
    expect(screen.getByText("This cannot be undone.")).toBeTruthy();
  });

  it("says when this is the last copy", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [atHome],
      others: [{ place: "offsite:t-b2", label: "B2", state: "missing" }],
    });
    open();
    expect(await screen.findByText("This is the last copy of this backup.")).toBeTruthy();
  });

  it("names a place it could not check instead of calling it the last copy", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [atHome],
      others: [{ place: "offsite:t-b2", label: "B2", state: "unreadable" }],
    });
    open();
    expect(await screen.findByText("Could not be checked: B2")).toBeTruthy();
    expect(screen.queryByText("This is the last copy of this backup.")).toBeNull();
  });

  it("names append-only places it leaves out when the whole row goes", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [atHome],
      others: [{ place: "offsite:t-b2", label: "B2", state: "append-only" }],
    });
    open([]);
    expect(await screen.findByText("Left out, append-only: B2")).toBeTruthy();
    expect(screen.getByText("Still held by: B2")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Delete everywhere" }));
    await waitFor(() => expect(fake.callsTo("deleteTimelineRow")).toHaveLength(1));
    expect(fake.callsTo("getTimelineDeletePreview")).toEqual([["containers", "nginx", "a1a1a1a1", []]]);
  });

  it("names the place it could not read when nothing came back to delete", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [],
      others: [{ place: "offsite:t-b2", label: "B2", state: "unreadable" }],
    });
    const { onDone, onClose } = open(["offsite:t-b2"]);
    expect(await screen.findByText("Could not be checked: B2")).toBeTruthy();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onDone).not.toHaveBeenCalled();
  });

  it("names the append-only place it left out when nothing came back to delete", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [],
      others: [{ place: "offsite:t-b2", label: "B2", state: "append-only" }],
    });
    const { onClose } = open(["offsite:t-b2"]);
    expect(await screen.findByText("Left out, append-only: B2")).toBeTruthy();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("reloads the list when the place no longer holds the backup", async () => {
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [],
      others: [{ place: "local", label: "", state: "missing" }],
    });
    const { onDone, onClose } = open();
    expect(await screen.findByText("This backup is no longer at the chosen place.")).toBeTruthy();
    await waitFor(() => expect(onDone).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows the translated refusal and still reloads", async () => {
    fake.reply("getTimelineDeletePreview", { ok: true, delete: [atHome], others: [] });
    fake.reply("deleteTimelineRow", { ok: false, code: "domain-busy", error: "server sentence", deleted: [], skipped: [] });
    const { onDone } = open();
    fireEvent.click(await screen.findByRole("button", { name: "Delete" }));
    expect(await screen.findByText("A backup is running. Choose again once it has finished.")).toBeTruthy();
    expect(onDone).toHaveBeenCalled();
  });

  it("names the places a refused delete had already emptied", async () => {
    const atB2 = { place: "offsite:t-b2", label: "B2", snapshotIds: ["a1a1a1a1"] };
    fake.reply("getTimelineDeletePreview", { ok: true, delete: [atHome, atB2], others: [] });
    fake.reply("deleteTimelineRow", {
      ok: false,
      error: "the credentials of Hetzner have expired",
      deleted: [atB2],
      skipped: [],
    });
    open([]);
    fireEvent.click(await screen.findByRole("button", { name: "Delete everywhere" }));
    expect(await screen.findByText("the credentials of Hetzner have expired")).toBeTruthy();
    expect(await screen.findByText("Already deleted at: B2")).toBeTruthy();
  });
});
