// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// #191, third report, and the one that was actually his.
//
// The look is reconciled with the server as the page boots (main.tsx). On an
// instance with a PASSWORD that boot happens while the login screen is up, and
// /api/display-prefs is not in the auth gate's public list, so it answers 401
// and the reconcile returns empty-handed. Signing in flips this component's
// state and renders the app WITHOUT reloading the page, so nothing asked again:
// the server held every setting and the browser never got one.
//
// Measured on the running build before this test was written: sign in with a
// cleared browser and the only display-prefs traffic is "GET -> 401", with the
// page left on a white background in English while the server had the lot.
//
// So: whenever the gate opens, reconcile. That is what this pins.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

const syncSpy = vi.fn(async () => {});
const authState = { authed: false };

vi.mock("../lib/displayPrefs", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/displayPrefs")>()),
  sync: () => syncSpy(),
}));

vi.mock("../lib/api", () => ({
  getAuth: async () => ({ ok: true, enabled: true, authed: authState.authed }),
  getSettings: async () => ({ ok: true, settings: null }),
  getHealth: async () => ({ ok: true, version: "v8.5.4" }),
}));

// The login screen and the app shell are irrelevant here; both are heavy and
// neither is what this is about.
vi.mock("../pages/Login", () => ({
  LoginPage: ({ onLogin }: { onLogin: () => void }) => (
    <button data-testid="signin" onClick={onLogin}>sign in</button>
  ),
}));
vi.mock("../../components/Sidebar", () => ({ Sidebar: () => null }));
vi.mock("../components/Sidebar", () => ({ Sidebar: () => null }));
vi.mock("../components/WhatsNewDialog", () => ({ WhatsNewDialog: () => null }));

import { Layout } from "./Layout";

beforeEach(() => {
  syncSpy.mockClear();
  authState.authed = false;
  localStorage.clear();
});
afterEach(cleanup);

describe("reconciling the look once the auth gate opens", () => {
  it("asks the server again after a sign-in, not only at boot", async () => {
    render(
      <MemoryRouter>
        <Layout />
      </MemoryRouter>
    );

    // Blocked: the boot-time call already happened in main.tsx and got a 401,
    // so nothing here may have asked yet.
    await screen.findByTestId("signin");
    expect(syncSpy, "still locked, nothing to reconcile with").not.toHaveBeenCalled();

    // Sign in the way LoginPage does it: report success, no page reload.
    authState.authed = true;
    screen.getByTestId("signin").click();

    await waitFor(
      () => expect(syncSpy, "signing in is the moment the settings become readable").toHaveBeenCalledTimes(1),
      { timeout: 2000 }
    );
  });

  it("also reconciles when the session was already valid", async () => {
    // No password, or a cookie that survived: the gate opens straight away and
    // main.tsx's call has already succeeded. Asking once more costs one GET and
    // keeps this component from having to know why the gate opened.
    authState.authed = true;

    render(
      <MemoryRouter>
        <Layout />
      </MemoryRouter>
    );

    await waitFor(() => expect(syncSpy).toHaveBeenCalledTimes(1), { timeout: 2000 });
  });
});
