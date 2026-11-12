// @vitest-environment jsdom
// The Pairing page of Settings: three numbered step cards that play one
// scene, the one card of the phrase, the devices that wait, the members as
// the cards of the Instances tab, the other apps of the fortress, the relay
// card with its four routes and the ZFS servers. Outside a group the phrase
// card offers two tiles, generate or enter a phrase, each opening a window;
// the entry window checks typed words against the word list as they arrive.
// On a phrase the card is the fortress's, with the name of this instance,
// Show phrase, Change and Leave, and after a minute alone it offers to enter
// the other instance's words instead.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import { I18nProvider, countText, en, useT } from "../../../lib/i18n";
import type { FortressState, GroupState, MemberRoles, RelayMode, ZFSReplicaServer } from "../../../lib/api";
import { server } from "../../../components/zfs/replica/replica.testsupport";
import { memberRoles, roleRequest, roleSide } from "../../instances/instances.testsupport";
import { checkPhrase, splitPhrase } from "../../../lib/phraseWords";
import { LOGIN_PASSWORD_FIELD } from "../shared";
import { storeShowOthers } from "@glimstone/core";

let group: GroupState;
let fortress: FortressState;
let zfs: boolean;
let roles: MemberRoles[];
let servers: ZFSReplicaServer[];
const relayCalls: Array<Record<string, unknown>> = [];
// While set, a relay save waits here for the test to answer it.
let heldRelay: Array<() => void> | null = null;
const joined: string[] = [];
let joinAnswer: Record<string, unknown> = {};
let left = 0;
const decisions: string[] = [];
const removed: string[] = [];

// The BIP39 test vector: twelve listed words that check out.
const PHRASE = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";

function makeGroup(over: Partial<GroupState> = {}): GroupState {
  return {
    ok: true,
    active: true,
    instanceId: "self",
    name: "cellar",
    passwordSet: true,
    members: [{ id: "m1", name: "attic", version: "v9.2.0", direct: false, relay: true, address: "https://192.168.1.21:3443" }],
    away: [],
    settling: false,
    relay: {
      mode: "project",
      url: "",
      projectUrl: "wss://parleyport.halleluja.design/relay/connect",
      connected: true,
      serve: false,
      serveClients: 0,
    },
    joinedAgo: 300,
    memberSeen: true,
    waiting: [],
    selfAddress: "https://192.168.1.20:3443",
    ...over,
  };
}

function fortressOf(over: Partial<FortressState> = {}): FortressState {
  return { ok: true, joined: true, phrase: true, defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [], removed: [], removedHere: false, ...over };
}
const noFortress = () => fortressOf({ joined: false, phrase: false });

const alone = (over: Partial<GroupState> = {}) => makeGroup({ members: [], memberSeen: false, joinedAgo: 90, ...over });
const outside = (over: Partial<GroupState> = {}) => makeGroup({ active: false, members: [], memberSeen: false, joinedAgo: 0, ...over });
const noRelay = () => ({ ...makeGroup().relay, mode: "off" as RelayMode, connected: false });

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getGroup: () => Promise.resolve(group),
    getFortress: () => Promise.resolve(fortress),
    showFortressPhrase: (password: string) =>
      Promise.resolve(!fortress.passwordSet || password === "secret" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong" }),
    leaveFortress: () => {
      left++;
      group = outside();
      fortress = noFortress();
      return Promise.resolve(fortress);
    },
    listRoles: () => Promise.resolve({ ok: true, members: roles, self: { receiver: [], fetcher: [], zfs: [] } }),
    listZFSReplicaServers: () => Promise.resolve(servers),
    getZFSReplicaKey: () => Promise.resolve({ publicKey: "ssh-ed25519 AAAA bombvault" }),
    setRelay: (patch: { mode?: RelayMode; url?: string; serve?: boolean }) => {
      relayCalls.push(patch);
      const answer = { ...group, relay: { ...group.relay, ...patch } };
      group = answer;
      if (!heldRelay) return Promise.resolve(answer);
      const held = heldRelay;
      return new Promise((resolve) => held.push(() => resolve(answer)));
    },
    createPhrase: () => {
      group = makeGroup({ members: [], memberSeen: false, joinedAgo: 0 });
      fortress = fortressOf({ passwordSet: group.passwordSet });
      return Promise.resolve({ ok: true, phrase: PHRASE, group });
    },
    showPhrase: (password: string) =>
      Promise.resolve(
        !group.passwordSet || password === "secret" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong" },
      ),
    joinGroup: (phrase: string) => {
      joined.push(phrase);
      if (joinAnswer.ok === false) return Promise.resolve(joinAnswer);
      fortress = fortressOf();
      return Promise.resolve((group = makeGroup({ members: [], memberSeen: false, joinedAgo: 0 })));
    },
    leaveGroup: () => {
      left++;
      group = outside();
      fortress = noFortress();
      return Promise.resolve(group);
    },
    approveMember: (id: string) => {
      decisions.push("approve " + id);
      group = { ...group, waiting: group.waiting.filter((w) => w.id !== id) };
      return Promise.resolve(group);
    },
    removeMember: (id: string) => {
      removed.push(id);
      group = { ...group, behind: (group.behind ?? []).filter((m) => m.id !== id) };
      return Promise.resolve({ ok: true });
    },
    refuseMember: (id: string) => {
      decisions.push("refuse " + id);
      group = { ...group, waiting: group.waiting.filter((w) => w.id !== id) };
      return Promise.resolve(group);
    },
  };
});

