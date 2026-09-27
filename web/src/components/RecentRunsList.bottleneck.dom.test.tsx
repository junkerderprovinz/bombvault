// @vitest-environment jsdom
// A slow backup that one thing clearly held back says so under its row.
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
    target: "plex",
    domain: "container",
    ...over,
  } as Run;
}

afterEach(cleanup);

it("names the brake under a slow run and nothing under the others", async () => {
  listRuns.mockResolvedValue({
    ok: true,
    runs: [
      run({ id: "slow", bottleneck: { kind: "disk", name: "disk1", role: "target", share: 0.976 } }),
      run({ id: "cpu", bottleneck: { kind: "cpu", share: 0.93 } }),
      run({ id: "limit", bottleneck: { kind: "cpulimit", share: 0.99 } }),
      run({ id: "plain" }),
    ],
  });
  await act(async () => {
    render(<RecentRunsList name="plex" domain="container" t={t} />);
  });
  expect(screen.getByText("Slower than usual. The target disk disk1 was 98% busy.")).toBeTruthy();
  expect(screen.getByText("Slower than usual. The CPU was 93% busy.")).toBeTruthy();
  expect(screen.getByText("Slower than usual. BombVault used 99% of the CPU limit of its container.")).toBeTruthy();
  expect(screen.getAllByText(/Slower than usual/)).toHaveLength(3);
});
