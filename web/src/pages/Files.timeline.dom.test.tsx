// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { FileSetView } from "../lib/api";
import { useT } from "../lib/i18n";
import { placementView, renderWithProviders, stubEventSource } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listRuns: () => Promise.resolve({ ok: true, runs: [] }),
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
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("opens the timeline of the set by its id", async () => {
    renderWithProviders(<Row />);
    fireEvent.click(screen.getByRole("button", { name: "Backups" }));
    await waitFor(() => expect(fake.callsTo("getTimeline")).toEqual([["files", "set1"]]));
    expect(screen.queryByRole("tab", { name: "Off-site" })).toBeNull();
  });
});
