// @vitest-environment jsdom
// The login form's passkey button and what it says when a key is refused.
import { render, screen, cleanup, fireEvent, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const loginWithPasskey = vi.fn();
const finishPasskeyWithCode = vi.fn();
vi.mock("../lib/api", () => ({
  login: vi.fn(),
  loginWithPasskey: () => loginWithPasskey(),
  finishPasskeyWithCode: (...a: unknown[]) => finishPasskeyWithCode(...a),
  passkeyStatus: () => Promise.resolve({ ok: true, supported: true, here: 1 }),
  passkeysAvailableInBrowser: () => true,
}));

import { LoginPage } from "./Login";

afterEach(cleanup);

describe("LoginPage, the passkey", () => {
  it("says in the reader's language that the key has to check a PIN while two-factor is on", async () => {
    loginWithPasskey.mockResolvedValue({ ok: false, code: "passkeyNeedsVerification", error: "with two-factor authentication on" });
    render(<LoginPage onLogin={vi.fn()} />);
    const button = await screen.findByRole("button", { name: en["auth.signInWithPasskey"] });
    await act(async () => {
      fireEvent.click(button);
    });
    expect(await screen.findByText(en["auth.passkeyNeedsVerification"])).toBeTruthy();
  });

  it("asks for the code after a key the server cannot take as the second factor", async () => {
    loginWithPasskey.mockResolvedValue({ ok: false, needCode: true, ceremonyId: "h1" });
    finishPasskeyWithCode.mockResolvedValue({ ok: true });
    const onLogin = vi.fn();
    render(<LoginPage onLogin={onLogin} />);
    const button = await screen.findByRole("button", { name: en["auth.signInWithPasskey"] });
    await act(async () => {
      fireEvent.click(button);
    });
    const field = screen.getByLabelText(en["auth.codeLabel"]);
    fireEvent.change(field, { target: { value: "123456" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["auth.signIn"] }));
    });
    expect(finishPasskeyWithCode).toHaveBeenCalledWith("h1", "123456");
    expect(onLogin).toHaveBeenCalled();
  });
});
