// The launcher is the start screen of the Android app. It runs from the app's
// own assets, before any server is open, and talks to the app through the
// `bombvaultApp` object the app injects. Every message is JSON. The app
// answers a fetch with the response and everything else with the whole state.

export interface Server {
  id: string;
  name: string;
  /** Empty for a group member whose address is not known yet. */
  url: string;
  /** The server came from the paired group. */
  member?: boolean;
}

export interface FoundServer {
  name: string;
  url: string;
  version: string;
}

export type ProblemKind = "untrusted" | "changed" | "unreachable";

/** Why the last attempt to open a server came back to the launcher. */
export interface Problem {
  id: string;
  kind: ProblemKind;
  /** SHA-256 of the certificate the server presented, for the two certificate kinds. */
  fingerprint?: string;
  /** What the WebView reported, for an unreachable server. */
  detail?: string;
}

/** An instance of the group as its announce names it. */
export interface GroupMember {
  id: string;
  name: string;
  version: string;
}

/** Why a pairing code was refused, in the terms of BombVault's own pairing card. */
export type PairError =
  | { reason: "count"; count: number }
  | { reason: "word"; word: string; position: number }
  | { reason: "checksum" };

export interface LauncherState {
  servers: Server[];
  found: FoundServer[];
  /** Whether the app is in a BombVault group, and connected to its relay. */
  group: {
    paired: boolean;
    connected: boolean;
    /** The instances found with words not yet taken over, while the person
     *  looks before adding them. */
    joining?: { members: GroupMember[] };
  };
  /** Whether the app may use the camera, for the scanner. */
  camera: boolean;
  problem?: Problem;
  pairError?: PairError;
}

/** A server's answer to the activity question: its runs, the progress events
 *  in flight and the next scheduled runs. Status 0 means it was not reached and
 *  -1 that its certificate is not the one trusted for it; the body then says why. */
export interface Fetched {
  status: number;
  body: string;
}

export type Request =
  | { op: "state" }
  | { op: "save"; server: { id?: string; name: string; url: string }; open?: boolean }
  | { op: "remove"; id: string }
  | { op: "open"; id: string }
  | { op: "trust"; id: string; fingerprint: string }
  | { op: "dismiss" }
  | { op: "activity"; ticket: number; id: string }
  | { op: "scan" }
  | { op: "join"; code: string }
  | { op: "adopt" }
  | { op: "cancelJoin" }
  | { op: "leave" }
  | { op: "camera" }
  | { op: "paste"; ticket: number };

type Reply =
  | ({ op: "state" } & LauncherState)
  | ({ op: "activity"; ticket: number } & Fetched)
  | { op: "pasted"; ticket: number; text: string };

export interface Bridge {
  send(req: Request): void;
  /** Calls `onState` with every state the app reports and returns the unsubscribe. */
  listen(onState: (s: LauncherState) => void): () => void;
  /** Asks what runs on the server, over the group or with the app's session for it. */
  activity(id: string): Promise<Fetched>;
  /** The phone's clipboard as text; a WebView cannot read it itself. */
  paste(): Promise<string>;
}

/** The transport underneath a Bridge: the injected object, or a stand-in. */
export interface Port {
  postMessage(data: string): void;
  addEventListener(type: "message", fn: (e: { data: string }) => void): void;
  removeEventListener(type: "message", fn: (e: { data: string }) => void): void;
}

export function bridgeOver(port: Port): Bridge {
  let ticket = 0;
  const waiting = new Map<number, (f: Fetched) => void>();
  const pasting = new Map<number, (text: string) => void>();
  port.addEventListener("message", (e) => {
    const reply = JSON.parse(e.data) as Reply;
    if (reply.op === "activity") {
      waiting.get(reply.ticket)?.({ status: reply.status, body: reply.body });
      waiting.delete(reply.ticket);
    } else if (reply.op === "pasted") {
      pasting.get(reply.ticket)?.(reply.text);
      pasting.delete(reply.ticket);
    }
  });
  const send = (req: Request) => port.postMessage(JSON.stringify(req));
  return {
    send,
    listen(onState) {
      const fn = (e: { data: string }) => {
        const reply = JSON.parse(e.data) as Reply;
        if (reply.op === "state") onState(reply);
      };
      port.addEventListener("message", fn);
      return () => port.removeEventListener("message", fn);
    },
    activity(id) {
      ticket += 1;
      const t = ticket;
      return new Promise((resolve) => {
        waiting.set(t, resolve);
        send({ op: "activity", ticket: t, id });
      });
    },
    paste() {
      ticket += 1;
      const t = ticket;
      return new Promise((resolve) => {
        pasting.set(t, resolve);
        send({ op: "paste", ticket: t });
      });
    },
  };
}

export function appBridge(): Bridge | null {
  const port = (window as unknown as { bombvaultApp?: Port }).bombvaultApp;
  return port ? bridgeOver(port) : null;
}

/**
 * normalizeAddress turns what someone typed into the URL the app opens, or
 * null when it cannot be one. A bare host gets https, since that is how
 * BombVault serves by default.
 */
export function normalizeAddress(input: string): string | null {
  const raw = input.trim();
  if (raw === "") return null;
  const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`;
  let u: URL;
  try {
    u = new URL(withScheme);
  } catch {
    return null;
  }
  if ((u.protocol !== "https:" && u.protocol !== "http:") || u.hostname === "") return null;
  return u.origin + u.pathname;
}

/** The address as the list shows it, the way it would be typed. */
export function displayAddress(url: string): string {
  return url.replace(/^https:\/\//, "").replace(/\/$/, "");
}

export function origin(url: string): string {
  try {
    return new URL(url).origin;
  } catch {
    return url;
  }
}
