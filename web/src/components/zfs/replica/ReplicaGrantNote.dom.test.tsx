// @vitest-environment jsdom
// A paired instance that asks to pull an item gets nothing until someone here
// answers, and an allowed one keeps its access only until it is revoked.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { ZFSReplica, ZFSReplicaGrant } from "../../../lib/api";
import { group, replica } from "./replica.testsupport";

let current: ZFSReplica;
const decisions: string[] = [];

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getZFSReplica: () => Promise.resolve(current),
    getGroup: () => Promise.resolve(group()),
    listZFSReplicaServers: () => Promise.resolve([]),
    decideZFSReplicaGrant: (_id: string, peer: string, decision: string) => {
      decisions.push(`${peer}:${decision}`);
      const next = decision === "allow" ? "allowed" : decision === "refuse" ? "refused" : "revoked";
      current = { ...current, grants: current.grants.map((g) => (g.peer === peer ? { ...g, state: next } : g)) };
      return Promise.resolve({ ok: true });
    },
  };
});

vi.mock("../../../lib/progress", () => ({ useProgress: () => ({}) }));

const { ReplicaGrantNote, ReplicaGrantRows } = await import("./ReplicaGrantNote");

function grant(state: ZFSReplicaGrant["state"]): ZFSReplicaGrant {
  return {
    peer: "peer-1",
    fingerprint: "SHA256:q9TrF0rT",
    state,
    askedAt: new Date(Date.now() - 30_000).toISOString(),
    decidedAt: state === "asked" ? "" : new Date(Date.now() - 3 * 86400_000).toISOString(),
  };
}

function Both() {
  return (
    <>
      <ReplicaGrantNote itemId="zfs1" name="cache/appdata" />
      <ReplicaGrantRows itemId="zfs1" grants={current.grants} />
    </>
  );
}

function renderBoth() {
  return render(
    <I18nProvider>
      <ToastProvider>
        <Both />
      </ToastProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  decisions.length = 0;
  current = replica({ target: { kind: "none", id: "" }, grants: [grant("asked")] });
});

afterEach(cleanup);

describe("pull grants", () => {
  it("names who asks, with the key and when", async () => {
    renderBoth();
    expect(await screen.findByText("tower-2 wants to fetch cache/appdata")).toBeTruthy();
    expect(screen.getByText("Key SHA256:q9TrF0rT · asked just now")).toBeTruthy();
    expect(screen.getByLabelText(en["zfs.replica.grant.askedHint"])).toBeTruthy();
  });

  it("allows a request and says so", async () => {
    renderBoth();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.grant.accept"] }));
    await waitFor(() => expect(decisions).toEqual(["peer-1:allow"]));
    expect(await screen.findByText("tower-2 may fetch cache/appdata now.")).toBeTruthy();
  });

  it("refuses a request", async () => {
    renderBoth();
    fireEvent.click(await screen.findByRole("button", { name: en["zfs.replica.grant.decline"] }));
    await waitFor(() => expect(decisions).toEqual(["peer-1:refuse"]));
    expect(await screen.findByText("tower-2 gets no access.")).toBeTruthy();
  });

  it("shows nothing for a refused or revoked request", async () => {
    current = replica({ grants: [grant("refused"), { ...grant("revoked"), peer: "peer-2" }] });
    const { container } = renderBoth();
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("revokes an allowed grant only after the question is answered", async () => {
    current = replica({ grants: [grant("allowed")] });
    renderBoth();
    expect(await screen.findByText("May fetch: tower-2")).toBeTruthy();
    expect(screen.getByText("allowed 3 days ago")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.grant.revoke"] }));
    expect(
      await screen.findByText(
        "Revoke access for tower-2? BombVault deletes the key of tower-2 on this server. What is already there stays there.",
      ),
    ).toBeTruthy();
    expect(decisions).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: en["zfs.replica.grant.revokeConfirm"] }));
    await waitFor(() => expect(decisions).toEqual(["peer-1:revoke"]));
  });
});
