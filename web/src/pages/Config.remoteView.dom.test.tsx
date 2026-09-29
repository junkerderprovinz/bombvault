// @vitest-environment jsdom
// Config keeps its own backup trigger in remote view (POST /api/config/backup
// is on the allowlist), and the snapshot list is metadata, but the self-backup
// toggle, the snapshot delete button and the item's anomaly sensitivity editor
// have no route on the allowlist, so they must disappear rather than merely
// disable while a peer's instance is open.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

function baseSettings(): Settings {
  return {
    configEnabled: true,
    configPath: "backups/config",
    configOffsite: "",
    configSchedule: "off",
    containersPath: "backups/containers",
    registryAuths: [],
    anomalySensitivity: "balanced",
    anomalyNotifyMin: "critical",
  } as unknown as Settings;
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: baseSettings() }),
    putSettings: () => Promise.resolve({ ok: true }),
    listConfigSnapshots: () =>
      Promise.resolve({
        ok: true,
        snapshots: [{ id: "a1b2c3d4e5f6", time: "2026-09-01T10:00:00Z", paths: [], tags: [], hostname: "tower" }],
      }),
    getAnomalyItems: () => Promise.resolve({ ok: true, items: [] }),
    getAnomalySummary: () => Promise.resolve({ ok: true, summary: { enabled: true } }),
  };
});

const { Config } = await import("./Config");

afterEach(cleanup);

async function renderConfig(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <ToastProvider>
              <Config />
            </ToastProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

it("offers the self-backup toggle and the snapshot delete button locally", async () => {
  await renderConfig(["/config"]);
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["config.backupNow"] })).toBeTruthy();
  expect(screen.getAllByRole("switch", { name: en["config.enabled"] })[0]).toBeTruthy();
  expect(screen.getByRole("button", { name: en["snapshots.delete"] })).toBeTruthy();
});

it("hides the self-backup toggle and the snapshot delete button while a peer's instance is open, and keeps the backup trigger and the list", async () => {
  await renderConfig(["/config?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("a1b2c3d4")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["config.backupNow"] })).toBeTruthy();
  expect(screen.queryByRole("switch", { name: en["config.enabled"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["snapshots.delete"] })).toBeNull();
});
