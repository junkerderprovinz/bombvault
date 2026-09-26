// @vitest-environment jsdom
// Connect & preview saves step 3's addresses. One that a storage place owns
// stays as the server keeps it, and the page shows and says so.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
import { stubEventSource } from "../lib/placement.testsupport";

const stored = {
  containersPath: "backups/containers",
  vmsPath: "backups/vms",
  flashPath: "backups/flash",
  filesPath: "backups/files",
  configPath: "backups/config",
  configOffsite: "",
  containersOffsite: "",
  vmsOffsite: "",
  flashOffsite: "",
  filesOffsite: "",
  encryptionEnabled: true,
} as Settings;
const puts: Settings[] = [];
let kept: string[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getSettings: () => Promise.resolve({ ok: true, settings: { ...stored }, hostMountRoot: "/host/user" }),
  putSettings: (s: Settings) => {
    puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [], kept });
  },
  detectEncryption: () => Promise.resolve({ ok: true, verdict: "absent", repos: [] }),
  discover: () => Promise.resolve({ ok: true, discovered: 0 }),
  discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
  discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
  getVMSSH: () => Promise.resolve({ ok: false }),
}));

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places: [], unplaced: [] }),
}));

const { default: Recovery } = await import("./Recovery");

beforeEach(() => {
  stubEventSource();
  puts.length = 0;
  kept = [];
});
afterEach(cleanup);

async function connect(path: string) {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <Recovery />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  fireEvent.change(screen.getByDisplayValue("backups/containers"), { target: { value: path } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["recovery.connectPreview"] }));
  });
}

describe("Recovery's Connect & preview", () => {
  it("shows the stored path again and says why when a storage place keeps it", async () => {
    kept = ["containersPath"];
    await connect("backups/old");
    expect(puts[0]?.containersPath).toBe("backups/old");
    expect(screen.getByDisplayValue("backups/containers")).toBeTruthy();
    expect(screen.queryByDisplayValue("backups/old")).toBeNull();
    expect(screen.getByText(en["recovery.placeKept"])).toBeTruthy();
  });

  it("keeps a saved path as typed and says nothing about places", async () => {
    await connect("backups/old");
    expect(screen.getByDisplayValue("backups/old")).toBeTruthy();
    expect(screen.queryByText(en["recovery.placeKept"])).toBeNull();
  });
});
