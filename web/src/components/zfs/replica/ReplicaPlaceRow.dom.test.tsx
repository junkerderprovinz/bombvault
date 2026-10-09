// @vitest-environment jsdom
// The replica among an item's storage locations, and the sheet behind it: the
// snapshots on the target, the commands to use one there, and bringing one
// back here without touching the item itself.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ZFSReplica } from "../../../lib/api";
import { group, replica, server } from "./replica.testsupport";

let current: ZFSReplica;
const restores: string[] = [];

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getZFSReplica: () => Promise.resolve(current),
    listZFSReplicaServers: () => Promise.resolve([server()]),
    getGroup: () => Promise.resolve(group()),
    restoreZFSReplica: (_id: string, snapshot: string) => {
      restores.push(snapshot);
      return Promise.resolve({ ok: true, runId: "r2", dataset: "cache/appdata-bombvault-restore-1760003000" });
    },
  };
});

vi.mock("../../../lib/progress", () => ({ useProgress: () => ({}) }));

const { ReplicaPlaceRow } = await import("./ReplicaPlaceRow");

function renderRow() {
  return render(
    <I18nProvider>
      <ToastProvider>
        <ReplicaPlaceRow itemId="zfs1" name="cache/appdata" />
      </ToastProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  current = replica();
  restores.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("replica storage row", () => {
  it("is absent while the item has no replica", async () => {
    current = replica({ target: { kind: "none", id: "" } });
    const { container } = renderRow();
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("counts the snapshots on the target", async () => {
    renderRow();
    expect(await screen.findByText("Backup-NAS")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.place"])).toBeTruthy();
    expect(screen.getByText(countText(en["zfs.replica.snapshots"], "en", 2))).toBeTruthy();
  });

  it("says a pulling instance is still waiting instead of offering snapshots", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "waiting", snapshots: [] });
    renderRow();
    expect(await screen.findByText("Waiting for tower-2")).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.view"] })).toBeNull();
  });

  it("hands out the clone and takeover commands with the real names", async () => {
    renderRow();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.view"] }));
    expect(screen.getByRole("dialog", { name: "Replica of cache/appdata on Backup-NAS" })).toBeTruthy();
    expect(
      screen.getByText(
        "zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata",
      ),
    ).toBeTruthy();
    expect(screen.getByText(/^zfs inherit -r readonly backup\/bombvault-replica\/tower\/cache\/appdata/)).toBeTruthy();
  });

  it("brings the picked snapshot back as a new dataset after the question", async () => {
    renderRow();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.view"] }));
    fireEvent.click(screen.getByRole("combobox", { name: en["zfs.replica.sheet.snapshot"] }));
    const older = new Date("2026-10-05T01:40:00Z").toLocaleString();
    fireEvent.click(screen.getByRole("option", { name: older }));
    expect(screen.getByText(/@bombvault-replica-20261005014000 /)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.restore"] }));
    expect(await screen.findByText(new RegExp(`from ${older.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} from Backup-NAS`))).toBeTruthy();
    expect(restores).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.restoreConfirm"] }));
    await waitFor(() => expect(restores).toEqual(["bombvault-replica-20261005014000"]));
    expect(await screen.findByText("Bringing back: cache/appdata-bombvault-restore-1760003000")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "Replica of cache/appdata on Backup-NAS" })).toBeNull();
  });
});
