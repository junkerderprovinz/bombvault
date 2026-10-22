// @vitest-environment jsdom
// The login form's passkey button and what it says when a key is refused.
import { render, screen, cleanup, fireEvent, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const loginWithPasskey = vi.fn();
vi.mock("../lib/api", () => ({
  login: vi.fn(),
  loginWithPasskey: () => loginWithPasskey(),
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
});
