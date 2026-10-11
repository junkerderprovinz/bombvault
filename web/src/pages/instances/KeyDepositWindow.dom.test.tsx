// @vitest-environment jsdom
// The Key deposit window is both sides of the deposits on one server. Its own
// side: when the recovery code was made, the way to a new one, and a line per
// keeper with where the deposit stands there. The keeper's side: what this
// server keeps for whom, and the requests to hand a deposit out.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import type { DepositRefusal, Deposits } from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { depositRelease, deposits, keptDeposit, NO_SELF_ROLES, NOW_S, sentDeposit } from "./instances.testsupport";
import type { InstancesState } from "./useInstances";

const NEW_CODE = "B4TXW-8JHQZ-R2MKC-6YPDN";
const SINCE = new Date(2026, 2, 9, 12).toISOString();
const ago = (seconds: number) => new Date((NOW_S - seconds) * 1000).toISOString();

type Refusal = { ok: false; code?: DepositRefusal; error: string };
let refusal: Refusal | null = null;
const sent: string[] = [];
const decided: [string, string][] = [];
const dropped: string[] = [];
const copied: string[] = [];

function answer<T extends object>(call: string, ok: T): Promise<Refusal | ({ ok: true } & T)> {
  sent.push(call);
  return Promise.resolve(refusal ?? { ok: true, ...ok });
}

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    newDepositCode: () => answer("code", { recoveryCode: NEW_CODE }),
    renewDeposits: () => answer("renew", {}),
    askKeeper: (memberId: string) => answer(`ask:${memberId}`, {}),
    decideRelease: (id: string, decision: string) => {
      decided.push([id, decision]);
      return answer("decide", { state: decision === "allow" ? "allowed" : "declined" });
    },
    dropKeptDeposit: (ownerId: string) => {
      dropped.push(ownerId);
      return answer("drop", {});
    },
  };
});

vi.mock("../../lib/clipboard", () => ({
  copyText: (text: string) => {
    copied.push(text);
    return Promise.resolve(true);
  },
}));

const { KeyDepositWindow } = await import("./KeyDepositWindow");

let reloads = 0;
let closed = 0;

async function open(list: Partial<Deposits>) {
  const reload = () => Promise.resolve();
  const state: InstancesState = {
    loading: false,
    errors: [],
    self: null,
    selfRoles: NO_SELF_ROLES,
    receiverServer: false,
    deposits: deposits(list),
    data: { members: [], peers: [], received: [], pulls: [], offers: [], roles: [], destinations: [], groupReceivers: [], replicas: [] },
    reloadPeers: reload,
    reloadOffers: reload,
    reloadReceived: reload,
    reloadPulls: reload,
    reloadRoles: reload,
    reloadDeposits: () => {
      reloads++;
      return Promise.resolve();
    },
    reloadReplicas: reload,
    reloadServer: reload,
  };
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <KeyDepositWindow state={state} onClose={() => closed++} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
  return screen.getByRole("dialog", { name: en["keeper.window.title"] });
}

async function click(el: Element) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const own = () => screen.getByRole("region", { name: en["keeper.own.title"] });
const keeps = () => screen.getByRole("region", { name: en["keeper.kept.title"] });
const keeper = (id: string) => own().querySelector(`[data-keeper="${id}"]`) as HTMLElement;
const request = (id: string) => keeps().querySelector(`[data-release="${id}"]`) as HTMLElement;
const kept = (id: string) => keeps().querySelector(`[data-kept="${id}"]`) as HTMLElement;
const question = () => screen.getByRole("dialog", { name: en["confirmDialog.title"] });

beforeEach(() => {
  refusal = null;
  sent.length = 0;
  decided.length = 0;
  dropped.length = 0;
  copied.length = 0;
  reloads = 0;
  closed = 0;
});

afterEach(cleanup);

