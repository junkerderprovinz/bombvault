// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Bridge, Fetched, LauncherState, Request } from "./bridge";
import { Launcher } from "./Launcher";

// A Back from one test arrives as a popstate after it ends, so it is let in
// before the page goes, and the next test starts on a clean entry.
afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 0));
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
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: { paired: false, connected: true, joining: { members } }, camera: false, app });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByText("Offsite")).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Add all 2/ })));
    expect(sent).toContainEqual({ op: "adopt" });
  });

  it("shows only what the words found and the step that takes it over", async () => {
    const members = [{ id: "a", name: "Tower", version: "v9.7.0" }];
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: { paired: false, connected: true, joining: { members } }, camera: false, app });
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
    await act(async () => fireEvent.click(await screen.findByRole("button", { name: /^Remove all$/ })));
    expect(sent).toContainEqual({ op: "removeAll" });
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
});
