// @vitest-environment jsdom
// The shared repository this instance holds: the form sets it up, the card
// shows how it stands, an ask to join is allowed or declined like a role
// request, and a member is removed only after a question that says plainly
// how far removing goes.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { SharedHolding, SharedServer } from "../../lib/api";

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
let holdings: SharedHolding[] = [];
const takenBack: string[] = [];
let told = true;
const sent: { folder: string; port: number }[] = [];
const decided: [string, string][] = [];
const removed: string[] = [];
let checks = 0;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSharedRepo: () => Promise.resolve({ ok: true, server: initial, defaultPort: 8001, memberships: [], holdings }),
    takeBackSharedRepo: (id: string) => {
      takenBack.push(id);
      holdings = [];
      return Promise.resolve({ ok: true });
    },
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
      <I18nProvider>
        <ToastProvider>
          <Owned />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

const named = (key: keyof typeof en, name: string) => en[key].replaceAll("{name}", name);

afterEach(() => {
  cleanup();
  initial = null;
  holdings = [];
  takenBack.length = 0;
  told = true;
  sent.length = 0;
  decided.length = 0;
  removed.length = 0;
  checks = 0;
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

  it("says plainly that another server took the repository over and takes it back after a question", async () => {
    initial = server;
    holdings = [{ repo: "sr1", takenBy: "cube", takenAt: Math.floor(Date.now() / 1000) - 60 }];
    await renderCard();
    expect(screen.getByText(named("shared.takenOver", "cube"))).toBeTruthy();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.takeBack"] }));
    });
    expect(takenBack).toEqual([]);
    expect(screen.getByText(named("shared.takeBackAsk", "cube"))).toBeTruthy();
    const buttons = screen.getAllByRole("button", { name: en["shared.takeBack"] });
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });
    expect(takenBack).toEqual(["sr1"]);
    expect(screen.queryByText(named("shared.takenOver", "cube"))).toBeNull();
  });

  it("names the servers that have still to hear it holds the repository", async () => {
    initial = server;
    holdings = [{ repo: "sr1", stillToTell: ["tower", "attic"] }];
    await renderCard();
    expect(screen.getByText(en["shared.stillToTell"].replace("{names}", "tower, attic"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["shared.takeBack"] })).toBeNull();
  });

  it("says how many keys of checks of other servers its upkeep still has to remove", async () => {
    initial = server;
    holdings = [{ repo: "sr1", proofKeys: 2 }];
    await renderCard();
    expect(screen.getByText(countText(en["shared.proofKeys"], "en", 2))).toBeTruthy();
  });
});
