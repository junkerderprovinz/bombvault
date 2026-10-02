// @vitest-environment jsdom
// The Pairing tab of Settings: one sentence on what pairing does, three
// numbered step cards, the phrase card and the relay card. The phrase card
// offers two tiles, generate or enter a phrase, each opening a window; the
// entry window checks typed words against the word list as they arrive, and
// after a minute alone the card offers the same two windows as the way out.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import { I18nProvider, en, useT } from "../../../lib/i18n";
import type { GroupState, RelayMode } from "../../../lib/api";
import { checkPhrase, splitPhrase } from "../../../lib/phraseWords";
import { LOGIN_PASSWORD_FIELD } from "../shared";

let group: GroupState;
const relayCalls: Array<Record<string, unknown>> = [];
const joined: string[] = [];
let joinAnswer: Record<string, unknown> = {};
let left = 0;
const selfAddressCalls: string[] = [];
const probeCalls: string[] = [];
let probeAnswer: Record<string, unknown> | null = null;

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
    selfAddress: "https://192.168.1.20:3443",
    selfAddressManual: false,
    ...over,
  };
}

const alone = (over: Partial<GroupState> = {}) => makeGroup({ members: [], memberSeen: false, joinedAgo: 90, ...over });
const outside = (over: Partial<GroupState> = {}) => makeGroup({ active: false, members: [], memberSeen: false, joinedAgo: 0, ...over });
const noRelay = () => ({ ...makeGroup().relay, mode: "off" as RelayMode, connected: false });

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getGroup: () => Promise.resolve(group),
    setRelay: (patch: { mode?: RelayMode; url?: string; serve?: boolean }) => {
      relayCalls.push(patch);
      group = { ...group, relay: { ...group.relay, ...patch } };
      return Promise.resolve(group);
    },
    createPhrase: () => {
      group = makeGroup({ members: [], memberSeen: false, joinedAgo: 0 });
      return Promise.resolve({ ok: true, phrase: PHRASE, group });
    },
    showPhrase: (password: string) =>
      Promise.resolve(
        !group.passwordSet || password === "secret" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong" },
      ),
    joinGroup: (phrase: string) => {
      joined.push(phrase);
      return Promise.resolve(joinAnswer.ok === false ? joinAnswer : (group = makeGroup({ members: [], memberSeen: false, joinedAgo: 0 })));
    },
    leaveGroup: () => {
      left++;
      group = outside();
      return Promise.resolve(group);
    },
    setDirectAddress: (url: string) => {
      selfAddressCalls.push(url);
      group = { ...group, selfAddress: url, selfAddressManual: url !== "" };
      return Promise.resolve(group);
    },
    probeAddress: (url: string) => {
      probeCalls.push(url);
      if (probeAnswer) return Promise.resolve(probeAnswer);
      return Promise.resolve(group);
    },
  };
});

const { PairingSection } = await import("./PairingSection");
const { pairStage } = await import("./pairStage");

