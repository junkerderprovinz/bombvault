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

    await fillAndSubmit("ABCD-EFGH", "correct horse battery");

    await waitFor(() => expect(onLogin).toHaveBeenCalledTimes(1));
    expect(finishPasswordReset).toHaveBeenCalledWith("ABCD-EFGH", "correct horse battery");
  });

  it("sends nothing while the two passwords differ", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH", "correct horse battery", "correct horse batterx");

    expect(screen.getByRole("alert").textContent).toBe(en["auth.passwordMismatch"]);
    expect(finishPasswordReset).not.toHaveBeenCalled();
  });

  it("sends nothing for a password below the server's minimum", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH", "short");

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(finishPasswordReset).not.toHaveBeenCalled();
  });

  it("says so when the code is wrong and keeps what was typed", async () => {
    finishPasswordReset.mockResolvedValue({ ok: false, code: "resetCodeInvalid", error: "that code is not right" });
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH", "correct horse battery");

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(en["auth.resetCodeInvalid"]));
    expect((document.getElementById("bv-reset-password") as HTMLInputElement).value).toBe("correct horse battery");
    expect(startPasswordReset).toHaveBeenCalledTimes(1);
  });

  it("has a new code written when the old one has expired", async () => {
    finishPasswordReset.mockResolvedValue({ ok: false, code: "resetCodeExpired", error: "that code is no longer valid" });
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await fillAndSubmit("ABCD-EFGH", "correct horse battery");

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(en["auth.resetCodeExpired"]));
    expect(startPasswordReset).toHaveBeenCalledTimes(2);
    expect((document.getElementById("bv-reset-code") as HTMLInputElement).value).toBe("");
  });

  it("goes back to the sign-in form", async () => {
    render(<LoginPage onLogin={vi.fn()} />);
    await openReset();

    await press(en["common.back"]);

    expect(document.getElementById("bv-password")).not.toBeNull();
    expect(document.getElementById("bv-reset-code")).toBeNull();
  });
});
