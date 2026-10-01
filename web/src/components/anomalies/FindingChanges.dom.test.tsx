// @vitest-environment jsdom
// Opening the comparison asks the server to diff the last good backup against
// the first affected one; it polls while that runs and then names the folders.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { countText, en } from "../../lib/i18n";
import type { AnomalyChanges, AnomalyView } from "../../lib/api";

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
  render(<FindingChanges a={finding} t={t} />);
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: /Compare with the backup before/ }));
  });
  expect(screen.getByRole("status").textContent).toBe("Working it out…");
  await act(async () => {
    await vi.advanceTimersByTimeAsync(2000);
  });
  expect(getAnomalyChanges).toHaveBeenCalledTimes(2);
  expect(screen.getByText(/Nearly all of it is in .*data\/index\//)).toBeTruthy();
  expect(screen.getByText(/looks like a search index/)).toBeTruthy();
  expect(screen.getByText("1051 files")).toBeTruthy();
  expect(screen.queryByText(/changed/)).toBeNull();
  expect(screen.getByText("data/index/")).toBeTruthy();
});

it("only offers a comparison where the server can make one", () => {
  expect(findingHasChanges(finding)).toBe(true);
  expect(findingHasChanges({ ...finding, metric: "failure_streak" })).toBe(false);
  expect(findingHasChanges({ ...finding, scopeKind: "zfsds" })).toBe(false);
  expect(findingHasChanges({ ...finding, domain: "flash" })).toBe(false);
});
