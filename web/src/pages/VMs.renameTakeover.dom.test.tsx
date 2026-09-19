// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { Snapshot, VM } from "../lib/api";
import type { TranslationKey } from "../lib/i18n";

class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);
vi.stubGlobal(
  "fetch",
  vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 })),
);

const snap = (id: string): Snapshot => ({ id, time: "2026-09-01T00:00:00Z", paths: [], tags: [], hostname: "tower" });

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    listVMs: vi.fn(async () => ({ ok: true, vms: [] })),
    listVMSnapshots: vi.fn(async () => ({ ok: true, snapshots: [snap("a"), snap("b")] })),
    takeOverVM: vi.fn(async () => ({ ok: true })),
    unlinkVMAlias: vi.fn(async () => ({ ok: true })),
  };
});

const { listVMs, listVMSnapshots, takeOverVM, unlinkVMAlias } = await import("../lib/api");
const { VMRow, VMs } = await import("./VMs");
const { en } = await import("../lib/i18n");
const { ToastProvider } = await import("../lib/toast");

const t = ((key: TranslationKey) => en[key]) as unknown as Parameters<typeof VMRow>[0]["t"];

// On TrueNAS the name a user reads and the libvirt name differ, and only the
// libvirt name reaches a route.
const win11: VM = {
  name: "win11",
  libvirtName: "1_win11",
  state: "running",
  method: "graceful",
  includeInSchedule: false,
  lastBackup: null,
  lastBackupStarted: null,
};

function renderRow(vm: VM, linkCandidates?: string[], onRefresh: () => void = () => {}) {
  return render(
    <ToastProvider>
      <VMRow vm={vm} t={t} onRefresh={onRefresh} linkCandidates={linkCandidates} index={0} />
    </ToastProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe("taking over a renamed VM's entry", () => {
  const suggested: VM = { ...win11, renameFrom: "1_windows-11", renameReason: "libvirt-uuid" };

  it("names the entry, the evidence and its backup count", async () => {
    renderRow(suggested);
    expect(screen.getByText("Looks like 1_windows-11")).toBeTruthy();
    expect(await screen.findByText("same VM (UUID), 2 backups")).toBeTruthy();
    expect(listVMSnapshots).toHaveBeenCalledWith("1_windows-11");
  });

  it("sends the libvirt name, never the display name, then reloads the list", async () => {
    const onRefresh = vi.fn();
    renderRow(suggested, undefined, onRefresh);
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));

    await waitFor(() => expect(takeOverVM).toHaveBeenCalledWith("1_win11", "1_windows-11"));
    expect(takeOverVM).not.toHaveBeenCalledWith("win11", expect.anything());
    await waitFor(() => expect(onRefresh).toHaveBeenCalled());
  });

  it("follows a new suggestion on the same card after the earlier one was dismissed", () => {
    const view = renderRow(suggested);
    fireEvent.click(screen.getByRole("button", { name: "Not this one" }));
    view.rerender(
      <ToastProvider>
        <VMRow vm={{ ...suggested, renameFrom: "1_win-11" }} t={t} onRefresh={() => {}} index={0} />
      </ToastProvider>,
    );

    expect(screen.getByText("Looks like 1_win-11")).toBeTruthy();
  });

  it("unlinks a former name under the libvirt name", async () => {
    renderRow({ ...win11, lastBackup: 1_757_000_000, aliases: ["1_windows-11"] });
    fireEvent.click(screen.getByRole("button", { name: "Unlink 1_windows-11" }));
    const dialog = await screen.findByRole("dialog");
    const buttons = within(dialog).getAllByRole("button");
    fireEvent.click(buttons[buttons.length - 1]);

    await waitFor(() => expect(unlinkVMAlias).toHaveBeenCalledWith("1_win11", "1_windows-11"));
  });

  it("links by hand under the libvirt name", async () => {
    renderRow(win11, ["1_windows-11"]);
    fireEvent.click(screen.getByRole("button", { name: "Link to an entry…" }));
    fireEvent.click(screen.getByRole("button", { name: "Take over" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Take over" }));

    await waitFor(() => expect(takeOverVM).toHaveBeenCalledWith("1_win11", "1_windows-11"));
  });

  it("lists only the VMs that are no longer defined", async () => {
    vi.mocked(listVMs).mockResolvedValue({
      ok: true,
      vms: [
        win11,
        { ...win11, name: "debian", libvirtName: "1_debian", lastBackup: 1_757_000_000 },
        { ...win11, name: "1_windows-11", libvirtName: "1_windows-11", state: "not-installed" },
      ],
    });
    render(
      <ToastProvider>
        <VMs />
      </ToastProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Link to an entry…" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Former entry" }));

    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["1_windows-11"]);
  });
});
