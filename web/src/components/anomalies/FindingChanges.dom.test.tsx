// @vitest-environment jsdom
// Showing the comparison asks the server to diff the last good backup against
// the first affected one; it polls while that runs and then lists the folders
// in a table of what went away, came in and changed.
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { countText, en } from "../../lib/i18n";
import type { AnomalyChanges, AnomalyView, ChangeFolder } from "../../lib/api";

const getAnomalyChanges = vi.fn();
vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, getAnomalyChanges: (...a: unknown[]) => getAnomalyChanges(...a) };
});

const { FindingChanges, findingHasChanges } = await import("./FindingChanges");

const t = ((key: string, n?: number) => countText((en as Record<string, string>)[key] ?? key, "en", n)) as never;

const MB = 1024 * 1024;
const finding = { id: "f1", scopeKind: "item", domain: "container", metric: "source_files_shrink" } as AnomalyView;

function indexRebuild(): { ok: true; changes: AnomalyChanges } {
  const index = {
    path: "data/index",
    removedBytes: 3400 * MB,
    removedFiles: 1012,
    addedBytes: 463 * MB,
    addedFiles: 39,
    changedBytes: 0,
    changedFiles: 0,
  };
  return {
    ok: true,
    changes: {
      state: "ready",
      fromAt: 1758000000,
      toAt: 1758600000,
      summary: { total: index, folders: [index], focus: "data/index", regenerable: true },
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  getAnomalyChanges.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("traces an index rebuild to its folder and says it can be excluded", async () => {
  getAnomalyChanges.mockResolvedValueOnce({ ok: true, changes: { state: "running" } }).mockResolvedValueOnce(indexRebuild());
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });
  expect(screen.getByRole("status").textContent).toBe("Working it out…");
  await act(async () => {
    await vi.advanceTimersByTimeAsync(2000);
  });
  expect(getAnomalyChanges).toHaveBeenCalledTimes(2);
  expect(screen.getByText(/Nearly all of it is in .*data\/index\//)).toBeTruthy();
  expect(screen.getByText(/looks like a search index/)).toBeTruthy();
  expect(screen.getByRole("rowheader", { name: "data/index/" })).toBeTruthy();
});

function ready(summary: NonNullable<AnomalyChanges["summary"]>) {
  getAnomalyChanges.mockResolvedValue({ ok: true, changes: { state: "ready", fromAt: 1758000000, toAt: 1758600000, summary } });
}

const folder = (path: string, over: Partial<ChangeFolder> = {}): ChangeFolder => ({
  path,
  removedBytes: 0,
  removedFiles: 0,
  addedBytes: 0,
  addedFiles: 0,
  changedBytes: 0,
  changedFiles: 0,
  ...over,
});

it("lists each folder with what was removed, added and changed, and leaves a cell empty where nothing was", async () => {
  const photos = folder("data/photos", { addedBytes: 1100 * MB, addedFiles: 12 });
  const other = folder("", { removedBytes: 2 * MB, removedFiles: 3, addedBytes: 4 * MB, addedFiles: 68, changedBytes: 4 * MB, changedFiles: 31 });
  ready({ total: folder("", { addedFiles: 80, removedFiles: 3, changedFiles: 31 }), folders: [photos], other });
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });

  const table = screen.getByRole("table");
  expect(within(table).getAllByRole("columnheader").map((th) => th.textContent)).toEqual([
    en["storage.folder"],
    en["anomaly.changes.col.removed"],
    en["anomaly.changes.col.added"],
    en["anomaly.changes.col.changed"],
  ]);
  const cells = (name: string) =>
    within(within(table).getByRole("row", { name: new RegExp(`^${name}`) }))
      .getAllByRole("cell")
      .map((td) => td.textContent);
  expect(cells("data/photos/")).toEqual(["", "1.1 GB", ""]);
  expect(cells(en["anomaly.changes.rest"])).toEqual(["2.0 MB", "4.0 MB", "31 files"]);
});

it("says so instead of drawing a table when both backups hold the same files", async () => {
  ready({ total: folder(""), folders: [] });
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });
  expect(screen.getByText(en["anomaly.changes.none"])).toBeTruthy();
  expect(screen.queryByRole("table")).toBeNull();
});

it("warns that the figures are a lower bound when the comparison could not follow every file", async () => {
  const photos = folder("data/photos", { changedBytes: MB, changedFiles: 5 });
  ready({ total: photos, folders: [photos], partial: true });
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });
  expect(screen.getByText(en["breakdown.partial"])).toBeTruthy();
});

it("offers to try again when the comparison failed", async () => {
  getAnomalyChanges.mockResolvedValueOnce({ ok: false, error: "the repository is locked" }).mockResolvedValueOnce(indexRebuild());
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });
  expect(screen.getByText(en["breakdown.failed"].replace("{error}", "the repository is locked"))).toBeTruthy();
  await act(async () => {
    screen.getByRole("button", { name: en["breakdown.retry"] }).click();
  });
  expect(getAnomalyChanges).toHaveBeenLastCalledWith("f1", true);
  expect(screen.getByRole("table")).toBeTruthy();
});

it("only offers a comparison where the server can make one", () => {
  expect(findingHasChanges(finding)).toBe(true);
  expect(findingHasChanges({ ...finding, metric: "failure_streak" })).toBe(false);
  expect(findingHasChanges({ ...finding, scopeKind: "zfsds" })).toBe(false);
  expect(findingHasChanges({ ...finding, domain: "flash" })).toBe(false);
});

it("says plainly when no folder holds both backups", async () => {
  getAnomalyChanges.mockResolvedValue({ ok: true, changes: { state: "apart", fromAt: 1758000000, toAt: 1758600000 } });
  await act(async () => {
    render(<FindingChanges a={finding} t={t} />);
  });
  expect(screen.getByText(en["anomaly.changes.apart"])).toBeTruthy();
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.queryByRole("button", { name: en["breakdown.retry"] })).toBeNull();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(4000);
  });
  expect(getAnomalyChanges).toHaveBeenCalledTimes(1);
});
