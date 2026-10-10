// @vitest-environment jsdom
// The receiving side of a ZFS replica, as one instance's window shows it.
// Nothing may arrive before someone here allows it, an Allow has to carry
// exactly the pool, root and keep rule shown, and taking the permission back
// needs a second answer.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ZFSReceiveDecision, ZFSReceiveRequest, ZFSReplicaKeep, ZFSReplicaPool } from "../../../lib/api";
import { receiveRequest } from "./replica.testsupport";

let requests: ZFSReceiveRequest[] = [];
let pools: ZFSReplicaPool[] = [];
const decisions: [string, ZFSReceiveDecision][] = [];
const keeps: [string, ZFSReplicaKeep][] = [];
let decide: () => { ok: boolean; code?: string } = () => ({ ok: true });

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    listZFSReceiveRequests: () => Promise.resolve(requests),
    listZFSLocalPools: () => Promise.resolve(pools),
    decideZFSReceiveRequest: (id: string, decision: ZFSReceiveDecision) => {
      decisions.push([id, decision]);
      return Promise.resolve(decide());
    },
    patchZFSReceiveRequest: (id: string, keep: ZFSReplicaKeep) => {
      keeps.push([id, keep]);
      return Promise.resolve({ ok: true });
    },
  };
});

const { ZFSReceiveRequests } = await import("./ZFSReceiveRequests");

function renderCard(peer = "peer-1") {
  return render(
    <I18nProvider>
      <ToastProvider>
        <ZFSReceiveRequests peer={peer} />
      </ToastProvider>
    </I18nProvider>,
  );
}

const TB = 1024 ** 4;

