// @vitest-environment jsdom
// The Snapshots page lists the server's own snapshots without waking sleeping
// disks, reads those only when asked, and states what a delete frees.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import type { HostSnapshot, HostSnapshotList } from "../lib/api";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const calls = vi.hoisted(() => ({
  list: [] as boolean[],
  estimate: [] as unknown[],
  deleted: [] as unknown[],
  answer: (() => ({ ok: true })) as (wake: boolean) => unknown,
  paired: false,
}));

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    openedAsPairedPhone: () => calls.paired,
    listHostSnapshots: (wake: boolean) => {
      calls.list.push(wake);
      return Promise.resolve(calls.answer(wake));
    },
    estimateHostSnapshots: (refs: unknown) => {
      calls.estimate.push(refs);
      return Promise.resolve({ ok: true, zfsBytes: 2048 });
    },
    deleteHostSnapshots: (refs: { kind: string; source: string; name: string }[]) => {
      calls.deleted.push(refs);
      return Promise.resolve({ ok: true, results: refs.map((ref) => ({ ref, code: "ok" })) });
    },
  };
});

const { Snapshots } = await import("./Snapshots");

function snap(name: string): HostSnapshot {
  return {
    kind: "zfs",
    source: "tank/apps",
    name,
    label: name,
    createdAt: 1_700_000_000,
    usedBytes: 1024,
    referencedBytes: 4096,
    newSinceBytes: 512,
    holds: [],
    clones: [],
    owner: "",
    readOnly: true,
    caps: { rename: true, hold: true, delete: true, estimate: true },
  };
}

function list(wake: boolean): HostSnapshotList {
  return {
    ok: true,
    kinds: { zfs: { code: "ok" }, btrfs: { code: "missing" }, vm: { code: "ok" } },
    pools: [
      { kind: "zfs", name: "tank", state: "awake", read: true, woke: false },
      { kind: "zfs", name: "disk1", state: "asleep", read: wake, woke: wake },
    ],
    sources: [
      { kind: "zfs", source: "tank/apps", group: "tank", usedBytes: 1, newSinceLatestBytes: 1, canRecursive: true, canCreate: true },
      { kind: "zfs", source: "tank/media", group: "tank", usedBytes: 1, newSinceLatestBytes: 1, canRecursive: true, canCreate: true },
      { kind: "vm", source: "win11", group: "vm", usedBytes: -1, newSinceLatestBytes: -1, canRecursive: false, canCreate: false, reason: "needs-qcow2" },
    ],
    snapshots: [snap("before-update")],
    hiddenSystem: 0,
    plans: [],
  };
}

async function open() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={["/snapshots"]}>
            <Snapshots />
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

beforeEach(() => {
  calls.list = [];
  calls.estimate = [];
  calls.deleted = [];
  calls.answer = list;
  calls.paired = false;
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("the Snapshots page", () => {
  it("leaves sleeping disks alone until asked and then says they woke", async () => {
    await open();
    expect(calls.list).toEqual([false]);
    expect(screen.getByText(en["hostSnap.asleep.text"].replace("{pools}", "disk1"))).toBeTruthy();
    expect(screen.getByText("before-update")).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["hostSnap.asleep.read"] }));
    });
    expect(calls.list).toEqual([false, true]);
    expect(screen.getByText(en["hostSnap.asleep.woke"].replace("{pools}", "disk1"))).toBeTruthy();
  });

  it("states what a delete frees before it deletes", async () => {
    await open();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.delete"] }));
    });
    expect(calls.estimate).toHaveLength(1);
    expect(screen.getByText(/That frees about 2(\.0)? KB\./)).toBeTruthy();
    expect(calls.deleted).toHaveLength(0);
  });

  it("points to the SSH setup when the host is out of reach", async () => {
    calls.answer = () => ({ ...list(false), kinds: { zfs: { code: "ssh-missing" }, btrfs: { code: "ssh-missing" }, vm: { code: "ssh-missing" } }, snapshots: [], pools: [] });
    await open();
    expect(screen.getByText(en["hostSnap.noHost"])).toBeTruthy();
    expect(screen.queryByText(en["hostSnap.empty"])).toBeNull();
  });
});

describe("the Snapshots page on a paired phone", () => {
  it("says the snapshots are managed at the server and asks for nothing", async () => {
    calls.paired = true;
    await open();
    expect(screen.getByText(en["hostSnap.pairedOnly"])).toBeTruthy();
    expect(calls.list).toEqual([]);
  });
});

describe("the Snapshots page without a snapshot of a source", () => {
  it("says what a snapshot is beside a backup and offers the first snapshot of each source", async () => {
    await open();
    expect(screen.getByText(en["hostSnap.versusBackup"])).toBeTruthy();
    expect(screen.getByText("tank/media")).toBeTruthy();
    expect(screen.getByText("win11")).toBeTruthy();
    expect(screen.getByText(en["hostSnap.code.needsQcow2"])).toBeTruthy();
    // One per card that has snapshots, one for tank/media, and the page's own.
    expect(screen.getAllByRole("button", { name: en["hostSnap.create.title"] })).toHaveLength(3);
  });
});

describe("what a snapshot is beside a backup", () => {
  const explainer = en["hostSnap.versusBackup"];
  const inBubble = () => screen.queryAllByLabelText((_, el) => (el?.getAttribute("aria-label") ?? "").includes(explainer));

  it("stands at the end of the page until it is understood, then waits in the (i) of the cards", async () => {
    await open();
    const note = screen.getByText(explainer);
    const cards = [...document.querySelectorAll("[data-search-card]")];
    expect(cards.length).toBeGreaterThan(0);
    for (const card of cards) expect(card.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(inBubble()).toHaveLength(0);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.gotIt"] }));
    });
    expect(screen.queryByText(explainer)).toBeNull();
    expect(inBubble().length).toBeGreaterThan(0);
  });

  it("stays out of the way in this browser once understood", async () => {
    localStorage.setItem("bv-snapshots-explained", "1");
    await open();
    expect(screen.queryByText(explainer)).toBeNull();
  });
});