describe("this server's own deposit", () => {
  it("says how to make one while there is none", async () => {
    await open({});
    expect(own().textContent).toContain(en["keeper.own.none"]);
    expect(within(own()).queryByRole("button")).toBeNull();
  });

  it("says when the recovery code was made, and explains the deposit in the heading", async () => {
    const win = await open({ codeSince: SINCE });
    expect(within(own()).getByTestId("code-since").textContent).toBe(`Recovery code made on ${new Date(SINCE).toLocaleDateString()}`);
    expect(within(win.parentElement as HTMLElement).getByLabelText(en["keeper.window.tip"])).toBeTruthy();
  });

  it("makes a new code only after saying that the old one stops working, and shows it once", async () => {
    await open({ codeSince: SINCE });
    await click(within(own()).getByRole("button", { name: en["keeper.own.newCode"] }));
    expect(question().textContent).toContain(en["keeper.own.newCodeAsk"]);
    expect(sent).toEqual([]);
    await click(within(question()).getByRole("button", { name: en["keeper.own.newCode"] }));
    expect(sent).toEqual(["code"]);
    const shown = screen.getByRole("dialog", { name: en["keeper.code.title"] });
    expect([...within(shown).getByTestId("recovery-code").children].map((li) => li.textContent)).toEqual(NEW_CODE.split("-"));
    const done = within(shown).getByRole("button", { name: en["common.done"] }) as HTMLButtonElement;
    expect(done.disabled).toBe(true);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.getByRole("dialog", { name: en["keeper.code.title"] })).toBeTruthy();
    expect(closed).toBe(0);
    await click(within(shown).getByRole("switch", { name: en["keeper.code.written"] }));
    await click(done);
    expect(screen.queryByRole("dialog", { name: en["keeper.code.title"] })).toBeNull();
    expect(document.body.textContent).not.toContain("B4TXW");
    expect(reloads).toBe(1);
  });

  it("keeps the code it has when the question is cancelled", async () => {
    await open({ codeSince: SINCE });
    await click(within(own()).getByRole("button", { name: en["keeper.own.newCode"] }));
    await click(within(question()).getByRole("button", { name: en["common.cancel"] }));
    expect(sent).toEqual([]);
    expect(screen.queryByRole("dialog", { name: en["keeper.code.title"] })).toBeNull();
  });

  it("copies the whole code, hyphens included", async () => {
    await open({ codeSince: SINCE });
    await click(within(own()).getByRole("button", { name: en["keeper.own.newCode"] }));
    await click(within(question()).getByRole("button", { name: en["keeper.own.newCode"] }));
    const shown = screen.getByRole("dialog", { name: en["keeper.code.title"] });
    await click(within(shown).getByRole("button", { name: en["common.copy"] }));
    expect(copied).toEqual([NEW_CODE]);
    expect(within(shown).getByRole("button", { name: en["common.copied"] })).toBeTruthy();
  });

  it("lists every keeper with where the deposit stands there and when it was sent", async () => {
    await open({
      codeSince: SINCE,
      sent: [
        sentDeposit({ sentAt: ago(2 * 3600) }),
        sentDeposit({ memberId: "m2", name: "barn", state: "stale", sentAt: ago(3 * 86400) }),
        sentDeposit({ memberId: "m3", name: "", state: "declined", sentAt: ago(40 * 86400) }),
      ],
    });
    expect(keeper("m1").textContent).toBe(`attic${en["instances.role.on"]}sent 2 hours ago`);
    expect(keeper("m1").querySelector("button")).toBeNull();
    expect(keeper("m2").textContent).toContain(`barn${en["keeper.state.stale"]}sent 3 days ago`);
    expect(keeper("m3").textContent).toContain(`m3${en["keeper.state.declined"]}`);
  });

  it("says what an outdated deposit means, and sends the current one on request", async () => {
    await open({ codeSince: SINCE, sent: [sentDeposit({ memberId: "m2", name: "barn", state: "stale" })] });
    expect(keeper("m2").textContent).toContain(
      "barn still holds an older deposit. If you made a new code since, only the old code opens it. This server sends the current one as soon as it reaches barn.",
    );
    await click(within(keeper("m2")).getByRole("button", { name: en["keeper.sendNow"] }));
    expect(sent).toEqual(["renew"]);
    expect(reloads).toBe(1);
  });

  it("says that the keeper holds something this server did not send, and sends the deposit again", async () => {
    await open({ codeSince: SINCE, sent: [sentDeposit({ memberId: "m2", name: "barn", state: "changed" })] });
    expect(keeper("m2").textContent).toContain(en["keeper.state.changed"]);
    expect(keeper("m2").textContent).toContain(en["keeper.changedText"].replaceAll("{peer}", "barn"));
    await click(within(keeper("m2")).getByRole("button", { name: en["keeper.sendNow"] }));
    expect(sent).toEqual(["renew"]);
  });

  it("says what a deposit deleted there means, and asks the keeper again on request", async () => {
    await open({ codeSince: SINCE, sent: [sentDeposit({ memberId: "m3", name: "shed", state: "declined" })] });
    expect(keeper("m3").textContent).toContain("The deposit was deleted on shed. shed takes a new one only when you ask again.");
    await click(within(keeper("m3")).getByRole("button", { name: en["keeper.askAgain"] }));
    expect(sent).toEqual(["ask:m3"]);
  });

  it("shows a state it does not know in the server's word, as a warning, with the way to send again", async () => {
    await open({ codeSince: SINCE, sent: [sentDeposit({ state: "someday" as "held", sentAt: ago(3600) })] });
    expect(keeper("m1").textContent).toBe(`atticsomedaysent 1 hour ago${en["keeper.sendNow"]}`);
    expect(within(keeper("m1")).getByText("someday").className).toContain("statusWarn");
    await click(within(keeper("m1")).getByRole("button", { name: en["keeper.sendNow"] }));
    expect(sent).toEqual(["renew"]);
  });

  it("says in the server's words why it may not make a new code", async () => {
    refusal = { ok: false, error: "set a login password before making a new recovery code" };
    await open({ codeSince: SINCE });
    await click(within(own()).getByRole("button", { name: en["keeper.own.newCode"] }));
    await click(within(question()).getByRole("button", { name: en["keeper.own.newCode"] }));
    expect(screen.getByText("set a login password before making a new recovery code")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: en["keeper.code.title"] })).toBeNull();
  });

  it("says why sending did not work", async () => {
    refusal = { ok: false, code: "member-gone", error: "the server's own words" };
    await open({ codeSince: SINCE, sent: [sentDeposit({ memberId: "m2", name: "barn", state: "stale" })] });
    await click(within(keeper("m2")).getByRole("button", { name: en["keeper.sendNow"] }));
    expect(screen.getByText(en["deposit.code.member-gone"])).toBeTruthy();
  });
});

