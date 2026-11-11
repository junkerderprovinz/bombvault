// @vitest-environment jsdom
// A ZFS server's page has an address of its own under Instances. An address
// that names no server leads back to the grid.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import { group, server } from "../components/zfs/replica/replica.testsupport";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const servers = vi.hoisted(() => ({ list: () => Promise.resolve<unknown[]>([]) }));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listZFSReplicaServers: () => servers.list(),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    getGroup: () => Promise.resolve(group()),
  };
});

const { ZFSServer } = await import("./ZFSServer");

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname}</div>;
}

async function openAt(path: string) {
  window.history.replaceState(null, "", path);
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[path]}>
            <Routes>
              <Route path="/instances/zfs/:id" element={<ZFSServer />} />
              <Route path="*" element={<Where />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

beforeEach(() => {
  servers.list = () => Promise.resolve([server()]);
});

afterEach(cleanup);

describe("a ZFS server's page", () => {
  it("shows the server the address names", async () => {
    await openAt("/instances/zfs/nas");
    expect(screen.getByRole("heading", { level: 1, name: en["zfs.replica.servers.title"] })).toBeTruthy();
    expect(screen.getByText("Backup-NAS")).toBeTruthy();
    expect(screen.getByText("root@192.168.1.30:22")).toBeTruthy();
  });

  it("goes back to the grid", async () => {
    await openAt("/instances/zfs/nas");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.back"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/instances");
  });

  it("leads to the grid for an address that names no server", async () => {
    await openAt("/instances/zfs/gone");
    expect(screen.getByTestId("where").textContent).toBe("/instances");
  });
});

describe("a tap that leaves before the servers arrive", () => {
  it("keeps the page the tap opened", async () => {
    let answer = (_: unknown[]) => {};
    servers.list = () => new Promise((resolve) => (answer = resolve));
    await openAt("/instances/zfs/gone");

    // The tap's page is in the address, and the router has not drawn it yet.
    window.history.pushState(null, "", "/restore");
    await act(async () => answer([server()]));

    expect(screen.queryByTestId("where")).toBeNull();
  });
});
