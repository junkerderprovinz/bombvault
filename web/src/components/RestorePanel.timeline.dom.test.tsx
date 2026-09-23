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

function Panel({ installed = true }: { installed?: boolean }) {
  const { t } = useT();
  return <RestorePanel name="nginx" t={t} open installed={installed} />;
}

const b2 = timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true });

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

  it("calls a container config-only once every place has answered, not before", async () => {
    fake.reply("getTimeline", { ok: true, places: [timelinePlace(), { ...b2, state: "unchecked" }], rows: [] });
    fake.reply("getTimelinePlace", { ok: true, place: b2, rows: [] });
    renderWithProviders(<Panel />);

    fireEvent.click(await screen.findByRole("button", { name: "Check" }));
    expect(screen.queryByText(/Config-only backup/)).toBeNull();
    expect(await screen.findByText(/Config-only backup/)).toBeTruthy();
  });

  it("offers no recreate while a place is unchecked", async () => {
    fake.reply("getTimeline", { ok: true, places: [timelinePlace(), { ...b2, state: "unchecked" }], rows: [] });
    renderWithProviders(<Panel installed={false} />);

    await screen.findByRole("button", { name: "Check" });
    expect(screen.queryByRole("button", { name: "Recreate from saved config" })).toBeNull();
  });

  it("restores the snapshot of the place picked in the row", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [timelinePlace(), b2],
      rows: [timelineRow("a1a1a1a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1a1a1a1"), timelineMark("offsite:t-b2", "b9b9b9b9"))],
    });
    renderWithProviders(<Panel />);
    fireEvent.click(await screen.findByRole("tab", { name: "B2" }));
    fireEvent.click(screen.getByRole("button", { name: "Restore…" }));
    fireEvent.click(screen.getAllByRole("switch")[0]);
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(restore).toHaveBeenCalledWith("nginx", "b9b9b9b9", true, "offsite:t-b2", false));
  });
});
