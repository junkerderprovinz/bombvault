// @vitest-environment jsdom
// The Pairing tab: three numbered step cards, the phrase card and the relay
// card, whose selector switches the route picture and shows the address field
// for an own relay only.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { GroupState, RelayMode } from "../lib/api";

let group: GroupState;
const relayCalls: Array<Record<string, unknown>> = [];
let joinAnswer: Record<string, unknown> = {};

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
    ...over,
  };
}

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
    joinGroup: () => Promise.resolve(joinAnswer),
  };
});

const { Pairing } = await import("./Pairing");

async function renderTab() {
  await act(async () => {
    render(
      <I18nProvider>
        <Pairing embedded />
      </I18nProvider>,
    );
  });
}

beforeEach(() => {
  group = makeGroup();
  relayCalls.length = 0;
  joinAnswer = {};
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("step cards", () => {
  it("number the three steps and name the buttons they refer to", async () => {
    await renderTab();
    const headings = screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");
    expect(headings.slice(0, 3)).toEqual([
      `1${en["pairing.step1Title"]}`,
      `2${en["pairing.step2Title"]}`,
      `3${en["pairing.step3Title"]}`,
    ]);
    expect(screen.getByText(en["pairing.create"], { selector: "strong" })).not.toBeNull();
    expect(screen.getByText(en["pairing.enter"], { selector: "strong" })).not.toBeNull();
    expect(screen.getByText(en["instances.title"], { selector: "strong" })).not.toBeNull();
  });
});

describe("relay card", () => {
  it("offers the three routes and starts on the stored one", async () => {
    await renderTab();
    for (const key of ["relay.project", "relay.own", "relay.off"] as const) {
      expect(screen.getByRole("tab", { name: en[key] })).not.toBeNull();
    }
    expect(screen.getByRole("tab", { name: en["relay.project"] }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("img", { name: en["relay.projectAlt"] })).not.toBeNull();
    expect(screen.queryByLabelText(en["relay.addressLabel"], { exact: false })).toBeNull();
  });

  it("switches the picture and saves the route when another is picked", async () => {
    await renderTab();
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

describe("phrase card", () => {
  it("names the mistyped word and its place", async () => {
    group = makeGroup({ active: false, members: [] });
    joinAnswer = { ok: false, reason: "unknown_word", word: "recieve", position: 7 };
    await renderTab();
    fireEvent.click(screen.getByRole("button", { name: en["pairing.enter"] }));
    fireEvent.change(screen.getByLabelText(en["pairing.enterLabel"]), { target: { value: "a b c" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pairing.join"] }));
    });
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("recieve");
    expect(alert.textContent).toContain("7");
  });

  it("lists the members and how each is reached", async () => {
    await renderTab();
    expect(screen.getByText("attic")).not.toBeNull();
    expect(screen.getAllByText(en["pairing.viaRelay"]).length).toBeGreaterThan(0);
  });
});

describe("phrase card without a login password", () => {
  it("says to set one and offers nothing that makes or takes a phrase", async () => {
    group = makeGroup({ active: false, members: [], passwordSet: false });
    await renderTab();
    expect(screen.getByText(en["pairing.needsPassword"])).not.toBeNull();
    for (const key of ["pairing.create", "pairing.enter"] as const) {
      expect((screen.getByRole("button", { name: en[key] }) as HTMLButtonElement).disabled).toBe(true);
    }
  });

  it("does not show the phrase of a paired instance whose password was removed", async () => {
    group = makeGroup({ passwordSet: false });
    await renderTab();
    expect(screen.getByText(en["pairing.needsPassword"])).not.toBeNull();
    expect((screen.getByRole("button", { name: en["pairing.show"] }) as HTMLButtonElement).disabled).toBe(true);
  });
});
