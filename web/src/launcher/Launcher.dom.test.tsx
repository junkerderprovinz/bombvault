// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Bridge, Fetched, LauncherState, Request } from "./bridge";
import { Launcher } from "./Launcher";

// jsdom lands a Back a few tasks after it was asked for, so a test that steps
// back waits for its popstate itself: one that lands in the next test opens
// the page it went to there.
afterEach(() => {
  cleanup();
  localStorage.clear();
  window.history.replaceState(null, "");
});

const alone = { paired: false, connected: false };
const app = { version: "9.7.0", versionCode: 90700, android: "15", model: "Pixel 8", deviceName: "" };

/** A bridge whose app answers with `state` and whose servers answer `answers`. */
function fakeBridge(state: LauncherState, answers: Record<string, Fetched> = {}, look?: Fetched) {
  const sent: Request[] = [];
  const bridge: Bridge = {
    send: (req) => sent.push(req),
    listen(onState) {
      queueMicrotask(() => onState(state));
      return () => undefined;
    },
    activity: (id) => Promise.resolve(answers[id] ?? { status: 0, body: "refused" }),
    look: () => Promise.resolve(look ?? { status: 0, body: "refused" }),
    paste: () => Promise.resolve(""),
  };
  return { bridge, sent };
}

const server = { id: "s1", name: "Tower", url: "https://192.168.1.10:3443/" };