const { PairingSection } = await import("./PairingSection");
const { pairStage } = await import("./pairStage");

function Tab() {
  const { t } = useT();
  let hue = 0;
  return (
    <PairingSection
      t={t}
      nextHue={() => hue++}
      zfs={zfs}
      nameRow={(name) => <div data-testid="name-row">{name}</div>}
      giveBack={{ on: false, onChange: () => {} }}
    />
  );
}

let router: ReturnType<typeof createMemoryRouter>;

async function renderTab() {
  router = createMemoryRouter(
    [
      {
        path: "/settings/:page",
        element: (
          <I18nProvider>
            <Tab />
          </I18nProvider>
        ),
      },
    ],
    { initialEntries: ["/settings/pairing"] },
  );
  await act(async () => {
    render(<RouterProvider router={router} />);
  });
}

function phraseCard(): HTMLElement {
  return document.querySelector("[data-stage]")!.closest('[data-glim="FortressPairing"], .glim-notch-card') as HTMLElement;
}

function stage(): string | null {
  return document.querySelector("[data-stage]")?.getAttribute("data-stage") ?? null;
}

function tile(key: "pairing.create" | "pairing.enter" | "pairing.twoTitle"): HTMLButtonElement {
  return screen.getByRole("button", { name: new RegExp(en[key].replace(/[?]/g, "\\?")) }) as HTMLButtonElement;
}

function stubClipboard(readText: () => Promise<string>) {
  Object.defineProperty(navigator, "clipboard", { value: { readText }, configurable: true });
}

function dialog(): HTMLElement {
  return screen.getByRole("dialog");
}

function pairButton(): HTMLButtonElement {
  return within(dialog()).getByRole("button", { name: en["pairing.join"] }) as HTMLButtonElement;
}

beforeEach(() => {
  group = makeGroup();
  fortress = fortressOf();
  zfs = false;
  roles = [];
  servers = [];
  relayCalls.length = 0;
  heldRelay = null;
  joined.length = 0;
  joinAnswer = {};
  left = 0;
  decisions.length = 0;
  removed.length = 0;
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("step cards", () => {
  it("number the three steps and name the button they refer to", async () => {
    await renderTab();
    const headings = screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");
    expect(headings.slice(0, 3)).toEqual([
      `1${en["pairing.step1Title"]}`,
      `2${en["pairing.stepOtherTitle"]}`,
      `3${en["pairing.step3Title"]}`,
    ]);
    expect(screen.getByText(en["pairing.enter"], { selector: "strong" })).not.toBeNull();
  });

  it("draw one scene over the three cards: generate and copy, enter and paste, paired", async () => {
    await renderTab();
    const pictures = [...document.querySelectorAll<SVGSVGElement>("svg[data-step]")];
    expect(pictures.map((svg) => svg.getAttribute("data-step"))).toEqual(["1", "2", "3"]);
    const texts = pictures.map((svg) => [...svg.querySelectorAll("text")].map((node) => node.textContent));
    expect(texts[0]).toEqual(expect.arrayContaining([en["pairing.create"], en["common.copy"], en["common.copied"], "anchor", "lunar"]));
    expect(texts[1]).toEqual(expect.arrayContaining([en["pairing.enter"], en["pairing.pic.pasteWords"], en["pairing.paste"], "anchor", "lunar"]));
    expect(texts[2]).toEqual([en["pairing.paired"]]);
    // Each instance in the third picture is the app's own logo.
    expect(pictures[2].querySelectorAll('image[href="/logo.svg"]')).toHaveLength(2);
  });

  it("show every word of the phrase in the frame the pictures rest on", async () => {
    await renderTab();
    const words = document.querySelectorAll('svg[data-step="1"] .glim-step-show-early');
    expect(words).toHaveLength(12);
    const moments = [...words].map((word) => Number((word as SVGElement).style.getPropertyValue("--step-at")));
    expect(moments).toEqual([...moments].sort((a, b) => a - b));
    expect(moments[0]).toBeGreaterThan(0.09);
  });

  it("walk one pointer through all three, outside any card's own picture", async () => {
    await renderTab();
    const pointers = document.querySelectorAll(".glim-step-pointer");
    expect(pointers).toHaveLength(1);
    expect(pointers[0].closest("svg[data-step]")).toBeNull();
    expect(pointers[0].getAttribute("aria-hidden")).toBe("true");
  });
});

describe("the tiles outside a group", () => {
  it("offers generate and enter as tiles and opens no window before either is pressed", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    expect(tile("pairing.create").disabled).toBe(false);
    expect(tile("pairing.enter").disabled).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("say another app removed this instance, until it joins again", async () => {
    group = outside();
    fortress = fortressOf({ joined: false, phrase: false, removedHere: true });
    await renderTab();
    expect(phraseCard().textContent).toContain("Another app in the fortress removed BombVault. Enter the current phrase to come back in.");
    fortress = noFortress();
    cleanup();
    await renderTab();
    expect(phraseCard().textContent).not.toContain("removed BombVault");
  });

  it("opens a window on Enter phrase and keeps Pair off until twelve listed words are there", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    const field = within(dialog()).getByLabelText(en["pairing.enterLabel"]);
    expect(pairButton().disabled).toBe(true);
    fireEvent.change(field, { target: { value: PHRASE.split(" ").slice(0, 7).join(" ") + " " } });
    expect(screen.getByText(en["pairing.wordCount"].replace("{n}", "7"))).not.toBeNull();
    expect(pairButton().disabled).toBe(true);
    fireEvent.change(field, { target: { value: PHRASE } });
    expect(pairButton().disabled).toBe(false);
  });

  it("names an unknown word with its place before anything is sent", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    const words = PHRASE.split(" ");
    words[6] = "unveel";
    fireEvent.change(within(dialog()).getByLabelText(en["pairing.enterLabel"]), { target: { value: words.join(" ") } });
    expect(within(dialog()).getByRole("alert").textContent).toBe(
      en["pairing.errUnknownWord"].replace("{position}", "7").replace("{word}", "unveel"),
    );
    expect(pairButton().disabled).toBe(true);
    expect(joined).toEqual([]);
  });

  it("takes a numbered list pasted with line breaks, sends the bare words and closes the window", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    const numbered = PHRASE.split(" ")
      .map((w, i) => `${i + 1}. ${w},`)
      .join("\n");
    fireEvent.change(within(dialog()).getByLabelText(en["pairing.enterLabel"]), { target: { value: numbered } });
    expect(within(dialog()).getByRole("alert").textContent).toBe("");
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(joined).toEqual([PHRASE]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("fills the field from the clipboard with Paste", async () => {
    stubClipboard(() => Promise.resolve(PHRASE));
    group = outside();
    fortress = noFortress();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["pairing.paste"] }));
    });
    expect((within(dialog()).getByLabelText(en["pairing.enterLabel"]) as HTMLTextAreaElement).value).toBe(PHRASE);
    expect(pairButton().disabled).toBe(false);
  });

  it("says how to paste by hand when the browser keeps the clipboard", async () => {
    stubClipboard(() => Promise.reject(new Error("denied")));
    group = outside();
    fortress = noFortress();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["pairing.paste"] }));
    });
    expect(within(dialog()).getByRole("alert").textContent).toBe(en["pairing.pasteRefused"]);
  });

  it("says the words do not fit together when the server finds the checksum wrong", async () => {
    group = outside();
    fortress = noFortress();
    joinAnswer = { ok: false, reason: "checksum" };
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    fireEvent.change(within(dialog()).getByLabelText(en["pairing.enterLabel"]), { target: { value: PHRASE } });
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(within(dialog()).getByRole("alert").textContent).toBe(en["pairing.errChecksum"]);
  });

  it("closes the window with Escape", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the words in a window, then the next step and a new-group badge", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    await act(async () => {
      fireEvent.click(tile("pairing.create"));
    });
    expect(within(dialog()).getAllByRole("listitem").some((li) => li.textContent === "12about")).toBe(true);
    expect(within(dialog()).getByRole("button", { name: en["common.copy"] })).not.toBeNull();
    fireEvent.click(within(dialog()).getByRole("button", { name: en["common.close"] }));
    expect(stage()).toBe("new");
    expect(screen.getByText(en["pairing.stateNew"])).not.toBeNull();
    expect(screen.getByText(en["pairing.nextTitle"])).not.toBeNull();
    expect(screen.getByText(en["pairing.waitNext"])).not.toBeNull();
  });

  it("leaves the empty group and opens the entry window when the phrase already exists elsewhere", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    await act(async () => {
      fireEvent.click(tile("pairing.create"));
    });
    fireEvent.click(within(dialog()).getByRole("button", { name: en["common.close"] }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pairing.notFirstButton"] }));
    });
    expect(left).toBe(1);
    expect(stage()).toBe("unpaired");
    expect(within(dialog()).getByLabelText(en["pairing.enterLabel"])).not.toBeNull();
  });
});

