// @vitest-environment jsdom
// The shared repository this instance holds: the form sets it up, the card
// shows how it stands, an ask to join is allowed or declined like a role
// request, and a member is removed only after a question that says plainly
// how far removing goes.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { SharedServer } from "../../lib/api";

const server: SharedServer = {
  containerName: "rest-server-shared",
  folder: "user/restic-shared",
  hostPath: "/mnt/user/restic-shared",
  port: 8001,
  user: "vault-box",
  repo: "user/restic-shared/shared",
  url: "http://192.168.1.20:8001/shared",
  needsHost: false,
  createdAt: 1,
  present: true,
  check: "protected",
  checkDetail: "",
  checkedAt: 0,
  members: [],
  joins: [],
  copiesIn: false,
  id: "sr-1",
  former: [],
};

const withMembers: SharedServer = {
  ...server,
  members: [
    { memberId: "m-attic", name: "attic", host: "bv-attic", keys: ["k1", "k2"], lastSnapshotAt: Math.floor(Date.now() / 1000) - 120 },
    { memberId: "m-barn", name: "barn", host: "bv-barn", keys: ["k3"] },
  ],
  joins: [{ memberId: "m-cellar", name: "cellar", host: "bv-cellar", state: "asked", askedAt: Math.floor(Date.now() / 1000) - 60 }],
};

let initial: SharedServer | null = null;
let told = true;
const sent: { folder: string; port: number }[] = [];
const decided: [string, string][] = [];
const removed: string[] = [];
let checks = 0;
let copiedIn = 0;
const forgotten: [string, string, string][] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSharedRepo: () => Promise.resolve({ ok: true, server: initial, defaultPort: 8001, memberships: [] }),
    getReceiverServer: () => Promise.resolve({ ok: true, server: null, hostMountRoot: "/host/user" }),
    setUpSharedRepo: (input: { folder: string; port: number }) => {
      sent.push(input);
      return Promise.resolve({ ok: true, server: { ...server, port: input.port } });
    },
    checkSharedRepo: () => {
      checks++;
      return Promise.resolve({ ok: true, server: { ...server, check: "unprotected", checkDetail: "DELETE went through" } });
    },
    decideSharedJoin: (memberId: string, decision: string) => {
      decided.push([memberId, decision]);
      const joins = decision === "allow" ? withMembers.joins.map((j) => ({ ...j, state: "allowed" as const })) : [];
      return Promise.resolve({ ok: true, server: { ...withMembers, joins }, told });
    },
    forgetSharedFormer: (id: string, host: string, name: string) => {
      forgotten.push([id, host, name]);
      return Promise.resolve({ ok: true });
    },
    copyIntoOwnSharedRepo: () => {
      copiedIn++;
      const self = { memberId: "m-self", name: "vault-box", host: "bv-self", self: true };
      return Promise.resolve({ ok: true, server: { ...server, copiesIn: true, members: [self] } });
    },
    removeSharedMember: (memberId: string) => {
      removed.push(memberId);
      return Promise.resolve({ ok: true, server: { ...withMembers, members: withMembers.members.filter((m) => m.memberId !== memberId) }, told });
    },
  };
});

const { SharedRepoCard } = await import("./SharedRepoCard");
const { useSharedRepo } = await import("./useSharedRepo");

function Owned() {
  const shared = useSharedRepo(true);
  return shared.loaded ? <SharedRepoCard shared={shared} hueIndex={1} /> : null;
}

async function renderCard() {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <Owned />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
  });
}

const named = (key: keyof typeof en, name: string) => en[key].replaceAll("{name}", name);

afterEach(() => {
  cleanup();
  initial = null;
  told = true;
  sent.length = 0;
  decided.length = 0;
  removed.length = 0;
  checks = 0;
  copiedIn = 0;
  forgotten.length = 0;
});

