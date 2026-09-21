import { createElement, type ReactElement } from "react";
import { fireEvent, render, type RenderResult } from "@testing-library/react";
import type {
  DefaultImpact,
  DefaultRow,
  DroppedTarget,
  HomeOption,
  NamedRepo,
  PlacementChange,
  PlacementOptions,
  PlacementView,
  SendToOption,
  TargetImpact,
  TargetOption,
  TargetPreview,
  UploadEstimate,
} from "./api";
import { I18nProvider } from "./i18n";
import { ToastProvider } from "./toast";

export function placementView(over?: Partial<PlacementView>): PlacementView {
  return {
    segment: "local-offsite",
    repo: "",
    repoLabel: "",
    repoKind: "domain",
    repoOff: false,
    homeFollows: false,
    copiesFollow: true,
    skip: [],
    locked: false,
    lockReason: "",
    segmentLocks: {},
    paused: false,
    unreadable: false,
    ...over,
  };
}

export function homeOption(over?: Partial<HomeOption>): HomeOption {
  return { id: "", name: "", location: "backups/containers", kind: "domain", scheme: "", ...over };
}

export function targetOption(over?: Partial<TargetOption>): TargetOption {
  return { id: "t-b2", name: "B2", enabled: true, primary: true, appendOnly: false, hint: "", ...over };
}

export function sendToOption(over?: Partial<SendToOption>): SendToOption {
  return { kind: "direct", repoId: "", targetId: "t-b2", name: "B2", location: "", ...over };
}

export function defaultRow(over?: Partial<DefaultRow>): DefaultRow {
  return {
    domain: "containers",
    exists: true,
    home: "",
    homeKind: "domain",
    homeOff: false,
    skip: [],
    paused: false,
    confirmedAt: 1_758_170_400,
    counts: { follow: 0, own: 0, open: 0, chosenNoRun: 0 },
    unreadable: false,
    ...over,
  };
}

export function placementOptions(over?: Partial<PlacementOptions>): PlacementOptions {
  return {
    domain: "containers",
    unreadable: false,
    paused: false,
    homes: [homeOption(), homeOption({ id: "repo-nas", name: "NAS Keller", location: "/mnt/remotes/nas/bv", kind: "local" })],
    targets: [targetOption()],
    sendTo: [sendToOption()],
    segmentLocks: {},
    default: defaultRow(),
    ...over,
  };
}

export function targetImpact(over?: Partial<TargetImpact>): TargetImpact {
  return { targetId: "t-b2", name: "B2", items: 15, snapshots: 210, unknown: false, uncheckable: [], ...over };
}

export function defaultImpact(over?: Partial<DefaultImpact>): DefaultImpact {
  return { dropped: [], added: [], openTakeHome: 0, home: "", skip: [], ...over };
}

export function targetPreview(over?: Partial<TargetPreview>): TargetPreview {
  return { items: 15, formerlyExcluded: [], defaultExcludes: false, snapshots: 420, bytes: null, unreadable: [], ...over };
}

export function uploadEstimate(over?: Partial<UploadEstimate>): UploadEstimate {
  return { targetId: "t-b2", name: "B2", snapshots: 40, uncheckable: [], ...over };
}

export function droppedTarget(over?: Partial<DroppedTarget>): DroppedTarget {
  return { targetId: "t-b2", name: "B2", copies: 14, appendOnly: false, ...over };
}

export function namedRepo(over?: Partial<NamedRepo>): NamedRepo {
  return {
    id: "repo-nas",
    name: "NAS Keller",
    repo: "/mnt/remotes/nas/bv",
    credsRef: "",
    storageClass: "",
    limitUpload: 0,
    limitDownload: 0,
    immutable: false,
    enabled: true,
    inUse: 0,
    companionOf: "",
    companionLost: false,
    ...over,
  };
}

function applied(change: PlacementChange): PlacementView {
  const view = placementView();
  if (change.home) {
    Object.assign(view, "follow" in change.home ? { homeFollows: true } : { repo: change.home.repo, homeFollows: false });
  }
  if (change.copies) {
    Object.assign(
      view,
      "follow" in change.copies
        ? { copiesFollow: true }
        : { skip: change.copies.skip, copiesFollow: false, segment: change.copies.skip[0] === "*" ? "local" : "local-offsite" }
    );
  }
  return view;
}

type Reply = (...args: unknown[]) => unknown;