describe("the badge and the members", () => {
  it("says paired only once another member is found", () => {
    expect(pairStage(outside(), 0, false)).toBe("unpaired");
    expect(pairStage(makeGroup(), 5, true)).toBe("paired");
    expect(pairStage(alone({ joinedAgo: 5 }), 5, true)).toBe("new");
    expect(pairStage(alone({ joinedAgo: 5 }), 5, false)).toBe("searching");
    expect(pairStage(alone(), 60, true)).toBe("alone");
    expect(pairStage(alone({ memberSeen: true }), 600, false)).toBe("gone");
  });

  it("reads Searching with a running timer in the first minute after joining", async () => {
    group = alone({ joinedAgo: 12 });
    await renderTab();
    expect(stage()).toBe("searching");
    expect(screen.getByText(en["pairing.stateSearching"])).not.toBeNull();
    expect(screen.getByTestId("wait-row").textContent).toBe(`${en["pairing.waitPhrase"]}0:12`);
    expect(within(screen.getByTestId("members")).queryByText(en["pairing.paired"])).toBeNull();
  });

  it("turns to Still alone once the minute passes on the page's own clock", async () => {
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "setTimeout", "clearTimeout", "Date"] });
    group = alone({ joinedAgo: 55 });
    await renderTab();
    expect(stage()).toBe("searching");
    await act(async () => {
      vi.advanceTimersByTime(6_000);
    });
    expect(stage()).toBe("alone");
    expect(screen.getByText(en["pairing.stateAlone"])).not.toBeNull();
  });

  it("draws this instance first, then each member with how it is reached, as the Instances tab does", async () => {
    group = makeGroup({
      members: [
        { id: "m1", name: "attic", version: "v9.2.0", direct: false, relay: true, address: "" },
        { id: "m2", name: "garage", version: "v9.2.0", direct: true, relay: false, address: "https://192.168.1.22:3443" },
      ],
    });
    await renderTab();
    expect(screen.getByTestId("pair-state").textContent).toBe(en["pairing.paired"]);
    const cards = [...screen.getByTestId("members").querySelectorAll<HTMLElement>("[data-instance]")];
    expect(cards.map((c) => c.dataset.instance)).toEqual(["self", "m1", "m2"]);
    expect(cards[0].textContent).toContain(en["instances.thisInstance"]);
    expect(cards[1].textContent).toContain(en["instances.connected"]);
    expect(cards[1].textContent).toContain(en["pairing.viaRelay"]);
    expect(cards[2].textContent).toContain(en["pairing.direct"]);
    for (const card of cards) expect(card.querySelector('img[src="/logo.svg"]')).toBeTruthy();
  });

  it("offers no second way to words once paired, since the card holds the one phrase", async () => {
    await renderTab();
    expect(within(phraseCard()).queryByRole("button", { name: en["pairing.notFirstButton"] })).toBeNull();
    expect(screen.getAllByRole("button", { name: "Show phrase" })).toHaveLength(1);
  });

  it("keeps the members out of the phrase card and puts this instance's name at its top", async () => {
    await renderTab();
    expect(within(phraseCard()).queryByText("attic")).toBeNull();
    expect(within(phraseCard()).getByTestId("name-row").textContent).toBe("Tower");
    expect(phraseCard().querySelector('[data-fortress="open-change"]')).toBeTruthy();
    expect(phraseCard().querySelector('[data-fortress="open-leave"]')).toBeTruthy();
  });

  it("marks a phone that runs the app", async () => {
    group = makeGroup({
      members: [{ id: "m3", name: "Pixel 8", version: "", direct: false, relay: true, address: "", kind: "android" }],
    });
    await renderTab();
    const card = screen.getByTestId("members").querySelector('[data-instance="m3"]')!;
    expect(card.textContent).toContain(en["fleet.kindAndroid"]);
  });

  it("keeps a server that is shut down and a phone with the app closed as inactive, with when each was last seen", async () => {
    const now = Math.floor(Date.now() / 1000);
    group = makeGroup({
      away: [
        { id: "m2", name: "garage", address: "https://192.168.1.22:3443", lastSeen: now - 2 * 3600 },
        { id: "m3", name: "Pixel 8", kind: "android", address: "", lastSeen: now - 86400 },
      ],
    });
    await renderTab();
    const cards = [...screen.getByTestId("members").querySelectorAll<HTMLElement>("[data-instance]")];
    expect(cards).toHaveLength(4);
    const ago = (key: "time.hoursAgo" | "time.daysAgo", n: number) => en["instances.lastSeen"].replace("{time}", countText(en[key], "en", n));
    expect(cards[2].textContent).toContain("garage");
    expect(cards[2].textContent).toContain(ago("time.hoursAgo", 2));
    expect(cards[3].textContent).toContain(en["fleet.kindAndroid"]);
    expect(cards[3].textContent).toContain(ago("time.daysAgo", 1));
    for (const card of cards.slice(2)) {
      const badge = within(card).getByText(en["instances.inactive"]);
      expect(badge.closest('[data-new="pairing-inactive"]')).toBeTruthy();
      expect(card.textContent).not.toContain(en["instances.connected"]);
    }
  });

  it("says a member that is away is being checked while the group is still coming back", async () => {
    group = makeGroup({ settling: true, away: [{ id: "m2", name: "garage", address: "", lastSeen: 1 }] });
    await renderTab();
    const card = screen.getByTestId("members").querySelector('[data-instance="m2"]')!;
    expect(card.textContent).toContain(en["instances.checking"]);
    expect(card.textContent).not.toContain(en["instances.inactive"]);
  });

  it("shows the words again only after the password in a window", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Show phrase" }));
    });
    fireEvent.change(within(dialog()).getByLabelText("Password of BombVault"), { target: { value: "secret" } });
    await act(async () => {
      fireEvent.click(document.querySelector<HTMLButtonElement>('[data-fortress="unlock"]')!);
    });
    expect(document.querySelectorAll('[data-fortress="words"] li')).toHaveLength(12);
  });
});

