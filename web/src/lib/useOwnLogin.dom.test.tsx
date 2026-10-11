// @vitest-environment jsdom
/**
 * A call refused to the session the phone app opens this instance with asks
 * for a sign-in at this instance, and the sign-in starts by dropping that
 * session: the login screen then comes up on the same page.
 */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { I18nProvider, en } from "./i18n";
import { secretsSentence } from "./secretsRefusal";

const logout = vi.fn(() => Promise.resolve({ ok: true }));

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return { ...actual, logout: () => logout() };
});

const { useOwnLogin } = await import("./useOwnLogin");

const reload = vi.fn();
let handled: boolean | null = null;

function Caller({ code }: { code?: string }) {
  const ownLogin = useOwnLogin();
  return (
    <>
      <button type="button" onClick={() => (handled = ownLogin.handled({ code }))}>
        call
      </button>
      {ownLogin.dialog}
    </>
  );
}

async function call(code?: string) {
  render(
    <I18nProvider>
      <Caller code={code} />
    </I18nProvider>
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "call" }));
  });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  handled = null;
});

describe("a call refused to the phone app's session", () => {
  it("asks to sign in here, and signs out of that session before the login screen", async () => {
    vi.stubGlobal("location", { ...window.location, reload });
    await call("own-login-needed");
    expect(handled).toBe(true);
    const ask = screen.getByRole("dialog");
    expect(ask.textContent).toContain(en["secrets.ownLoginNeeded"]);
    await act(async () => {
      fireEvent.click(within(ask).getByRole("button", { name: en["secrets.signInHere"] }));
    });
    expect(logout).toHaveBeenCalledTimes(1);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("leaves the session alone when the person does not want to sign in now", async () => {
    vi.stubGlobal("location", { ...window.location, reload });
    await call("own-login-needed");
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(logout).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it("leaves every other answer to the caller", async () => {
    await call("password-not-set");
    expect(handled).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the sentence of a refusal for the session", () => {
  const t = (key: keyof typeof en) => en[key];

  it("says what is missing for each of the two refusals, and nothing for any other answer", () => {
    expect(secretsSentence(t, "password-not-set")).toBe(en["secrets.passwordNotSet"]);
    expect(secretsSentence(t, "own-login-needed")).toBe(en["secrets.ownLoginNeeded"]);
    expect(secretsSentence(t, "member-gone")).toBeUndefined();
    expect(secretsSentence(t, undefined)).toBeUndefined();
  });
});
