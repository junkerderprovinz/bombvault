// @vitest-environment jsdom
// The shared repositories this instance copies into or asked to join: each
// says where it stands, that its owner prunes it and who else is in, points a
// new member to Storage locations, and leaves only after a question.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { SharedMembership } from "../../lib/api";

let memberships: SharedMembership[] = [];
const left: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSharedRepo: () => Promise.resolve({ ok: true, server: null, defaultPort: 8001, memberships }),
    leaveSharedRepo: (ownerId: string) => {
      left.push(ownerId);
      memberships = memberships.map((m) => (m.ownerId === ownerId ? { ...m, state: "removed" } : m));
      return Promise.resolve({ ok: true });
    },
  };
});

const { SharedMembershipsCard } = await import("./SharedMembershipsCard");
const { useSharedRepo } = await import("./useSharedRepo");

function Member() {
  const shared = useSharedRepo(true);
  return shared.memberships.length > 0 ? <SharedMembershipsCard shared={shared} hueIndex={2} /> : null;
}

function Where() {
  return <span data-testid="where">{useLocation().pathname}</span>;
}

async function renderCard() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/instances"]}>
        <I18nProvider>
          <ToastProvider>
            <Routes>
              <Route path="*" element={<Member />} />
            </Routes>
            <Where />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
  });
}

const named = (key: keyof typeof en, name: string) => en[key].replaceAll("{name}", name);

function membership(over: Partial<SharedMembership> = {}): SharedMembership {
  return {
    ownerId: "m-tower",
    ownerName: "tower",
    state: "joined",
    repo: "http://192.168.1.20:8001/shared",
    askedAt: 1,
    joinedAt: 2,
    together: [
      { memberId: "m-attic", name: "attic", host: "bv-attic" },
      { memberId: "m-barn", name: "barn", host: "bv-barn" },
    ],
    ...over,
  };
}

afterEach(() => {
  cleanup();
  memberships = [];
  left.length = 0;
});