describe("after a minute alone", () => {
  it("says to enter the words over there, or to take the other instance's words", async () => {
    group = alone();
    await renderTab();
    expect(screen.getByText(`${en["pairing.aloneLead"]} ${en["pairing.notYetBody"]}`)).not.toBeNull();
    expect(tile("pairing.twoTitle")).not.toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("joins the other group in one step from its window", async () => {
    group = alone();
    await renderTab();
    fireEvent.click(tile("pairing.twoTitle"));
    const field = within(dialog()).getByLabelText(en["pairing.enterLabelOther"]);
    fireEvent.change(field, { target: { value: PHRASE } });
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(left).toBe(0);
    expect(joined).toEqual([PHRASE]);
    expect(stage()).toBe("searching");
  });

  it("names the other network only when no relay is in use", async () => {
    group = alone();
    await renderTab();
    expect(screen.queryByText(en["pairing.otherNetTitle"])).toBeNull();
    cleanup();
    group = alone({ relay: noRelay() });
    await renderTab();
    expect(screen.getByText(en["pairing.otherNetTitle"])).not.toBeNull();
  });

  it("says in one line that the relay is down and opens what to check", async () => {
    group = alone({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    const card = within(phraseCard());
    expect(card.getByText(en["relay.notConnected"])).not.toBeNull();
    expect(card.queryByText(en["pairing.relayCheckFilter"])).toBeNull();
    fireEvent.click(card.getByRole("button", { name: en["pairing.relayCheckOpen"] }));
    const window = within(screen.getByRole("dialog", { name: en["pairing.relayCheckOpen"] }));
    expect(window.getByText("parleyport.halleluja.design", { selector: "span" })).not.toBeNull();
    expect(window.getByText(en["pairing.relayCheckFilter"])).not.toBeNull();
  });
});

describe("devices on an older version", () => {
  it("names them and asks to update everything together", async () => {
    group = makeGroup({ older: ["attic", "Pixel"] });
    await renderTab();
    const note = screen.getByTestId("older");
    expect(note.textContent).toBe(en["pairing.older"].replace("{names}", "attic, Pixel"));
  });

  it("says nothing while every device is up to date", async () => {
    await renderTab();
    expect(screen.queryByTestId("older")).toBeNull();
  });
});

describe("pairing by the phrase alone", () => {
  it("has nowhere to type the address of an instance, this one's own included", async () => {
    for (const g of [alone({ relay: noRelay() }), makeGroup(), outside()]) {
      group = g;
      await renderTab();
      expect(document.querySelector('input[type="url"]:not(#relay-address)')).toBeNull();
      cleanup();
    }
  });
});

describe("devices that wait", () => {
  const phone = "phone-" + "ab".repeat(32);

  it("lists a device that asks to join with its key, and lets it in or turns it away", async () => {
    group = makeGroup({
      waiting: [
        { id: phone, name: "Pixel", kind: "android", fingerprint: "1a2b 3c4d 5e6f 7a8b", online: true },
        { id: "c".repeat(40), name: "barn", fingerprint: "0000 1111 2222 3333", online: false },
      ],
      fingerprint: "9f8e 7d6c 5b4a 3928",
    });
    await renderTab();
    const list = within(screen.getByTestId("waiting"));
    expect(screen.getByText(en["pairing.pendingTitle"])).toBeTruthy();
    expect(list.getByText(en["pairing.fingerprint"].replace("{fingerprint}", "1a2b 3c4d 5e6f 7a8b"))).toBeTruthy();
    expect(list.getByText(en["pairing.ownKey"].replace("{fingerprint}", "9f8e 7d6c 5b4a 3928"))).toBeTruthy();
    await act(async () => {
      fireEvent.click(list.getAllByRole("button", { name: en["pairing.approve"] })[0]);
    });
    await act(async () => {
      fireEvent.click(within(screen.getByTestId("waiting")).getByRole("button", { name: en["pairing.refuse"] }));
    });
    expect(decisions).toEqual(["approve " + phone, "refuse " + "c".repeat(40)]);
    expect(screen.queryByTestId("waiting")).toBeNull();
  });

  it("says when this instance waits on another one, with nothing to press", async () => {
    group = makeGroup({ waiting: [{ id: "d".repeat(40), name: "attic", online: true, there: true }], fingerprint: "9f8e 7d6c 5b4a 3928" });
    await renderTab();
    const list = within(screen.getByTestId("waiting"));
    expect(list.getByText(en["pairing.awaiting"])).toBeTruthy();
    expect(list.queryByRole("button", { name: en["pairing.approve"] })).toBeNull();
  });
});

describe("the word list", () => {
  it("drops list numbers and punctuation from a paste", () => {
    expect(splitPhrase("1. orbit,\n2) Velvet;  12: absorb.")).toEqual(["orbit", "velvet", "absorb"]);
  });

  it("does not flag the word still being typed", () => {
    expect(checkPhrase("abandon aba").unknown).toEqual([]);
    expect(checkPhrase("abandon aba ").unknown).toEqual([1]);
  });
});

describe("the instances card", () => {
  it("offers a ZFS server without BombVault after the members and connects one over SSH", async () => {
    zfs = true;
    await renderTab();
    const row = screen.getByTestId("zfs-row");
    const card = row.closest(".glim-notch-card") as HTMLElement;
    expect(card.textContent).toContain(en["pairing.group.zfsTitle"]);
    expect(row.textContent).toContain(en["pairing.group.zfsNone"]);
    expect(within(card).getByLabelText(en["pairing.group.zfsTip"])).not.toBeNull();
    expect(screen.getByTestId("members").compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: en["pairing.group.zfsConnect"] }));
    });
    expect(screen.getByRole("dialog", { name: en["zfs.replica.addServer"] })).not.toBeNull();
  });

  it("names the ZFS servers that are connected already", async () => {
    zfs = true;
    servers = [server(), server({ id: "old", name: "Old box" })];
    await renderTab();
    expect(screen.getByTestId("zfs-row").textContent).toContain("Backup-NAS, Old box");
  });

  it("has no ZFS row while the ZFS section is off", async () => {
    servers = [server()];
    await renderTab();
    expect(screen.queryByTestId("zfs-row")).toBeNull();
  });

  it("draws the members without the network picture, which the Instances page shows", async () => {
    zfs = true;
    servers = [server()];
    roles = [memberRoles({ byMember: roleSide({ receiver: roleRequest({ state: "allowed" }) }) })];
    await renderTab();
    expect(screen.getByTestId("members").querySelectorAll("[data-instance]").length).toBeGreaterThan(1);
    expect(screen.queryByTestId("net-picture")).toBeNull();
  });

  it("draws no members outside a group, only the tiles to make or enter a phrase", async () => {
    group = outside();
    fortress = noFortress();
    await renderTab();
    expect(screen.queryByTestId("net-picture")).toBeNull();
    expect(screen.queryByTestId("members")).toBeNull();
    expect(screen.queryByTestId("pair-state")).toBeNull();
    expect(tile("pairing.create")).not.toBeNull();
  });
});