describe("what this server keeps for others", () => {
  it("says so while it keeps nothing", async () => {
    await open({ codeSince: SINCE });
    expect(keeps().textContent).toContain(en["keeper.kept.none"]);
  });

  it("names whose deposit it keeps, since when, and when it was last renewed", async () => {
    await open({ kept: [keptDeposit({ createdAt: ago(30 * 86400), updatedAt: ago(3600) })] });
    expect(kept("m1").textContent).toBe(`Key deposit of atticfirst received 30 days ago · last renewed 1 hour ago${en["common.delete"]}`);
    expect(keeps().textContent).not.toContain(en["keeper.kept.none"]);
  });

  it("deletes a deposit after a question in the house form, with no red on it", async () => {
    await open({ kept: [keptDeposit()] });
    await click(within(kept("m1")).getByRole("button", { name: en["common.delete"] }));
    expect(question().textContent).toContain(
      "Delete the key deposit of attic? A new install can then no longer get attic's key from this server. attic learns of it at its next renewal and has to ask again.",
    );
    expect(dropped).toEqual([]);
    const confirm = within(question()).getByRole("button", { name: en["common.delete"] });
    expect(confirm.className).not.toMatch(/statusFail|bg-accent/);
    await click(confirm);
    expect(dropped).toEqual(["m1"]);
    expect(screen.getByText(en["keeper.kept.deleted"])).toBeTruthy();
    expect(reloads).toBe(1);
  });

  it("keeps a deposit when the question is cancelled", async () => {
    await open({ kept: [keptDeposit()] });
    await click(within(kept("m1")).getByRole("button", { name: en["common.delete"] }));
    await click(within(question()).getByRole("button", { name: en["common.cancel"] }));
    expect(dropped).toEqual([]);
  });

  it("still lists a deposit it deleted, with what the owner has to do and nothing left to delete", async () => {
    await open({ kept: [keptDeposit({ state: "declined" })] });
    expect(kept("m1").textContent).toBe(
      "Key deposit of atticDeleted on this server. attic has to ask again before this server keeps a new one.",
    );
    expect(within(kept("m1")).queryByRole("button")).toBeNull();
  });
});

