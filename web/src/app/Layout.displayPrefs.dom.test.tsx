// @vitest-environment jsdom
// main.tsx reconciles the look with the server at boot. On a password-protected
// instance that happens behind the login screen and gets a 401, and signing in
// renders the app without a reload, so Layout has to reconcile again whenever
// the auth gate opens.
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

// Stubs for the heavy parts that play no role here.
vi.mock("../pages/Login", () => ({
  LoginPage: ({ onLogin }: { onLogin: () => void }) => (
    <button data-testid="signin" onClick={onLogin}>sign in</button>
  ),
}));
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
    // No password, or a session that survived: the gate opens straight away.
    authState.authed = true;

    render(
      <MemoryRouter>
        <Layout />
      </MemoryRouter>
    );

    await waitFor(() => expect(syncSpy).toHaveBeenCalledTimes(1), { timeout: 2000 });
  });
});