describe("SharedMembershipsCard", () => {
  it("names the owner as the one who prunes and the other members it shares with", async () => {
    memberships = [membership()];
    await renderCard();
    const card = screen.getByRole("region", { name: "tower" });
    expect(within(card).getByText(named("shared.offer.tile", "tower"))).toBeTruthy();
    expect(within(card).getByText(en["shared.member.joined"])).toBeTruthy();
    expect(within(card).getByText(named("shared.member.prunesValue", "tower"))).toBeTruthy();
    expect(within(card).getByText("attic, barn")).toBeTruthy();
    expect(within(card).getByText(en["shared.member.readable"])).toBeTruthy();
  });

  it("asks a new member to switch its sections on and leads it to Storage locations", async () => {
    memberships = [membership()];
    await renderCard();
    expect(screen.getByText(en["shared.member.switchOn"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.member.openStorage"] }));
    });
    expect(screen.getByTestId("where").textContent).toBe("/storage");
  });

  it("says where an ask stands and that nobody else is in yet", async () => {
    memberships = [membership({ state: "asked", repo: undefined, together: [] })];
    await renderCard();
    expect(screen.getByText(en["shared.member.asked"])).toBeTruthy();
    expect(screen.getByText(named("shared.member.askedNote", "tower"))).toBeTruthy();
    expect(screen.getByText(en["shared.member.alone"])).toBeTruthy();
    expect(screen.queryByText(en["shared.member.switchOn"])).toBeNull();
  });

  it("offers no way out of a repository this server is not in", async () => {
    memberships = [membership({ state: "refused", together: [] })];
    await renderCard();
    expect(screen.getByText(named("shared.member.refusedNote", "tower"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["shared.member.leave"] })).toBeNull();
  });

  it("says a join expired and that this server may ask again", async () => {
    memberships = [membership({ state: "expired", repo: undefined, together: [] })];
    await renderCard();
    expect(screen.getByText(en["shared.member.expired"])).toBeTruthy();
    expect(screen.getByText(named("shared.member.expiredNote", "tower"))).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["shared.member.leave"] })).toBeNull();
  });

  it("leaves only after a question, and then shows the repository as left", async () => {
    memberships = [membership()];
    await renderCard();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.member.leave"] }));
    });
    expect(left).toEqual([]);
    expect(screen.getByText(named("shared.member.leaveAsk", "tower"))).toBeTruthy();

    const buttons = screen.getAllByRole("button", { name: en["shared.member.leave"] });
    await act(async () => {
      fireEvent.click(buttons[buttons.length - 1]);
    });
    expect(left).toEqual(["m-tower"]);
    expect(await screen.findByText(named("shared.member.left", "tower"))).toBeTruthy();
    expect(screen.getByText(en["shared.member.removed"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["shared.member.leave"] })).toBeNull();
  });

  it("says when the last copy went through and that a later one waited for the owner", async () => {
    const now = Math.floor(Date.now() / 1000);
    memberships = [membership({ lastCopyAt: now - 7200, deferredAt: now - 60 })];
    await renderCard();
    const card = screen.getByRole("region", { name: "tower" });
    expect(within(card).getByText(en["shared.member.lastCopy"])).toBeTruthy();
    expect(within(card).getByText(named("shared.member.copyWaited", "tower"))).toBeTruthy();
  });

  it("shows what the owner measured and when it tidies up next", async () => {
    memberships = [
      membership({
        space: { at: 300, total: 1200, hosts: [{ host: "bv-attic", name: "attic", bytes: 1000 }, { host: "bv-me", name: "me", bytes: 900 }] },
        nextWindow: 1_900_000_000,
      }),
    ];
    await renderCard();
    const card = screen.getByRole("region", { name: "tower" });
    expect(within(card).getByText(/^Holds 1\.2 KB\./)).toBeTruthy();
    expect(within(card).getByText(en["shared.member.window"])).toBeTruthy();
    expect(within(card).queryByText(en["shared.member.windowUnknown"])).toBeNull();
  });

  it("says the owner has not measured nor named its window yet", async () => {
    memberships = [membership()];
    await renderCard();
    expect(screen.getByText(en["sharedUpkeep.notMeasured"])).toBeTruthy();
    expect(screen.getByText(en["shared.member.windowUnknown"])).toBeTruthy();
  });

  it("says the new holder named an address this server has not followed, and why", async () => {
    memberships = [
      membership({
        move: { holder: "cube", repo: "rest:http://192.168.1.30:8001/shared", reason: "the repository at the new address is another one", at: 3 },
      }),
    ];
    await renderCard();
    const card = screen.getByRole("region", { name: "tower" });
    expect(within(card).getByText(named("shared.member.moveHeld", "cube"))).toBeTruthy();
    expect(within(card).getByText(en["shared.member.moveAddress"])).toBeTruthy();
    expect(within(card).getByText("rest:http://192.168.1.30:8001/shared")).toBeTruthy();
    expect(within(card).getByText("the repository at the new address is another one")).toBeTruthy();
  });

  it("names no new address while the holder named none it could use", async () => {
    memberships = [membership({ move: { holder: "cube", repo: "", reason: "no usable address", at: 3 } })];
    await renderCard();
    expect(screen.getByText(named("shared.member.moveHeld", "cube"))).toBeTruthy();
    expect(screen.queryByText(en["shared.member.moveAddress"])).toBeNull();
  });

  it("says no copy went through yet", async () => {
    memberships = [membership()];
    await renderCard();
    expect(screen.getByText(en["shared.member.noCopy"])).toBeTruthy();
    expect(screen.queryByText(named("shared.member.copyWaited", "tower"))).toBeNull();
    expect(screen.queryByText(named("shared.member.moveHeld", "tower"))).toBeNull();
  });

  it("stays in when the question is cancelled", async () => {
    memberships = [membership()];
    await renderCard();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["shared.member.leave"] }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.cancel"] }));
    });
    expect(left).toEqual([]);
    expect(screen.getByText(en["shared.member.joined"])).toBeTruthy();
  });
});
