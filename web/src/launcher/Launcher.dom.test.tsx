// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Bridge, Fetched, LauncherState, Request } from "./bridge";
import { Launcher } from "./Launcher";

afterEach(cleanup);

const alone = { paired: false, connected: false };

/** A bridge whose app answers with `state` and whose servers answer `answers`. */
function fakeBridge(state: LauncherState, answers: Record<string, Fetched> = {}) {
  const sent: Request[] = [];
  const bridge: Bridge = {
    send: (req) => sent.push(req),
    listen(onState) {
      queueMicrotask(() => onState(state));
      return () => undefined;
    },
    activity: (id) => Promise.resolve(answers[id] ?? { status: 0, body: "refused" }),
    paste: () => Promise.resolve(""),
  };
  return { bridge, sent };
}

const server = { id: "s1", name: "Tower", url: "https://192.168.1.10:3443/" };

describe("Launcher", () => {
  it("offers to add a server when there is none", async () => {
    const { bridge } = fakeBridge({ servers: [], found: [], group: alone, camera: false });
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
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false }, { s1: { status: 200, body } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(/nextcloud .*42%/)).toBeTruthy();
    expect(screen.getByText("Connected")).toBeTruthy();
  });

  it("asks for a sign-in when the server refuses the session", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false }, { s1: { status: 401, body: "" } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Sign-in needed")).toBeTruthy();
  });

  it("sends the person to the certificate when the server shows another one", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [], group: alone, camera: false }, { s1: { status: -1, body: "Trust anchor not found" } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Check certificate")).toBeTruthy();
  });

  it("opens a server with a tap on its card", async () => {
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByText("Tower"));
    expect(sent).toContainEqual({ op: "open", id: "s1" });
  });

  it("names a group member whose address is not known yet", async () => {
    const { bridge } = fakeBridge({ servers: [{ id: "m1", name: "Parents", url: "", member: true }], found: [], group: { paired: true, connected: true }, camera: false });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Address not known yet")).toBeTruthy();
  });

  it("pairs with typed words and says why words were refused", async () => {
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: alone, camera: false, pairError: { reason: "checksum" } });
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
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: { paired: false, connected: true, joining: { members } }, camera: false });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    expect(await screen.findByText("Offsite")).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Add all 2/ })));
    expect(sent).toContainEqual({ op: "adopt" });
  });

  it("asks for the camera before the scanner shows anything", async () => {
    const { bridge, sent } = fakeBridge({ servers: [], found: [], group: alone, camera: false });
    render(<Launcher bridge={bridge} />);
    fireEvent.click((await screen.findAllByRole("button", { name: /Add server/ }))[0]);
    fireEvent.click(await screen.findByRole("button", { name: /Scan QR code/ }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Allow access/ })));
    expect(sent).toContainEqual({ op: "camera" });
  });

  it("shows the fingerprint before a certificate is trusted", async () => {
    const fingerprint = "AB:CD:EF";
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], group: alone, camera: false, problem: { id: "s1", kind: "untrusted", fingerprint } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(fingerprint)).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Trust and open/ })));
    expect(sent).toContainEqual({ op: "trust", id: "s1", fingerprint });
  });
});
