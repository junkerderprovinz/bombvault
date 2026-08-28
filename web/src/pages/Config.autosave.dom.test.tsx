// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Self-backup page (#182, manilx) — the toggle saves itself.
//
// "switching the setting autosaves ... here i need to select save button." The
// card kept a Save button because it used to own three text fields; once those
// moved to Settings (the path row and the off-site card), a lone toggle sitting
// behind a button was the only control of its kind left in the app.
//
// What this pins is the SAVE, not the button's absence: a toggle that flips
// visually and forgets the change on reload is the failure worth catching.
// ---------------------------------------------------------------------------
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
// A no-op stand-in keeps this test about the toggle rather than about SSE.
class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const putBodies: Settings[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: baseSettings() }),
    putSettings: (s: Settings) => {
      putBodies.push(s);
      return Promise.resolve({ ok: true });
    },
    listConfigSnapshots: () => Promise.resolve({ ok: true, snapshots: [] }),
  };
});

const { Config } = await import("./Config");

beforeEach(() => {
  putBodies.length = 0;
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
  // The NEW value has to reach the server. Reading it from component state
  // instead would send the old one, because an autosave fires from inside the
  // change handler, before the state update has landed.
  expect(putBodies.at(-1)?.configEnabled).toBe(false);
});

it("no longer offers a save button on this card", async () => {
  await renderPage();
  // Not a style preference: a button next to a control that already saved is
  // what made manilx expect the toggle NOT to have saved.
  expect(screen.queryByRole("button", { name: en["settings.save"] })).toBeNull();
});