function routePicture(key: "relay.pic.relay" | "relay.pic.instance" | "relay.pic.direct"): HTMLElement | null {
  return screen.queryByRole("img", { name: en[key] });
}

describe("relay card", () => {
  it("says first what a relay is for, then the two steps: choose, done", async () => {
    await renderTab();
    const steps = screen.getByTestId("relay-steps");
    expect(steps.textContent).toContain(en["relay.lead"]);
    expect(within(steps).getAllByRole("listitem").map((s) => s.textContent)).toEqual([`1${en["relay.stepPick"]}`, `2${en["relay.stepDone"]}`]);
  });

  it("goes back to the project relay in one click from keeping everything at home", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.offLink"] }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.backToProject"] }));
    });
    expect(relayCalls).toEqual([{ mode: "off" }, { mode: "project" }]);
    expect(screen.queryByTestId("no-relay-phone")).toBeNull();
  });

  it("says whether the relay is connected", async () => {
    await renderTab();
    expect(screen.getByTestId("relay-state").textContent).toBe(en["instances.connected"]);
    cleanup();
    group = makeGroup({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    expect(screen.getByTestId("relay-state").textContent).toBe(en["instances.notConnected"]);
    cleanup();
    group = makeGroup({ relay: noRelay() });
    await renderTab();
    expect(screen.getByTestId("relay-state").textContent).toBe(en["relay.off"]);
  });

  it("says No group yet before there is a phrase to connect with", async () => {
    group = outside({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    expect(screen.getByTestId("relay-state").textContent).toBe(en["relay.noGroup"]);
    expect(screen.queryByTestId("relay-check")).toBeNull();
  });

  it("shows the route in use as a picture beside what it needs", async () => {
    await renderTab();
    expect(routePicture("relay.pic.relay")).not.toBeNull();
    expect(screen.getByText("parleyport.halleluja.design", { exact: false })).not.toBeNull();
    expect(screen.getByText(en["relay.projectNeed"])).not.toBeNull();
    expect(screen.getByText(en["relay.e2e"], { exact: false })).not.toBeNull();
  });

  it("opens what the relay sees in a window", async () => {
    await renderTab();
    expect(screen.queryByText(en["relay.projectSees"])).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: en["relay.sees.title"] }));
    const win = dialog();
    expect(within(win).getByText(en["relay.projectSees"])).not.toBeNull();
    expect(within(win).getByText(en["relay.notSeesContent"])).not.toBeNull();
    expect(within(win).getByText(en["relay.notSeesBackups"])).not.toBeNull();
    fireEvent.click(within(win).getByRole("button", { name: en["common.close"] }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says that nothing leaves the network where there is no relay to see anything", async () => {
    group = makeGroup({ relay: noRelay() });
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: en["relay.sees.title"] }));
    expect(within(dialog()).getByText(en["relay.offSees"])).not.toBeNull();
    expect(within(dialog()).queryByText(en["relay.notSeesContent"])).toBeNull();
  });

  it("offers three routes up front, keeping everything at home as a small link under them, and starts on the stored one", async () => {
    await renderTab();
    const routes = screen.getAllByRole("radio").map((route) => route.textContent);
    expect(routes).toEqual([en["relay.project"], en["relay.own"]]);
    expect(screen.getByRole("button", { name: en["relay.offLink"] })).toBeTruthy();
    expect(screen.getByRole("radio", { name: en["relay.project"] }).getAttribute("aria-checked")).toBe("true");
    expect(screen.queryByLabelText(en["relay.addressLabel"], { exact: false })).toBeNull();
  });

  it("switches the picture and saves the route when another is picked", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.offLink"] }));
    });
    expect(relayCalls).toContainEqual({ mode: "off" });
    expect(screen.getByTestId("no-relay-phone").textContent).toBe(en["relay.offNote"]);
    expect(routePicture("relay.pic.direct")).not.toBeNull();
    expect(routePicture("relay.pic.relay")).toBeNull();
    expect(screen.queryByTestId("own-relay")).toBeNull();
  });

  it("moves through the routes with the arrow keys and saves only the one pressed", async () => {
    await renderTab();
    const project = screen.getByRole("radio", { name: en["relay.project"] });
    project.focus();
    fireEvent.keyDown(project, { key: "ArrowRight" });
    expect(document.activeElement).toBe(screen.getByRole("radio", { name: en["relay.own"] }));
    expect(relayCalls).toEqual([]);
    expect(project.getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.click(document.activeElement!);
    });
    expect(relayCalls).toEqual([{ mode: "own" }]);
  });

  it("keeps the last pick when the answer to an earlier one comes later", async () => {
    await renderTab();
    const held: Array<() => void> = [];
    heldRelay = held;
    await act(async () => {
      fireEvent.click(screen.getByRole("radio", { name: en["relay.own"] }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.offLink"] }));
    });
    expect(held).toHaveLength(2);
    await act(async () => {
      held[1]();
      held[0]();
    });
    expect(screen.getByTestId("no-relay-phone")).toBeTruthy();
    expect(document.getElementById("relay-address")).toBeNull();
  });

  it("shows the address field and where to get a relay for an own relay", async () => {
    await renderTab();
    expect(screen.queryByTestId("own-relay")).toBeNull();
    await act(async () => {
      fireEvent.click(screen.getByRole("radio", { name: en["relay.own"] }));
    });
    const own = within(screen.getByTestId("own-relay"));
    expect(own.getByText(en["relay.containerName"], { selector: "strong" })).not.toBeNull();
    expect(own.getByText(en["relay.containerAlso"], { exact: false })).not.toBeNull();
    expect(own.queryByRole("switch")).toBeNull();
    expect(document.getElementById("relay-address")).not.toBeNull();
    expect(screen.getByText(en["relay.pic.selfHosted"])).not.toBeNull();
  });

  it("offers a paired BombVault as the relay under a quiet link, stored as an own relay", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.instanceLink"] }));
    });
    expect(relayCalls).toEqual([{ mode: "own" }]);
    expect(screen.queryByRole("button", { name: en["relay.instanceLink"] })).toBeNull();
    expect(routePicture("relay.pic.instance")).not.toBeNull();
    expect(screen.queryByTestId("own-relay")).toBeNull();
    const route = within(screen.getByTestId("instance-relay"));
    expect((document.getElementById("relay-address") as HTMLInputElement).placeholder).toContain("/relay/connect");
    await act(async () => {
      fireEvent.click(route.getByRole("switch", { name: en["relay.serve"] }));
    });
    expect(relayCalls).toContainEqual({ serve: true });
  });

  it("moves between the two own routes without saving the same mode twice", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["relay.instanceLink"] }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("radio", { name: en["relay.own"] }));
    });
    expect(relayCalls).toEqual([{ mode: "own" }]);
    expect(screen.getByTestId("own-relay")).not.toBeNull();
  });

  it("starts on the BombVault route when the stored address belongs to a paired instance", async () => {
    group = makeGroup({ relay: { ...makeGroup().relay, mode: "own", url: "wss://192.168.1.21:8443/relay" } });
    await renderTab();
    expect(screen.queryByRole("button", { name: en["relay.instanceLink"] })).toBeNull();
    expect(routePicture("relay.pic.instance")).not.toBeNull();
  });

  it("says what to check while the relay is not connected", async () => {
    await renderTab();
    expect(screen.queryByTestId("relay-check")).toBeNull();
    cleanup();
    group = makeGroup({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    const check = within(screen.getByTestId("relay-check"));
    expect(check.getByText("parleyport.halleluja.design", { selector: "span" })).not.toBeNull();
    expect(check.getByText(en["pairing.relayCheckFilter"])).not.toBeNull();
    expect(check.getByRole("button", { name: en["pairing.checkAgain"] })).not.toBeNull();
  });

  it("saves a typed address after a pause, without a save button", async () => {
    group = makeGroup({ relay: { ...makeGroup().relay, mode: "own" } });
    await renderTab();
    vi.useFakeTimers();
    const input = document.getElementById("relay-address") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "relay.example.org" } });
    expect(relayCalls).toEqual([]);
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    expect(relayCalls).toContainEqual({ url: "relay.example.org" });
    expect(screen.queryByRole("button", { name: en["settings.save"] })).toBeNull();
  });

  it("warns that an address without TLS carries the relay key in plain text", async () => {
    group = makeGroup({ relay: { ...makeGroup().relay, mode: "own", url: "wss://relay.example.org" } });
    await renderTab();
    expect(screen.queryByText(en["relay.plaintextWarning"])).toBeNull();
    for (const address of ["ws://192.168.1.9:8080", "http://tower.local:3000"]) {
      fireEvent.change(document.getElementById("relay-address") as HTMLInputElement, { target: { value: address } });
      expect(screen.getByText(en["relay.plaintextWarning"])).not.toBeNull();
    }
    fireEvent.change(document.getElementById("relay-address") as HTMLInputElement, { target: { value: "https://relay.example.org" } });
    expect(screen.queryByText(en["relay.plaintextWarning"])).toBeNull();
  });
});

