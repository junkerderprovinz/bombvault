// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Activity log line structure at phone width.
//
// The assertion is structural rather than a measurement: an overflow metric
// on the element itself passes vacuously, because the element grows with its
// content.
// At phone width every log line is two stacked blocks: the fixed prefix (date
// + clock + glyph + domain) on the first line, the message on its own
// full-width line below, so the message column is never as narrow as the
// leftover space after the prefix (a narrow column is what breaks words
// mid-word). The message wraps at word boundaries (wrap-break-word) and
// break-all must not appear anywhere in the log. The desktop face keeps the
// one-line row the phone change must not touch.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { I18nProvider } from "../lib/i18n";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import type { Run } from "../lib/api";
import { ActivityLog } from "./ActivityLog";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: () => Promise.resolve({ ok: true, runs: [makeRun()] }),
    getScheduleNext: () => Promise.resolve([]),
  };
});

/** One finished run: enough for buildLogLines to produce a history line. */
function makeRun(): Run {
  const now = Math.floor(Date.now() / 1000);
  return {
    id: "run-1",
    targetId: "containers:plex",
    kind: "backup",
    status: "success",
    startedAt: now - 60,
    finishedAt: now - 30,
    snapshotId: "abc12345",
    bytes: 1024,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "containers",
  };
}

const mqlListeners = new Set<() => void>();
let desktopMatches = false;

function installMatchMedia() {
  window.matchMedia = ((query: string) => ({
    get matches() {
      return query === DESKTOP_QUERY ? desktopMatches : false;
    },
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: (_type: string, l: () => void) => mqlListeners.add(l),
    removeEventListener: (_type: string, l: () => void) => mqlListeners.delete(l),
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

beforeEach(() => {
  installMatchMedia();
  // The progress singleton opens one EventSource on mount; jsdom has none.
  vi.stubGlobal(
    "EventSource",
    class {
      onmessage: ((ev: unknown) => void) | null = null;
      onerror: ((ev: unknown) => void) | null = null;
      close() {}
    }
  );
  desktopMatches = false;
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function renderLog() {
  return render(
    <I18nProvider>
      <ActivityLog />
    </I18nProvider>
  );
}

/** The scrollable log list (the only max-h-96 scroller in the component). */
function logList(container: HTMLElement): HTMLElement {
  const list = container.querySelector(".max-h-96") as HTMLElement;
  expect(list).toBeTruthy();
  return list;
}

describe("ActivityLog line structure", () => {
  it("at phone width stacks the message under the prefix at full width", async () => {
    renderLog();
    await new Promise((r) => setTimeout(r, 0));
    const list = logList(document.body as HTMLElement);
    expect(list.children.length).toBeGreaterThan(0);
    for (const line of Array.from(list.children)) {
      expect(line.className).toContain("flex-col");
      // Prefix row first, message as its own block below.
      expect(line.children.length).toBe(2);
      const message = line.children[1] as HTMLElement;
      expect(message.className).toContain("w-full");
      expect(message.className).toContain("wrap-break-word");
    }
    expect(list.innerHTML).not.toContain("break-all");
  });

  it("on desktop keeps the one-line row the phone change must not touch", async () => {
    desktopMatches = true;
    renderLog();
    await new Promise((r) => setTimeout(r, 0));
    const list = logList(document.body as HTMLElement);
    expect(list.children.length).toBeGreaterThan(0);
    for (const line of Array.from(list.children)) {
      expect(line.className).toContain("items-start");
      expect(line.className).not.toContain("flex-col");
      const message = line.lastElementChild as HTMLElement;
      expect(message.className).toContain("flex-1");
      expect(message.className).toContain("wrap-break-word");
      expect(message.className).not.toContain("w-full");
    }
  });
});
