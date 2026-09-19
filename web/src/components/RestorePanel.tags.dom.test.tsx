// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { TranslationKey } from "../lib/i18n";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    getSettings: vi.fn(() => new Promise(() => {})),
    listRuns: vi.fn(async () => ({ ok: true, runs: [] })),
    listSnapshots: vi.fn(async () => ({
      ok: true,
      snapshots: [
        {
          id: "0123456789",
          time: "2026-09-01T00:00:00Z",
          paths: [],
          hostname: "tower",
          tags: [
            "container:radarr",
            "container:radarr-movies",
            "container:radarr-old",
            "formerly:radarr-movies",
            "p1",
            "before-upgrade",
          ],
        },
      ],
    })),
  };
});

const { RestorePanel } = await import("./RestorePanel");
const { AdvancedProvider } = await import("../lib/advanced");
const { en } = await import("../lib/i18n");

const t = ((key: TranslationKey) => en[key]) as unknown as Parameters<typeof RestorePanel>[0]["t"];

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("hides the tags of the entry's own name and its former names", async () => {
  localStorage.setItem("bombvault.advanced", "1");
  render(
    <AdvancedProvider>
      <RestorePanel name="radarr" aliases={["radarr-movies", "radarr-old"]} t={t} open />
    </AdvancedProvider>,
  );

  expect(await screen.findByText("before-upgrade")).toBeTruthy();
  expect(screen.queryByText("container:radarr")).toBeNull();
  expect(screen.queryByText("container:radarr-movies")).toBeNull();
  expect(screen.queryByText("container:radarr-old")).toBeNull();
});

it("hides the formerly marker tag a takeover leaves on older backups", async () => {
  localStorage.setItem("bombvault.advanced", "1");
  render(
    <AdvancedProvider>
      <RestorePanel name="radarr" aliases={["radarr-movies", "radarr-old"]} t={t} open />
    </AdvancedProvider>,
  );

  expect(await screen.findByText("before-upgrade")).toBeTruthy();
  expect(screen.queryByText("formerly:radarr-movies")).toBeNull();
});
