// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Bridge, Fetched, LauncherState, Request } from "./bridge";
import { Launcher } from "./Launcher";

afterEach(cleanup);

/** A bridge whose app answers with `state` and whose servers answer `answers`. */
function fakeBridge(state: LauncherState, answers: Record<string, Fetched> = {}) {
  const sent: Request[] = [];
  const bridge: Bridge = {
    send: (req) => sent.push(req),
    listen(onState) {
      queueMicrotask(() => onState(state));
      return () => undefined;
    },
    fetch: (id, path) => Promise.resolve(answers[`${id} ${path.split("?")[0]}`] ?? { status: 0, body: "refused" }),
  };
  return { bridge, sent };
}

const server = { id: "s1", name: "Tower", url: "https://192.168.1.10:3443/" };

describe("Launcher", () => {
  it("offers to add a server when there is none", async () => {
    const { bridge } = fakeBridge({ servers: [], found: [] });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(/No servers yet/)).toBeTruthy();
  });

  it("shows what runs on a server above its card", async () => {
    const now = Math.floor(Date.now() / 1000);
    const { bridge } = fakeBridge(
      { servers: [server], found: [] },
      {
        "s1 /api/runs": { status: 200, body: JSON.stringify({ ok: true, runs: [] }) },
        "s1 /api/progress": {
          status: 200,
          body: `data: ${JSON.stringify({ key: "container:nextcloud", phase: "backup", percent: 42, active: true, startedAt: now })}\n\n`,
        },
        "s1 /api/schedule/next": { status: 200, body: JSON.stringify({ ok: true, runs: [] }) },
      }
    );
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(/nextcloud .*42%/)).toBeTruthy();
    expect(screen.getByText("Connected")).toBeTruthy();
  });

  it("asks for a sign-in when the server refuses the session", async () => {
    const { bridge } = fakeBridge(
      { servers: [server], found: [] },
      { "s1 /api/runs": { status: 401, body: "" }, "s1 /api/progress": { status: 401, body: "" } }
    );
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Sign-in needed")).toBeTruthy();
  });

  it("sends the person to the certificate when the server shows another one", async () => {
    const { bridge } = fakeBridge({ servers: [server], found: [] }, { "s1 /api/runs": { status: -1, body: "Trust anchor not found" } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText("Check certificate")).toBeTruthy();
  });

  it("opens a server with a tap on its card", async () => {
    const { bridge, sent } = fakeBridge({ servers: [server], found: [] });
    render(<Launcher bridge={bridge} />);
    fireEvent.click(await screen.findByText("Tower"));
    expect(sent).toContainEqual({ op: "open", id: "s1" });
  });

  it("shows the fingerprint before a certificate is trusted", async () => {
    const fingerprint = "AB:CD:EF";
    const { bridge, sent } = fakeBridge({ servers: [server], found: [], problem: { id: "s1", kind: "untrusted", fingerprint } });
    render(<Launcher bridge={bridge} />);
    expect(await screen.findByText(fingerprint)).toBeTruthy();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: /Trust and open/ })));
    expect(sent).toContainEqual({ op: "trust", id: "s1", fingerprint });
  });
});
