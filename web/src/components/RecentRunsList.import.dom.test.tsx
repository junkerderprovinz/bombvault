// @vitest-environment jsdom
// An import leaves a restore point like a backup does, so it is listed with
// the container's backups and says what it was.
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { countText, en } from "../lib/i18n";
import type { Run } from "../lib/api";

const listRuns = vi.fn();
vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: (...a: unknown[]) => listRuns(...a),
    getAnomalies: () => Promise.resolve({ ok: true, anomalies: [] }),
    getAnomalySummary: () => Promise.resolve({ ok: false }),
  };
});

const { RecentRunsList } = await import("./RecentRunsList");

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof RecentRunsList
>[0]["t"];

function run(over: Partial<Run>): Run {
  return {
    id: "r1",
    targetId: "t1",
    kind: "backup",
    status: "success",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_600,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: "radarr",
    domain: "container",
    ...over,
  } as Run;
}

afterEach(cleanup);

it("lists an import among the backups and names it", async () => {
  listRuns.mockResolvedValue({
    ok: true,
    runs: [
      run({ id: "imported", kind: "import", startedAt: 1_690_000_000, finishedAt: 1_690_000_000 }),
      run({ id: "backup" }),
      run({ id: "restore", kind: "restore" }),
      run({ id: "other", kind: "import", target: "sonarr" }),
    ],
  });
  await act(async () => {
    render(<RecentRunsList name="radarr" domain="container" t={t} />);
  });
  expect(screen.getAllByText("Import")).toHaveLength(1);
  expect(document.querySelectorAll(".rounded-full")).toHaveLength(2);
});
