// @vitest-environment jsdom
/**
 * The failures worth guarding in two-factor enrolment are the ones that lock
 * somebody out of their own backups:
 *
 *   - the status line reads the server's answer, not the local step, so a
 *     half-finished enrolment never looks armed;
 *   - the recovery codes are shown exactly once, so they must appear and must
 *     not be swept away by the click that confirmed the code;
 *   - turning the factor off asks for a live code, so a session somebody
 *     walked away from cannot quietly remove it.
 */
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const setupTOTP = vi.fn();
const confirmTOTP = vi.fn();
const disableTOTP = vi.fn();
vi.mock("../../lib/api", () => ({
  setupTOTP: () => setupTOTP(),
  confirmTOTP: (c: string) => confirmTOTP(c),
  disableTOTP: (c: string) => disableTOTP(c),
}));

const push = vi.fn();
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push }) }));
vi.mock("../../lib/clipboard", () => ({ copyText: vi.fn() }));

import { TwoFactorCard } from "./TwoFactorCard";

const URI = "otpauth://totp/BombVault:tower?secret=GEZDGNBVGY3TQOJQ&issuer=BombVault";

beforeEach(() => {
  setupTOTP.mockReset();
  confirmTOTP.mockReset();
  disableTOTP.mockReset();
  push.mockReset();
});
afterEach(cleanup);

function click(name: RegExp) {
  return act(async () => {
    fireEvent.click(screen.getByRole("button", { name }));
  });
}

describe("TwoFactorCard", () => {
  it("says nothing can be set up before a password exists", () => {
    render(<TwoFactorCard passwordSet={false} enabled={false} onChanged={vi.fn()} />);
    expect(screen.getByText(/set a login password first/i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /set up/i })).toBeNull();
  });

  it("shows the QR code and the typed-out secret after setup", async () => {
    setupTOTP.mockResolvedValue({ ok: true, secret: "GEZDGNBVGY3TQOJQ", uri: URI });
    render(<TwoFactorCard passwordSet enabled={false} onChanged={vi.fn()} />);
    await click(/set up/i);
    await waitFor(() => expect(screen.getByText("GEZDGNBVGY3TQOJQ")).toBeTruthy());
    // The QR for a phone that can scan, and the secret for one that cannot.
    // Either one alone leaves somebody stuck.
    expect(document.querySelector("svg[role=img]")).toBeTruthy();
  });

  it("does not claim the factor is on while the enrolment is unfinished", async () => {
    setupTOTP.mockResolvedValue({ ok: true, secret: "GEZDGNBVGY3TQOJQ", uri: URI });
    render(<TwoFactorCard passwordSet enabled={false} onChanged={vi.fn()} />);
    await click(/set up/i);
    await waitFor(() => expect(screen.getByText("GEZDGNBVGY3TQOJQ")).toBeTruthy());
    // `enabled` is still false: the status line follows the server, not the step.
    expect(screen.getByText(/^Off\./)).toBeTruthy();
  });

  it("shows the recovery codes once the code is accepted, and keeps them on screen", async () => {
    setupTOTP.mockResolvedValue({ ok: true, secret: "GEZDGNBVGY3TQOJQ", uri: URI });
    confirmTOTP.mockResolvedValue({ ok: true, recoveryCodes: ["abcde-fghij", "klmno-pqrst"] });
    const onChanged = vi.fn();
    render(<TwoFactorCard passwordSet enabled={false} onChanged={onChanged} />);

    await click(/set up/i);
    await waitFor(() => expect(document.getElementById("bv-totp-confirm")).not.toBeNull());
    fireEvent.change(document.getElementById("bv-totp-confirm")!, { target: { value: "123456" } });
    await click(/confirm/i);

    await waitFor(() => expect(screen.getByText("abcde-fghij")).toBeTruthy());
    expect(screen.getByText("klmno-pqrst")).toBeTruthy();
    expect(onChanged).toHaveBeenCalled();
    // They are stored hashed, so this screen is the only chance. It must not be
    // dismissed by anything but a deliberate acknowledgement.
    expect(screen.getByRole("button", { name: /written them down/i })).toBeTruthy();
  });

  it("refuses to confirm with an empty code", async () => {
    setupTOTP.mockResolvedValue({ ok: true, secret: "GEZDGNBVGY3TQOJQ", uri: URI });
    render(<TwoFactorCard passwordSet enabled={false} onChanged={vi.fn()} />);
    await click(/set up/i);
    await waitFor(() => expect(document.getElementById("bv-totp-confirm")).not.toBeNull());
    const confirm = screen.getByRole("button", { name: /confirm/i }) as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);
    expect(confirmTOTP).not.toHaveBeenCalled();
  });

  it("asks for a live code before turning the factor off", async () => {
    disableTOTP.mockResolvedValue({ ok: true });
    render(<TwoFactorCard passwordSet enabled recoveryLeft={6} onChanged={vi.fn()} />);
    expect(screen.getByText(/6 recovery codes left/i)).toBeTruthy();

    // The first press only opens the code field; nothing is sent yet.
    await click(/turn off/i);
    await waitFor(() => expect(document.getElementById("bv-totp-disable")).not.toBeNull());
    expect(disableTOTP).not.toHaveBeenCalled();

    fireEvent.change(document.getElementById("bv-totp-disable")!, { target: { value: "654321" } });
    const buttons = screen.getAllByRole("button", { name: /turn off/i });
    await act(async () => { fireEvent.click(buttons[buttons.length - 1]); });
    await waitFor(() => expect(disableTOTP).toHaveBeenCalledWith("654321"));
  });

  it("reports the server's own refusal rather than a generic failure", async () => {
    setupTOTP.mockResolvedValue({ ok: false, error: "two-factor authentication is already on" });
    render(<TwoFactorCard passwordSet enabled={false} onChanged={vi.fn()} />);
    await click(/set up/i);
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith("two-factor authentication is already on", "fail"),
    );
  });
});
