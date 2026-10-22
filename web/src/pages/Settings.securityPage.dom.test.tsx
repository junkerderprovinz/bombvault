// @vitest-environment jsdom
// Settings > Security: the login password. The api module is mocked, so the
// password write is a captured call.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { SetPasswordResponse, Settings } from "../lib/api";

const writes: { password: string; current: string; code: string }[] = [];
let authOn = true;
let totpOn = false;
let answer: SetPasswordResponse | null = null;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: {} as Settings, hostMountRoot: "/host/user", platform: "unraid" }),
    getAuth: () => Promise.resolve({ enabled: authOn, authed: authOn, totp: totpOn, minPasswordLen: 12 }),
    setAuthPassword: (password: string, current = "", code = "") => {
      writes.push({ password, current, code });
      return Promise.resolve(answer ?? { ok: true, enabled: password !== "" });
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

async function signedInCard(): Promise<HTMLElement> {
  await renderPage();
  await waitFor(() => expect(within(passwordCard()).getByText(en["auth.authOn"])).toBeTruthy());
  return passwordCard();
}

function typeInto(card: HTMLElement, label: string, value: string) {
  fireEvent.change(within(card).getByLabelText(label), { target: { value } });
}

function save(card: HTMLElement) {
  fireEvent.click(within(card).getByRole("button", { name: en["settings.save"] }));
}

function typeNewPassword(card: HTMLElement) {
  typeInto(card, en["auth.changePassword"], "correct horse battery");
  typeInto(card, en["auth.confirmPassword"], "correct horse battery");
}

beforeEach(() => {
  stubBrowser();
  writes.length = 0;
  authOn = true;
  totpOn = false;
  answer = null;
});

afterEach(() => {
  cleanup();
});

describe("the login password", () => {
  it("asks before empty fields turn the login off, and keeps it on when declined", async () => {
    const card = await signedInCard();
    typeInto(card, en["auth.currentPassword"], "the old one");
    save(card);
    const ask = await screen.findByRole("dialog");
    expect(within(ask).getByText(en["auth.turnOffAsk"])).toBeTruthy();
    fireEvent.click(within(ask).getByRole("button", { name: en["common.cancel"] }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(writes).toEqual([]);
  });

  it("turns the login off once the question is answered", async () => {
    const card = await signedInCard();
    typeInto(card, en["auth.currentPassword"], "the old one");
    save(card);
    fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: en["auth.loginDisable"] }));
    await waitFor(() => expect(writes).toEqual([{ password: "", current: "the old one", code: "" }]));
  });

  it("sets a first password without asking", async () => {
    authOn = false;
    await renderPage();
    const card = passwordCard();
    const [pw, again] = card.querySelectorAll<HTMLInputElement>("input");
    fireEvent.change(pw, { target: { value: "correct horse battery" } });
    fireEvent.change(again, { target: { value: "correct horse battery" } });
    save(card);
    await waitFor(() => expect(writes.map((w) => w.password)).toEqual(["correct horse battery"]));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("changes nothing without the current password", async () => {
    const card = await signedInCard();
    typeNewPassword(card);
    save(card);
    expect(await within(card).findByText(en["auth.currentPasswordWrong"])).toBeTruthy();
    expect(writes).toEqual([]);
  });

  it("sends the current password and the code with a change", async () => {
    totpOn = true;
    const card = await signedInCard();
    await waitFor(() => expect(within(card).getByLabelText(en["auth.codeLabel"])).toBeTruthy());
    typeInto(card, en["auth.currentPassword"], "the old one");
    typeNewPassword(card);
    save(card);
    expect(await within(card).findByText(en["auth.codeHint"])).toBeTruthy();
    expect(writes).toEqual([]);
    typeInto(card, en["auth.codeLabel"], "123456");
    save(card);
    await waitFor(() => expect(writes).toEqual([{ password: "correct horse battery", current: "the old one", code: "123456" }]));
  });

  it("says in the reader's language when the current password was wrong", async () => {
    answer = { ok: false, code: "currentPasswordWrong", error: "the current password is not right" };
    const card = await signedInCard();
    typeInto(card, en["auth.currentPassword"], "a guess");
    typeNewPassword(card);
    save(card);
    expect(await screen.findByText(en["auth.currentPasswordWrong"])).toBeTruthy();
  });
});
