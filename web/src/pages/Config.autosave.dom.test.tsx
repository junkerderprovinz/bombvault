// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(): Settings {
  return {
    configEnabled: true,
    configPath: "backups/config",
    configOffsite: "",
    configSchedule: "off",
    containersPath: "backups/containers",
    registryAuths: [],
  } as unknown as Settings;
}

// jsdom has no EventSource, and the page opens the progress stream on mount.
class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const putBodies: Settings[] = [];
const timelineCalls: unknown[][] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: baseSettings() }),
    putSettings: (s: Settings) => {
      putBodies.push(s);
      return Promise.resolve({ ok: true });
    },
    getTimeline: (...args: unknown[]) => {
      timelineCalls.push(args);
      return Promise.resolve({ ok: true, places: [], rows: [] });
    },
  };
});

const { Config } = await import("./Config");

beforeEach(() => {
  putBodies.length = 0;
  timelineCalls.length = 0;
});

afterEach(() => {
  cleanup();
});

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Config />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

it("persists the self-backup toggle without a save button", async () => {
  await renderPage();

  const toggle = screen.getAllByRole("switch", { name: en["config.enabled"] })[0];
  await act(async () => {
    fireEvent.click(toggle);
  });

  await waitFor(() => expect(putBodies.length).toBeGreaterThan(0));
  // The save runs inside the change handler, before the state update lands, so
  // it has to send the new value rather than the one in component state.
  expect(putBodies.at(-1)?.configEnabled).toBe(false);
});

it("has no save button on this card", async () => {
  await renderPage();
  expect(screen.queryByRole("button", { name: en["settings.save"] })).toBeNull();
});

it("lists the settings backups as the config timeline", async () => {
  await renderPage();
  await waitFor(() => expect(timelineCalls).toEqual([["config", "config"]]));
});
