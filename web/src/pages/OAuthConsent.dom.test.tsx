// @vitest-environment jsdom
// The consent page is where an OAuth client gets its access, so what matters is
// what it refuses to do: answer without a signed-in operator, follow a return
// address the server did not accept, or grant the start permission by default.
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";
import type { OAuthConsentInfo } from "../lib/api";

const getAuth = vi.fn();
const getOAuthConsent = vi.fn();
const answerOAuthConsent = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getAuth: () => getAuth(),
    getOAuthConsent: (...a: unknown[]) => getOAuthConsent(...a),
    answerOAuthConsent: (...a: unknown[]) => answerOAuthConsent(...a),
    passkeyStatus: () => Promise.resolve({ ok: true, supported: false }),
  };
});

const { OAuthConsent } = await import("./OAuthConsent");

const assign = vi.fn();

function info(over: Partial<OAuthConsentInfo> = {}): OAuthConsentInfo {
  return {
    ok: true,
    client: { id: "bvc_1", name: "Something", known: "chatgpt", redirectHost: "chatgpt.com", loopback: false },
    ticket: "ticket-1",
    limitReached: false,
    grantLimit: 10,
    ...over,
  };
}

/** The Allow button once its start-up pause is over. */
async function armedAllow(): Promise<HTMLButtonElement> {
  const allow = (await screen.findByRole("button", { name: en["oauth.accept"] })) as HTMLButtonElement;
  await waitFor(() => expect(allow.disabled).toBe(false), { timeout: 2000 });
  return allow;
}

beforeEach(() => {
  getAuth.mockReset();
  getOAuthConsent.mockReset();
  answerOAuthConsent.mockReset();
  assign.mockReset();
  vi.stubGlobal("location", { ...window.location, search: "?client_id=bvc_1&state=s", assign });
  getAuth.mockResolvedValue({ enabled: true, authed: true });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the OAuth consent page", () => {
  it("asks for the login first and checks nothing before it", async () => {
    getAuth.mockResolvedValue({ enabled: true, authed: false });
    render(<OAuthConsent />);
    await screen.findByText(en["auth.loginTitle"]);
    expect(getOAuthConsent).not.toHaveBeenCalled();
  });

  it("shows who asks and where the answer goes, with the start switch off", async () => {
    getOAuthConsent.mockResolvedValue(info());
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.asks"].replace("{name}", "ChatGPT"));
    expect(getOAuthConsent).toHaveBeenCalledWith("?client_id=bvc_1&state=s");
    expect(screen.getByText(en["oauth.returnsTo"].replace("{host}", "chatgpt.com"))).toBeTruthy();
    expect(screen.queryByText(en["oauth.unverified"])).toBeNull();
    expect(screen.getByText("chatgpt.com").className).not.toContain("truncate");
    const toggle = screen.getByRole("switch", { name: en["mcp.allowStart"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("puts Deny before Allow, so the forward action sits on the right", async () => {
    getOAuthConsent.mockResolvedValue(info());
    render(<OAuthConsent />);
    const allow = await screen.findByRole("button", { name: en["oauth.accept"] });
    const deny = screen.getByRole("button", { name: en["oauth.decline"] });
    expect(deny.compareDocumentPosition(allow) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("sends the operator's answer and follows the redirect the server built", async () => {
    getOAuthConsent.mockResolvedValue(info());
    answerOAuthConsent.mockResolvedValue({ ok: true, redirect: "https://chatgpt.com/cb?code=c&state=s" });
    render(<OAuthConsent />);
    fireEvent.click(await screen.findByRole("switch", { name: en["mcp.allowStart"] }));
    fireEvent.click(await armedAllow());
    await waitFor(() => expect(assign).toHaveBeenCalledWith("https://chatgpt.com/cb?code=c&state=s"));
    expect(answerOAuthConsent).toHaveBeenCalledWith("ticket-1", true, true);
    expect(screen.getByText(en["oauth.returning"].replace("{host}", "chatgpt.com"))).toBeTruthy();
  });

  it("denies with the start switch left as it was", async () => {
    getOAuthConsent.mockResolvedValue(info());
    answerOAuthConsent.mockResolvedValue({ ok: true, redirect: "https://chatgpt.com/cb?error=access_denied" });
    render(<OAuthConsent />);
    fireEvent.click(await screen.findByRole("button", { name: en["oauth.decline"] }));
    await waitFor(() => expect(answerOAuthConsent).toHaveBeenCalledWith("ticket-1", false, false));
    expect(assign).toHaveBeenCalledWith("https://chatgpt.com/cb?error=access_denied");
  });

  it("warns about a link from someone else even for a client it knows", async () => {
    getOAuthConsent.mockResolvedValue(info());
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.startedHere"].replace("{name}", "ChatGPT"));
  });

  it("warns about a client it does not know and about a loopback return", async () => {
    getOAuthConsent.mockResolvedValue(
      info({ client: { id: "bvc_2", name: "Claude Code", known: "", redirectHost: "localhost", loopback: true } })
    );
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.unverified"]);
    expect(screen.getByText(en["oauth.loopback"])).toBeTruthy();
    expect(screen.getByText("Claude Code")).toBeTruthy();
  });

  it("shows a refused return address and never follows it", async () => {
    getOAuthConsent.mockResolvedValue({ ok: false, code: "oauth-bad-redirect", error: "x" });
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.errorBadRedirect"]);
    expect(assign).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: en["oauth.accept"] })).toBeNull();
  });

  it("shows an unsupported request and stays on the page, whatever address comes with it", async () => {
    getOAuthConsent.mockResolvedValue({
      ok: false,
      code: "oauth-invalid-request",
      error: "x",
      redirect: "https://attacker.example/fake-login?error=invalid_request",
    });
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.errorInvalid"]);
    expect(assign).not.toHaveBeenCalled();
    expect(screen.queryByText(en["oauth.returning"].replace("{host}", "attacker.example"))).toBeNull();
  });

  it("holds Allow back for a moment after the page shows and after it regains focus", async () => {
    getOAuthConsent.mockResolvedValue(info());
    render(<OAuthConsent />);
    const allow = (await screen.findByRole("button", { name: en["oauth.accept"] })) as HTMLButtonElement;
    expect(allow.disabled).toBe(true);
    fireEvent.click(allow);
    await armedAllow();
    fireEvent.focus(window);
    expect(allow.disabled).toBe(true);
    fireEvent(document, new Event("visibilitychange"));
    expect(allow.disabled).toBe(true);
    await armedAllow();
    expect(answerOAuthConsent).not.toHaveBeenCalled();
  });

  it("keeps Allow disabled while the grant limit is reached", async () => {
    getOAuthConsent.mockResolvedValue(info({ limitReached: true }));
    render(<OAuthConsent />);
    await screen.findByText(en["oauth.limitReached"]);
    expect((screen.getByRole("button", { name: en["oauth.accept"] }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: en["oauth.decline"] }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("explains an expired page after the answer was refused", async () => {
    getOAuthConsent.mockResolvedValue(info());
    answerOAuthConsent.mockResolvedValue({ ok: false, code: "oauth-consent-expired", error: "x" });
    render(<OAuthConsent />);
    fireEvent.click(await armedAllow());
    await screen.findByText(en["oauth.errorExpired"]);
    expect(assign).not.toHaveBeenCalled();
  });
});