beforeEach(() => {
  requests = [receiveRequest()];
  pools = [
    { name: "tank", sizeBytes: 8 * TB, freeBytes: 3 * TB },
    { name: "backup", sizeBytes: 4 * TB, freeBytes: 2 * TB },
  ];
  decisions.length = 0;
  decide = () => ({ ok: true });
  keeps.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("receiving a ZFS replica", () => {
  it("stays away while no request waits and none is allowed", async () => {
    requests = [receiveRequest({ state: "refused" }), receiveRequest({ id: "rq2", state: "revoked" })];
    const { container } = renderCard();
    await act(async () => undefined);
    expect(container.textContent).toBe("");
  });

  it("shows only the instance it was opened for", async () => {
    requests = [
      receiveRequest(),
      receiveRequest({ id: "rq2", peer: "peer-2", peerName: "attic", sourceServer: "attic", item: "tank/media" }),
      receiveRequest({ id: "rq3", peer: "peer-2", peerName: "attic", sourceServer: "attic", state: "allowed", root: "tank/bombvault-replica" }),
    ];
    renderCard("peer-2");
    expect(await screen.findByText("attic wants to replicate tank/media here")).toBeTruthy();
    expect(screen.getByText("cache/appdata from attic")).toBeTruthy();
    expect(screen.queryByText("tower-2 wants to replicate cache/appdata here")).toBeNull();
  });

  it("stays away for an instance that asked for nothing", async () => {
    const { container } = renderCard("peer-9");
    await act(async () => undefined);
    expect(container.textContent).toBe("");
  });

  it("shows who asks for what, with the proposal filled in", async () => {
    renderCard();
    expect(await screen.findByText("tower-2 wants to replicate cache/appdata here")).toBeTruthy();
    const members = screen.getByRole("list", { name: en["zfs.replica.members"] });
    expect(within(members).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "cache/appdata",
      "cache/appdata/vm-disk",
    ]);
    const root = (await screen.findByLabelText(en["zfs.replica.server.root"])) as HTMLInputElement;
    await waitFor(() => expect(root.value).toBe("tank/bombvault-replica"));
    expect(screen.getByText("for example tank/bombvault-replica/tower-2/cache/appdata")).toBeTruthy();
    expect(screen.getByText("14 daily, 8 weekly")).toBeTruthy();
  });

  it("names the asking instance every time the hint mentions it", async () => {
    renderCard();
    await screen.findByText("tower-2 wants to replicate cache/appdata here");
    expect(screen.getByLabelText(en["zfs.receive.askedHint"].replaceAll("{peer}", "tower-2"))).toBeTruthy();
  });

  it("allows with the pool, root and keep rule on screen", async () => {
    renderCard();
    const poolPicker = await screen.findByRole("radiogroup", { name: en["zfs.receive.pool"] });
    fireEvent.click(await within(poolPicker).findByRole("radio", { name: /^backup/ }));
    const root = screen.getByLabelText(en["zfs.replica.server.root"]) as HTMLInputElement;
    expect(root.value).toBe("backup/bombvault-replica");
    fireEvent.change(root, { target: { value: "backup/from-tower" } });
    const keep = screen.getByRole("radiogroup", { name: en["zfs.receive.keep"] });
    fireEvent.click(within(keep).getByRole("radio", { name: en["zfs.replica.keep.long"] }));

    fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.allow"] }));
    await waitFor(() =>
      expect(decisions).toEqual([
        [
          "rq1",
          {
            decision: "allow",
            pool: "backup",
            root: "backup/from-tower",
            keep: { preset: "long", own: [0, 14, 8, 0, 0] },
            item: "cache/appdata",
            members: ["cache/appdata", "cache/appdata/vm-disk"],
          },
        ],
      ]),
    );
    expect(await screen.findByText("tower-2 may send cache/appdata here now.")).toBeTruthy();
  });

  it("reads the requests again and says so when the source asked for more since they were shown", async () => {
    renderCard();
    // Allow refuses until a pool is on screen, so the pools have to be there first.
    await screen.findByRole("radiogroup", { name: en["zfs.receive.pool"] });
    const members = ["cache/appdata", "cache/appdata/vm-disk", "cache/appdata/media"];
    decide = () => {
      requests = [receiveRequest({ members })];
      return { ok: false, code: "request-changed" };
    };
    fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(await screen.findByText(en["zfs.code.request-changed"])).toBeTruthy();
    const list = screen.getByRole("list", { name: en["zfs.replica.members"] });
    await waitFor(() => expect(within(list).getAllByRole("listitem").map((li) => li.textContent)).toEqual(members));
  });

  it("declines without asking for a pool", async () => {
    pools = [];
    renderCard();
    expect(await screen.findByText(en["zfs.receive.noPools"])).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.decline"] }));
    await waitFor(() => expect(decisions).toEqual([["rq1", { decision: "refuse" }]]));
    expect(await screen.findByText("tower-2 gets no access.")).toBeTruthy();
  });

  it("does not allow into no pool", async () => {
    pools = [];
    renderCard();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.getByText(en["zfs.replica.add.needPool"])).toBeTruthy();
    expect(decisions).toEqual([]);
  });

  it("lists an allowed replica with its folder and last transfer", async () => {
    requests = [
      receiveRequest({
        state: "allowed",
        pool: "tank",
        root: "tank/bombvault-replica",
        decidedAt: new Date(Date.now() - 24 * 3600 * 1000).toISOString(),
        lastReceived: new Date(Date.now() - 2 * 3600 * 1000).toISOString(),
        bytes: 118 * 1024 * 1024,
      }),
    ];
    renderCard();
    expect(await screen.findByText("cache/appdata from tower-2")).toBeTruthy();
    expect(screen.getByText("tank/bombvault-replica/tower-2")).toBeTruthy();
    expect(screen.getByText("Last received 2 hours ago · 118.0 MB")).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["zfs.receive.allow"] })).toBeNull();
  });

  it("says when an allowed replica has not arrived yet", async () => {
    requests = [receiveRequest({ state: "allowed", root: "tank/bombvault-replica" })];
    renderCard();
    expect(await screen.findByText(en["zfs.receive.nothingYet"])).toBeTruthy();
  });

  it("changes what an allowed replica keeps once the typing stops", async () => {
    requests = [receiveRequest({ state: "allowed", root: "tank/bombvault-replica" })];
    renderCard();
    const days = (await screen.findByLabelText(en["zfs.replica.keep.daily"])) as HTMLInputElement;
    vi.useFakeTimers();
    try {
      fireEvent.change(days, { target: { value: "3" } });
      fireEvent.change(days, { target: { value: "30" } });
      expect(keeps).toEqual([]);
      await act(async () => vi.advanceTimersByTime(1000));
    } finally {
      vi.useRealTimers();
    }
    await waitFor(() => expect(keeps).toEqual([["rq1", { preset: "own", own: [0, 30, 3, 0, 0] }]]));
  });

  it("revokes only after the question is answered", async () => {
    requests = [receiveRequest({ state: "allowed", root: "tank/bombvault-replica" })];
    renderCard();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.receive.revoke"] }));
    expect(await screen.findByText(/^Revoke access for tower-2\? tower-2 can then send nothing more here/)).toBeTruthy();
    expect(decisions).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.revokeConfirm"] }));
    await waitFor(() => expect(decisions).toEqual([["rq1", { decision: "revoke" }]]));
    expect(await screen.findByText(en["zfs.receive.revoked"])).toBeTruthy();
  });

  it("asks about an instance whose name holds a dollar sign exactly as it is named", async () => {
    requests = [receiveRequest({ state: "allowed", peerName: "Lab$&", root: "tank/bombvault-replica" })];
    renderCard();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.receive.revoke"] }));
    const asked = "Revoke access for Lab$&? Lab$& can then send nothing more here";
    expect(await screen.findByText((text) => text.startsWith(asked))).toBeTruthy();
  });
});
