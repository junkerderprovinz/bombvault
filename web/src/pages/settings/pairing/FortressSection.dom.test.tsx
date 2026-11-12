// @vitest-environment jsdom
// The fortress on the Pairing page: its card at the top shows the one phrase
// behind the password with its QR code, changes it and leaves; below, the
// other apps stand in the cards of the Instances tab, with their logos, and
// giving backups back sits under the permission it goes with.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { storeShowOthers } from "@glimstone/core";
import jsQR from "jsqr";
import { I18nProvider, en } from "../../../lib/i18n";
import type { FortressMember, FortressState, GroupState } from "../../../lib/api";
import { fortressApps } from "./fortressModel";
import { FortressBlock, OtherApps, type GiveBack } from "./FortressSection";

const PHRASE = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
const NEW_PHRASE = "zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong";
const LOGO = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4=";

let view: FortressState;
const calls: string[] = [];
let changed = 0;

function state(over: Partial<FortressState> = {}): FortressState {
  return { ok: true, joined: true, phrase: true, defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [], ...over };
}

function group(over: Partial<GroupState> = {}): GroupState {
  return {
    ok: true,
    active: true,
    instanceId: "self",
    name: "cellar",
    passwordSet: true,
    members: [],
    away: [],
    settling: false,
    relay: { mode: "project", url: "", projectUrl: "wss://parleyport.halleluja.design/relay/connect", connected: true, serve: false, serveClients: 0 },
    joinedAgo: 300,
    memberSeen: true,
    waiting: [],
    selfAddress: "",
    ownAddresses: [],
    ...over,
  };
}

const member = (over: Partial<FortressMember>): FortressMember => ({
  id: "k1",
  app: "knightloader",
  name: "KnightLoader",
  version: "2.1.0",
  device: "Bottich",
  deployment: "container",
  kind: "instance",
  online: true,
  relay: false,
  ...over,
});

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    showFortressPhrase: (password: string) =>
      Promise.resolve(password === "hunter2" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong", error: "wrong" }),
    showPhrase: (password: string) => {
      calls.push("group show");
      return Promise.resolve(password === "hunter2" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong" });
    },
    changeFortressPhrase: (password: string) => {
      calls.push(`change ${password}`);
      if (view.passwordSet && password !== "hunter2") return Promise.resolve({ ok: false, code: "passwordWrong", error: "wrong" });
      return Promise.resolve({ ok: true, phrase: NEW_PHRASE, left: ["Cellar"] });
    },
    leaveFortress: () => {
      calls.push("leave");
      return Promise.resolve(state({ joined: false, phrase: false }));
    },
    setFortressPermission: (p: { action: string; on: boolean }) => {
      calls.push(`permission ${p.action} ${p.on}`);
      return Promise.resolve({ ok: true });
    },
  };
});

vi.mock("../../../lib/toast", () => ({ useToast: () => ({ push: () => {} }) }));

function renderBlock(g: GroupState = group()) {
  return render(
    <I18nProvider>
      <FortressBlock view={view} group={g} onChanged={() => changed++} hueIndex={3}>
        <p data-testid="own-row">name</p>
      </FortressBlock>
    </I18nProvider>,
  );
}

function renderApps(giveBack?: GiveBack) {
  return render(
    <I18nProvider>
      <OtherApps view={view} apps={fortressApps(view.members, String)} group={group()} giveBack={giveBack} hueStart={4} onChanged={() => changed++} />
    </I18nProvider>,
  );
}

const button = (hook: string) => document.querySelector<HTMLButtonElement>(`[data-fortress="${hook}"]`)!;

