// @vitest-environment jsdom
// A ZFS server's page has an address of its own under Instances. An address
// that names no server leads back to the grid.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import { group, server } from "../components/zfs/replica/replica.testsupport";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listZFSReplicaServers: () => Promise.resolve([server()]),
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
