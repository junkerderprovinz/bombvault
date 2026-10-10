// @vitest-environment jsdom
// The card where a ZFS item picks its replica. It has to say plainly where the
// replica stands, send exactly the rule someone set, and never offer a run to
// a paired instance that has not allowed it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider, de, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ProgressMap } from "../../../lib/progress";
import type { ZFSReplica, ZFSReplicaPatch } from "../../../lib/api";
import { group, replica, server } from "./replica.testsupport";

let current: ZFSReplica;
let progress: ProgressMap = {};
const patches: ZFSReplicaPatch[] = [];
let runs = 0;
// What a PATCH answers. The store keeps the change either way, as the server
// does when only the request to a paired instance fails.
let patchAnswer: { ok: boolean; code?: string } = { ok: true };

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getZFSReplica: () => Promise.resolve(current),
    listZFSReplicaServers: () => Promise.resolve([server(), server({ id: "off", name: "Old box", enabled: false })]),
    getGroup: () => Promise.resolve(group()),
    patchZFSReplica: (_id: string, patch: ZFSReplicaPatch) => {
      patches.push(patch);
      current = { ...current, ...patch };
      return Promise.resolve(patchAnswer);
    },
    runZFSReplica: () => {
      runs += 1;
      return Promise.resolve({ ok: true, runId: "r1" });
    },
  };
});

vi.mock("../../../lib/progress", () => ({ useProgress: () => progress }));

const { ReplicaCard } = await import("./ReplicaCard");

function renderCard() {
  return render(
    <I18nProvider>
      <ToastProvider>
        <ReplicaCard itemId="zfs1" name="cache/appdata" />
      </ToastProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  current = replica();
  progress = {};
  patches.length = 0;
  runs = 0;
  patchAnswer = { ok: true };
  localStorage.clear();
});

afterEach(cleanup);