async function showWith(password: string) {
  await act(async () => fireEvent.click(button("show")));
  fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: password } });
  await act(async () => fireEvent.click(button("unlock")));
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  storeShowOthers(true);
  view = state();
  calls.length = 0;
  changed = 0;
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("the card of the phrase", () => {
  it("holds the page's own rows above Show phrase and Change in one row, and Leave", () => {
    renderBlock();
    const card = document.querySelector('[data-glim="FortressPairing"]')!;
    expect(card.querySelector('[data-testid="own-row"]')).toBeTruthy();
    const rows = card.querySelectorAll(".glim-field-row");
    expect([...rows[0].querySelectorAll("button")].map((b) => b.textContent)).toEqual(["Show phrase", "Change"]);
    expect(rows[1].querySelector('[data-fortress="open-leave"]')).toBeTruthy();
  });

  it("shows the phrase with its QR code only after the password", async () => {
    renderBlock();
    await showWith("wrong");
    expect((await screen.findByRole("alert")).textContent).toBe("That password is not right.");
    fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: "hunter2" } });
    await act(async () => fireEvent.click(button("unlock")));
    expect(button("words").querySelectorAll("li")).toHaveLength(12);
    expect(document.querySelector('[data-fortress="qr"]')).toBeTruthy();
    expect(screen.getByText(en["pairing.qrCaption"])).toBeTruthy();
  });

  it("puts an own relay's address into the code under the words", async () => {
    renderBlock(group({ relay: { ...group().relay, mode: "own", url: "wss://relay.example/relay/connect" } }));
    await showWith("hunter2");
    const svg = document.querySelector<SVGSVGElement>('[data-fortress="qr"]')!;
    const path = svg.querySelector("path")!.getAttribute("d")!;
    const side = Number(svg.getAttribute("viewBox")!.split(" ")[2]) - 8;
    const scale = 4;
    const size = (side + 8) * scale;
    const pixels = new Uint8ClampedArray(size * size * 4).fill(255);
    for (const [, x, y, w] of path.matchAll(/M(\d+) (\d+)h(\d+)/g)) {
      for (let px = Number(x); px < Number(x) + Number(w); px++) {
        for (let dy = 0; dy < scale; dy++) {
          for (let dx = 0; dx < scale; dx++) {
            const at = ((Number(y) + 4) * scale + dy) * size + (px + 4) * scale + dx;
            pixels.fill(0, at * 4, at * 4 + 3);
          }
        }
      }
    }
    expect(jsQR(pixels, size, size)?.data).toBe(`${PHRASE}\nwss://relay.example/relay/connect`);
  });

  it("says a phone needs a relay beside the code when none is in use", async () => {
    renderBlock(group({ relay: { ...group().relay, mode: "off", connected: false } }));
    await showWith("hunter2");
    expect(screen.getByText(en["pairing.phoneNeedsRelay"])).toBeTruthy();
  });

  it("shows the group's phrase where the fortress was never opened on it", async () => {
    view = state({ joined: false, phrase: false });
    renderBlock();
    await showWith("hunter2");
    expect(calls).toEqual(["group show"]);
    expect(button("open-change")).toBeNull();
  });

  it("asks for the password before the phrase changes, and a wrong one changes nothing", async () => {
    renderBlock();
    fireEvent.click(button("open-change"));
    await act(async () => fireEvent.click(button("change")));
    expect(calls).toEqual([]);
    fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: "wrong" } });
    await act(async () => fireEvent.click(button("change-unlock")));
    expect(calls).toEqual(["change wrong"]);
    expect((await screen.findByRole("alert")).textContent).toBe("That password is not right.");
    expect(document.querySelector('[data-fortress="words"]')).toBeNull();
  });

  it("keeps the phrase when the password window closes", async () => {
    renderBlock();
    fireEvent.click(button("open-change"));
    await act(async () => fireEvent.click(button("change")));
    await act(async () => fireEvent.keyDown(document.activeElement ?? document.body, { key: "Escape" }));
    expect(calls).toEqual([]);
    expect((await screen.findByRole("alert")).textContent).toBe(en["fortress.changeKept"]);
  });

  it("changes the phrase without asking where BombVault has no password", async () => {
    view = state({ passwordSet: false });
    renderBlock();
    fireEvent.click(button("open-change"));
    await act(async () => fireEvent.click(button("change")));
    expect(calls).toEqual(["change "]);
    expect(button("words").textContent).toContain("wrong");
    expect(changed).toBe(1);
  });

  it("changes the phrase for the password, names who missed it, and leaves", async () => {
    renderBlock();
    fireEvent.click(button("open-change"));
    expect(screen.getByText(en["fortress.changeAway"])).toBeTruthy();
    await act(async () => fireEvent.click(button("change")));
    fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: "hunter2" } });
    await act(async () => fireEvent.click(button("change-unlock")));
    expect(button("words").textContent).toContain("wrong");
    expect(screen.getByText(en["fortress.changedLeft"].replace("{devices}", "Cellar"))).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    fireEvent.click(button("open-leave"));
    await act(async () => fireEvent.click(button("leave")));
    expect(calls).toEqual(["change hunter2", "leave"]);
    expect(changed).toBe(2);
  });

  it("offers neither showing nor changing a phrase BombVault does not keep", () => {
    view = state({ phrase: false });
    renderBlock(group({ active: false }));
    expect(screen.getByText(en["fortress.noPhraseKept"])).toBeTruthy();
    expect(button("show")).toBeNull();
    expect(button("open-change")).toBeNull();
    expect(button("open-leave")).toBeTruthy();
  });
});

describe("the other apps", () => {
  it("stand in the cards of the Instances tab with their logos, and the corner holds no second Show phrase", () => {
    view = state({
      members: [member({ logo: LOGO }), member({ id: "t1", app: "tollroll", name: "TollRoll", deployment: "plugin", online: false })],
    });
    renderApps();
    const knight = document.querySelector('[data-instance="app-knightloader"]')!;
    expect(knight.querySelector("img")?.getAttribute("src")).toBe(LOGO);
    expect(knight.textContent).toContain("Connected");
    const toll = document.querySelector('[data-instance="app-tollroll"]')!;
    expect(toll.querySelector("img")).toBeNull();
    expect(toll.querySelector("svg")).toBeTruthy();
    expect(toll.textContent).toContain("Not reachable");
    expect(document.querySelector("[data-fortress-app]")).toBeNull();
    expect(document.querySelector(".glim-fortress-actions")).toBeNull();
    expect(button("show")).toBeNull();
  });

  it("word giving an app its own backup back, with the switch for it under that permission alone", () => {
    view = state({
      members: [member({}), member({ id: "s1", app: "shiplog", name: "ShipLog" })],
      permissions: [
        { from: "bombvault", to: "knightloader", action: "backup.import", on: true },
        { from: "bombvault", to: "shiplog", action: "backup.import", on: true },
      ],
    });
    const flipped: boolean[] = [];
    renderApps({ on: false, onChange: (on) => flipped.push(on) });
    expect(screen.getAllByText(en["fortress.action.backupImport"])).toHaveLength(2);
    const toggles = screen.getAllByRole("switch", { name: en["appImport.toggle"] });
    expect(toggles).toHaveLength(1);
    fireEvent.click(toggles[0]);
    expect(flipped).toEqual([true]);
  });
});
