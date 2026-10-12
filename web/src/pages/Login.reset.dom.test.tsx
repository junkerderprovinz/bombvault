// @vitest-environment jsdom
// The way back in for somebody who lost the password: the login page asks the
// server to write a code to its container log, then takes the code and a new
// password.
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const startPasswordReset = vi.fn();
const finishPasswordReset = vi.fn();
vi.mock("../lib/api", () => ({
  login: vi.fn(),
  loginWithPasskey: vi.fn(),
  passkeyStatus: vi.fn(),
  passkeysAvailableInBrowser: () => false,
  getAuth: () => Promise.resolve({ ok: true, enabled: true, authed: false, minPasswordLen: 12 }),
  startPasswordReset: (...a: unknown[]) => startPasswordReset(...a),
  finishPasswordReset: (...a: unknown[]) => finishPasswordReset(...a),
}));

import { LoginPage } from "./Login";

beforeEach(() => {
  startPasswordReset.mockReset().mockResolvedValue({ ok: true });
  finishPasswordReset.mockReset();
});
afterEach(cleanup);

function type(id: string, value: string) {
  fireEvent.change(document.getElementById(id) as HTMLInputElement, { target: { value } });
}

async function press(name: string) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name }));
  });
}

async function openReset() {
  await press(en["auth.forgotPassword"]);
  await waitFor(() => expect(document.getElementById("bv-reset-code")).not.toBeNull());
}

async function fillAndSubmit(code: string, password: string, confirm = password) {
  type("bv-reset-code", code);
  type("bv-reset-password", password);
  type("bv-reset-confirm", confirm);
  await press(en["auth.setPassword"]);
}

describe("LoginPage, a forgotten password", () => {
  it("has the code written to the log before it asks for one", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    expect(document.getElementById("bv-reset-code")).toBeNull();

    await openReset();

    expect(startPasswordReset).toHaveBeenCalledTimes(1);
    expect(document.getElementById("bv-password")).toBeNull();
  });

  it("stays on the sign-in form when the server writes no code", async () => {
    startPasswordReset.mockResolvedValue({ ok: false, error: "authentication is not enabled" });
    render(<LoginPage onLogin={vi.fn()} />);

    await press(en["auth.forgotPassword"]);

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe("authentication is not enabled"));
    expect(document.getElementById("bv-reset-code")).toBeNull();
  });

  it("sets the password with the code and signs in", async () => {
    finishPasswordReset.mockResolvedValue({ ok: true });
    const onLogin = vi.fn();
    render(<LoginPage onLogin={onLogin} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "correct horse battery");

    await waitFor(() => expect(onLogin).toHaveBeenCalledTimes(1));
    expect(finishPasswordReset).toHaveBeenCalledWith("ABCD-EFGH-JKLM", "correct horse battery");
  });

  it("says what the reset removed before it signs in", async () => {
    finishPasswordReset.mockResolvedValue({
      ok: true,
      removed: { passkeys: 2, apiTokens: 1, mcpKeys: 0, twoFactor: true, phonePairingOff: true },
    });
    const onLogin = vi.fn();
    render(<LoginPage onLogin={onLogin} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "correct horse battery");

    const status = await screen.findByRole("status");
    expect(status.textContent).toContain("2 passkeys removed");
    expect(status.textContent).toContain("1 API token revoked");
    expect(status.textContent).toContain(en["auth.twoFactorOffNow"]);
    expect(status.textContent).toContain(en["auth.resetDonePhones"]);
    expect(status.textContent).not.toContain("MCP");
    expect(status.textContent).toContain(en["auth.resetDoneHint"]);
    expect(onLogin).not.toHaveBeenCalled();

    await press(en["common.done"]);

    expect(onLogin).toHaveBeenCalledTimes(1);
  });

  it("lists phone pairing only when the answer says it was switched off", async () => {
    finishPasswordReset.mockResolvedValue({
      ok: true,
      removed: { passkeys: 0, apiTokens: 0, mcpKeys: 0, twoFactor: false, phonePairingOff: false },
    });
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "correct horse battery");

    const status = await screen.findByRole("status");
    expect(status.textContent).toContain(en["auth.resetDoneSessions"]);
    expect(status.textContent).not.toContain(en["auth.resetDonePhones"]);
  });

  it("sends nothing while the two passwords differ", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "correct horse battery", "correct horse batterx");

    expect(screen.getByRole("alert").textContent).toBe(en["auth.passwordMismatch"]);
    expect(finishPasswordReset).not.toHaveBeenCalled();
  });

  it("sends nothing for a password below the server's minimum", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "short");

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(finishPasswordReset).not.toHaveBeenCalled();
  });

  it("says so when the code is refused, keeps what was typed and asks for a code in case it expired", async () => {
    finishPasswordReset.mockResolvedValue({ ok: false, code: "resetCodeRejected", error: "that code is wrong or has expired" });
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH-JKLM", "correct horse battery");

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(en["auth.resetCodeRejected"]));
    expect((document.getElementById("bv-reset-code") as HTMLInputElement).value).toBe("ABCD-EFGH-JKLM");
    expect((document.getElementById("bv-reset-password") as HTMLInputElement).value).toBe("correct horse battery");
    expect(startPasswordReset).toHaveBeenCalledTimes(2);
  });

  it("goes back to the sign-in form", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await press(en["common.back"]);

    expect(document.getElementById("bv-password")).not.toBeNull();
    expect(document.getElementById("bv-reset-code")).toBeNull();
  });
});