describe("phrase card without a login password", () => {
  it("warns plainly, leads to the password field on the Security page and still pairs", async () => {
    group = outside({ passwordSet: false });
    fortress = noFortress();
    await renderTab();
    const note = screen.getByTestId("no-password");
    expect(note.textContent).toContain(en["pairing.noPasswordTitle"]);
    expect(note.textContent).toContain(en["pairing.noPasswordHint"]);
    await act(async () => {
      fireEvent.click(within(note).getByRole("button", { name: en["auth.setPassword"] }));
    });
    expect(router.state.location.pathname).toBe("/settings/security");
    expect(router.state.location.hash).toBe(`#${LOGIN_PASSWORD_FIELD}`);
    expect(tile("pairing.enter").disabled).toBe(false);
    await act(async () => {
      fireEvent.click(tile("pairing.create"));
    });
    expect(within(dialog()).getAllByRole("listitem")).toHaveLength(12);
  });

  it("shows the words without asking for a password there is none of", async () => {
    group = makeGroup({ passwordSet: false });
    fortress = fortressOf({ passwordSet: false });
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Show phrase" }));
    });
    expect(screen.queryByLabelText("Password of BombVault")).toBeNull();
    expect(document.querySelectorAll('[data-fortress="words"] li')).toHaveLength(12);
  });

  it("stands between the three steps and the fortress's card, and stays away on this device once closed", async () => {
    group = makeGroup({ passwordSet: false });
    await renderTab();
    const note = screen.getByTestId("no-password");
    expect(phraseCard().contains(note)).toBe(false);
    expect(note.compareDocumentPosition(phraseCard()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(document.querySelector("[data-step-frame]")!.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(note).getByRole("button", { name: en["common.close"] }));
    });
    expect(screen.queryByTestId("no-password")).toBeNull();
    cleanup();
    await renderTab();
    expect(screen.queryByTestId("no-password")).toBeNull();
  });

  it("keeps the warning out of the way once a password is set", async () => {
    await renderTab();
    expect(screen.queryByTestId("no-password")).toBeNull();
  });
});

