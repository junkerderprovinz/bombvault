// @vitest-environment jsdom
// The card a ZFS user looks at first. Everything needed to repair a broken
// connection has to be on it, including the key and the command that
// authorizes it, because a basic-mode user with VM backups off never sees the
// Settings card that otherwise carries them.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { ZFSConnectionResult } from "../../lib/api";

let connection: ZFSConnectionResult;
let connectionCalls = 0;
let connectionFails = false;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    zfsConnection: () => {
      connectionCalls += 1;
      if (connectionFails) return Promise.reject(new actual.ApiError(502, "HTTP 502 Bad Gateway"));
      return Promise.resolve(connection);
    },
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower", publicKey: "ssh-ed25519 AAAAKEY bombvault" }),
  };
});

const { ZFSConnectionCard } = await import("./ZFSConnectionCard");

function result(overrides: Partial<ZFSConnectionResult>): ZFSConnectionResult {
  return {
    ok: true,
    code: "ok",
    target: "root@192.168.1.10",
    uriTarget: "",
    version: "2.3.4",
    detail: "",
    zfsBinary: "/usr/sbin/zfs",
    propagation: "slave",
    unpropagated: [],
    ...overrides,
  };
}

function renderCard() {
  render(
    <I18nProvider>
      <ToastProvider>
        <ZFSConnectionCard />
      </ToastProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  connectionCalls = 0;
  connectionFails = false;
  connection = result({});
});

afterEach(cleanup);

describe("ZFS connection card", () => {
  it("says the test could not run and offers it again", async () => {
    connectionFails = true;
    renderCard();
    expect(await screen.findByText(en["common.networkError"])).toBeTruthy();
    connectionFails = false;
    fireEvent.click(screen.getByRole("button", { name: en["zfs.connection.test"] }));
    expect(await screen.findByText("Connected as root@192.168.1.10")).toBeTruthy();
    expect(connectionCalls).toBe(2);
  });

  it("collapses a working connection to the target and the version", async () => {
    renderCard();
    expect(screen.getByText(en["zfs.connection.testing"])).toBeTruthy();
    await waitFor(() => expect(connectionCalls).toBe(1));
    expect(screen.getByText("Connected as root@192.168.1.10")).toBeTruthy();
    expect(screen.getByText("OpenZFS 2.3.4")).toBeTruthy();
    expect(screen.queryByText(en["zfs.connection.details"])).toBeNull();
  });

  it("names the fallback address it had to use", async () => {
    connection = result({ code: "host-fallback" });
    renderCard();
    await screen.findByText("Connected as root@192.168.1.10");
    expect(screen.getByText(en["zfs.code.host-fallback"])).toBeTruthy();
  });

  it("shows the sentence, the fix and the raw output of a failure", async () => {
    connection = result({
      ok: false,
      code: "zfs-permission",
      version: "",
      detail: "cannot create snapshot: permission denied",
    });
    renderCard();
    await screen.findByText(en["zfs.code.zfs-permission"]);
    expect(screen.getByLabelText(en["zfs.fix.zfs-permission"])).toBeTruthy();
    screen.getByRole("button", { name: en["zfs.connection.details"] }).click();
    expect(await screen.findByText("cannot create snapshot: permission denied")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.connection.test"] })).toBeTruthy();
  });

  it("hands out the key and the authorize command when the server refused it", async () => {
    connection = result({ ok: false, code: "ssh-auth", version: "", detail: "Permission denied (publickey)." });
    renderCard();
    await screen.findByText(en["zfs.code.ssh-auth"]);
    expect(await screen.findByText("ssh-ed25519 AAAAKEY bombvault")).toBeTruthy();
    expect(screen.getByText(en["zfs.connection.authorize"])).toBeTruthy();
    expect(screen.getByText(/chmod 600/)).toBeTruthy();
    expect(screen.getByRole("button", { name: en["vm.ssh.copyCmd"] })).toBeTruthy();
  });

  it("warns about missing propagation while the connection itself works", async () => {
    connection = result({ code: "propagation-missing", unpropagated: ["/mnt/cache"] });
    renderCard();
    await screen.findByText(en["zfs.code.propagation-missing"]);
    expect(screen.getByLabelText(en["zfs.fix.propagation-missing"])).toBeTruthy();
    expect(screen.getByText("Connected as root@192.168.1.10")).toBeTruthy();
  });

  it("names both targets of a mismatch", async () => {
    connection = result({
      ok: false,
      code: "uri-mismatch",
      target: "root@192.168.1.10",
      uriTarget: "root@tower.local",
      version: "",
    });
    renderCard();
    await screen.findByText(
      "LIBVIRT_URI points to root@tower.local, but zfs commands go to root@192.168.1.10.",
    );
  });
});
