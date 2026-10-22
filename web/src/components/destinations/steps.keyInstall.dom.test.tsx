// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { Provider } from "../../lib/api";
import { en, I18nProvider } from "../../lib/i18n";
import type { DestinationDraftState } from "./useDestinationDraft";

const installDraftKey = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  installDraftKey: (d: unknown) => installDraftKey(d),
}));

const { KeyInstall, manualKeyCommand } = await import("./steps");

const KEY = "ssh-ed25519 AAAAC3Nza bombvault";
const STORAGE_BOX = { id: "storagebox", name: "Hetzner Storage Box", route: "rclone", group: "server", auth: "sshkey", preset: { port: "23" } } as Provider;
const SFTP = { id: "sftp", name: "SFTP", route: "rclone", group: "server", auth: "sshkey" } as Provider;

function draft(picked: Provider, settings: Record<string, string>, runTest = vi.fn(async () => true)): DestinationDraftState {
  return { picked, settings, publicKey: KEY, runTest } as unknown as DestinationDraftState;
}

async function show(d: DestinationDraftState) {
  await act(async () => {
    render(
      <I18nProvider>
        <KeyInstall draft={d} />
      </I18nProvider>
    );
  });
}

async function typeAndAdd(password: string) {
  await act(async () => {
    fireEvent.change(screen.getByLabelText(en["dest.keyInstall.password"]), { target: { value: password } });
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["dest.keyInstall.add"] }));
  });
}

afterEach(() => {
  cleanup();
  installDraftKey.mockReset();
});

it("builds the manual command for the host, user and port in the form", () => {
  expect(manualKeyCommand(true, KEY, { host: "u1.your-storagebox.de", user: "u1-sub1" }, "23")).toBe(
    `echo "${KEY}" | ssh -p23 u1-sub1@u1.your-storagebox.de install-ssh-key`
  );
  expect(manualKeyCommand(false, KEY, { host: " nas ", user: "backup" }, "2222")).toBe(
    `ssh -p2222 backup@nas 'umask 077; mkdir -p ~/.ssh && echo "${KEY}" >> ~/.ssh/authorized_keys'`
  );
  expect(manualKeyCommand(true, KEY, {}, "23")).toContain("<user>@<host>");
});

it("shows a Storage Box the install-ssh-key command and the note about sub-accounts", async () => {
  await show(draft(STORAGE_BOX, { host: "u1.your-storagebox.de", user: "u1", port: "22" }));
  expect(screen.getByText(`echo "${KEY}" | ssh -p23 u1@u1.your-storagebox.de install-ssh-key`)).toBeTruthy();
  expect(screen.getByText(en["dest.keyInstall.subAccount"])).toBeTruthy();
});

it("waits for host, user and password before it signs in", async () => {
  await show(draft(SFTP, { host: "nas" }));
  const add = screen.getByRole("button", { name: en["dest.keyInstall.add"] }) as HTMLButtonElement;
  expect(add.disabled).toBe(true);
  expect(screen.queryByText(en["dest.keyInstall.subAccount"])).toBeNull();
});

it("sends the password once, forgets it and tests the connection", async () => {
  installDraftKey.mockResolvedValue({ ok: true });
  const runTest = vi.fn(async () => true);
  await show(draft(SFTP, { host: "nas", user: "backup" }, runTest));
  await typeAndAdd("secret");
  expect(installDraftKey).toHaveBeenCalledWith({ provider: "sftp", settings: { host: "nas", user: "backup" }, password: "secret" });
  expect(runTest).toHaveBeenCalledTimes(1);
  expect((screen.getByLabelText(en["dest.keyInstall.password"]) as HTMLInputElement).value).toBe("");
});

it("says SSH support is off when a Storage Box refuses the session", async () => {
  installDraftKey.mockResolvedValue({ ok: false, code: "session-refused", error: "the server refused a session" });
  const runTest = vi.fn(async () => true);
  await show(draft(STORAGE_BOX, { host: "u1.your-storagebox.de", user: "u1-sub1" }, runTest));
  await typeAndAdd("secret");
  expect(screen.getByRole("alert").textContent).toBe(en["dest.keyInstall.err.sshOff"]);
  expect(runTest).not.toHaveBeenCalled();
});

it("explains a wrong password and an unreachable server", async () => {
  installDraftKey.mockResolvedValue({ ok: false, code: "password-refused", error: "the server refused the password" });
  await show(draft(SFTP, { host: "nas", user: "backup" }));
  await typeAndAdd("wrong");
  expect(screen.getByRole("alert").textContent).toBe(en["dest.keyInstall.err.password"]);

  installDraftKey.mockResolvedValue({ ok: false, code: "unreachable", error: "dial tcp: i/o timeout" });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["dest.keyInstall.add"] }));
  });
  expect(screen.getByRole("alert").textContent).toBe(en["dest.keyInstall.err.unreachable"]);
});

it("shows the server's own words for a failure without a code", async () => {
  installDraftKey.mockResolvedValue({ ok: false, error: "sshconn: install-ssh-key: invalid key" });
  await show(draft(STORAGE_BOX, { host: "h", user: "u" }));
  await typeAndAdd("secret");
  expect(screen.getByRole("alert").textContent).toBe("sshconn: install-ssh-key: invalid key");
});