describe("replica card", () => {
  it("offers only the targets that can take a replica, plus none", async () => {
    renderCard();
    const picker = await screen.findByRole("radiogroup", { name: en["zfs.replica.target"] });
    const names = within(picker).getAllByRole("radio").map((r) => r.textContent);
    expect(names).toEqual([en["zfs.replica.targetNone"], "Backup-NAS", "tower-2"]);
  });

  it("says when a replica has never run", async () => {
    current = replica({ state: "never", lastRun: "", snapshots: [], members: replica().members.map((m) => ({ ...m, state: "never" })) });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.state.never"])).toBeTruthy();
    expect(screen.getAllByText(en["zfs.replica.member.never"])).toHaveLength(2);
    expect(screen.queryByText(/^Last run/)).toBeNull();
  });

  it("names when a current replica ran and what it sent", async () => {
    renderCard();
    expect(await screen.findByText("Replicated 2 hours ago")).toBeTruthy();
    expect(screen.getAllByText(en["zfs.replica.member.ok"])).toHaveLength(2);
    expect(screen.getByText("Last run 2 hours ago: 118.0 MB sent in 42s.")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.volume"])).toBeTruthy();
    expect(screen.getByText("to backup/bombvault-replica/tower/cache/appdata")).toBeTruthy();
  });

  it("shows a run in progress and holds the button until it ends", async () => {
    progress = { "zfs-replica:zfs1": { phase: "replicate", percent: 10, active: true, lastSeen: Date.now() } };
    renderCard();
    expect(await screen.findAllByText(en["zfs.replica.state.running"])).not.toHaveLength(0);
    const button = screen.getByRole("button", { name: en["zfs.replica.replicateNow"] }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
  });

  it("puts the reason of a failed dataset in its own words", async () => {
    current = replica({
      state: "failed",
      code: "no-common-base",
      members: [{ ...replica().members[0], state: "failed", code: "no-common-base" }],
    });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.state.failed"])).toBeTruthy();
    expect(screen.getByText(en["zfs.code.no-common-base"])).toBeTruthy();
  });

  it("sends nothing to a paired instance that has not allowed it yet", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "asked" });
    renderCard();
    expect(await screen.findByText("Waiting for tower-2 to allow it")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.askedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
    // The rule here is only what the receiving instance is offered.
    expect(screen.getByRole("radiogroup", { name: "Suggested to tower-2" })).toBeTruthy();
    expect(screen.queryByRole("radiogroup", { name: en["zfs.replica.keepTarget"] })).toBeNull();
  });

  it("names the paired instance wherever a language repeats it", async () => {
    localStorage.setItem("bv-lang", "de");
    current = replica({ target: { kind: "peer", id: "peer-1" }, peerState: "allowed" });
    renderCard();
    expect(await screen.findByText(de["zfs.replica.peer.allowedText"].replaceAll("{peer}", "tower-2"))).toBeTruthy();
  });

  it("runs to a paired instance that allowed it and leaves the keep rule to it", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, peerState: "allowed" });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.peer.allowedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.getByText("Replicated 2 hours ago")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeTruthy();
    expect(screen.queryByRole("radiogroup", { name: "Suggested to tower-2" })).toBeNull();
    expect(screen.getByRole("radiogroup", { name: en["zfs.replica.when"] })).toBeTruthy();
  });

  it("shows no path for a paired instance, whose pool and folder are its own", async () => {
    current = replica({
      target: { kind: "peer", id: "peer-1" },
      peerState: "allowed",
      members: replica().members.map((m) => ({ ...m, targetPath: `tower/${m.dataset}` })),
    });
    renderCard();
    expect(await screen.findByText("cache/appdata")).toBeTruthy();
    expect(screen.queryAllByText(/^to /)).toHaveLength(0);
    expect(screen.getByText("+31.0 MB in the last run")).toBeTruthy();
  });

  it("says a paired instance declined and offers neither a run nor a new request", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "refused" });
    renderCard();
    expect(await screen.findByText("Declined by tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.refusedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
    expect(screen.queryByRole("button", { name: en["zfs.replica.peer.askAgain"] })).toBeNull();
  });

  it("says when the paired instance has receiving switched off", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "off" });
    renderCard();
    expect(await screen.findByText("Receiving is switched off on tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.offText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("says in German when the paired instance has receiving switched off", async () => {
    localStorage.setItem("bv-lang", "de");
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "off" });
    renderCard();
    expect(await screen.findByText("Empfangen ist bei tower-2 ausgeschaltet")).toBeTruthy();
  });

  it("says a dataset a restart cut off is picked up by the next run", async () => {
    current = replica({
      state: "failed",
      code: "interrupted",
      members: [{ ...replica().members[0], state: "failed", code: "interrupted" }],
    });
    renderCard();
    expect(await screen.findByText(en["zfs.code.interrupted"])).toBeTruthy();
  });

  it("asks a paired instance again after it revoked the permission", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, peerState: "revoked" });
    renderCard();
    expect(await screen.findByText("Access revoked by tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.revokedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.peer.askAgain"] }));
    await waitFor(() => expect(patches).toEqual([{ target: { kind: "peer", id: "peer-1" } }]));
    expect(await screen.findByText("Request sent to tower-2.")).toBeTruthy();
  });

  it("sends a preset as it is picked", async () => {
    renderCard();
    const keep = await screen.findByRole("radiogroup", { name: en["zfs.replica.keepTarget"] });
    expect(screen.getByText("7 daily, 3 weekly")).toBeTruthy();
    vi.useFakeTimers();
    try {
      fireEvent.click(within(keep).getByRole("radio", { name: en["zfs.replica.keep.long"] }));
      expect(screen.getByText("14 daily, 8 weekly, 12 monthly, 3 yearly")).toBeTruthy();
      expect(screen.queryByLabelText(en["zfs.replica.keep.daily"])).toBeNull();
      await act(async () => vi.advanceTimersByTime(1000));
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(patches).toEqual([{ keep: { preset: "long", own: [0, 7, 3, 0, 0] } }]));
  });

  it("sends own numbers once the typing stops", async () => {
    renderCard();
    const days = (await screen.findByLabelText(en["zfs.replica.keep.daily"])) as HTMLInputElement;
    vi.useFakeTimers();
    try {
      fireEvent.change(days, { target: { value: "1" } });
      fireEvent.change(days, { target: { value: "14" } });
      expect(patches).toEqual([]);
      await act(async () => vi.advanceTimersByTime(1000));
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(patches).toEqual([{ keep: { preset: "own", own: [0, 14, 3, 0, 0] } }]));
  });

  it("switches to an own plan and shows the schedule editor", async () => {
    renderCard();
    const when = await screen.findByRole("radiogroup", { name: en["zfs.replica.when"] });
    fireEvent.click(within(when).getByRole("radio", { name: en["zfs.replica.ownPlan"] }));
    await waitFor(() => expect(patches).toEqual([{ afterBackup: false }]));
    expect(screen.getByRole("group", { name: en["zfs.replica.plan"] })).toBeTruthy();
  });

  it("points the replica at another target", async () => {
    renderCard();
    const picker = await screen.findByRole("radiogroup", { name: en["zfs.replica.target"] });
    fireEvent.click(within(picker).getByRole("radio", { name: "tower-2" }));
    await waitFor(() => expect(patches).toEqual([{ target: { kind: "peer", id: "peer-1" } }]));
    expect(await screen.findByText("Request sent to tower-2.")).toBeTruthy();
  });

  it("starts a run on request", async () => {
    renderCard();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.replicateNow"] }));
    await waitFor(() => expect(runs).toBe(1));
  });

  it("shows what the server holds after a change it stored but answered with a refusal", async () => {
    patchAnswer = { ok: false, code: "peer-unreachable" };
    renderCard();
    const picker = await screen.findByRole("radiogroup", { name: en["zfs.replica.target"] });
    fireEvent.click(within(picker).getByRole("radio", { name: "tower-2" }));
    expect(await screen.findByText(en["zfs.code.peer-unreachable"])).toBeTruthy();
    await waitFor(() =>
      expect(within(picker).getByRole("radio", { name: "tower-2" }).getAttribute("aria-checked")).toBe("true"),
    );
  });

  it("sends a schedule and a keep rule edited close together in one save", async () => {
    current = replica({ afterBackup: false, cadence: "daily 03:00" });
    renderCard();
    const keep = await screen.findByRole("radiogroup", { name: en["zfs.replica.keepTarget"] });
    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByRole("tab", { name: en["cadence.weekly"] }));
      fireEvent.click(within(keep).getByRole("radio", { name: en["zfs.replica.keep.long"] }));
      await act(async () => vi.advanceTimersByTime(1000));
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0].cadence).toMatch(/^weekly/);
    expect(patches[0].keep?.preset).toBe("long");
  });

  it("offers no interval in days for its own plan, which a replica cannot keep", async () => {
    current = replica({ afterBackup: false, cadence: "daily 03:00" });
    renderCard();
    await screen.findByRole("group", { name: en["zfs.replica.plan"] });
    expect(screen.queryByRole("tab", { name: en["cadence.everyN"] })).toBeNull();
  });

  it("says why the replica waits and offers no run while ZFS backups are off", async () => {
    current = replica({ state: "waiting" });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.state.waiting"])).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.code.domain-off"])).toBeTruthy();
    expect(screen.queryByText(/Waiting for/)).toBeNull();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("names a switched-off server as the reason the replica waits", async () => {
    current = replica({ target: { kind: "server", id: "off" }, state: "waiting" });
    renderCard();
    expect(await screen.findByLabelText(en["zfs.code.server-disabled"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("gives the reason the server names for a waiting replica", async () => {
    current = replica({ state: "waiting", code: "server-disabled" });
    renderCard();
    expect(await screen.findByLabelText(en["zfs.code.server-disabled"])).toBeTruthy();
  });

  it("holds the run button while a bring back has the replica", async () => {
    progress = { "zfs-replica-restore:zfs1": { phase: "replicate", percent: 10, active: true, lastSeen: Date.now() } };
    renderCard();
    const button = (await screen.findByRole("button", { name: en["zfs.replica.replicateNow"] })) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
  });

  it("hides everything but the picker while there is no target", async () => {
    current = replica({ target: { kind: "none", id: "" }, state: "never" });
    renderCard();
    await screen.findByRole("radiogroup", { name: en["zfs.replica.target"] });
    expect(screen.queryByText(en["zfs.replica.members"])).toBeNull();
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });
});
