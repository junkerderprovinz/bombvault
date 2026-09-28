// @vitest-environment jsdom
// The Pairing tab: one sentence on what pairing does, three numbered step
// cards, the phrase card and the relay card. The phrase card asks first whether this is the first instance, checks
// typed words against the word list as they arrive, and says after a minute
// alone what probably went wrong.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { GroupState, RelayMode } from "../lib/api";
import { checkPhrase, splitPhrase } from "../lib/phraseWords";

let group: GroupState;
const relayCalls: Array<Record<string, unknown>> = [];
const joined: string[] = [];
let joinAnswer: Record<string, unknown> = {};
let left = 0;

// The BIP39 test vector: twelve listed words that check out.
const PHRASE = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";

function makeGroup(over: Partial<GroupState> = {}): GroupState {
  return {
    ok: true,
    active: true,
    instanceId: "self",
    name: "cellar",
    passwordSet: true,
    members: [{ id: "m1", name: "attic", version: "v9.2.0", direct: false, relay: true }],
    relay: {
      mode: "project",
      url: "",
      projectUrl: "wss://relay.halleluja.design/relay/connect",
      connected: true,
      serve: false,
      serveClients: 0,
    },
    joinedAgo: 300,
    memberSeen: true,
    ...over,
  };
}

const alone = (over: Partial<GroupState> = {}) => makeGroup({ members: [], memberSeen: false, joinedAgo: 90, ...over });
const outside = (over: Partial<GroupState> = {}) => makeGroup({ active: false, members: [], memberSeen: false, joinedAgo: 0, ...over });

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
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
    joinGroup: (phrase: string) => {
      joined.push(phrase);
      return Promise.resolve(joinAnswer.ok === false ? joinAnswer : (group = makeGroup({ members: [], memberSeen: false, joinedAgo: 0 })));
    },
    leaveGroup: () => {
      left++;
      group = outside();
      return Promise.resolve(group);
    },
  };
});

const { Pairing } = await import("./Pairing");
const { pairStage } = await import("./instances/PhraseCard");

async function renderTab() {
  await act(async () => {
    render(
      <I18nProvider>
        <Pairing embedded />
      </I18nProvider>,
    );
  });
}

function stage(): string | null {
  return document.querySelector("[data-stage]")?.getAttribute("data-stage") ?? null;
}

function pairButton(within_: HTMLElement = document.body): HTMLButtonElement {
  return within(within_).getByRole("button", { name: en["pairing.join"] }) as HTMLButtonElement;
}

beforeEach(() => {
  group = makeGroup();
  relayCalls.length = 0;
  joined.length = 0;
  joinAnswer = {};
  left = 0;
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("step cards", () => {
  it("open with what pairing does in one sentence", async () => {
    await renderTab();
    expect(screen.getByText(en["pairing.lead"], { exact: false })).not.toBeNull();
  });

  it("number the three steps and name the answers they refer to", async () => {
    await renderTab();
    const headings = screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");
    expect(headings.slice(0, 3)).toEqual([
      `1${en["pairing.step1Title"]}`,
      `2${en["pairing.step2Title"]}`,
      `3${en["pairing.step3Title"]}`,
    ]);
    expect(screen.getByText(en["pairing.create"], { selector: "strong" })).not.toBeNull();
    expect(screen.getByText(en["pairing.enter"], { selector: "strong" })).not.toBeNull();
  });
});

describe("the question outside a group", () => {
  it("offers yes and no as tiles and shows no input before an answer", async () => {
    group = outside();
    await renderTab();
    expect(screen.getByText(en["pairing.ask"])).not.toBeNull();
    const yes = screen.getByRole("button", { name: new RegExp(en["pairing.create"]) });
    const no = screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) });
    expect(yes.getAttribute("aria-pressed")).toBe("false");
    expect(no.getAttribute("aria-pressed")).toBe("false");
    expect(screen.queryByLabelText(en["pairing.enterLabel"])).toBeNull();
  });

  it("opens the input on no and keeps pair off until twelve listed words are there", async () => {
    group = outside();
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) }));
    expect(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) }).getAttribute("aria-pressed")).toBe("true");
    const field = screen.getByLabelText(en["pairing.enterLabel"], { exact: false });
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
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) }));
    const words = PHRASE.split(" ");
    words[6] = "unveel";
    fireEvent.change(screen.getByLabelText(en["pairing.enterLabel"], { exact: false }), { target: { value: words.join(" ") } });
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toBe(en["pairing.errUnknownWord"].replace("{position}", "7").replace("{word}", "unveel"));
    expect(pairButton().disabled).toBe(true);
    expect(joined).toEqual([]);
  });

  it("takes a numbered list pasted with line breaks and sends the bare words", async () => {
    group = outside();
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) }));
    const numbered = PHRASE.split(" ")
      .map((w, i) => `${i + 1}. ${w},`)
      .join("\n");
    fireEvent.change(screen.getByLabelText(en["pairing.enterLabel"], { exact: false }), { target: { value: numbered } });
    expect(screen.queryByRole("alert")?.textContent ?? "").toBe("");
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(joined).toEqual([PHRASE]);
  });

  it("says the words do not fit together when the server finds the checksum wrong", async () => {
    group = outside();
    joinAnswer = { ok: false, reason: "checksum" };
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.enter"]) }));
    fireEvent.change(screen.getByLabelText(en["pairing.enterLabel"], { exact: false }), { target: { value: PHRASE } });
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(screen.getByRole("alert").textContent).toBe(en["pairing.errChecksum"]);
  });

  it("shows the words, the next step and a new-group badge after yes", async () => {
    group = outside();
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.create"]) }));
    });
    expect(stage()).toBe("new");
    expect(screen.getByText(en["pairing.stateNew"])).not.toBeNull();
    expect(screen.getAllByRole("listitem").some((li) => li.textContent === "12about")).toBe(true);
    expect(screen.getByText(en["pairing.nextTitle"])).not.toBeNull();
    expect(screen.getByText(en["pairing.waitNext"])).not.toBeNull();
  });

  it("leaves the empty group and opens the input when it was not the first instance after all", async () => {
    group = outside();
    await renderTab();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: new RegExp(en["pairing.create"]) }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pairing.notFirstButton"] }));
    });
    expect(left).toBe(1);
    expect(stage()).toBe("unpaired");
    expect(screen.getByLabelText(en["pairing.enterLabel"], { exact: false })).not.toBeNull();
  });
});