function Tab() {
  const { t } = useT();
  let hue = 0;
  return <PairingSection t={t} nextHue={() => hue++} />;
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

function stage(): string | null {
  return document.querySelector("[data-stage]")?.getAttribute("data-stage") ?? null;
}

function tile(key: "pairing.create" | "pairing.enter" | "pairing.notYetTitle" | "pairing.twoTitle"): HTMLButtonElement {
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
  relayCalls.length = 0;
  joined.length = 0;
  joinAnswer = {};
  left = 0;
  selfAddressCalls.length = 0;
  probeCalls.length = 0;
  probeAnswer = null;
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("step cards", () => {
  it("open with what pairing does in one sentence", async () => {
    await renderTab();
    expect(screen.getByText(en["pairing.lead"], { exact: false })).not.toBeNull();
  });

  it("number the three steps and name the button they refer to", async () => {
    await renderTab();
    const headings = screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");
    expect(headings.slice(0, 3)).toEqual([
      `1${en["pairing.step1Title"]}`,
      `2${en["pairing.step2Title"]}`,
      `3${en["pairing.step3Title"]}`,
    ]);
    expect(screen.getByText(en["pairing.enter"], { selector: "strong" })).not.toBeNull();
  });
});

describe("the tiles outside a group", () => {
  it("offers generate and enter as tiles and opens no window before either is pressed", async () => {
    group = outside();
    await renderTab();
    expect(tile("pairing.create").disabled).toBe(false);
    expect(tile("pairing.enter").disabled).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("opens a window on Enter phrase and keeps Pair off until twelve listed words are there", async () => {
    group = outside();
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
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["pairing.paste"] }));
    });
    expect(within(dialog()).getByRole("alert").textContent).toBe(en["pairing.pasteRefused"]);
  });

  it("says the words do not fit together when the server finds the checksum wrong", async () => {
    group = outside();
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
    await renderTab();
    fireEvent.click(tile("pairing.enter"));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the words in a window, then the next step and a new-group badge", async () => {
    group = outside();
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
    expect(screen.getByText("0:12")).not.toBeNull();
    expect(screen.queryByText(en["pairing.paired"])).toBeNull();
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

  it("lists this instance first, then each member with how it is reached", async () => {
    group = makeGroup({
      members: [
        { id: "m1", name: "attic", version: "v9.2.0", direct: false, relay: true, address: "" },
        { id: "m2", name: "garage", version: "v9.2.0", direct: true, relay: false, address: "https://192.168.1.22:3443" },
      ],
    });
    await renderTab();
    expect(screen.getByTestId("pair-state").textContent).toBe(en["pairing.paired"]);
    const rows = within(screen.getByTestId("members")).getAllByRole("listitem");
    expect(rows.map((r) => r.textContent)).toEqual([
      `cellar${en["instances.thisInstance"]}`,
      `attic${en["pairing.viaRelay"]}`,
      `garage${en["pairing.direct"]}`,
    ]);
  });

  it("shows the words again only after the password in a window", async () => {
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: en["pairing.show"] }));
    const win = dialog();
    expect(within(win).queryAllByRole("listitem")).toEqual([]);
    fireEvent.change(within(win).getByLabelText(en["pairing.passwordLabel"]), { target: { value: "secret" } });
    await act(async () => {
      fireEvent.click(within(win).getByRole("button", { name: en["pairing.show"] }));
    });
    expect(within(dialog()).getAllByRole("listitem")).toHaveLength(12);
  });
});

