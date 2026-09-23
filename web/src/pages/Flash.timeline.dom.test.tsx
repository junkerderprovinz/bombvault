// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import {
  renderWithProviders,
  stubEventSource,
  timelineMark,
  timelinePlace,
  timelineRow,
} from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const flashDownloadURL = vi.fn((..._args: unknown[]) => "#");

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  flashDownloadURL: (...args: unknown[]) => flashDownloadURL(...args),
  getSettings: () =>
    Promise.resolve({
      ok: true,
      settings: { flashZipExportEnabled: false, flashZipExportPath: "", flashZipExportKeep: 0 },
      platform: "unraid",
      hostMountRoot: "/host/user",
    }),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  listRuns: () => Promise.resolve({ ok: true, runs: [] }),
}));

const { Flash } = await import("./Flash");

stubEventSource();

describe("flash backups", () => {
  beforeEach(() => {
    fake.reset();
    flashDownloadURL.mockClear();
  });
  afterEach(cleanup);

  it("downloads the backup from the place picked in the row", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [timelinePlace(), timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true })],
      rows: [timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"), timelineMark("offsite:t-b2", "b9b9b9b9"))],
    });
    renderWithProviders(<Flash />);
    fireEvent.click(await screen.findByRole("tab", { name: "B2" }));
    fireEvent.click(screen.getByRole("button", { name: "Download (.zip)" }));
    expect(flashDownloadURL).toHaveBeenCalledWith("b9b9b9b9", "offsite:t-b2");
    await waitFor(() => expect(fake.callsTo("getTimeline")).toEqual([["flash", "flash"]]));
  });
});
