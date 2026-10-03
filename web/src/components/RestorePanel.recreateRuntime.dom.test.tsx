// @vitest-environment jsdom
// Recreating a removed container from its definition can hit a GPU or runtime
// this host lacks, and offers the recreate again without them.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { Run } from "../lib/api";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());

let runs: Run[] = [];
const restore = vi.fn((name: string, ...rest: unknown[]) => {
  const refused = rest[4] !== true;
  runs = [
    {
      id: `r${runs.length + 1}`,
      targetId: "t1",
      kind: "restore",
      status: refused ? "failed" : "success",
      startedAt: 1_700_000_000,
      finishedAt: 1_700_000_010,
      snapshotId: "",
      bytes: 0,
      error: refused
        ? "restore failed: the container used a GPU or runtime this host does not have: Error response from daemon: unknown or invalid runtime name: nvidia"
        : "restored without the GPU or runtime the container used",
      target: name,
      domain: "container",
    },
    ...runs,
  ];
  return Promise.resolve({ ok: true, started: true });
});

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    ...fake.api,
    listRuns: () => Promise.resolve({ ok: true, runs }),
    getSettings: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
    checkRestore: () =>
      Promise.resolve({
        ok: true,
        ready: true,
        checks: [
          { id: "repository", status: "ok" },
          { id: "key", status: "ok" },
          { id: "snapshot", status: "ok", detail: "latest" },
          { id: "space", status: "ok", need: 1, free: 10 },
        ],
        plan: null,
      }),
    restore: (name: string, ...rest: unknown[]) => restore(name, ...rest),
  };
});

const { RestorePanel } = await import("./RestorePanel");

const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as Parameters<typeof RestorePanel>[0]["t"];

afterEach(() => {
  cleanup();
  runs = [];
});

it("recreates again without the GPU and runtime when asked", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RestorePanel name="plex" t={t} open installed={false} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: en["snapshots.recreate"] }).hasAttribute("disabled")).toBe(false),
  );
  fireEvent.click(screen.getByRole("button", { name: en["snapshots.recreate"] }));
  await act(async () => {
    fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
  });
  const retry = await screen.findByRole("button", { name: en["restore.withoutRuntime"] }, { timeout: 8000 });
  expect(document.body.textContent).toContain(en["restore.noRuntime"].replace("{name}", "plex"));
  await act(async () => {
    fireEvent.click(retry);
  });
  await waitFor(() => expect(restore).toHaveBeenCalledTimes(2));
  expect(restore.mock.calls[1]).toEqual(["plex", "latest", true, "local", false, true]);
  expect(
    await screen.findByText(en["restore.withoutRuntimeDone"].replace("{name}", "plex"), undefined, { timeout: 8000 }),
  ).toBeTruthy();
}, 30000);
