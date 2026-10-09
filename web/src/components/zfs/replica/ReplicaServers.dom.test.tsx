// @vitest-environment jsdom
// The ZFS servers replicas go to: listed with what they hold and how full
// they are, each with a page of its own, and removable without leaving an
// item pointing at nothing.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ZFSReplica, ZFSReplicaServer, ZFSReplicaServerPatch } from "../../../lib/api";
import { group, replica, server } from "./replica.testsupport";

let servers: ZFSReplicaServer[];
let current: ZFSReplica;
const deleted: string[] = [];
const patched: ZFSReplicaServerPatch[] = [];

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    listZFSReplicaServers: () => Promise.resolve(servers),
    getGroup: () => Promise.resolve(group()),
    getZFSReplica: () => Promise.resolve(current),
    listZFSDatasets: () =>
      Promise.resolve({ ok: true, datasets: [{ id: "zfs1", dataset: "cache/appdata" }] }),
    patchZFSReplicaServer: (_id: string, patch: ZFSReplicaServerPatch) => {
      patched.push(patch);
      servers = servers.map((s) => ({ ...s, ...patch }));
      return Promise.resolve({ ok: true });
    },
    deleteZFSReplicaServer: (id: string, detach: boolean) => {
      deleted.push(`${id}:${detach}`);
      servers = servers.filter((s) => s.id !== id);
      return Promise.resolve({ ok: true });
    },
  };
});

vi.mock("../../../lib/progress", () => ({ useProgress: () => ({}) }));

const { ReplicaServers } = await import("./ReplicaServers");

function renderList() {
  render(
    <I18nProvider>
      <ToastProvider>
        <ReplicaServers />
      </ToastProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  servers = [server({ usedBy: ["zfs1"] }), server({ id: "old", name: "Old box", enabled: false })];
  current = replica();
  deleted.length = 0;
  patched.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("ZFS servers", () => {
  it("says how to get one while there is none", async () => {
    servers = [];
    renderList();
    expect(await screen.findByText(en["zfs.replica.servers.empty"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.replica.addServer"] })).toBeTruthy();
  });

  it("lists each server with its use and free space, or that it is off", async () => {
    renderList();
    expect(await screen.findByText("Backup-NAS")).toBeTruthy();
    expect(screen.getByText("1 in use", { exact: false })).toBeTruthy();
    expect(screen.getByText("3.1 TB free of 8.0 TB")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.servers.off"])).toBeTruthy();
  });

  it("opens a server with the items that replicate there", async () => {
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /Backup-NAS/ }));
    expect(await screen.findByText("Replica of cache/appdata", { exact: false })).toBeTruthy();
    expect(screen.getByText("root@192.168.1.30:22")).toBeTruthy();
    expect(screen.getByText("backup/bombvault-replica/tower/cache/appdata")).toBeTruthy();
    expect(screen.getByText("Replicated 2 hours ago")).toBeTruthy();
  });

  it("shows where an entry's copy really lives, under the folder the server keeps for it", async () => {
    const path = "backup/bombvault-replica/Tower-B-ro/cache/appdata";
    current = replica({ members: [{ ...replica().members[0], targetPath: path }] });
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /Backup-NAS/ }));
    expect(await screen.findByText(path)).toBeTruthy();
    expect(screen.queryByText("backup/bombvault-replica/tower/cache/appdata")).toBeNull();
  });

  it("builds the example path from the folder the server reports", async () => {
    servers = [server({ usedBy: [], folder: "Tower-B-ro" })];
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /Backup-NAS/ }));
    expect(await screen.findByText("for example backup/bombvault-replica/Tower-B-ro/tank/appdata")).toBeTruthy();
  });

  it("switches a server off from its page", async () => {
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /Backup-NAS/ }));
    fireEvent.click(await screen.findByRole("switch", { name: en["zfs.replica.server.enabled"] }));
    await waitFor(() => expect(patched).toEqual([{ enabled: false }]));
  });

  it("detaches the items that use a server it removes", async () => {
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /Backup-NAS/ }));
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.server.remove"] }));
    expect(await screen.findByText(/^Remove Backup-NAS\? BombVault then sends nothing there/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.removeConfirm"] }));
    await waitFor(() => expect(deleted).toEqual(["nas:true"]));
    expect(await screen.findByText("Old box")).toBeTruthy();
  });
});
