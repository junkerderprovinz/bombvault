// @vitest-environment jsdom
// A container from another server whose network this host lacks, such as an
// Unraid br0 network on a plain Docker host, is restored onto a network the
// user picks here.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

let missingNetwork = "";
const foreignRestore = vi.fn((req: unknown) => {
  void req;
  return Promise.resolve({ ok: true, started: true });
});
const checkRestore = vi.fn((req: unknown) => {
  void req;
  return Promise.resolve({
    ok: true,
    ready: true,
    checks: [
      { id: "repository", status: "ok" },
      { id: "key", status: "ok" },
      { id: "snapshot", status: "ok" },
      { id: "space", status: "ok", need: 1, free: 2 },
    ],
    plan: null,
  });
});

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({
        ok: true,
        settings: {
          restoreFolder: "",
          configPath: "backups/config",
          configOffsite: "",
          containersPath: "backups/containers",
          vmsPath: "backups/vms",
          flashPath: "backups/flash",
          filesPath: "backups/files",
          zfsPath: "backups/zfs",
          encryptionEnabled: true,
        },
        hostMountRoot: "/host/user",
      }),
    detectEncryption: () => Promise.resolve({ ok: false }),
    discover: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverVMs: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverFiles: () => Promise.resolve({ ok: true, discovered: 0 }),
    discoverZFS: () => Promise.resolve({ ok: true, discovered: 0 }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    listZFSDatasets: () => Promise.resolve({ ok: true, datasets: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getVMSSH: () => Promise.resolve({ ok: true, host: "tower" }),
    foreignOpen: () =>
      Promise.resolve({
        ok: true,
        session: "s1",
        inventory: { containers: [{ name: "web", snapshots: [] }], vms: [], fileSets: [], dbDumps: [], zfs: [] },
      }),
    foreignClose: () => Promise.resolve({ ok: true }),
    foreignContainerWarnings: () =>
      Promise.resolve({
        ok: true,
        warnings: [],
        missingNetwork,
        networks: missingNetwork ? ["bridge", "proxy"] : [],
      }),
    foreignRestore: (req: unknown) => foreignRestore(req),
    checkRestore: (req: unknown) => checkRestore(req),
  };
});

const Recovery = (await import("./Recovery")).default;

async function connect() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Recovery />
        </ToastProvider>
      </I18nProvider>
    );
  });
  const field = (label: string) =>
    (screen.getByText(label).closest("div") as HTMLElement).querySelector("input") as HTMLInputElement;
  fireEvent.change(field(en["recovery.foreignKey"]), { target: { value: "a".repeat(64) } });
  fireEvent.change(field(en["recovery.foreignLocation"]), { target: { value: "backups/other" } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: en["recovery.foreignConnect"] }));
  });
}

function containerRow(): HTMLElement {
  return screen.getByRole("button", { name: en["recovery.foreignRestore"] }).closest(".glim-hue") as HTMLElement;
}

async function restore(row: HTMLElement) {
  await act(async () => {
    fireEvent.click(within(row).getByRole("button", { name: en["recovery.foreignRestore"] }));
  });
}

beforeEach(() => {
  foreignRestore.mockClear();
  checkRestore.mockClear();
  missingNetwork = "";
});

afterEach(cleanup);

describe("a container whose network this server lacks", () => {
  it("restores onto bridge unless another network is picked", async () => {
    missingNetwork = "br0.20";
    await connect();
    const row = containerRow();
    expect(within(row).getByText("br0.20")).toBeTruthy();

    await restore(row);

    expect(checkRestore.mock.calls[0][0]).toMatchObject({ kind: "foreign", name: "web", network: "bridge" });
    expect(foreignRestore.mock.calls[0][0]).toMatchObject({ domain: "containers", item: "web", network: "bridge" });
  });

  it("restores onto the picked network", async () => {
    missingNetwork = "br0.20";
    await connect();
    const row = containerRow();
    fireEvent.click(within(row).getByRole("combobox", { name: en["recovery.foreignNetwork"] }));
    fireEvent.click(screen.getByRole("option", { name: "proxy" }));

    await restore(row);

    expect(foreignRestore.mock.calls[0][0]).toMatchObject({ network: "proxy" });
  });

  it("asks nothing when the network exists here", async () => {
    await connect();
    const row = containerRow();
    expect(within(row).queryByRole("combobox", { name: en["recovery.foreignNetwork"] })).toBeNull();

    await restore(row);

    expect((foreignRestore.mock.calls[0][0] as { network?: string }).network).toBeUndefined();
  });
});
