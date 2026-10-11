// @vitest-environment jsdom
// The page of one ZFS server replicas go to: what replicates there and where
// each copy lives, and the server switched off, made to forget its host key
// or removed without leaving an item pointing at nothing.
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
const forgotten: string[] = [];
const onBack = vi.fn();

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    listZFSReplicaServers: () => Promise.resolve(servers),
    getGroup: () => Promise.resolve(group()),
    getZFSReplica: () => Promise.resolve(current),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [{ id: "zfs1", dataset: "cache/appdata" }] }),
    patchZFSReplicaServer: (_id: string, patch: ZFSReplicaServerPatch) => {
      patched.push(patch);
      servers = servers.map((s) => ({ ...s, ...patch }));
      return Promise.resolve({ ok: true });
    },
    forgetZFSReplicaHostKey: (id: string) => {
      forgotten.push(id);
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

const { ReplicaServerPage } = await import("./ReplicaServerPage");

async function renderServer() {
  render(
    <I18nProvider>
      <ToastProvider>
        <ReplicaServerPage serverId="nas" onBack={onBack} />
      </ToastProvider>
    </I18nProvider>
  );
  await screen.findByText("Backup-NAS");
}

beforeEach(() => {
  servers = [server({ usedBy: ["zfs1"] }), server({ id: "old", name: "Old box", enabled: false })];
  current = replica();
  deleted.length = 0;
  patched.length = 0;
  forgotten.length = 0;
  onBack.mockClear();
  localStorage.clear();
});

afterEach(cleanup);

describe("a ZFS server", () => {
  it("names how to reach it, how full it is and the items that replicate there", async () => {
    await renderServer();
    expect(await screen.findByText("Replica of cache/appdata", { exact: false })).toBeTruthy();
    expect(screen.getByText("root@192.168.1.30:22")).toBeTruthy();
    expect(screen.getByText("3.1 TB free of 8.0 TB")).toBeTruthy();
    expect(screen.getByText("backup/bombvault-replica/tower/cache/appdata")).toBeTruthy();
    expect(screen.getByText("Replicated 2 hours ago")).toBeTruthy();
  });

  it("shows where an entry's copy really lives, under the folder the server keeps for it", async () => {
    const path = "backup/bombvault-replica/Tower-B-ro/cache/appdata";
    current = replica({ members: [{ ...replica().members[0], targetPath: path }] });
    await renderServer();
    expect(await screen.findByText(path)).toBeTruthy();
    expect(screen.queryByText("backup/bombvault-replica/tower/cache/appdata")).toBeNull();
  });

  it("builds the example path from the folder the server reports", async () => {
    servers = [server({ usedBy: [], folder: "Tower-B-ro" })];
    await renderServer();
    expect(await screen.findByText("for example backup/bombvault-replica/Tower-B-ro/tank/appdata")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.servers.unused"], { exact: false })).toBeTruthy();
  });

  it("is switched off from its page", async () => {
    await renderServer();
    fireEvent.click(await screen.findByRole("switch", { name: en["zfs.replica.server.enabled"] }));
    await waitFor(() => expect(patched).toEqual([{ enabled: false }]));
  });

  it("forgets its host key only once the question is answered", async () => {
    await renderServer();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.server.forgetHostKey"] }));
    expect(await screen.findByText(/^Forget the host key of Backup-NAS\? The next connection pins/)).toBeTruthy();
    expect(forgotten).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.forgetHostKeyConfirm"] }));
    await waitFor(() => expect(forgotten).toEqual(["nas"]));
    expect(await screen.findByText(en["zfs.replica.server.hostKeyForgotten"])).toBeTruthy();
  });

  it("detaches the items that use it when it is removed, and leaves its page", async () => {
    await renderServer();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.server.remove"] }));
    expect(await screen.findByText(/^Remove Backup-NAS\? BombVault then sends nothing there/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.server.removeConfirm"] }));
    await waitFor(() => expect(deleted).toEqual(["nas:true"]));
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1));
  });
});