describe("a request to hand a deposit out", () => {
  it("warns of an instance never seen here, and says how often it asked", async () => {
    await open({ kept: [keptDeposit()], releases: [depositRelease({ requests: 3, requestsLastHour: 2 })] });
    const row = request("rel1");
    expect(row.textContent).toContain("new-attic asks for the key deposit of attic.");
    const unknown = within(row).getByText(en["keeper.release.unknown"]);
    expect(unknown.tagName).toBe("SPAN");
    expect(unknown.className).toContain("statusWarn");
    expect(row.textContent).not.toContain("First seen here");
    expect(row.textContent).toContain(`${countText(en["keeper.release.count"], "en", 3).replace("{n}", "3")}, 2 in the last hour`);
    expect(row.textContent).toContain(en["instances.request"]);
  });

  it("says since when it has known an instance it has seen before", async () => {
    await open({ releases: [depositRelease({ knownSince: SINCE, requests: 1, requestsLastHour: 1 })] });
    const row = request("rel1");
    expect(row.textContent).toContain(`First seen here on ${new Date(SINCE).toLocaleDateString()}.`);
    expect(within(row).queryByText(en["keeper.release.unknown"])).toBeNull();
    expect(row.textContent).toContain("1 request from it so far, 1 in the last hour");
  });

  it("says in one sentence what Allow hands out", async () => {
    await open({ releases: [depositRelease()] });
    expect(request("rel1").textContent).toContain("Allow hands out a sealed copy. Only the recovery code opens it.");
  });

  it("allows it, and says how long the instance has to collect the deposit", async () => {
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(decided).toEqual([["rel1", "allow"]]);
    expect(screen.getByText("Allowed. new-attic can collect the deposit within 30 minutes.")).toBeTruthy();
    expect(reloads).toBe(1);
  });

  it("refuses it", async () => {
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.decline"] }));
    expect(decided).toEqual([["rel1", "refuse"]]);
    expect(screen.getByText("new-attic gets no deposit.")).toBeTruthy();
  });

  it("says so when the request was answered in the meantime, and reads the list again", async () => {
    refusal = { ok: false, code: "release-decided", error: "the server's own words" };
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.getByText(en["deposit.code.release-decided"])).toBeTruthy();
    expect(reloads).toBe(1);
  });

  it("asks for a sign-in at this instance before the phone app's session allows one", async () => {
    refusal = { ok: false, code: "own-login-needed", error: "sign in at this instance's web interface" };
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.allow"] }));
    const ask = screen.getAllByRole("dialog").at(-1)!;
    expect(ask.textContent).toContain(en["secrets.ownLoginNeeded"]);
    expect(within(ask).getByRole("button", { name: en["secrets.signInHere"] })).toBeTruthy();
    expect(screen.queryByText("sign in at this instance's web interface")).toBeNull();
  });

  it("says that a login password is needed before it allows one", async () => {
    refusal = { ok: false, code: "password-not-set", error: "set a login password before handing out a key deposit" };
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.getByText(en["secrets.passwordNotSet"])).toBeTruthy();
  });

  it("says in the server's words why it may not allow one", async () => {
    refusal = { ok: false, error: "set a login password before handing out a key deposit" };
    await open({ releases: [depositRelease()] });
    await click(within(request("rel1")).getByRole("button", { name: en["zfs.receive.allow"] }));
    expect(screen.getByText("set a login password before handing out a key deposit")).toBeTruthy();
    expect(screen.queryByText(/can collect the deposit/)).toBeNull();
  });

  it("names an instance by its id when it has no name, and the owner too", async () => {
    await open({ releases: [depositRelease({ requesterName: "", ownerName: "" })] });
    expect(request("rel1").textContent).toContain("m9 asks for the key deposit of m1.");
  });

  it("shows only the requests that still wait", async () => {
    await open({
      releases: [
        depositRelease(),
        depositRelease({ id: "rel2", state: "allowed" }),
        depositRelease({ id: "rel3", state: "declined" }),
        depositRelease({ id: "rel4", state: "released" }),
        depositRelease({ id: "rel5", state: "expired" }),
      ],
    });
    expect([...keeps().querySelectorAll("[data-release]")].map((el) => el.getAttribute("data-release"))).toEqual(["rel1"]);
  });
});
