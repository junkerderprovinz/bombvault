// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { timelineMark, timelinePlace, timelineRow } from "./placement.testsupport";

const fake = await vi.hoisted(async () => (await import("./placement.testsupport")).createPlacementApi());

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  ...fake.api,
}));

const { useTimeline } = await import("./useTimeline");

const b2 = timelinePlace({ place: "offsite:t-b2", label: "B2", kind: "target", remote: true, state: "unchecked" });
const opened = () => ({
  ok: true,
  places: [timelinePlace(), b2],
  rows: [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("local", "a1"))],
});
const b2Read = () => ({
  ok: true,
  place: { ...b2, state: "read" },
  rows: [timelineRow("a1", "2026-09-18T03:00:00Z", timelineMark("offsite:t-b2", "b9"))],
});

describe("useTimeline", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("asks nothing while closed and lists no remote place when it opens", async () => {
    fake.reply("getTimeline", opened());
    const { rerender, result } = renderHook(({ open }) => useTimeline("containers", "nginx", open), {
      initialProps: { open: false },
    });
    expect(fake.callsTo("getTimeline")).toEqual([]);
    rerender({ open: true });
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    expect(fake.callsTo("getTimeline")).toEqual([["containers", "nginx"]]);
    expect(fake.callsTo("getTimelinePlace")).toEqual([]);
  });

  it("loads one place on request and joins its marks by key", async () => {
    fake.reply("getTimeline", opened());
    fake.reply("getTimelinePlace", b2Read());
    const { result } = renderHook(() => useTimeline("containers", "nginx", true));
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    await act(() => result.current.loadPlace("offsite:t-b2"));
    expect(fake.callsTo("getTimelinePlace")).toEqual([["containers", "nginx", "offsite:t-b2"]]);
    expect(result.current.rows[0].places.map((m) => m.place)).toEqual(["local", "offsite:t-b2"]);
    expect(result.current.places[1].state).toBe("read");
  });

  it("marks a place it cannot list unreadable", async () => {
    fake.reply("getTimeline", opened());
    fake.reply("getTimelinePlace", { ok: false, code: "unknown-target", error: "gone" });
    const { result } = renderHook(() => useTimeline("containers", "nginx", true));
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    await act(() => result.current.loadPlace("offsite:t-b2"));
    expect(result.current.places[1]).toMatchObject({ state: "unreadable", error: "gone" });
  });

  it("lists the places it had loaded again on reload", async () => {
    fake.reply("getTimeline", opened(), opened());
    fake.reply("getTimelinePlace", b2Read(), b2Read());
    const { result } = renderHook(() => useTimeline("containers", "nginx", true));
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    await act(() => result.current.loadPlace("offsite:t-b2"));
    act(() => result.current.reload());
    await waitFor(() => expect(fake.callsTo("getTimelinePlace")).toHaveLength(2));
    expect(fake.callsTo("getTimeline")).toHaveLength(2);
    await waitFor(() => expect(result.current.rows[0].places).toHaveLength(2));
  });
});
