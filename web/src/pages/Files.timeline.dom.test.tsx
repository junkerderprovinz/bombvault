// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { FileSetView } from "../lib/api";
import { useT } from "../lib/i18n";
import {
  placementView,
  renderWithProviders,
  stubEventSource,
  timelineMark,
  timelinePlace,
  timelineRow,
} from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const deleteFileSetBackups = vi.hoisted(() => vi.fn(async () => ({ ok: true })));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listRuns: () => Promise.resolve({ ok: true, runs: [] }),
  deleteFileSetBackups,
}));

const { FileSetRow } = await import("./Files");

stubEventSource();

const set: FileSetView = {
  id: "set1",
  name: "Photos",
  path: "photos",
  excludes: [],
  enabled: true,
  lastBackup: 0,
  pathExists: true,
  placement: placementView(),
};

function Row() {
  const { t } = useT();
  return (
    <FileSetRow
      set={set}
      hostMountRoot="/host/user"
      restoreFolder="restore"
      t={t}
      onRefresh={() => {}}
      onEdit={() => {}}
      onPlacement={() => {}}
      index={0}
    />
  );
}

describe("folder set backups", () => {
  beforeEach(() => {
    fake.reset();
    deleteFileSetBackups.mockClear();
  });
  afterEach(cleanup);

  it("opens the timeline of the set by its id", async () => {
    renderWithProviders(<Row />);
    fireEvent.click(screen.getByRole("button", { name: "Backups" }));
    await waitFor(() => expect(fake.callsTo("getTimeline")).toEqual([["files", "set1"]]));
    expect(screen.queryByRole("tab", { name: "Off-site" })).toBeNull();
  });

  it("deletes the local backups its question names", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [timelinePlace()],
      rows: [timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"))],
    });
    renderWithProviders(<Row />);
    fireEvent.click(screen.getByRole("button", { name: "Backups" }));
    fireEvent.click(await screen.findByRole("button", { name: "Delete all backups" }));

    expect(await screen.findByText(/ALL local backups/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(deleteFileSetBackups).toHaveBeenCalledWith("set1"));
  });
});