describe("a group that moved into the fortress", () => {
  it("names who has not followed under the members and keeps the one card of the phrase", async () => {
    group = makeGroup({ fortress: true, oldPhrase: ["attic", "pixel"] });
    await renderTab();
    expect(screen.getByText(en["pairing.oldPhrase"].replace("{names}", "attic and pixel"))).toBeTruthy();
    expect(screen.getAllByRole("button", { name: "Show phrase" })).toHaveLength(1);
    expect(stage()).toBe("paired");
  });

  it("draws the devices that did not come along by name and kind, without an address, and takes one out on the second click", async () => {
    group = makeGroup({
      fortress: true,
      away: [{ id: "m2", name: "garage", address: "https://192.168.1.22:3443", lastSeen: 1 }],
      behind: [
        { id: "m3", name: "barn" },
        { id: "p1", name: "Pixel 8", kind: "android" },
      ],
    });
    await renderTab();
    const list = screen.getByTestId("behind");
    const cards = [...list.querySelectorAll<HTMLElement>("[data-instance]")];
    expect(cards.map((c) => c.dataset.instance)).toEqual(["m3", "p1"]);
    expect(cards[0].textContent).toContain(en["pairing.behindTitle"]);
    expect(cards[0].textContent).toContain(en["pairing.kindInstance"]);
    expect(cards[1].textContent).toContain(en["fleet.kindAndroid"]);
    expect(within(cards[0]).getByLabelText(en["pairing.behindTip"])).toBeTruthy();
    expect(list.textContent).not.toMatch(/192\.168|:3443/);

    await act(async () => {
      fireEvent.click(within(cards[0]).getByRole("button", { name: en["fleet.remove"] }));
    });
    expect(removed).toEqual([]);
    await act(async () => {
      fireEvent.click(within(cards[0]).getByRole("button", { name: en["fleet.confirmRemove"] }));
    });
    expect(removed).toEqual(["m3"]);
    expect(screen.getByTestId("behind").querySelectorAll("[data-instance]")).toHaveLength(1);
  });

  it("shows no list of devices left behind while nobody is", async () => {
    group = makeGroup({ fortress: true, behind: [] });
    await renderTab();
    expect(screen.queryByTestId("behind")).toBeNull();
  });

  it("says nothing about the old phrase once everybody followed", async () => {
    group = makeGroup({ fortress: true, oldPhrase: [] });
    await renderTab();
    expect(screen.getByTestId("members")).toBeTruthy();
    expect(screen.queryByText(/old phrase/)).toBeNull();
  });
});