describe("after a minute alone", () => {
  it("offers the two ways out as tiles, with no hint panel", async () => {
    group = alone();
    await renderTab();
    expect(screen.getByText(en["pairing.aloneLead"])).not.toBeNull();
    expect(tile("pairing.notYetTitle")).not.toBeNull();
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

  it("offers Can't find it? only without a relay, and searches an address on demand", async () => {
    group = alone();
    await renderTab();
    expect(screen.queryByText(en["pairing.cantFindTitle"])).toBeNull();
    cleanup();

    group = alone({ relay: noRelay() });
    await renderTab();
    const disclosure = screen.getByRole("button", { name: en["pairing.cantFindTitle"] });
    expect(disclosure.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByLabelText(en["pairing.cantFindLabel"])).toBeNull();

    fireEvent.click(disclosure);
    expect(disclosure.getAttribute("aria-expanded")).toBe("true");
    const field = screen.getByLabelText(en["pairing.cantFindLabel"]) as HTMLInputElement;
    const search = screen.getByRole("button", { name: en["pairing.cantFindSearch"] }) as HTMLButtonElement;
    expect(search.disabled).toBe(true);

    fireEvent.change(field, { target: { value: "https://192.168.2.20:3443" } });
    expect(search.disabled).toBe(false);
    await act(async () => {
      fireEvent.click(search);
    });
    expect(probeCalls).toEqual(["https://192.168.2.20:3443"]);
  });

  it("says in one line that the relay is down and opens what to check", async () => {
    group = alone({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    expect(screen.getByText(en["relay.notConnected"])).not.toBeNull();
    expect(screen.queryByText(en["pairing.relayCheckFilter"])).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: en["pairing.relayCheckOpen"] }));
    expect(screen.getByText("parleyport.halleluja.design", { selector: "span" })).not.toBeNull();
    expect(screen.getByText(en["pairing.relayCheckFilter"])).not.toBeNull();
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

function howItWorks(): HTMLButtonElement {
  return screen.getByRole("button", { name: en["relay.howItWorks"] }) as HTMLButtonElement;
}

describe("relay card", () => {
  it("says what the relay is for and whether it is connected", async () => {
    await renderTab();
    expect(screen.getByText(en["relay.lead"])).not.toBeNull();
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
    expect(screen.getByText(en["relay.noGroupLine"])).not.toBeNull();
  });

  it("keeps the picture and what the relay sees behind a disclosure", async () => {
    await renderTab();
    expect(howItWorks().getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("img", { name: en["relay.projectAlt"] })).toBeNull();
    expect(screen.queryByText(en["relay.e2e"], { exact: false })).toBeNull();
    await act(async () => {
      fireEvent.click(howItWorks());
    });
    expect(howItWorks().getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("img", { name: en["relay.projectAlt"] })).not.toBeNull();
    expect(screen.getByText(en["relay.projectSees"])).not.toBeNull();
    expect(screen.getByText(en["relay.e2e"], { exact: false })).not.toBeNull();
  });

  it("offers the three routes and starts on the stored one", async () => {
    await renderTab();
    for (const key of ["relay.project", "relay.own", "relay.off"] as const) {
      expect(screen.getByRole("tab", { name: en[key] })).not.toBeNull();
    }
    expect(screen.getByRole("tab", { name: en["relay.project"] }).getAttribute("aria-selected")).toBe("true");
    await act(async () => {
      fireEvent.click(howItWorks());
    });
    expect(screen.getByRole("img", { name: en["relay.projectAlt"] })).not.toBeNull();
    expect(screen.queryByLabelText(en["relay.addressLabel"], { exact: false })).toBeNull();
  });

  it("switches the picture and saves the route when another is picked", async () => {
    await renderTab();
    await act(async () => {
      fireEvent.click(howItWorks());
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("tab", { name: en["relay.off"] }));
    });
    expect(relayCalls).toContainEqual({ mode: "off" });
    expect(screen.getByRole("img", { name: en["relay.offAlt"] })).not.toBeNull();
    expect(screen.queryByTestId("own-relay")).toBeNull();
  });

  it("shows the sources and the address field only for an own relay", async () => {
    await renderTab();
    expect(screen.queryByTestId("own-relay")).toBeNull();
    await act(async () => {
      fireEvent.click(screen.getByRole("tab", { name: en["relay.own"] }));
    });
    expect(screen.queryByTestId("own-relay")).not.toBeNull();
    expect(screen.getByText(en["relay.containerName"], { selector: "span" })).not.toBeNull();
    expect(screen.getByRole("switch", { name: en["relay.serve"] })).not.toBeNull();
    expect(document.getElementById("relay-address")).not.toBeNull();
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

  it("prefills this instance's own address and saves a change after a pause", async () => {
    await renderTab();
    const input = screen.getByTestId("self-address-input") as HTMLInputElement;
    expect(input.value).toBe("https://192.168.1.20:3443");
    vi.useFakeTimers();
    fireEvent.change(input, { target: { value: "https://192.168.1.55:3443" } });
    expect(selfAddressCalls).toEqual([]);
    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    expect(selfAddressCalls).toEqual(["https://192.168.1.55:3443"]);
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
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pairing.show"] }));
    });
    expect(within(dialog()).queryByLabelText(en["pairing.passwordLabel"])).toBeNull();
    expect(within(dialog()).getAllByRole("listitem")).toHaveLength(12);
  });

  it("keeps the warning out of the way once a password is set", async () => {
    await renderTab();
    expect(screen.queryByTestId("no-password")).toBeNull();
  });
});
