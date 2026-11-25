// @vitest-environment jsdom
// Settings > Security: the address passkeys and sign-in services need, and the kit's sign-in tile. The api module is
// mocked, and the kit's contract answers from a stub of fetch.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

const saved: string[] = [];
let refuse = false;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: {} as Settings, hostMountRoot: "/host/user", platform: "unraid" }),
    getAuth: () => Promise.resolve({ ok: true, enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    getPublicAddress: () => Promise.resolve({ ok: true, address: "" }),
    setPublicAddress: (address: string) => {
      saved.push(address);
      return Promise.resolve(refuse ? { ok: false, code: "address-invalid" } : { ok: true, address });
    },
  };
});

const { SettingsPage } = await import("./Settings");

// The kit answers an open instance: everyone is admin, and nothing is set up yet.
function kitAnswer(url: string): unknown {
  const path = new URL(url, "http://bv.test").pathname.replace(/^.*\/api\/signin\/v1\//, "");
  if (path === "me") return { role: "admin", via: "open", owner: false, password: false, twoFactor: false, recoveryLeft: 0, fresh: true, askPassword: true };
  if (path === "settings") return { askPassword: true };
  if (path === "options") return { open: true, owner: true, minPassword: 12, signedIn: false, passkeys: false, services: [] };
  if (path === "roles") return { roles: ["admin"], serviceRole: "admin" };
  return { sessions: [], tokens: [], passkeys: [], services: [], accounts: [], presets: [] };
}

beforeEach(() => {
  saved.length = 0;
  refuse = false;
  window.matchMedia = ((query: string) => ({
    matches: true,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  vi.stubGlobal("fetch", (url: string) => Promise.resolve(new Response(JSON.stringify(kitAnswer(String(url))), { status: 200, headers: { "Content-Type": "application/json" } })));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function renderPage() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/settings/security"]}>
        <Routes>
          <Route
            path="/settings/:page"
            element={
              <I18nProvider>
                <ToastProvider>
                  <SettingsPage />
                </ToastProvider>
              </I18nProvider>
            }
          />
        </Routes>
      </MemoryRouter>,
    );
  });
}

describe("the Security page", () => {
  it("keeps the address once the field is left", async () => {
    await renderPage();
    const field = (await screen.findByLabelText(en["signin.address.label"])) as HTMLInputElement;
    fireEvent.change(field, { target: { value: "https://backup.example" } });
    await act(async () => {
      fireEvent.blur(field);
    });
    expect(saved).toEqual(["https://backup.example"]);
    expect(await screen.findByText(en["signin.address.saved"])).toBeTruthy();
  });

  it("says what is wrong with an address it refuses", async () => {
    refuse = true;
    await renderPage();
    const field = await screen.findByLabelText(en["signin.address.label"]);
    fireEvent.change(field, { target: { value: "backup" } });
    await act(async () => {
      fireEvent.blur(field);
    });
    expect(await screen.findByText(en["signin.address.invalid"])).toBeTruthy();
  });

  it("draws the sign-in tile with the password first", async () => {
    await renderPage();
    await waitFor(() => expect(screen.getAllByText(en["signin.tile.password.none"]).length).toBeGreaterThan(0));
  });

  it("offers no switch for phones, which always pair with the group's words", async () => {
    await renderPage();
    await waitFor(() => expect(screen.getAllByText(en["signin.tile.password.none"]).length).toBeGreaterThan(0));
    expect(screen.queryByText(en["apps.phone.title"])).toBeNull();
  });
});
