// @vitest-environment jsdom
// The card where a ZFS item picks its replica. It has to say plainly where the
// replica stands and send exactly the rule someone set. Starting a run belongs
// to the entry page, not to the card.
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
const takeOvers: string[] = [];
const onlyCalls: boolean[] = [];
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
    setZFSReplicaOnly: (on: boolean) => {
      onlyCalls.push(on);
      current = { ...current, only: on && current.only !== false, sectionOnly: on };
      return Promise.resolve({ ok: true });
    },
    updateOffsiteTarget: () => Promise.reject(new Error("the switch touched an off-site target")),
    takeOverZFSReplicaFolder: (id: string) => {
      takeOvers.push(id);
      return Promise.resolve({ ok: true });
    },
    patchZFSReplica: (_id: string, patch: ZFSReplicaPatch) => {
      patches.push(patch);
      current = { ...current, ...patch };
      return Promise.resolve(patchAnswer);
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
  takeOvers.length = 0;
  onlyCalls.length = 0;
  patchAnswer = { ok: true };
  localStorage.clear();
});

afterEach(cleanup);

describe("replica card", () => {
  it("offers only the targets that can take a replica, plus none", async () => {
    renderCard();
    fireEvent.click(await screen.findByRole("combobox", { name: en["zfs.replica.target"] }));
    const names = screen.getAllByRole("option").map((r) => r.textContent);
    expect(names).toEqual([
      en["zfs.replica.targetNone"],
      `Backup-NAS (${en["instances.overSsh"]})`,
      "tower-2 (paired instance)",
    ]);
  });

  it("tells a ZFS server over SSH from a paired instance in German too", async () => {
    localStorage.setItem("bv-lang", "de");
    renderCard();
    fireEvent.click(await screen.findByRole("combobox", { name: de["zfs.replica.target"] }));
    const names = screen.getAllByRole("option").map((r) => r.textContent);
    expect(names).toEqual([de["zfs.replica.targetNone"], "Backup-NAS (über SSH)", "tower-2 (gekoppelte Instanz)"]);
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

  it("shows a run in progress", async () => {
    progress = { "zfs-replica:zfs1": { phase: "replicate", percent: 10, active: true, lastSeen: Date.now() } };
    renderCard();
    expect(await screen.findAllByText(en["zfs.replica.state.running"])).not.toHaveLength(0);
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

  it("leaves the keep rule to a paired instance that allowed it", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, peerState: "allowed" });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.peer.allowedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.getByText("Replicated 2 hours ago")).toBeTruthy();
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

  it("says a paired instance declined and offers no new request", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "refused" });
    renderCard();
    expect(await screen.findByText("Declined by tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.refusedText"].replace("{peer}", "tower-2"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.replica.peer.askAgain"] })).toBeNull();
  });

  it("says when the paired instance has receiving switched off", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, state: "never", peerState: "off" });
    renderCard();
    expect(await screen.findByText("Receiving is switched off on tower-2")).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.peer.offText"].replace("{peer}", "tower-2"))).toBeTruthy();
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

  it("sends how many snapshots stay on this server once the typing stops", async () => {
    renderCard();
    const field = (await screen.findByLabelText(en["zfs.replica.keepSource"])) as HTMLInputElement;
    expect(field.value).toBe("0");
    vi.useFakeTimers();
    try {
      fireEvent.change(field, { target: { value: "-2" } });
      fireEvent.change(field, { target: { value: "7" } });
      await act(async () => vi.advanceTimersByTime(1000));
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(patches).toEqual([{ keepOnSource: 7 }]));
  });

  it("explains in a bubble what the snapshots on this server are for", async () => {
    renderCard();
    const field = await screen.findByLabelText(en["zfs.replica.keepSource"]);
    const row = field.closest("[data-search-row]") as HTMLElement;
    expect(row.getAttribute("data-search-row")).toBe(en["zfs.replica.keepSource"]);
    expect(within(row).getByRole("img", { name: en["zfs.replica.keepSourceHint"] })).toBeTruthy();
  });

  it("keeps the snapshots on this server editable for a paired instance that allowed the replica", async () => {
    current = replica({ target: { kind: "peer", id: "peer-1" }, peerState: "allowed", keepOnSource: 3 });
    renderCard();
    const field = (await screen.findByLabelText(en["zfs.replica.keepSource"])) as HTMLInputElement;
    expect(field.value).toBe("3");
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
    fireEvent.click(await screen.findByRole("combobox", { name: en["zfs.replica.target"] }));
    fireEvent.click(screen.getByRole("option", { name: "tower-2 (paired instance)" }));
    await waitFor(() => expect(patches).toEqual([{ target: { kind: "peer", id: "peer-1" } }]));
    expect(await screen.findByText("Request sent to tower-2.")).toBeTruthy();
  });

  it("shows what the server holds after a change it stored but answered with a refusal", async () => {
    patchAnswer = { ok: false, code: "peer-unreachable" };
    renderCard();
    const picker = await screen.findByRole("combobox", { name: en["zfs.replica.target"] });
    fireEvent.click(picker);
    fireEvent.click(screen.getByRole("option", { name: "tower-2 (paired instance)" }));
    expect(await screen.findByText(en["zfs.code.peer-unreachable"])).toBeTruthy();
    await waitFor(() => expect(picker.textContent).toContain("tower-2 (paired instance)"));
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

  it("stands every one-line control on the row of its label", async () => {
    renderCard();
    const rowOf = (control: HTMLElement) => control.closest("[data-search-row]");
    const target = await screen.findByRole("combobox", { name: en["zfs.replica.target"] });
    expect(rowOf(target)?.getAttribute("data-search-row")).toBe(en["zfs.replica.target"]);
    const when = screen.getByRole("radiogroup", { name: en["zfs.replica.when"] });
    expect(rowOf(when)?.getAttribute("data-search-row")).toBe(en["zfs.replica.when"]);
    const keep = screen.getByRole("radiogroup", { name: en["zfs.replica.keepTarget"] });
    expect(rowOf(keep)?.getAttribute("data-search-row")).toBe(en["zfs.replica.keepTarget"]);
    const days = screen.getByLabelText(en["zfs.replica.keep.daily"]);
    expect(rowOf(days)?.getAttribute("data-search-row")).toBe(en["zfs.replica.keep.daily"]);
  });

  it("offers no interval in days for its own plan, which a replica cannot keep", async () => {
    current = replica({ afterBackup: false, cadence: "daily 03:00" });
    renderCard();
    await screen.findByRole("group", { name: en["zfs.replica.plan"] });
    expect(screen.queryByRole("tab", { name: en["cadence.everyN"] })).toBeNull();
  });

  it("says why the replica waits while ZFS backups are off", async () => {
    current = replica({ state: "waiting" });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.state.waiting"])).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.code.domain-off"])).toBeTruthy();
    expect(screen.queryByText(/Waiting for/)).toBeNull();
  });

  it("names a switched-off server as the reason the replica waits", async () => {
    current = replica({ target: { kind: "server", id: "off" }, state: "waiting" });
    renderCard();
    expect(await screen.findByLabelText(en["zfs.code.server-disabled"])).toBeTruthy();
  });

  it("gives the reason the server names for a waiting replica", async () => {
    current = replica({ state: "waiting", code: "server-disabled" });
    renderCard();
    expect(await screen.findByLabelText(en["zfs.code.server-disabled"])).toBeTruthy();
  });

  it("takes over a folder an earlier install of this BombVault left, once asked to", async () => {
    current = replica({ state: "failed", code: "target-owned" });
    renderCard();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.takeOver"] }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(en["zfs.replica.takeOverConfirm"].replace("{server}", "Backup-NAS"))).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("button", { name: en["zfs.replica.takeOver"] }));
    await waitFor(() => expect(takeOvers).toEqual(["zfs1"]));
    expect(await screen.findByText(en["zfs.replica.takenOver"].replace("{server}", "Backup-NAS"))).toBeTruthy();
  });

  it("switches replica only on and off again as one setting of the section", async () => {
    renderCard();
    const toggle = await screen.findByRole("switch", { name: en["zfs.replica.onlyToggle"] });
    fireEvent.click(toggle);
    await waitFor(() => expect(onlyCalls).toEqual([true]));
    const switchOf = () => screen.getByRole("switch", { name: en["zfs.replica.onlyToggle"] });
    await waitFor(() => expect(switchOf().getAttribute("aria-checked")).toBe("true"));
    fireEvent.click(switchOf());
    await waitFor(() => expect(onlyCalls).toEqual([true, false]));
    expect(screen.queryByText("the switch touched an off-site target")).toBeNull();
  });

  it("shows the section's switch on an entry with a repository of its own and turns it off there", async () => {
    current = replica({ only: false, sectionOnly: true });
    renderCard();
    const toggle = await screen.findByRole("switch", { name: en["zfs.replica.onlyToggle"] });
    expect(toggle.getAttribute("aria-checked")).toBe("true");
    fireEvent.click(toggle);
    await waitFor(() => expect(onlyCalls).toEqual([false]));
  });

  it("offers no take-over for any other failure", async () => {
    current = replica({ state: "failed", code: "no-common-base" });
    renderCard();
    await screen.findByRole("combobox", { name: en["zfs.replica.target"] });
    expect(screen.queryByRole("button", { name: en["zfs.replica.takeOver"] })).toBeNull();
  });

  it("leaves starting a run to the entry page", async () => {
    renderCard();
    await screen.findByRole("combobox", { name: en["zfs.replica.target"] });
    expect(screen.queryByRole("button", { name: en["zfs.replica.replicateNow"] })).toBeNull();
  });

  it("explains in the When bubble that an entry without a place only sends the replica", async () => {
    current = replica({ only: true });
    renderCard();
    expect(await screen.findByLabelText(`${en["zfs.replica.whenHint"]} ${en["zfs.replica.onlyHint"]}`)).toBeTruthy();
  });

  it("leaves the When bubble alone for an entry that has a place", async () => {
    renderCard();
    expect(await screen.findByLabelText(en["zfs.replica.whenHint"])).toBeTruthy();
    expect(screen.queryByLabelText(new RegExp(en["zfs.replica.onlyHint"].slice(0, 30)))).toBeNull();
  });

  it("hides everything but the picker while there is no target", async () => {
    current = replica({ target: { kind: "none", id: "" }, state: "never" });
    renderCard();
    await screen.findByRole("combobox", { name: en["zfs.replica.target"] });
    expect(screen.queryByText(en["zfs.replica.members"])).toBeNull();
  });

  it("says how each copy was mounted on a server that mounts", async () => {
    const base = replica();
    current = replica({
      members: [
        { ...base.members[0], mount: "mounted" },
        { ...base.members[1], volume: false, mount: "locked" },
      ],
    });
    renderCard();
    expect(await screen.findByText(en["zfs.replica.mount.mounted"], { exact: false })).toBeTruthy();
    expect(screen.getByText(en["zfs.replica.mount.locked"], { exact: false })).toBeTruthy();
  });
});
