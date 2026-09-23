// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { useT } from "../lib/i18n";
import {
  renderWithProviders,
  stubEventSource,
  timelineMark,
  timelinePlace,
  timelineRow,
} from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const restore = vi.fn((..._args: unknown[]) => Promise.resolve({ ok: true, started: true }));

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  restore: (...args: unknown[]) => restore(...args),
  listRuns: () => Promise.resolve({ ok: true, runs: [] }),
}));

const { RestorePanel } = await import("./RestorePanel");

stubEventSource();

function Panel() {
  const { t } = useT();
  return <RestorePanel name="nginx" t={t} open />;
}

describe("RestorePanel", () => {
  beforeEach(() => {
    fake.reset();
    restore.mockClear();
  });
  afterEach(cleanup);

  it("lists the container's backups as its timeline, without a source switch", async () => {
    renderWithProviders(<Panel />);
    await waitFor(() => expect(fake.callsTo("getTimeline")).toEqual([["containers", "nginx"]]));
    expect(screen.queryByRole("tab", { name: "Off-site" })).toBeNull();
  });

  it("restores the snapshot of the place picked in the row", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [timelinePlace(), timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true })],
      rows: [timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"), timelineMark("offsite:t-b2", "b9b9b9b9"))],
    });
    renderWithProviders(<Panel />);
    fireEvent.click(await screen.findByRole("tab", { name: "B2" }));
    fireEvent.click(screen.getByRole("button", { name: "Restore…" }));
    fireEvent.click(screen.getAllByRole("switch")[0]);
    fireEvent.click(screen.getByRole("button", { name: "Restoring…" }));
    await waitFor(() => expect(restore).toHaveBeenCalledWith("nginx", "b9b9b9b9", true, "offsite:t-b2", false));
  });
});
