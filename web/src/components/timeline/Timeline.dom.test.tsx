// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { subscribePlacement } from "../../lib/placementEvents";
import { renderWithProviders, timelineMark, timelinePlace, timelineRow } from "../../lib/placement.testsupport";
import type { TimelinePick } from "./Timeline";

const fake = await vi.hoisted(async () =>
  (await import("../../lib/placement.testsupport")).createPlacementApi()
);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { Timeline } = await import("./Timeline");

const home = timelinePlace();
const b2 = timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true });
const hz = timelinePlace({ place: "offsite:t-hz", label: "Hetzner", kind: "target", remote: true });
const a1 = "a1a1a1a1";
const at = "2026-09-18T03:00:00Z";

function open() {
  const picks: TimelinePick[] = [];
  renderWithProviders(
    <Timeline
      domain="containers"
      itemKey="nginx"
      itemName="nginx"
      open
      renderActions={(pick) => {
        picks.push(pick);
        return (
          <button type="button" onClick={pick.onMissing}>
            gone
          </button>
        );
      }}
    />
  );
  return () => picks[picks.length - 1];
}

describe("Timeline", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("leads with the newest backup only and separates rows without lines", async () => {
    const a2 = "a2a2a2a2";
    fake.reply("getTimeline", {
      ok: true,
      places: [home],
      rows: [timelineRow(a2, "2026-09-19T03:00:00Z", timelineMark("local", a2)), timelineRow(a1, at, timelineMark("local", a1))],
    });
    const picks: TimelinePick[] = [];
    renderWithProviders(
      <Timeline
        domain="containers"
        itemKey="nginx"
        itemName="nginx"
        open
        renderActions={(pick) => {
          picks.push(pick);
          return null;
        }}
      />
    );
    await waitFor(() => expect(picks.length).toBeGreaterThanOrEqual(2));
    const lead = new Map(picks.map((p) => [p.row.key, p.lead]));
    expect(lead).toEqual(new Map([[a2, true], [a1, false]]));
    const group = screen.getByRole("group", { name: "nginx" });
    expect(group.innerHTML).not.toContain("border-");
  });

  it("reads only local places when it opens and names the ones not checked", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, { ...b2, state: "unchecked" }],
      rows: [timelineRow(a1, at, timelineMark("local", a1))],
    });
    open();
    expect(await screen.findByText("B2: not checked")).toBeTruthy();
    expect(fake.callsTo("getTimelinePlace")).toEqual([]);
  });

  it("checks exactly the place asked for", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, { ...b2, state: "unchecked" }, { ...hz, state: "unchecked" }],
      rows: [],
    });
    fake.reply("getTimelinePlace", {
      ok: true,
      place: b2,
      rows: [timelineRow(a1, at, timelineMark("offsite:t-b2", "b9b9b9b9"))],
    });
    open();
    fireEvent.click((await screen.findAllByRole("button", { name: "Check" }))[0]);
    expect(await screen.findByRole("tab", { name: "B2" })).toBeTruthy();
    expect(fake.callsTo("getTimelinePlace")).toEqual([["containers", "nginx", "offsite:t-b2"]]);
  });

  it("reads every place not checked yet for older backups", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, { ...b2, state: "unchecked" }, { ...hz, state: "unchecked" }],
      rows: [],
    });
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Load older" }));
    await waitFor(() => expect(fake.callsTo("getTimelinePlace")).toHaveLength(2));
    expect(fake.callsTo("getTimelinePlace").map((c) => c[2])).toEqual(["offsite:t-b2", "offsite:t-hz"]);
  });

  it("locks both ways of reading a place while that read is out", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, { ...b2, state: "unchecked" }],
      rows: [],
    });
    const answer = fake.hold("getTimelinePlace");
    open();
    const check = (await screen.findByRole("button", { name: "Check" })) as HTMLButtonElement;
    const older = screen.getByRole("button", { name: "Load older" }) as HTMLButtonElement;
    fireEvent.click(check);
    await waitFor(() => expect(check.disabled).toBe(true));
    expect(older.disabled).toBe(true);
    answer();
    await waitFor(() => expect(fake.callsTo("getTimelinePlace")).toHaveLength(1));
  });

  it("offers the item's location first and hands the page the chosen place's newest id", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, b2],
      rows: [timelineRow(a1, at, timelineMark("offsite:t-b2", "b9b9b9b9", "b8b8b8b8"), timelineMark("local", a1))],
    });
    const last = open();
    await waitFor(() => expect(last()?.source).toBe("local"));
    expect(last()?.snapshotId).toBe(a1);
    fireEvent.click(screen.getByRole("tab", { name: "B2" }));
    await waitFor(() => expect(last()?.source).toBe("offsite:t-b2"));
    expect(last()?.snapshotId).toBe("b9b9b9b9");
  });

  it("does not pick a place where the run is incomplete", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, b2],
      rows: [
        timelineRow(
          a1,
          at,
          { ...timelineMark("local", a1), incomplete: true },
          timelineMark("offsite:t-b2", "b1b1b1b1")
        ),
      ],
    });
    const last = open();
    await waitFor(() => expect(last()?.source).toBe("offsite:t-b2"));
    expect(await screen.findByRole("tab", { name: "Unraid · incomplete" })).toBeTruthy();
  });

  it("reads a place again when it lost the backup and offers the next one", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, b2],
      rows: [timelineRow(a1, at, timelineMark("local", a1), timelineMark("offsite:t-b2", "b1b1b1b1"))],
    });
    fake.reply("getTimelinePlace", { ok: true, place: home, rows: [] });
    const last = open();
    fireEvent.click(await screen.findByRole("button", { name: "gone" }));
    expect(await screen.findByText("Unraid no longer has this backup. Restore from B2?")).toBeTruthy();
    expect(fake.callsTo("getTimelinePlace")).toEqual([["containers", "nginx", "local"]]);
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(last()?.source).toBe("offsite:t-b2"));
  });

  it("says only here when every place was read and one holds the backup", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, b2],
      rows: [timelineRow(a1, at, timelineMark("local", a1))],
    });
    open();
    expect(await screen.findByText("only here")).toBeTruthy();
  });

  it("locks the delete of a place that only accepts new backups and says why where a keyboard reaches it", async () => {
    fake.reply("getTimeline", {
      ok: true,
      places: [{ ...home, appendOnly: true }, b2],
      rows: [timelineRow(a1, at, timelineMark("local", a1))],
    });
    open();
    const del = (await screen.findByRole("button", { name: "Delete" })) as HTMLButtonElement;
    expect(del.disabled).toBe(true);
    const why = screen.getByLabelText("The target is append-only. Nothing here may delete from it.");
    expect(why.getAttribute("tabindex")).toBe("0");
  });

  it("asks the server about every place when a whole row is deleted", async () => {
    fake.reply("getTimeline", { ok: true, places: [home, b2], rows: [timelineRow(a1, at, timelineMark("local", a1))] });
    // The first question is still on its way out when the second is asked, the
    // window in which nothing covers the row yet.
    const answerFirst = fake.hold("getTimelineDeletePreview");
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Delete everywhere" }));
    await waitFor(() => expect(fake.callsTo("getTimelineDeletePreview")).toEqual([["containers", "nginx", a1, []]]));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(fake.callsTo("getTimelineDeletePreview")).toHaveLength(2));
    expect(fake.callsTo("getTimelineDeletePreview")[1]).toEqual(["containers", "nginx", a1, ["local"]]);
    answerFirst();
  });

  async function deleteAt(tab: string, place: string) {
    fake.reply("getTimeline", {
      ok: true,
      places: [home, b2],
      rows: [timelineRow(a1, at, timelineMark("local", a1), timelineMark("offsite:t-b2", "b1b1b1b1"))],
    });
    fake.reply("getTimelineDeletePreview", {
      ok: true,
      delete: [{ place, label: tab, snapshotIds: [a1] }],
      others: [],
    });
    const changed = vi.fn();
    const unsubscribe = subscribePlacement(changed);
    open();
    fireEvent.click(await screen.findByRole("tab", { name: tab }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(fake.callsTo("deleteTimelineRow")).toHaveLength(1));
    await waitFor(() => expect(fake.callsTo("getTimeline")).toHaveLength(2));
    unsubscribe();
    return changed;
  }

  it("tells the cards to read their copies again after a delete at a target", async () => {
    expect(await deleteAt("B2", "offsite:t-b2")).toHaveBeenCalled();
  });

  it("leaves the cards alone after a delete at the item's own location", async () => {
    expect(await deleteAt("Unraid", "local")).not.toHaveBeenCalled();
  });
});