describe("one phrase for everything", () => {
  const knight = { id: "k1", app: "knightloader", name: "KnightLoader", version: "2.1.0", device: "Bottich", deployment: "container", kind: "instance", online: true, relay: false };

  it("shows the phrase in one place once other apps are in the fortress, and draws them under the members", async () => {
    storeShowOthers(true);
    fortress = fortressOf({ members: [knight] });
    await renderTab();
    expect(screen.getAllByRole("button", { name: "Show phrase" })).toHaveLength(1);
    expect(document.querySelector('[data-instance="app-knightloader"]')).toBeTruthy();
    expect(screen.getByTestId("members").compareDocumentPosition(document.querySelector('[data-instance="app-knightloader"]')!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("puts the switch for the other apps last in the fortress's card, and the fortress keeps none of its own", async () => {
    storeShowOthers(false);
    fortress = fortressOf({ members: [knight] });
    await renderTab();
    expect(document.querySelector('[data-fortress="others"]')).toBeNull();
    const card = phraseCard();
    const others = within(card).getByRole("switch", { name: "Show the other apps of the fortress" });
    expect(card.querySelector('[data-fortress="open-leave"]')!.compareDocumentPosition(others) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(document.querySelector('[data-instance="app-knightloader"]')).toBeNull();
    await act(async () => fireEvent.click(others));
    expect(document.querySelector('[data-instance="app-knightloader"]')).toBeTruthy();
    expect(within(card).getByRole("switch", { name: "Show the other apps of the fortress" }).getAttribute("aria-checked")).toBe("true");
  });

  it("has no line that waits for an instance while other apps are in the fortress", async () => {
    group = alone({ joinedAgo: 12 });
    fortress = fortressOf({ members: [knight] });
    await renderTab();
    expect(screen.queryByTestId("wait-row")).toBeNull();
    expect(screen.queryByText(en["pairing.searchingHint"])).toBeNull();
    expect(stage()).toBe("paired");
  });

  it("says outside a fortress that it waits for an instance with this phrase", async () => {
    group = alone({ joinedAgo: 12 });
    await renderTab();
    expect(screen.getByTestId("wait-row").textContent).toContain(en["pairing.waitPhrase"]);
    expect(en["pairing.waitPhrase"]).toMatch(/this phrase/);
  });

  it("leaves the group and the fortress together from the card", async () => {
    await renderTab();
    fireEvent.click(document.querySelector<HTMLButtonElement>('[data-fortress="open-leave"]')!);
    await act(async () => {
      fireEvent.click(document.querySelector<HTMLButtonElement>('[data-fortress="leave"]')!);
    });
    expect(left).toBe(1);
    expect(await screen.findByRole("button", { name: new RegExp(en["pairing.create"]) })).toBeTruthy();
  });
});
