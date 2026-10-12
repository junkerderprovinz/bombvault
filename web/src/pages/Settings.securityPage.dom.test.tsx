// @vitest-environment jsdom
// Settings > Security: the login password. The api module is mocked, so the
// password write is a captured call.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

const passwords: string[] = [];
let authOn = true;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: {} as Settings, hostMountRoot: "/host/user", platform: "unraid" }),
    getAuth: () => Promise.resolve({ enabled: authOn, authed: authOn, minPasswordLen: 12 }),
    setAuthPassword: (pw: string) => {
      passwords.push(pw);
      return Promise.resolve({ ok: true, enabled: pw !== "" });
    },
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    passkeyStatus: () => Promise.resolve({ ok: true, passkeys: [] }),
    getPhonePairing: () => Promise.resolve({ ok: true, enabled: false }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

function stubBrowser() {
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
}

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
      </MemoryRouter>
    );
  });
}

function passwordCard(): HTMLElement {
  return screen.getByRole("heading", { name: new RegExp(`^${en["auth.security"]}`) }).parentElement as HTMLElement;
}

beforeEach(() => {
  stubBrowser();
  passwords.length = 0;
  authOn = true;
});

afterEach(() => {
  cleanup();
});

describe("the login password", () => {
  it("asks before empty fields turn the login off, and keeps it on when declined", async () => {
    await renderPage();
    await waitFor(() => expect(within(passwordCard()).getByText(en["auth.authOn"])).toBeTruthy());
    fireEvent.click(within(passwordCard()).getByRole("button", { name: en["settings.save"] }));
    const ask = await screen.findByRole("dialog");
    expect(within(ask).getByText(en["auth.turnOffAsk"])).toBeTruthy();
    fireEvent.click(within(ask).getByRole("button", { name: en["common.cancel"] }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(passwords).toEqual([]);
  });

  it("turns the login off once the question is answered", async () => {
    await renderPage();
    await waitFor(() => expect(within(passwordCard()).getByText(en["auth.authOn"])).toBeTruthy());
    fireEvent.click(within(passwordCard()).getByRole("button", { name: en["settings.save"] }));
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["auth.loginDisable"] }));
    await waitFor(() => expect(passwords).toEqual([""]));
  });

  it("sets a first password without asking", async () => {
    authOn = false;
    await renderPage();
    const card = passwordCard();
    const [pw, again] = card.querySelectorAll<HTMLInputElement>("input");
    fireEvent.change(pw, { target: { value: "correct horse battery" } });
    fireEvent.change(again, { target: { value: "correct horse battery" } });
    fireEvent.click(within(card).getByRole("button", { name: en["settings.save"] }));
    await waitFor(() => expect(passwords).toEqual(["correct horse battery"]));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