// What every call answers when a test queued nothing for it.
const DEFAULTS: Record<string, Reply> = {
  getPlacementOptions: () => ({ ok: true, options: placementOptions() }),
  listPlacementDefaults: () => ({
    ok: true,
    defaults: [defaultRow(), defaultRow({ domain: "vms" }), defaultRow({ domain: "files" })],
  }),
  previewPlacementDefault: () => ({ ok: true, impact: defaultImpact() }),
  putPlacementDefault: () => ({ ok: true, default: defaultRow() }),
  getApplyDefaultPreview: () => ({ ok: true, reset: [], kept: [] }),
  applyPlacementDefault: () => ({ ok: true, reset: [], kept: [] }),
  getConfirmPreview: () => ({ ok: true, paused: true, targets: [], unmatched: [] }),
  confirmPlacementDefault: () => ({ ok: true }),
  setItemPlacement: (...args) => ({ ok: true, dropped: [], placement: applied(args[1] as PlacementChange) }),
  previewItemPlacement: () => ({ ok: true, added: [], dropped: [] }),
  getNewTargetPreview: () => ({ ok: true, preview: targetPreview() }),
  excludeFromTarget: () => ({ ok: true }),
  getDirectRepo: () => ({ ok: true, repo: null, suggestion: { location: "b2:bucket:containers-direct", note: "" } }),
  testDirectLocation: () => ({ ok: true, reachable: true, initialized: false }),
  createDirectRepo: (...args) => ({
    ok: true,
    repo: namedRepo({
      id: "repo-direct",
      // bv-convention-exception: user-message-is-translated -- fixture data for
      // a test double, standing in for the server's own naming; never rendered.
      name: (args[1] as string) || "B2 direct",
      repo: args[2] as string,
      companionOf: args[0] as string,
    }),
  }),
  connectRepo: (...args) => ({ ok: true, repo: namedRepo({ id: args[0] as string, companionOf: args[1] as string }) }),
  listRepos: () => ({ ok: true, repos: [] }),
  createFileSet: () => ({ ok: true, id: "set-new" }),
  createOffsiteTarget: (...args) => ({ ok: true, target: { ...(args[0] as object), id: "t-new", createdAt: 1 } }),
  updateOffsiteTarget: (...args) => ({ ok: true, target: args[1], warnings: [] }),
  acceptMeshOffer: () => ({ ok: true }),
  getSettings: () => ({ ok: true, platform: "unraid", hostMountRoot: "/host/user" }),
};

export interface ApiCall {
  fn: string;
  args: unknown[];
}

export interface PlacementApiFake {
  api: Record<string, (...args: unknown[]) => Promise<unknown>>;
  calls: ApiCall[];
  callsTo: (fn: string) => unknown[][];
  reply: (fn: string, ...replies: unknown[]) => void;
  hold: (fn: string) => (reply?: unknown) => void;
  maxInFlight: (fn: string) => number;
  reset: () => void;
}

export function createPlacementApi(): PlacementApiFake {
  const calls: ApiCall[] = [];
  const queued = new Map<string, unknown[]>();
  const gates = new Map<string, Promise<unknown>[]>();
  const inFlight = new Map<string, number>();
  const peak = new Map<string, number>();

  const api: PlacementApiFake["api"] = {};
  for (const [fn, fallback] of Object.entries(DEFAULTS)) {
    api[fn] = async (...args) => {
      calls.push({ fn, args });
      const reply = queued.get(fn)?.shift() ?? fallback(...args);
      const gate = gates.get(fn)?.shift();
      const now = (inFlight.get(fn) ?? 0) + 1;
      inFlight.set(fn, now);
      peak.set(fn, Math.max(peak.get(fn) ?? 0, now));
      try {
        const released = gate ? await gate : undefined;
        return released ?? reply;
      } finally {
        inFlight.set(fn, (inFlight.get(fn) ?? 1) - 1);
      }
    };
  }

  return {
    api,
    calls,
    callsTo: (fn) => calls.filter((c) => c.fn === fn).map((c) => c.args),
    reply: (fn, ...replies) => queued.set(fn, [...(queued.get(fn) ?? []), ...replies]),
    hold: (fn) => {
      let open: (reply?: unknown) => void = () => undefined;
      const gate = new Promise<unknown>((resolve) => {
        open = resolve;
      });
      gates.set(fn, [...(gates.get(fn) ?? []), gate]);
      return (reply) => open(reply);
    },
    maxInFlight: (fn) => peak.get(fn) ?? 0,
    reset: () => {
      calls.length = 0;
      queued.clear();
      gates.clear();
      inFlight.clear();
      peak.clear();
    },
  };
}

export function renderWithProviders(ui: ReactElement): RenderResult {
  return render(createElement(I18nProvider, { children: createElement(ToastProvider, { children: ui }) }));
}

/** Sends wheel notches downwards to a picker's trigger, the way a mouse does. */
export function wheel(el: Element, notches: number): void {
  for (let i = 0; i < notches; i++) {
    fireEvent.wheel(el, { deltaY: 100 });
  }
}

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}

/** jsdom has no EventSource, and cards subscribe to the progress stream. */
export function stubEventSource(): void {
  (globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;
}
