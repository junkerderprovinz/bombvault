// @vitest-environment jsdom
// The card shows a check, restore check or prune that runs for a domain,
// wherever it was started, and has to let go of it when it ends or dies.
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ProgressMap } from "../../lib/progress";

const getStatus = vi.fn();
let progressMap: ProgressMap = {};

vi.mock("../../lib/api", () => ({
  unlockDomain: vi.fn(),
  checkDomain: vi.fn().mockResolvedValue({ ok: true }),
  pruneDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: vi.fn().mockResolvedValue({ ok: true }),
  tamperTest: vi.fn().mockResolvedValue({ ok: true }),
  getStatus: (...a: unknown[]) => getStatus(...a),
  listItems: vi.fn().mockResolvedValue({ ok: true, items: [] }),
}));

vi.mock("../../lib/progress", async (orig) => ({
  ...(await orig<typeof import("../../lib/progress")>()),
  useProgress: () => progressMap,
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: () => {}, quiet: false, setQuiet: () => {} }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { countText, en } from "../../lib/i18n";
import { domain } from "../overview/overview.testsupport";

const t = ((key: string, n?: number) => countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<typeof IntegrityCard>[0]["t"];

function card() {
  return <IntegrityCard t={t} />;
}

function running(lastSeen: number): ProgressMap {
  return {
    "drill:files": { phase: "maintenance", percent: 40, active: true, lastSeen, done: 40, total: 100, unit: "packs" },
  };
}

beforeEach(() => {
  getStatus.mockReset();
  getStatus.mockResolvedValue({ ok: true, domains: [domain({ domain: "files" })] });
  progressMap = {};
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("shows how far a restore check started elsewhere has got", async () => {
  progressMap = running(Date.now());
  await act(async () => {
    render(card());
  });
  expect(screen.getByRole("progressbar")).toBeTruthy();
  expect(screen.getByText(/40 of 100 packs/)).toBeTruthy();
});

it("drops a bar whose run stopped reporting, as when BombVault restarted under it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  progressMap = running(Date.now());
  await act(async () => {
    render(card());
  });
  expect(screen.queryByRole("progressbar")).toBeTruthy();
  await act(async () => {
    vi.advanceTimersByTime(30_000);
  });
  expect(screen.queryByRole("progressbar")).toBeNull();
});

it("reads the checks again once a live run ends", async () => {
  progressMap = running(Date.now());
  const view = await act(async () => render(card()));
  const before = getStatus.mock.calls.length;

  progressMap = {};
  await act(async () => {
    view.rerender(card());
  });
  const after = getStatus.mock.calls.length;
  expect(after).toBe(before + 1);
});
