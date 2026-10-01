import { bridgeOver, type Bridge, type LauncherState, type Port, type Request } from "./bridge";

// A stand-in for the app, so `npm run dev` can show the launcher in a desktop
// browser. It keeps the list in localStorage, pretends to find two servers,
// pairs with any twelve words and answers the activity question with a
// running backup on the first server.

const KEY = "bv-launcher-preview";

const FOUND = [
  { name: "BombVault (Bottich)", url: "https://192.168.20.12:3443/", version: "9.7.0" },
  { name: "BombVault (Offsite)", url: "http://192.168.30.4:3080/", version: "9.6.0" },
];

const FINGERPRINT = "5E:0B:91:C2:7A:44:3F:18:D6:9A:02:BB:71:E5:C0:3D:48:9F:26:A1:0C:77:E2:5B:93:16:4D:F8:AA:61:07:C4";

export function previewBridge(): Bridge {
  const listeners = new Set<(e: { data: string }) => void>();
  const trusted = new Set<string>();
  let state: LauncherState = { servers: load(), found: [], group: { paired: false, connected: false } };

  function post(msg: object) {
    const data = JSON.stringify(msg);
    for (const fn of listeners) fn({ data });
  }
  function emit(next: Partial<LauncherState>) {
    state = { ...state, pairError: undefined, ...next };
    localStorage.setItem(KEY, JSON.stringify(state.servers));
    post({ op: "state", ...state });
  }

  setTimeout(() => emit({ found: FOUND }), 1200);

  function handle(req: Request) {
    switch (req.op) {
      case "state":
        emit({});
        return;
      case "save": {
        const id = req.server.id ?? crypto.randomUUID();
        const servers = state.servers.some((s) => s.id === id)
          ? state.servers.map((s) => (s.id === id ? { ...s, ...req.server, id } : s))
          : [...state.servers, { ...req.server, id }];
        emit({ servers });
        if (req.open) handle({ op: "open", id });
        return;
      }
      case "remove":
        emit({ servers: state.servers.filter((s) => s.id !== req.id), problem: undefined });
        return;
      case "open": {
        const server = state.servers.find((s) => s.id === req.id);
        if (!server) return;
        if (server.url.startsWith("http:")) {
          emit({ problem: { id: req.id, kind: "unreachable", detail: "net::ERR_CONNECTION_REFUSED" } });
        } else if (!trusted.has(req.id)) {
          emit({ problem: { id: req.id, kind: "untrusted", fingerprint: FINGERPRINT } });
        } else {
          emit({ problem: undefined });
        }
        return;
      }
      case "trust":
        trusted.add(req.id);
        emit({ problem: undefined });
        return;
      case "dismiss":
        emit({ problem: undefined });
        return;
      case "scan":
        handle({ op: "join", code: "abandon ability able about above absent absorb abstract absurd abuse access accident" });
        return;
      case "join": {
        const count = req.code.trim().split(/\s+/).filter(Boolean).length;
        if (count !== 12) {
          emit({ pairError: { reason: "count", count } });
          return;
        }
        const servers = [
          ...state.servers,
          { id: crypto.randomUUID(), name: "BombVault (Bottich)", url: "https://192.168.20.12:3443/", member: true },
          { id: crypto.randomUUID(), name: "BombVault (Eltern)", url: "", member: true },
        ];
        emit({ servers, group: { paired: true, connected: true } });
        return;
      }
      case "leave":
        emit({ servers: state.servers.filter((s) => s.url !== "").map((s) => ({ ...s, member: false })), group: { paired: false, connected: false } });
        return;
      case "activity": {
        const server = state.servers.find((s) => s.id === req.id);
        if (!server || server.url.startsWith("http:")) {
          post({ op: "activity", ticket: req.ticket, status: 0, body: "net::ERR_CONNECTION_REFUSED" });
          return;
        }
        post({ op: "activity", ticket: req.ticket, status: 200, body: answer() });
      }
    }
  }

  const port: Port = {
    postMessage: (data) => setTimeout(() => handle(JSON.parse(data) as Request), 150),
    addEventListener: (_, fn) => listeners.add(fn),
    removeEventListener: (_, fn) => listeners.delete(fn),
  };
  return bridgeOver(port);
}

function answer(): string {
  const now = Math.floor(Date.now() / 1000);
  const run = (id: string, target: string, status: string, startedAt: number, finishedAt: number | null) => ({
    id,
    targetId: target,
    kind: "backup",
    status,
    startedAt,
    finishedAt,
    snapshotId: "",
    bytes: 0,
    error: "",
    acknowledged: false,
    target,
    domain: "container",
  });
  return JSON.stringify({
    ok: true,
    runs: [
      run("r3", "nextcloud", "running", now - 95, null),
      run("r2", "immich", "success", now - 1500, now - 1260),
      run("r1", "paperless", "success", now - 1900, now - 1830),
    ],
    progress: [{ key: "container:nextcloud", phase: "backup", percent: 42, active: true, startedAt: now - 95 }],
    next: [{ job: "containers", domain: "containers", next: new Date((now + 3 * 3600) * 1000).toISOString() }],
  });
}

function load(): LauncherState["servers"] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? "[]") as LauncherState["servers"];
  } catch {
    return [];
  }
}