describe("the badge", () => {
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

  it("reads Paired and lists the members once one is there", async () => {
    await renderTab();
    expect(screen.getByText(en["pairing.paired"])).not.toBeNull();
    expect(screen.getByText("attic")).not.toBeNull();
    expect(screen.getAllByText(en["pairing.viaRelay"]).length).toBeGreaterThan(0);
  });
});

describe("after a minute alone", () => {
  it("suggests two groups with leaving first and entering the other words second", async () => {
    group = alone();
    await renderTab();
    expect(screen.getByText(en["pairing.aloneTitle"])).not.toBeNull();
    expect(screen.getByText(en["pairing.twoTitle"])).not.toBeNull();
    const other = screen.getByLabelText(en["pairing.enterLabelOther"]) as HTMLTextAreaElement;
    expect(other.disabled).toBe(true);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pairing.leave"] }));
    });
    expect(left).toBe(1);
    expect(stage()).toBe("fixing");
    expect(screen.getByText(en["pairing.fixLeft"])).not.toBeNull();
    const unlocked = screen.getByLabelText(en["pairing.enterLabelOther"]) as HTMLTextAreaElement;
    expect(unlocked.disabled).toBe(false);
    fireEvent.change(unlocked, { target: { value: PHRASE } });
    await act(async () => {
      fireEvent.click(pairButton());
    });
    expect(joined).toEqual([PHRASE]);
    expect(stage()).toBe("searching");
  });

  it("names the other network only when no relay is in use", async () => {
    group = alone();
    await renderTab();
    expect(screen.queryByText(en["pairing.otherNetTitle"])).toBeNull();
    cleanup();
    group = alone({ relay: { ...makeGroup().relay, mode: "off", connected: false } });
    await renderTab();
    expect(screen.getByText(en["pairing.otherNetTitle"])).not.toBeNull();
  });

  it("says what to check when the relay cannot be reached", async () => {
    group = alone({ relay: { ...makeGroup().relay, connected: false } });
    await renderTab();
    expect(screen.getByText(en["relay.notConnected"], { selector: "h3" })).not.toBeNull();
    expect(screen.getByText("relay.halleluja.design", { selector: "span" })).not.toBeNull();
    expect(screen.getByText(en["pairing.relayCheckFilter"])).not.toBeNull();
    expect(screen.getByTestId("relay-line").textContent).toContain(en["pairing.relayProjectDown"]);
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
    group = makeGroup({ relay: { ...makeGroup().relay, mode: "off", connected: false } });
    await renderTab();
    expect(screen.getByTestId("relay-state").textContent).toBe(en["relay.off"]);
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
  it("says to set one and offers nothing that makes or takes a phrase", async () => {
    group = outside({ passwordSet: false });
    await renderTab();
    expect(screen.getByText(en["pairing.needsPassword"])).not.toBeNull();
    for (const key of ["pairing.create", "pairing.enter"] as const) {
      expect((screen.getByRole("button", { name: new RegExp(en[key]) }) as HTMLButtonElement).disabled).toBe(true);
    }
  });

  it("does not show the phrase of a paired instance whose password was removed", async () => {
    group = makeGroup({ passwordSet: false });
    await renderTab();
    expect(screen.getByText(en["pairing.needsPassword"])).not.toBeNull();
    expect((screen.getByRole("button", { name: en["pairing.show"] }) as HTMLButtonElement).disabled).toBe(true);
  });
});
