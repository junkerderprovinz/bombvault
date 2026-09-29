// @vitest-environment jsdom
// The snapshot list shows one source at a time. Switching to off-site must not
// leave the local rows on screen, least of all when the off-site list fails
// and its error would otherwise sit right above them.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { InstanceProvider } from "../lib/instanceScope";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const pendingDelete = vi.hoisted(() => ({ resolve: (_: { ok: boolean }) => {} }));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listFlashSnapshots: (source?: string) =>
      Promise.resolve(
        source === "local"
          ? {
              ok: true,
              snapshots: [{ id: "a1b2c3d4e5f6", time: "2026-09-01T10:00:00Z", paths: [], tags: [], hostname: "tower" }],
            }
          : { ok: false, snapshots: [], error: "off-site repository unreachable" },
      ),
    deleteSnapshot: () =>
      new Promise((resolve) => {
        pendingDelete.resolve = resolve;
      }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  };
});

const { Flash } = await import("./Flash");

afterEach(cleanup);

async function renderPage() {
  await act(async () => {
    render(
      <MemoryRouter>
        <InstanceProvider>
          <I18nProvider>
            <ToastProvider>
              <Flash />
            </ToastProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
}

async function switchToOffsite() {
  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: en["source.offsite"] }));
  });
  expect(await screen.findByText("off-site repository unreachable")).toBeTruthy();
}

it("keeps the list when the active source is picked again", async () => {
  await renderPage();
  await act(async () => {
    fireEvent.click(screen.getByRole("tab", { name: en["source.local"] }));
  });
  expect(screen.getByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByRole("tab", { name: en["source.offsite"] }).hasAttribute("disabled")).toBe(false);
});

it("drops the local rows when the off-site list fails", async () => {
  await renderPage();
  await switchToOffsite();
  expect(screen.queryByText("a1b2c3d4")).toBeNull();
});

it("keeps the local rows away when a local delete finishes after the switch", async () => {
  await renderPage();
  fireEvent.click(screen.getByRole("button", { name: en["snapshots.delete"] }));
  const dialog = await screen.findByRole("dialog");
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: en["snapshots.delete"] }));
  });

  await switchToOffsite();
  await act(async () => {
    pendingDelete.resolve({ ok: true });
  });

  expect(screen.getByText("off-site repository unreachable")).toBeTruthy();
  expect(screen.queryByText("a1b2c3d4")).toBeNull();
});
