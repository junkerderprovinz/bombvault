// @vitest-environment jsdom
// The fortress on the Pairing page: its card at the top shows the one phrase
// behind the password with its QR code, changes it and leaves; below, the
// other apps stand in the cards of the Instances tab, with their logos, and
// giving backups back sits under the permission it goes with.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { fortressText, storeShowOthers } from "@glimstone/core";
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
const toasts: string[] = [];
let opened: { ok: boolean; path?: string; code?: string } = { ok: true, path: "/fortress/open/0123abcd" };
let changed = 0;

function state(over: Partial<FortressState> = {}): FortressState {
  return { ok: true, joined: true, phrase: true, defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [], removed: [], removedHere: false, ...over };
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
  opens: true,
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
    openFortressMember: (id: string) => {
      calls.push(`open ${id}`);
      return Promise.resolve(opened);
    },
    fortressMemberStatus: (id: string, lang: string) => {
      calls.push(`status ${id} ${lang}`);
      return Promise.resolve({
        ok: true,
        report: { state: "warning", needsYou: [{ severity: "warn", text: "2 updates waiting" }], work: [], next: [] },
        log: [{ at: 1000, level: "info", text: "ShipLog updated plex" }],
      });
    },
    removeFortressMember: (id: string) => {
      calls.push(`remove ${id}`);
      view = { ...view, members: [], removed: [{ app: "firesquire", name: "FireSquire", device: "Tower" }] };
      return Promise.resolve(view);
    },
  };
});

