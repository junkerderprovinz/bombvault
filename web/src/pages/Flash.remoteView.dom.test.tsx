// @vitest-environment jsdom
// Flash is one of remote view's own pages (its "backup now" trigger stays
// allowed), but the zip download and the delete button on each snapshot are
// local-only: a download cannot cross the group's JSON forward, and a delete
// is destructive. Both must disappear while a peer's instance is open, and
// the metadata list they sit in must still show.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
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

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listFlashSnapshots: () =>
      Promise.resolve({
        ok: true,
        snapshots: [{ id: "a1b2c3d4e5f6", time: "2026-09-01T10:00:00Z", paths: [], tags: [], hostname: "tower" }],
      }),
    listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  };
});

const { Flash } = await import("./Flash");

afterEach(cleanup);

async function renderFlash(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
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
}

it("shows the download and delete buttons locally", async () => {
  await renderFlash(["/flash"]);
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByText(en["flash.download"])).toBeTruthy();
  expect(screen.getByText(en["snapshots.delete"])).toBeTruthy();
});

it("hides the download and delete buttons while a peer's instance is open, and still lists the snapshot", async () => {
  await renderFlash(["/flash?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.queryByText(en["flash.download"])).toBeNull();
  expect(screen.queryByText(en["snapshots.delete"])).toBeNull();
});
