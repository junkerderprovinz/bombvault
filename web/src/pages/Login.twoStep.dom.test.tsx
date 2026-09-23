// @vitest-environment jsdom
// The login form's second factor. The code field appears only when the server
// asks for it, the password is kept for the second step, and the first
// needCode answer is not shown as an error.
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const login = vi.fn();
// Without WebAuthn in the browser the passkey status is never fetched.
vi.mock("../lib/api", () => ({
  login: (...a: unknown[]) => login(...a),
  loginWithPasskey: vi.fn(),
  passkeyStatus: vi.fn(),
  passkeysAvailableInBrowser: () => false,
}));

import { LoginPage } from "./Login";

beforeEach(() => login.mockReset());
afterEach(cleanup);

function typePassword(value: string) {
  const field = document.getElementById("bv-password") as HTMLInputElement;
  fireEvent.change(field, { target: { value } });
}

function typeCode(value: string) {
  const field = document.getElementById("bv-code") as HTMLInputElement;
  fireEvent.change(field, { target: { value } });
}

async function submit() {
  const button = screen.getByRole("button", { name: /sign in/i });
  await act(async () => { fireEvent.click(button); });
}

describe("LoginPage, the second factor", () => {
  it("shows no code field on an instance that has no second factor", async () => {
    login.mockResolvedValue({ ok: true });
    const onLogin = vi.fn();
    render(<LoginPage onLogin={onLogin} />);
    expect(document.getElementById("bv-code")).toBeNull();

    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(onLogin).toHaveBeenCalledTimes(1));
    expect(login).toHaveBeenCalledWith("hunter2hunter2", undefined);
  });

  it("reveals the code field when the server asks for one", async () => {
    login.mockResolvedValue({ ok: false, needCode: true });
    render(<LoginPage onLogin={vi.fn()} />);
    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(document.getElementById("bv-code")).not.toBeNull());
  });

  it("does not call the first needCode answer an error", async () => {
    login.mockResolvedValue({ ok: false, needCode: true, error: "enter the code" });
    render(<LoginPage onLogin={vi.fn()} />);
    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(document.getElementById("bv-code")).not.toBeNull());
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the password, and sends it again with the code", async () => {
    login.mockResolvedValueOnce({ ok: false, needCode: true });
    login.mockResolvedValueOnce({ ok: true });
    const onLogin = vi.fn();
    render(<LoginPage onLogin={onLogin} />);

    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(document.getElementById("bv-code")).not.toBeNull());
    expect((document.getElementById("bv-password") as HTMLInputElement).value).toBe(
      "hunter2hunter2",
    );

    typeCode("123456");
    await submit();
    await waitFor(() => expect(onLogin).toHaveBeenCalledTimes(1));
    expect(login).toHaveBeenLastCalledWith("hunter2hunter2", "123456");
  });

  it("says so when a code was tried and rejected", async () => {
    login.mockResolvedValueOnce({ ok: false, needCode: true });
    login.mockResolvedValueOnce({ ok: false, needCode: true, error: "that code is not valid" });
    render(<LoginPage onLogin={vi.fn()} />);

    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(document.getElementById("bv-code")).not.toBeNull());
    typeCode("000000");
    await submit();
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("not valid"));
  });

  it("will not submit an empty code once one is being asked for", async () => {
    login.mockResolvedValue({ ok: false, needCode: true });
    render(<LoginPage onLogin={vi.fn()} />);
    typePassword("hunter2hunter2");
    await submit();
    await waitFor(() => expect(document.getElementById("bv-code")).not.toBeNull());

    const button = screen.getByRole("button", { name: /sign in/i }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    typeCode("123456");
    await waitFor(() => expect(button.disabled).toBe(false));
  });

  it("reports a wrong password as a wrong password, not as a missing code", async () => {
    login.mockResolvedValue({ ok: false, error: "invalid password" });
    render(<LoginPage onLogin={vi.fn()} />);
    typePassword("wrongwrongwrong");
    await submit();
    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("invalid password"));
    expect(document.getElementById("bv-code")).toBeNull();
  });
});
