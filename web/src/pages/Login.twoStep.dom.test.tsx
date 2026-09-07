// @vitest-environment jsdom
/**
 * The login screen's second step (v8.6.0).
 *
 * Three things have to hold, and each one is a way the form could quietly
 * become worse than the single-field version it replaces:
 *
 *   - the code field is not there until the server asks for it, so an install
 *     without a second factor looks exactly as it always did;
 *   - the password is NOT retyped for the second step, because a form that
 *     clears the field people just filled in is a form people fight;
 *   - the first "needCode" answer is not an error message. It is the form
 *     discovering it has a second field, and calling that a failure trains
 *     people to ignore the red text that will matter next time.
 */
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const login = vi.fn();
vi.mock("../lib/api", () => ({ login: (...a: unknown[]) => login(...a) }));

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
    // One call, password only: nothing invented a code parameter.
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
    // The field still holds what was typed: nobody has to type it twice.
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