describe("SharedRepoCard", () => {
  it("sets up the shared repository from the form, with port 8001 to start from", async () => {
    await renderCard();
    expect(screen.getByText(en["shared.intro"])).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: en["shared.setUp"] }));
    expect(screen.getByRole("dialog", { name: en["shared.setUp"] })).toBeTruthy();
    const start = screen.getByRole("button", { name: en["receiver.server.start"] }) as HTMLButtonElement;
    expect(start.disabled).toBe(true);
    expect(screen.getByDisplayValue("8001")).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("user/restic-shared"), { target: { value: " user/restic-shared " } });
    await act(async () => {
      fireEvent.click(start);
    });

    expect(sent).toEqual([{ folder: "user/restic-shared", port: 8001 }]);
    expect(screen.queryByRole("dialog", { name: en["shared.setUp"] })).toBeNull();
    expect(screen.getByText(server.url)).toBeTruthy();
    expect(screen.getByText(en["receiver.server.protected"])).toBeTruthy();
    expect(screen.getByText(en["shared.noMembers"])).toBeTruthy();
    expect(screen.getByText(`${en["shared.prunes"]} ${en["shared.readable"]}`)).toBeTruthy();
  });

  it("checks again and shows what the check found", async () => {
    initial = server;
    await renderCard();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.server.checkAgain"] }));
    });
    expect(checks).toBe(1);
    expect(screen.getByText(en["receiver.server.unprotected"])).toBeTruthy();
    expect(screen.getByText("DELETE went through")).toBeTruthy();
  });

  it("asks for the receiving server's address while the repository has none", async () => {
    initial = { ...server, url: "", needsHost: true };
    await renderCard();
    expect(screen.getByText(en["shared.needsHost"])).toBeTruthy();
  });

  it("lists each member with its host, keys and last backup", async () => {
    initial = withMembers;
    await renderCard();
    expect(screen.getByText("attic")).toBeTruthy();
    expect(screen.getByText(/bv-attic · 2 keys · last backup 2 minutes ago/)).toBeTruthy();
    expect(screen.getByText(/bv-barn · 1 key · no backup yet/)).toBeTruthy();
  });

  it("allows an ask to join, and then offers to tell the member again", async () => {
    initial = withMembers;
    await renderCard();
    expect(screen.getByText(named("shared.joinAsk", "cellar"), { exact: false })).toBeTruthy();
    expect(screen.getByText(en["instances.request"])).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.allow"] }));
    });
    expect(decided).toEqual([["m-cellar", "allow"]]);
    expect(await screen.findByText(named("shared.allowed", "cellar"))).toBeTruthy();
    expect(screen.getByText(named("shared.joinAllowed", "cellar"), { exact: false })).toBeTruthy();
    expect(screen.getByRole("button", { name: en["shared.tellAgain"] })).toBeTruthy();
  });

  it("says when the one-time key of an allowed member that has not joined yet expires", async () => {
    initial = { ...withMembers, joins: [{ ...withMembers.joins[0], state: "allowed", expiresAt: 1_900_000_000 }] };
    await renderCard();
    expect(screen.getByText(/one-time key expires/)).toBeTruthy();
  });

  it("declines an ask and says so when the member could not be told", async () => {
    initial = withMembers;
    told = false;
    await renderCard();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["zfs.receive.decline"] }));
    });
    expect(decided).toEqual([["m-cellar", "refuse"]]);
    expect(await screen.findByText(named("shared.notTold", "cellar"))).toBeTruthy();
    expect(screen.queryByText(named("shared.joinAsk", "cellar"), { exact: false })).toBeNull();
  });

  it("removes a member only after a question that says only a new repository shuts it out for good", async () => {
    initial = withMembers;
    await renderCard();

    await act(async () => {
      fireEvent.click(screen.getAllByRole("button", { name: en["shared.remove"] })[0]);
    });
    expect(removed).toEqual([]);
    const ask = named("shared.removeAsk", "attic");
    expect(ask).toContain("only a new repository shuts a server out for good");
    expect(screen.getByText(ask)).toBeTruthy();

    const buttons = screen.getAllByRole("button", { name: en["shared.remove"] });
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });
    expect(removed).toEqual(["m-attic"]);
    expect(screen.queryByText("attic")).toBeNull();
    expect(screen.getByText("barn")).toBeTruthy();
  });

  it("lets this server copy its own backups in and then points to Storage locations", async () => {
    initial = server;
    await renderCard();
    expect(screen.getByText(en["shared.own.offer"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.own.start"] }));
    });
    expect(copiedIn).toBe(1);
    expect(screen.getByText(en["shared.own.on"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["shared.member.openStorage"] })).toBeTruthy();
    expect(screen.getByText(en["shared.own.self"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["shared.remove"] })).toBeNull();
  });

  it("forgets what a server that left still has there only once its name is typed", async () => {
    initial = { ...server, former: [{ host: "bv-attic", name: "attic", leftAt: Math.floor(Date.now() / 1000) - 3600, snapshots: 12, forgetting: false }] };
    await renderCard();
    expect(screen.getByText(en["shared.former"])).toBeTruthy();
    expect(screen.getByText(/bv-attic · 12 backups/)).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.former.forget"] }));
    });
    const go = screen.getAllByRole("button", { name: en["shared.former.forget"] }).at(-1)!;
    expect((go as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(screen.getByLabelText(en["offsiteRemoval.typeName"].replace("{name}", "attic")), { target: { value: "attic" } });
    await act(async () => {
      fireEvent.click(go);
    });
    expect(forgotten).toEqual([["sr-1", "bv-attic", "attic"]]);
    expect(await screen.findByText(named("shared.former.started", "attic"))).toBeTruthy();
  });

  it("says why forgetting what a server left failed", async () => {
    initial = { ...server, former: [{ host: "bv-attic", name: "attic", leftAt: 1, snapshots: 3, forgetting: false, forgetError: "disk full" }] };
    await renderCard();
    expect(screen.getByText(/disk full/)).toBeTruthy();
  });

  it("keeps a member when the question is cancelled", async () => {
    initial = withMembers;
    await renderCard();
    await act(async () => {
      fireEvent.click(screen.getAllByRole("button", { name: en["shared.remove"] })[0]);
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(removed).toEqual([]);
    expect(screen.getByText("attic")).toBeTruthy();
  });
});
