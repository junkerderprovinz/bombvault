// @vitest-environment jsdom
/**
 * The SMB / WebDAV destination form.
 *
 * Two behaviours are worth pinning. The fields have to follow the kind, since
 * an SMB share has a host and a share while a WebDAV server has a URL and a
 * vendor, and showing all four at once is how someone ends up filling in the
 * wrong pair. And the finished repository location has to be shown afterwards:
 * "name:share/path" is the shape people get wrong, and getting it wrong puts a
 * repository somewhere unexpected instead of producing an error.
 */
import { render, screen, cleanup, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const addRcloneRemote = vi.fn();
const pushed: { message: string; severity?: string }[] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  addRcloneRemote: (...a: unknown[]) => addRcloneRemote(...a),
}));

vi.mock("../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

import { RcloneRemoteForm } from "./RcloneRemoteForm";
import { en } from "../lib/i18n";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof RcloneRemoteForm
>[0]["t"];

afterEach(() => {
  cleanup();
  addRcloneRemote.mockReset();
  pushed.length = 0;
});

describe("RcloneRemoteForm", () => {
  it("offers the SMB fields first and no WebDAV URL", () => {
    render(<RcloneRemoteForm t={t} />);
    expect(screen.getByText(en["rcloneRemote.host"])).toBeTruthy();
    expect(screen.getByText(en["rcloneRemote.share"])).toBeTruthy();
    expect(screen.queryByText(en["rcloneRemote.url"])).toBeNull();
  });

  it("shows the finished path once the destination is stored", async () => {
    addRcloneRemote.mockResolvedValue({ ok: true, location: "rclone:nas:backups" });

    render(<RcloneRemoteForm t={t} />);
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["rcloneRemote.add"], "i") }));

    await waitFor(() => expect(screen.getByText("rclone:nas:backups")).toBeTruthy());
    expect(screen.getByText(en["rcloneRemote.useThisPath"])).toBeTruthy();
  });

  // The server's message names the field that is wrong, which beats a generic
  // failure by a wide margin when the usual cause is a typo in a password.
  it("shows the server's own refusal", async () => {
    addRcloneRemote.mockResolvedValue({
      ok: false,
      error: "the remote name may contain only letters, digits, dashes and underscores",
    });

    render(<RcloneRemoteForm t={t} />);
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["rcloneRemote.add"], "i") }));

    await waitFor(() => expect(pushed).toHaveLength(1));
    expect(pushed[0].message).toContain("letters, digits");
    expect(pushed[0].severity).toBe("fail");
  });
});
