// @vitest-environment jsdom
// Adding a ZFS server takes two steps, and the second one stays shut until the
// connection was tested, because its pools come from that test.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ZFSReplicaServerInput, ZFSReplicaTestResult } from "../../../lib/api";
import { group } from "./replica.testsupport";

let testAnswer: ZFSReplicaTestResult;
const tests: string[] = [];
const created: ZFSReplicaServerInput[] = [];

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getGroup: () => Promise.resolve(group()),
    getZFSReplicaKey: () => Promise.resolve({ publicKey: "ssh-ed25519 AAAAREPLICA bombvault-replica" }),
    testZFSReplicaConnection: (host: string, user: string, port: number) => {
      tests.push(`${user}@${host}:${port}`);
      return Promise.resolve(testAnswer);
    },
    createZFSReplicaServer: (body: ZFSReplicaServerInput) => {
      created.push(body);
      return Promise.resolve({ ok: true, id: "new", ...body });
    },
  };
});

const { AddReplicaServerDialog } = await import("./AddReplicaServerDialog");

let saved: string[] = [];

function renderDialog() {
  render(
    <I18nProvider>
      <ToastProvider>
        <AddReplicaServerDialog onClose={() => undefined} onSaved={(id) => saved.push(id)} />
      </ToastProvider>
    </I18nProvider>,
  );
}

const TB = 1024 ** 4;

beforeEach(() => {
  tests.length = 0;
  created.length = 0;
  saved = [];
  testAnswer = {
    ok: true,
    code: "ok",
    pools: [
      { name: "backup", sizeBytes: 8 * TB, freeBytes: 3.1 * TB },
      { name: "tank", sizeBytes: 2 * TB, freeBytes: 0.4 * TB },
    ],
  };
  localStorage.clear();
});

afterEach(cleanup);

function address() {
  return screen.getByLabelText(en["zfs.replica.server.address"]);
}

describe("adding a ZFS server", () => {
  it("hands out the key to put on the server", async () => {
    renderDialog();
    expect(await screen.findByText("ssh-ed25519 AAAAREPLICA bombvault-replica")).toBeTruthy();
    expect(screen.queryByText(/^zfs allow/)).toBeNull();
  });

  it("adds the zfs allow line for a user other than root", async () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText(en["zfs.replica.add.user"]), { target: { value: "bv" } });
    expect(await screen.findByText("zfs allow bv receive,create,mount,rollback,destroy,userprop <pool>")).toBeTruthy();
    expect(screen.getByText("Without root, bv needs these permissions on the pool:")).toBeTruthy();
  });

  it("keeps the pool step shut until the connection was tested", async () => {
    renderDialog();
    fireEvent.change(address(), { target: { value: "192.168.1.30" } });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.next"] }));
    expect(screen.getByText(en["zfs.replica.add.testFirst"])).toBeTruthy();
    expect((screen.getByRole("tab", { name: en["zfs.replica.add.stepPool"] }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("asks for an address before it tests", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.test"] }));
    expect(screen.getByText(en["zfs.replica.add.needHost"])).toBeTruthy();
    expect(tests).toEqual([]);
  });

  it("says why a test failed", async () => {
    testAnswer = { ok: false, code: "ssh-auth", pools: [] };
    renderDialog();
    fireEvent.change(address(), { target: { value: "192.168.1.30" } });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.test"] }));
    expect(await screen.findByText(en["zfs.code.ssh-auth"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.next"] }));
    expect(screen.getByText(en["zfs.replica.add.testFirst"])).toBeTruthy();
  });

  it("tests, picks a pool and adds the server under its root", async () => {
    renderDialog();
    fireEvent.change(address(), { target: { value: "192.168.1.30" } });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.test"] }));
    await waitFor(() => expect(tests).toEqual(["root@192.168.1.30:22"]));
    await screen.findByRole("button", { name: en["verdict.connected"] });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.next"] }));

    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.submit"] }));
    expect(screen.getByText(en["zfs.replica.add.needPool"])).toBeTruthy();

    fireEvent.click(screen.getByRole("radio", { name: "tank · 409.6 GB free" }));
    const root = screen.getByLabelText(en["zfs.replica.server.root"]) as HTMLInputElement;
    expect(root.value).toBe("tank/bombvault-replica");
    expect(screen.getByText("for example tank/bombvault-replica/tower/tank/appdata")).toBeTruthy();
    expect((screen.getByLabelText(en["zfs.replica.add.name"]) as HTMLInputElement).value).toBe("192.168.1.30");
    fireEvent.change(screen.getByLabelText(en["zfs.replica.add.name"]), { target: { value: "Backup-NAS" } });

    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.submit"] }));
    await waitFor(() =>
      expect(created).toEqual([
        { name: "Backup-NAS", host: "192.168.1.30", user: "root", port: 22, pool: "tank", root: "tank/bombvault-replica" },
      ]),
    );
    expect(saved).toEqual(["new"]);
  });

  it("goes back to the connection step", async () => {
    renderDialog();
    fireEvent.change(address(), { target: { value: "192.168.1.30" } });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.test"] }));
    await screen.findByRole("button", { name: en["verdict.connected"] });
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.add.next"] }));
    fireEvent.click(screen.getByRole("button", { name: en["common.back"] }));
    expect((address() as HTMLInputElement).value).toBe("192.168.1.30");
  });
});
