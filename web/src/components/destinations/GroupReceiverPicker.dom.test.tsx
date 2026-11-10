// @vitest-environment jsdom
// "From your group" offers the shared repositories the group's members hold
// beside their receivers. Picking one asks its owner to let this instance in
// and closes the wizard, since the location only follows once the owner says
// yes. One already asked for or joined is not offered a second time.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { GroupReceiver, SharedOffer, SharedMembershipState } from "../../lib/api";

let receivers: GroupReceiver[] = [];
let offers: SharedOffer[] = [];
let answer: SharedMembershipState = "asked";
const joined: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listGroupReceivers: () => Promise.resolve({ ok: true, receivers }),
    listSharedOffers: () => Promise.resolve({ ok: true, offers }),
    joinSharedRepo: (ownerId: string) => {
      joined.push(ownerId);
      return Promise.resolve({ ok: true, state: answer });
    },
  };
});

const { GroupReceiverPicker } = await import("./GroupReceiverPicker");

const picked: string[] = [];
let asked = 0;

async function renderPicker() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <GroupReceiverPicker onPick={(r) => picked.push(r.memberId)} onAsked={() => asked++} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

const tower: SharedOffer = { memberId: "m-tower", name: "tower", url: "http://192.168.1.20:8001/shared", needsAddress: false };
const tile = (name: string) => en["shared.offer.tile"].replace("{name}", name);

afterEach(() => {
  cleanup();
  receivers = [];
  offers = [];
  answer = "asked";
  joined.length = 0;
  picked.length = 0;
  asked = 0;
});

describe("GroupReceiverPicker", () => {
  it("shows a member's shared repository as a tile of its own beside its receiver", async () => {
    receivers = [{ memberId: "m-tower", name: "tower", url: "http://192.168.1.20:8000", needsAddress: false }];
    offers = [tower];
    await renderPicker();
    expect(screen.getByText(en["dest.group.fromGroup"])).toBeTruthy();
    expect(screen.getByRole("button", { name: /^tower/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: new RegExp(`^${tile("tower")}`) })).toBeTruthy();
  });

  it("asks the owner to let this instance in and closes, instead of filling the form", async () => {
    offers = [tower];
    await renderPicker();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: new RegExp(`^${tile("tower")}`) }));
    });
    expect(joined).toEqual(["m-tower"]);
    expect(picked).toEqual([]);
    expect(asked).toBe(1);
    expect(await screen.findByText(en["shared.offer.asked"].replace("{name}", "tower"))).toBeTruthy();
  });

  it("says so when the owner had let this instance in already", async () => {
    offers = [tower];
    answer = "joined";
    await renderPicker();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: new RegExp(`^${tile("tower")}`) }));
    });
    expect(await screen.findByText(en["shared.offer.joined"].replace("{name}", "tower"))).toBeTruthy();
  });

  it("does not ask twice for a repository asked for or joined already", async () => {
    offers = [
      { ...tower, state: "asked" },
      { memberId: "m-barn", name: "barn", url: "http://192.168.1.21:8001/shared", needsAddress: false, state: "joined" },
    ];
    await renderPicker();
    expect(screen.queryByRole("button", { name: new RegExp(`^${tile("tower")}`) })).toBeNull();
    expect(screen.queryByRole("button", { name: new RegExp(`^${tile("barn")}`) })).toBeNull();
    expect(screen.getByText(en["shared.member.asked"])).toBeTruthy();
    expect(screen.getByText(en["shared.member.joined"])).toBeTruthy();
    expect(joined).toEqual([]);
  });

  it("shows nothing when the group offers neither", async () => {
    await renderPicker();
    expect(screen.queryByText(en["dest.group.fromGroup"])).toBeNull();
  });
});
