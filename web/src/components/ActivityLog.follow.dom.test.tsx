// @vitest-environment jsdom
// The log follows its newest line until the reader scrolls up, and a line
// that arrives just as they do must not pull them back down.
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider, en } from "../lib/i18n";
import type { Run, ScheduleNext } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const schedule = vi.hoisted(() => ({ answer: (_: ScheduleNext[]) => {} }));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  const runs: Run[] = Array.from({ length: 20 }, (_, i) => ({
    id: `r${i}`,
    targetId: `t${i}`,
    kind: "backup",
    status: "success",
    startedAt: 1_700_000_000 + i * 3600,
    finishedAt: 1_700_000_060 + i * 3600,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: `app-${i}`,
    domain: "container",
  }) as Run);
  return {
    ...actual,
    listRuns: () => Promise.resolve({ ok: true, runs }),
    getScheduleNext: () => new Promise<ScheduleNext[]>((resolve) => (schedule.answer = resolve)),
  };
});

const { ActivityLog } = await import("./ActivityLog");

afterEach(() => {
  cleanup();
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

// jsdom has no layout, so the scroller gets a height and a scroll position.
function lay(el: HTMLElement) {
  let top = 0;
  Object.defineProperty(el, "clientHeight", { configurable: true, get: () => 200 });
  Object.defineProperty(el, "scrollHeight", { configurable: true, get: () => el.children.length * 40 });
  Object.defineProperty(el, "scrollTop", { configurable: true, get: () => top, set: (v: number) => (top = Math.min(v, el.scrollHeight - 200)) });
}

it("stays where the reader scrolled when a line arrives as they scroll up", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ActivityLog />
      </I18nProvider>
    );
  });
  const log = screen.getByTestId("activity-log").querySelector<HTMLElement>(".overflow-y-auto")!;
  lay(log);
  log.scrollTop = log.scrollHeight;

  // The reader scrolls up after the new line is on screen and before the
  // effects of that render run, which is how a browser can order the two.
  new MutationObserver((_, observer) => {
    observer.disconnect();
    log.scrollTop = 0;
    log.dispatchEvent(new Event("scroll"));
  }).observe(log, { childList: true });

  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
  schedule.answer([{ job: "containers", domain: "container", next: "2030-01-01T00:00:00Z" }]);

  await waitFor(() => expect(screen.getByRole("button", { name: en["activityLog.jumpToLatest"] })).toBeTruthy());
  expect(log.scrollTop).toBe(0);
});
