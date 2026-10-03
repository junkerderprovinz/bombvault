// @vitest-environment jsdom
// The retry after Docker refused the GPU asks the server for the same restore
// without the GPU and runtime.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";

const restore = vi.fn((..._a: unknown[]) => Promise.resolve({ ok: true, started: true }));
let runs: unknown[] = [];
const refused = {
  id: "r1",
  targetId: "t1",
  kind: "restore",
  domain: "container",
  target: "plex",
  status: "failed",
  startedAt: 1_700_000_000,
  finishedAt: 1_700_000_060,
  snapshotId: "",
  bytes: 0,
  acknowledged: false,
  error:
    'restore failed: the container used a GPU or runtime this host does not have: Error response from daemon: could not select device driver "" with capabilities: [[gpu]]',
};

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    restore: (...a: unknown[]) => restore(...a),
    checkRestore: () => Promise.resolve({ ok: true, ready: true, checks: [], plan: null }),
    listRuns: () => Promise.resolve({ ok: true, runs }),
  };
});

class StubEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as { EventSource?: unknown }).EventSource = StubEventSource;

const { RestoreAction } = await import("./RestoreAction");

afterEach(() => {
  cleanup();
  restore.mockClear();
  runs = [];
});

it("restores again without the GPU and runtime when asked", async () => {
  const t = ((k: string) => en[k as keyof typeof en] ?? k) as never;
  render(
    <I18nProvider>
      <RestoreAction
        domain="container"
        name="plex"
        snapshotId="aaaa1111"
        otherActive={{ active: false }}
        successMessage="done"
        requireConfirm={false}
        showLeaveStopped={false}
        showStartedHint={false}
        iconBadge
        t={t}
      />
    </I18nProvider>
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["snapshots.restore"] }));
  });
  await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  expect(restore.mock.calls[0][5]).toBeFalsy();
  runs = [refused];

  const retry = await screen.findByRole("button", { name: en["restore.withoutRuntime"] }, { timeout: 6000 });
  await act(async () => {
    fireEvent.click(retry);
  });
  await waitFor(() => expect(restore).toHaveBeenCalledTimes(2));
  expect(restore.mock.calls[1].slice(0, 2)).toEqual(["plex", "aaaa1111"]);
  expect(restore.mock.calls[1][5]).toBe(true);
}, 15000);