vi.mock("../../../lib/toast", () => ({ useToast: () => ({ push: (message: string) => toasts.push(message) }) }));

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
      <OtherApps view={view} apps={fortressApps(view.members, String)} group={group()} giveBack={giveBack} showOthers hueStart={4} onChanged={() => changed++} />
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
  toasts.length = 0;
  opened = { ok: true, path: "/fortress/open/0123abcd" };
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
  it("offer no filter while every permission names BombVault, since it would leave nothing out", () => {
    view = state({
      members: [member({})],
      permissions: [{ from: "bombvault", to: "knightloader", action: "backup.export", on: true }],
    });
    renderApps();
    expect(screen.queryByRole("radio", { name: /With BombVault/ })).toBeNull();
  });

  it("filter the permissions to the ones BombVault gives or takes", async () => {
    view = state({
      members: [member({}), member({ id: "s1", app: "shiplog", name: "ShipLog" })],
      permissions: [
        { from: "bombvault", to: "knightloader", action: "backup.export", on: true },
        { from: "shiplog", to: "knightloader", action: "ui.open", on: true },
      ],
    });
    renderApps();
    expect(document.querySelectorAll("[data-permission]")).toHaveLength(2);
    await act(async () => fireEvent.click(screen.getByRole("radio", { name: /With BombVault/ })));
    expect([...document.querySelectorAll("[data-permission]")].map((row) => row.getAttribute("data-permission"))).toEqual([
      "bombvault knightloader backup.export",
    ]);
  });

  it("show both apps' logos beside each permission, BombVault's own among them", () => {
    view = state({
      members: [member({ logo: LOGO })],
      permissions: [{ from: "bombvault", to: "knightloader", action: "backup.export", on: true }],
    });
    renderApps();
    const row = document.querySelector('[data-permission="bombvault knightloader backup.export"]')!;
    expect([...row.querySelectorAll("img")].map((img) => img.getAttribute("src"))).toEqual(["/logo.svg", LOGO]);
  });

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

  it("open an app's own pages in a tab of their own, from anywhere", async () => {
    view = state({ members: [member({ relay: true })] });
    const tabs: unknown[][] = [];
    const spy = vi.spyOn(window, "open").mockImplementation((...args: unknown[]) => {
      tabs.push(args);
      return null;
    });
    renderApps();
    const foot = document.querySelector('[data-instance="app-knightloader"]')!;
    await act(async () => fireEvent.click(within(foot).getByRole("button", { name: en["instances.open"] })));
    expect(calls).toEqual(["open k1"]);
    expect(tabs).toEqual([["/fortress/open/0123abcd", "_blank", "noopener"]]);
    spy.mockRestore();
  });

  it("say why an app did not open", async () => {
    view = state({ members: [member({})] });
    opened = { ok: false, code: "ui_no_login" };
    renderApps();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: en["instances.open"] })));
    expect(toasts).toEqual([fortressText("en").openRefused.ui_no_login]);
    opened = { ok: false, code: "password-not-set" };
    await act(async () => fireEvent.click(screen.getByRole("button", { name: en["instances.open"] })));
    expect(toasts[1]).toBe(en["fortress.openNeedsPassword"]);
  });

  it("show the state of an app without pages of its own, such as a plugin", async () => {
    view = state({ members: [member({ id: "s1", app: "shiplog", name: "ShipLog", deployment: "plugin", opens: false })] });
    renderApps();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: en["instances.open"] })));
    expect(calls).toEqual(["status s1 en"]);
    const status = document.querySelector('[data-glim="FortressStatusWindow"]')!;
    expect(status.textContent).toContain("2 updates waiting");
    expect(status.textContent).toContain("ShipLog updated plex");
  });

  it("ask before opening one of two members that claim the same app on the same server", async () => {
    view = state({ members: [member({ host: "Bottich" }), member({ id: "k2", device: "Bottich", host: "Bottich" })] });
    renderApps();
    const card = document.querySelector('[data-instance="app-knightloader"]')!;
    const [first] = within(card).getAllByRole("button", { name: en["instances.open"] });
    await act(async () => fireEvent.click(first));
    expect(calls).toEqual([]);
    const ask = document.querySelector('[data-glim="FortressDoubtWindow"]')!;
    expect(ask).toBeTruthy();
    await act(async () => fireEvent.click(ask.querySelector('[data-fortress="open-anyway"]')!));
    expect(calls).toEqual(["open k1"]);
  });

  it("give an app its own backup back with one switch, which also lets backups go back at all", async () => {
    view = state({
      members: [member({}), member({ id: "s1", app: "shiplog", name: "ShipLog" })],
      permissions: [
        { from: "bombvault", to: "knightloader", action: "backup.import", on: true },
        { from: "bombvault", to: "shiplog", action: "backup.import", on: false },
      ],
    });
    const flipped: boolean[] = [];
    renderApps({ on: false, onChange: (on) => flipped.push(on) });
    expect(screen.queryByRole("switch", { name: en["appImport.toggle"] })).toBeNull();
    expect(screen.getAllByText(en["appImport.toggleHint"])).toHaveLength(1);
    const toggles = screen.getAllByRole("switch").filter((s) => s.closest("[data-permission]"));
    expect(toggles.map((s) => s.getAttribute("aria-checked"))).toEqual(["false", "false"]);
    await act(async () => fireEvent.click(toggles[0]));
    expect(calls).toEqual([]);
    expect(flipped).toEqual([true]);
    await act(async () => fireEvent.click(toggles[1]));
    expect(calls).toEqual(["permission backup.import true"]);
    expect(flipped).toEqual([true, true]);
  });

  it("keep backups going back while another app still may take its own", async () => {
    view = state({
      members: [member({}), member({ id: "s1", app: "shiplog", name: "ShipLog" })],
      permissions: [
        { from: "bombvault", to: "knightloader", action: "backup.import", on: true },
        { from: "bombvault", to: "shiplog", action: "backup.import", on: true },
      ],
    });
    const flipped: boolean[] = [];
    renderApps({ on: true, onChange: (on) => flipped.push(on) });
    const toggles = screen.getAllByRole("switch").filter((s) => s.closest("[data-permission]"));
    await act(async () => fireEvent.click(toggles[0]));
    expect(calls).toEqual(["permission backup.import false"]);
    expect(flipped).toEqual([]);
  });
});

describe("removing another app from the fortress", () => {
  it("asks first from the app's own card, then offers the new phrase behind the password", async () => {
    view = state({ members: [member({ id: "fs1", app: "firesquire", name: "FireSquire", device: "Tower", deployment: "plugin" })] });
    renderApps();
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(screen.getByRole("dialog", { name: "Remove FireSquire on Tower?" })).toBeTruthy();
    expect(calls).toEqual([]);
    await act(async () => fireEvent.click(button("remove")));
    expect(calls).toEqual(["remove fs1"]);
    expect(changed).toBe(1);
    expect(screen.getByRole("dialog", { name: "Change the phrase now?" })).toBeTruthy();
    await act(async () => fireEvent.click(button("change")));
    fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: "hunter2" } });
    await act(async () => fireEvent.click(button("change-unlock")));
    expect(calls).toEqual(["remove fs1", "change hunter2"]);
    expect(button("words").textContent).toContain("wrong");
  });

  it("names who was removed until the phrase changes", () => {
    view = state({ members: [member({})], removed: [{ app: "firesquire", name: "FireSquire", device: "Tower" }] });
    renderApps();
    expect(button("removed").textContent).toContain("Removed: FireSquire on Tower. No new app can join until you change the phrase.");
  });
});
