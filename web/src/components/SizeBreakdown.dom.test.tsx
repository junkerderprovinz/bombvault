// @vitest-environment jsdom
// Opening the panel asks the server to work the breakdown out; it polls while
// that runs, then lists the largest rows and opens a folder one level down.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { countText, en } from "../lib/i18n";
import type { SizeBreakdown as Breakdown } from "../lib/api";

const getSizeBreakdown = vi.fn();
vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, getSizeBreakdown: (...a: unknown[]) => getSizeBreakdown(...a) };
});

const { SizeBreakdown } = await import("./SizeBreakdown");

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof SizeBreakdown
>[0]["t"];

function ready(over: Partial<Breakdown>): { ok: true; breakdown: Breakdown } {
  return {
    ok: true,
    breakdown: {
      state: "ready",
      root: "/mnt/user/appdata/plex",
      path: "",
      size: 3 * 1024 * 1024,
      files: 12,
      added: 1024 * 1024,
      time: "2026-09-27T02:00:00Z",
      children: [
        { name: "Library", dir: true, size: 2 * 1024 * 1024, files: 10, added: 1024 * 1024, open: true },
        { name: "Preferences.xml", size: 1024 * 1024, files: 1, added: 0 },
      ],
      other: { count: 3, size: 10, files: 1, added: 0 },
      ...over,
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  getSizeBreakdown.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("polls while the breakdown is worked out and then lists the rows", async () => {
  getSizeBreakdown
    .mockResolvedValueOnce({ ok: true, breakdown: { state: "running", path: "", size: 0, files: 0, added: 0, children: [] } })
    .mockResolvedValueOnce(ready({}));
  render(<SizeBreakdown domain="containers" item="plex" t={t} />);
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: /Size by folder/ }));
  });
  expect(screen.getByRole("status").textContent).toBe("Working it out…");
  await act(async () => {
    await vi.advanceTimersByTimeAsync(2000);
  });
  expect(getSizeBreakdown).toHaveBeenCalledTimes(2);
  expect(screen.getByText("3.0 MB in 12 files, 1.0 MB of it new.", { exact: false })).toBeTruthy();
  expect(screen.getByText("Preferences.xml")).toBeTruthy();
  expect(screen.getByText("3 more entries")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Preferences.xml" })).toBeNull();

  getSizeBreakdown.mockResolvedValueOnce(ready({ path: "Library", children: [] }));
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Library" }));
  });
  expect(getSizeBreakdown).toHaveBeenLastCalledWith("containers", "plex", "Library", false);
});

it("offers to try again after a failure", async () => {
  getSizeBreakdown
    .mockResolvedValueOnce({ ok: true, breakdown: { state: "failed", error: "repository busy", path: "", size: 0, files: 0, added: 0, children: [] } })
    .mockResolvedValueOnce(ready({}));
  render(<SizeBreakdown domain="files" item="set-1" t={t} />);
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: /Size by folder/ }));
  });
  expect(screen.getByText("Could not work it out. repository busy")).toBeTruthy();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: /Try again/ }));
  });
  expect(getSizeBreakdown).toHaveBeenLastCalledWith("files", "set-1", "", true);
  expect(screen.getByText("Preferences.xml")).toBeTruthy();
});
