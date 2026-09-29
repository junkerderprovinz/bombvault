// @vitest-environment jsdom
// A container's snapshot list shows one source at a time. After a switch to
// off-site fails, neither the local rows nor anything that acts on them may
// stay on screen under the off-site toggle.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdvancedProvider } from "../lib/advanced";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listSnapshots: (_name: string, source?: string) =>
      Promise.resolve(
        source === "local"
          ? {
              ok: true,
              snapshots: [{ id: "a1b2c3d4e5f6", time: "2026-09-01T10:00:00Z", paths: [], tags: [], hostname: "tower" }],
            }
          : { ok: false, snapshots: [], error: "off-site repository unreachable" },
      ),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getSettings: () => Promise.resolve({ ok: false }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  };
});

const { RestorePanel } = await import("./RestorePanel");

const t = ((key: string) => en[key as keyof typeof en] ?? key) as unknown as Parameters<typeof RestorePanel>[0]["t"];

beforeEach(() => localStorage.setItem("bombvault.advanced", "1"));
afterEach(() => {
  cleanup();
  localStorage.removeItem("bombvault.advanced");
});

it("drops the local rows when the off-site list fails", async () => {
  await act(async () => {
    render(
      <MemoryRouter>
        <InstanceProvider>
          <I18nProvider>
            <AdvancedProvider>
              <ToastProvider>
                <RestorePanel name="plex" t={t} open />
              </ToastProvider>
            </AdvancedProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();

  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: en["source.offsite"] }));
  });

  expect(await screen.findByText("off-site repository unreachable")).toBeTruthy();
  expect(screen.queryByText("a1b2c3d4")).toBeNull();
});
