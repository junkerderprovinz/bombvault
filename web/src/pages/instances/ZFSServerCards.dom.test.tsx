// @vitest-environment jsdom
// A ZFS server's card on a paired phone shows what the server holds without
// its address, which the phone is not told, and without the actions that
// change the server.
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import type { ZFSReplicaServer } from "../../lib/api";

const session = { phone: false };
const server: ZFSReplicaServer = {
  id: "s1",
  name: "backup",
  host: "backup.lan",
  user: "root",
  port: 22,
  pool: "tank",
  root: "tank/bombvault-replica",
  enabled: true,
  mountReadOnly: false,
  freeBytes: 3000,
  sizeBytes: 4000,
  usedBy: [],
  folder: "bottich",
  reachable: true,
};

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    openedAsPairedPhone: () => session.phone,
    listZFSReplicaServers: () =>
      Promise.resolve([session.phone ? { ...server, host: "", user: "", port: 0 } : server]),
  };
});

const { ZFSServerCards } = await import("./ZFSServerCards");

afterEach(() => {
  cleanup();
  session.phone = false;
});

function show() {
  render(
    <MemoryRouter>
      <I18nProvider>
        <ZFSServerCards replicas={[]} firstIndex={0} hot={null} />
      </I18nProvider>
    </MemoryRouter>,
  );
}

it("names the server's address and offers to edit and open it", async () => {
  show();
  expect(await screen.findByText("root@backup.lan")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["zfs.replica.server.edit"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["instances.open"] })).toBeTruthy();
});

it("shows a paired phone the server and its folder, without the address or the actions that change it", async () => {
  session.phone = true;
  show();
  expect(await screen.findByText("backup")).toBeTruthy();
  expect(screen.getByText("tank/bombvault-replica")).toBeTruthy();
  expect(screen.queryByText(/@/)).toBeNull();
  expect(screen.queryByRole("button", { name: en["zfs.replica.server.edit"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["instances.open"] })).toBeNull();
});
