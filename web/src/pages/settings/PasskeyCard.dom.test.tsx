// @vitest-environment jsdom
/**
 * On a stock Unraid installation passkeys cannot work: the template opens
 * https://[IP]:3443 with a certificate that covers only localhost, and WebAuthn
 * binds a credential to a domain. So the case this card meets most often is
 * explaining why there is nothing to click.
 */
import { render, screen, cleanup, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const passkeyStatus = vi.fn();
vi.mock("../../lib/api", () => ({
  passkeyStatus: (...a: unknown[]) => passkeyStatus(...a),
  registerPasskey: vi.fn(),
  deletePasskey: vi.fn(),
  passkeysAvailableInBrowser: () => true,
}));

import { PasskeyCard } from "./PasskeyCard";

beforeEach(() => passkeyStatus.mockReset());
afterEach(cleanup);

describe("PasskeyCard", () => {
  it("says why, instead of offering a button that cannot work", async () => {
    passkeyStatus.mockResolvedValue({
      ok: true,
      supported: false,
      // The server's reason is English. The card shows its own translated
      // text instead.
      reason: "ZZZ-SERVER-SENTENCE-ZZZ",
      total: 0,
      here: 0,
      passkeys: [],
    });
    render(<PasskeyCard passwordSet />);

    // The translated explanation, including what to do about it.
    await waitFor(() => expect(screen.getByText(/reverse proxy/i)).toBeTruthy());
    expect(screen.getByText(/host name/i)).toBeTruthy();
    expect(screen.queryByText(/ZZZ-SERVER-SENTENCE-ZZZ/)).toBeNull();
    expect(screen.queryByRole("button", { name: /set up/i })).toBeNull();
  });

  it("offers the button once the address can carry one", async () => {
    passkeyStatus.mockResolvedValue({
      ok: true,
      supported: true,
      rpId: "bombvault.example.com",
      total: 0,
      here: 0,
      passkeys: [],
    });
    render(<PasskeyCard passwordSet />);

    await waitFor(() => expect(screen.getByRole("button", { name: /set up/i })).toBeTruthy());
  });

  it("shows a key that belongs to another address, and says so", async () => {
    passkeyStatus.mockResolvedValue({
      ok: true,
      supported: true,
      rpId: "bombvault.example.com",
      total: 1,
      here: 0,
      passkeys: [
        {
          id: "1",
          name: "Handy",
          rpId: "other.example.com",
          usableHere: false,
          backedUp: true,
          createdAt: 0,
          lastUsedAt: 0,
          transports: "internal",
        },
      ],
    });
    render(<PasskeyCard passwordSet />);

    // Hiding it would make a key somebody registered look lost.
    await waitFor(() => expect(screen.getByText("Handy")).toBeTruthy());
    expect(screen.getByText(/other\.example\.com/)).toBeTruthy();
  });

  it("asks for a password first, because a passkey is never the only way in", async () => {
    passkeyStatus.mockResolvedValue({ ok: true, supported: true, total: 0, here: 0, passkeys: [] });
    render(<PasskeyCard passwordSet={false} />);

    await waitFor(() => expect(screen.getByText(/set a login password first/i)).toBeTruthy());
    expect(screen.queryByRole("button", { name: /set up/i })).toBeNull();
  });
});
