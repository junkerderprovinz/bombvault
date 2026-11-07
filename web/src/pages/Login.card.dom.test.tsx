// @vitest-environment jsdom
// The login page on GlimStone's sign-in card: the logo above the name, the
// focus in the first empty field, and a refused try that shakes its button
// and selects the field for the next one.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";

const login = vi.fn();
const startPasswordReset = vi.fn();
const finishPasswordReset = vi.fn();
vi.mock("../lib/api", () => ({
  login: (...a: unknown[]) => login(...a),
  loginWithPasskey: vi.fn(),
  passkeyStatus: vi.fn(),
  passkeysAvailableInBrowser: () => false,
  getAuth: () => Promise.resolve({ ok: true, enabled: true, authed: false, minPasswordLen: 12 }),
  startPasswordReset: (...a: unknown[]) => startPasswordReset(...a),
  finishPasswordReset: (...a: unknown[]) => finishPasswordReset(...a),
}));

import { LoginPage } from "./Login";

beforeEach(() => {
  login.mockReset();
  startPasswordReset.mockReset().mockResolvedValue({ ok: true });
  finishPasswordReset.mockReset();
});
afterEach(cleanup);

const field = (id: string) => document.getElementById(id) as HTMLInputElement;

function type(id: string, value: string) {
  fireEvent.change(field(id), { target: { value } });
}

async function press(name: string) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name }));
  });
}

describe("the sign-in card", () => {
  it("stands the logo above the instance's name, which is the page's heading", () => {
    const { container } = render(<LoginPage onLogin={vi.fn()} />);
    const head = container.querySelector(".glim-signin-head")!;
    expect(head.querySelector("img")).not.toBeNull();
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(en["auth.loginTitle"]);
    expect(head.contains(screen.getByRole("heading", { level: 1 }))).toBe(true);
  });

  it("opens with the focus in the password field", () => {
    render(<LoginPage onLogin={vi.fn()} />);
    expect(document.activeElement).toBe(field("bv-password"));
  });

  it("keeps a refused password selected for the next try and shakes Sign in", async () => {
    login.mockResolvedValue({ ok: false, error: "invalid password" });
    render(<LoginPage onLogin={vi.fn()} />);
    type("bv-password", "wrongwrongwrong");

    await press(en["auth.signIn"]);

    expect(screen.getByRole("alert").textContent).toBe("invalid password");
    expect(field("bv-password").value).toBe("wrongwrongwrong");
    expect(document.activeElement).toBe(field("bv-password"));
    await waitFor(() => expect(screen.getByRole("button", { name: en["auth.signIn"] }).className).toMatch(/\bglim-shake\b/));
  });

  it("empties a refused code and puts the focus back in it", async () => {
    login.mockResolvedValueOnce({ ok: false, needCode: true });
    login.mockResolvedValueOnce({ ok: false, needCode: true, error: "that code is not valid" });
    render(<LoginPage onLogin={vi.fn()} />);
    type("bv-password", "hunter2hunter2");
    await press(en["auth.signIn"]);
    await waitFor(() => expect(document.activeElement).toBe(field("bv-code")));

    type("bv-code", "000000");
    await press(en["auth.signIn"]);

    expect(field("bv-code").value).toBe("");
    expect(document.activeElement).toBe(field("bv-code"));
  });

  it("offers Back in place of Forgot password? in the code step, and Back returns to the password", async () => {
    login.mockResolvedValue({ ok: false, needCode: true });
    render(<LoginPage onLogin={vi.fn()} />);
    type("bv-password", "hunter2hunter2");
    await press(en["auth.signIn"]);

    expect(screen.queryByRole("button", { name: en["auth.forgotPassword"] })).toBeNull();
    await press(en["common.back"]);

    expect(document.getElementById("bv-code")).toBeNull();
    expect(field("bv-password").value).toBe("hunter2hunter2");
    expect(screen.getByRole("button", { name: en["auth.forgotPassword"] })).toBeTruthy();
  });

  it("selects a refused code from the log and shakes Set password", async () => {
    finishPasswordReset.mockResolvedValue({ ok: false, code: "resetCodeRejected" });
    render(<LoginPage onLogin={vi.fn()} />);
    await press(en["auth.forgotPassword"]);
    await waitFor(() => expect(document.activeElement).toBe(field("bv-reset-code")));
    type("bv-reset-code", "ABCD-EFGH-JKLM");
    type("bv-new", "correct horse battery");
    type("bv-confirm", "correct horse battery");

    await press(en["auth.setPassword"]);

    expect(document.activeElement).toBe(field("bv-reset-code"));
    await waitFor(() => expect(screen.getByRole("button", { name: en["auth.setPassword"] }).className).toMatch(/\bglim-shake\b/));
  });
});

describe("the sign-in page's layout", () => {
  const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");
  const rule = (selector: string) => {
    const at = css.indexOf(`${selector} {`);
    expect(at, `${selector} in index.css`).toBeGreaterThan(-1);
    return css.slice(at, css.indexOf("}", at));
  };

  // A card centred outright jumps by half a line whenever a reason appears
  // above Sign in. Its top edge stays where a card of the usual height starts
  // when centred, and the card grows downwards.
  it("fixes the card's top edge instead of centring it", () => {
    const page = rule(".glim-signin");
    expect(page).toMatch(/align-items:\s*flex-start/);
    expect(page).toMatch(/padding:\s*max\(1rem, calc\(50dvh - 14rem\)\) 1rem 1rem/);
  });

  it("draws every logo 7.5rem tall at its own proportions", () => {
    const logo = rule(".glim-signin-logo > img");
    expect(logo).toMatch(/height:\s*7\.5rem/);
    expect(logo).toMatch(/width:\s*auto/);
  });
});