describe("Launcher", () => {
  it("offers to add a server when there is none", async () => {
    const { bridge } = fakeBridge({ servers: [], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(/No servers yet/)).toBeTruthy();
  });

  it("shows what runs on a server above its card", async () => {
    const now = Math.floor(Date.now() / 1000);
    const body = JSON.stringify({
      ok: true,
      runs: [],
      progress: [{ key: "container:nextcloud", phase: "backup", percent: 42, active: true, startedAt: now }],
      next: [],
    });
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app }, { s1: { status: 200, body } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(/nextcloud .*42%/)).toBeTruthy();
    expect(screen.getByText("Connected")).toBeTruthy();
  });

  it("asks for a sign-in when the server refuses the session", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app }, { s1: { status: 401, body: "" } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Sign-in needed")).toBeTruthy();
  });

  it("sends the person to the certificate when the server shows another one", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app }, { s1: { status: -1, body: "Trust anchor not found" } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Check certificate")).toBeTruthy();
  });

  it("opens a server with a tap on its card", async () => {
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByText("Tower"));
    expect(sent).toContainEqual({ op: "open", id: "s1" });
  });

  it("names a group member whose address is not known yet", async () => {
    const { bridge } = fakeBridge({ servers: [{ id: "m1", name: "Parents", url: "", member: true }], found: [], group: { paired: true, connected: true }, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Address not known yet")).toBeTruthy();
  });

  it("pairs with typed words and says why words were refused", async () => {
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: alone, camera: false, app, pairError: { reason: "checksum" } });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByText(/don't fit together/)).toBeTruthy();
    const words = "legal winner thank year wave sausage worth useful legal winner thank yellow";
    fireEvent.change(screen.getByPlaceholderText(/Paste or type the words/), { target: { value: words } });
    expect(screen.getByText("12 of 12 words")).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /^Pair$/ })));
    expect(sent).toContainEqual({ op: "join", code: words });
  });

  it("lists the instances a phrase finds and takes them over at once", async () => {
    const members = [
      { id: "a", name: "Tower", version: "v9.7.0" },
      { id: "b", name: "Offsite", version: "v9.7.0" },
    ];
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: { paired: false, connected: true, joining: { members, connected: true } }, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByText("Offsite")).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Add all 2/ })));
    expect(sent).toContainEqual({ op: "adopt" });
  });

  it("shows only what the words found and the step that takes it over", async () => {
    const members = [{ id: "a", name: "Tower", version: "v9.7.0" }];
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: { paired: false, connected: true, joining: { members, connected: true } }, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByRole("button", { name: /Add this instance/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Pair$/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Scan QR code/ })).toBeNull();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Cancel/ })));
    expect(sent).toContainEqual({ op: "cancelJoin" });
  });

  it("asks for the camera before the scanner shows anything", async () => {
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    fireEvent.click(await screen.findByRole("button", { name: /Scan QR code/ }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Allow access/ })));
    expect(sent).toContainEqual({ op: "camera" });
  });

  it("shows the fingerprint before a certificate is trusted", async () => {
    const fingerprint = "AB:CD:EF";
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app, problem: { id: "s1", kind: "untrusted", fingerprint } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(fingerprint)).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Trust and open/ })));
    expect(sent).toContainEqual({ op: "trust", id: "s1", fingerprint });
  });

  it("asks before it trusts a new key a server answered with over the relay", async () => {
    const fingerprint = "12:34:56";
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app, problem: { id: "s1", kind: "keyChanged", fingerprint } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Server key changed")).toBeTruthy();
    expect(screen.getByText(fingerprint)).toBeTruthy();
    expect(screen.getByText(/Tower answered over the relay/)).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Trust new key/ })));
    expect(sent).toContainEqual({ op: "trustKey", id: "s1", fingerprint });
  });

  it("says when neither the address nor the relay reached a server", async () => {
    const away = { id: "s2", name: "Attic", url: "", member: true };
    const { bridge } = fakeBridge({ servers: [server, away], found: [], group: alone, camera: false, app, problem: { id: "s1", kind: "unreachable", relay: true } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Neither its address nor the relay answers.")).toBeTruthy();

    const { bridge: again } = fakeBridge({ servers: [server, away], found: [], group: alone, camera: false, app, problem: { id: "s2", kind: "unreachable", relay: true } });
    render(<Launcher bridge={again} />);
    expect(await screen.findByText("The relay can't reach it right now.")).toBeTruthy();
  });

  it("says what a network error means and keeps the raw one in an info bubble", async () => {
    const detail = "net::ERR_CONNECTION_TIMED_OUT";
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app, problem: { id: "s1", kind: "unreachable", detail } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("It didn't answer in time.")).toBeTruthy();
    expect(screen.queryByText(detail)).toBeNull();
    expect(screen.getByLabelText(`The phone reported: ${detail}`)).toBeTruthy();
  });

  it("renames the phone from the settings behind the gear", async () => {
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByRole("button", { name: "Settings" }));
    const field = screen.getByRole("textbox", { name: "Device name" });
    expect(field.getAttribute("placeholder")).toBe("Pixel 8");
    fireEvent.change(field, { target: { value: "Jo's phone" } });
    fireEvent.blur(field);
    expect(sent).toContainEqual({ op: "deviceName", name: "Jo's phone" });
  });

  it("removes every server only after asking", async () => {
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByRole("button", { name: "Settings" }));
    fireEvent.click(screen.getByRole("button", { name: /Remove all servers/ }));
    expect(sent).not.toContainEqual({ op: "removeAll" });
    // The closing dialog and the page both step back here. jsdom reads both
    // steps from the same entry and fires one popstate for them.
    const back = new Promise((resolve) => window.addEventListener("popstate", resolve, { once: true }));
    await act(async () => fireEvent.click(await screen.findByRole("button", { name: /^Remove all$/ })));
    expect(sent).toContainEqual({ op: "removeAll" });
    await act(() => back);
  });

  it("looks like the first server until a look is set here", async () => {
    const look = { status: 200, body: JSON.stringify({ ok: true, prefs: { "bv-accent": "#ff7eb6", "bv-lang": "fr" }, stored: true }) };
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app }, {}, look);
    render(<Launcher bridge={bridge} />);
    await screen.findByText("Tower");
    await act(async () => undefined);
    expect(localStorage.getItem("bv-accent")).toBe("#ff7eb6");
    // The language stays the phone's own.
    expect(localStorage.getItem("bv-lang")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    fireEvent.click(screen.getByRole("switch", { name: "Follow the first server" }));
    expect(screen.getByText(/no longer follows the server/)).toBeTruthy();
  });

  it("offers Dark, Light and System for the theme, and starts on System", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByRole("button", { name: "Settings" }));

    const theme = within(screen.getByRole("radiogroup", { name: "Theme" })).getAllByRole("radio");
    expect(theme.map((option) => option.textContent)).toEqual(["Dark", "Light", "System"]);
    expect(theme.map((option) => option.getAttribute("aria-checked"))).toEqual(["false", "false", "true"]);
  });

  it("goes back to the device's theme when System is picked after a choice", async () => {
    localStorage.setItem("bv-theme", "light");
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByRole("button", { name: "Settings" }));
    const theme = screen.getByRole("radiogroup", { name: "Theme" });
    expect(within(theme).getByRole("radio", { name: "Light" }).getAttribute("aria-checked")).toBe("true");

    fireEvent.click(within(theme).getByRole("radio", { name: "System" }));

    expect(localStorage.getItem("bv-theme")).toBe("system");
    expect(within(theme).getByRole("radio", { name: "System" }).getAttribute("aria-checked")).toBe("true");
  });

  it("opens the whole log from the activity card and shows one server at a time", async () => {
    const now = Math.floor(Date.now() / 1000);
    const answer = (key: string, percent: number) => ({
      status: 200,
      body: JSON.stringify({ ok: true, runs: [], progress: [{ key, phase: "backup", percent, active: true, startedAt: now }], next: [] }),
    });
    const other = { id: "s2", name: "Attic", url: "https://192.168.1.11:3443/" };
    const { bridge } = fakeBridge(
      { servers: [server, other], found: [], group: alone, camera: false, app },
      { s1: answer("container:nextcloud", 42), s2: answer("container:immich", 17) }
    );
    render(<Launcher bridge={bridge} />);
    await screen.findByText(/nextcloud .*42%/);
    fireEvent.click(screen.getByRole("button", { name: "Activity Log" }));
    expect(await screen.findByText(/immich .*17%/)).toBeTruthy();
    fireEvent.click(screen.getByRole("tab", { name: "Tower" }));
    expect(screen.queryByText(/immich .*17%/)).toBeNull();
    expect(screen.getByText(/nextcloud .*42%/)).toBeTruthy();
  });

  it("tells servers with the same name apart by their address", async () => {
    const twin = { id: "s2", name: server.name, url: "https://192.168.1.11:3443/" };
    const { bridge } = fakeBridge({ servers: [server, twin], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByRole("button", { name: "Activity Log" }));
    expect(await screen.findByRole("tab", { name: "192.168.1.11:3443" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: new URL(server.url).host })).toBeTruthy();
  });
  it("keeps looking for a while before it says nothing turned up", async () => {
    let push: (s: LauncherState) => void = () => undefined;
    const start: LauncherState = { servers: [], found: [], group: alone, camera: false, app };
    const bridge: Bridge = {
      ...fakeBridge(start).bridge,
      listen(onState) {
        push = onState;
        queueMicrotask(() => onState(start));
        return () => undefined;
      },
    };
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    const words = "legal winner thank year wave sausage worth useful legal winner thank yellow";
    fireEvent.change(await screen.findByPlaceholderText(/Paste or type the words/), { target: { value: words } });
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /^Pair$/ })));
    act(() => push({ ...start, group: { ...alone, joining: { members: [], connected: false } } }));
    expect(await screen.findByText("Looking for other instances")).toBeTruthy();
    expect(screen.queryByText(/No instance of this group/)).toBeNull();
    expect(screen.queryByText("Relay not reachable")).toBeNull();
  });

  it("goes on looking after Android rebuilt the screen during the look", async () => {
    localStorage.setItem("bv-look-until", String(Date.now() + 20_000));
    window.history.replaceState({ view: "pairing" }, "");
    const group = { paired: false, connected: false, joining: { members: [], connected: false } };
    const { bridge } = fakeBridge({ servers: [], found: [], group, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Looking for other instances")).toBeTruthy();
    expect(screen.queryByText(/No instance of this group/)).toBeNull();
    expect(screen.queryByText("Relay not reachable")).toBeNull();
  });

  it("names the relay when the group it was given shows nobody", async () => {
    const group = { paired: false, connected: false, joining: { members: [], connected: false } };
    const { bridge } = fakeBridge({ servers: [], found: [], group, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByText(/set to No relay/)).toBeTruthy();
    expect(screen.getByText("Relay not reachable")).toBeTruthy();
  });

  it("opens on the page the history holds, as after Android rebuilt the screen", async () => {
    window.history.replaceState({ view: "settings" }, "");
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByRole("heading", { name: "Settings" })).toBeTruthy();
  });

  it("calls a group member that does not answer inactive, with when the group last had it, and not red", async () => {
    const seen = Math.floor(Date.now() / 1000) - 3 * 3600;
    const member = { id: "m1", name: "Parents", url: "", member: true, seen };
    const { bridge } = fakeBridge({ servers: [member], found: [], group: { paired: true, connected: true }, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Inactive")).toBeTruthy();
    expect(screen.getByText("Last seen 3 hours ago")).toBeTruthy();
    expect(screen.queryByText("Not connected")).toBeNull();
  });

  it("still calls a server added by hand that does not answer not connected", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Not connected")).toBeTruthy();
    expect(screen.queryByText("Inactive")).toBeNull();
  });

  it("says a server is being checked until its first answer", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
    bridge.activity = () => new Promise<Fetched>(() => undefined);
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Checking")).toBeTruthy();
    expect(screen.queryByText("Not connected")).toBeNull();
  });

  it("shows a server that answered while another one is still being asked", async () => {
    const away = { id: "s2", name: "Attic", url: "https://192.168.1.11:3443/" };
    const ok = { status: 200, body: JSON.stringify({ ok: true, runs: [], progress: [], next: [] }) };
    const { bridge } = fakeBridge({ servers: [server, away], found: [], group: alone, camera: false, app });
    bridge.activity = (id) => (id === "s1" ? Promise.resolve(ok) : new Promise<Fetched>(() => undefined));
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Connected")).toBeTruthy();
  });

  it("shows what the servers said a moment ago while they are asked again", async () => {
    const ok = { status: 200, body: JSON.stringify({ ok: true, runs: [], progress: [], next: [] }) };
    const state = { servers: [server], found: [], group: alone, camera: false, app };
    const first = render(<Launcher bridge={fakeBridge(state, { s1: ok }).bridge} />);
    await screen.findByText("Connected");
    first.unmount();

    const { bridge: again } = fakeBridge(state);
    again.activity = () => new Promise<Fetched>(() => undefined);
    render(<Launcher bridge={again} />);
    expect(await screen.findByText("Connected")).toBeTruthy();
  });

  it("asks again as soon as the app is back in front, and not while it is away", async () => {
    let visibility: DocumentVisibilityState = "visible";
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => visibility });
    try {
      let asked = 0;
      const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, app });
      bridge.activity = () => {
        asked += 1;
        return Promise.resolve({ status: 200, body: "{}" });
      };
      render(<Launcher bridge={bridge} />);
      await screen.findByText("Tower");
      await act(async () => undefined);
      expect(asked).toBe(1);

      visibility = "hidden";
      act(() => {
        document.dispatchEvent(new Event("visibilitychange"));
      });
      expect(asked).toBe(1);

      visibility = "visible";
      act(() => {
        document.dispatchEvent(new Event("visibilitychange"));
      });
      expect(asked).toBe(2);
    } finally {
      delete (document as { visibilityState?: unknown }).visibilityState;
    }
  });
});
