// @vitest-environment jsdom
// The dots of what is new: one on every changed element in view, one on what
// leads to a change elsewhere, and none where nothing can be new.
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useContext, type ReactNode } from "react";
import { MemoryRouter, Link, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AttentionContext } from "../lib/attention";
import { I18nProvider, en } from "../lib/i18n";
import { CHANGES, WHATS_NEW_RELEASE } from "../lib/whatsNew";
import { beginWhatsNew, markSeen, showDots } from "../lib/whatsNewStore";

let desktop = true;
vi.mock("../lib/useMediaQuery", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/useMediaQuery")>()),
  useIsDesktop: () => desktop,
}));

const { WhatsNewDots } = await import("./WhatsNewDots");

const change = (id: string) => CHANGES.find((c) => c.id === id)!;
const text = (id: string) => en[change(id).text];
const ON_OVERVIEW = CHANGES.filter((c) => c.page === "/dashboard").map((c) => c.id);
const ON_BACKUPS = CHANGES.filter((c) => c.way[0] === "/backups").map((c) => c.id);

function Shell({ fresh = false, loaded = true, children }: { fresh?: boolean; loaded?: boolean; children: ReactNode }) {
  const base = useContext(AttentionContext);
  return (
    <AttentionContext.Provider value={{ ...base, fresh, items: { ...base.items, loaded } }}>
      {children}
      <WhatsNewDots />
    </AttentionContext.Provider>
  );
}

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

async function renderAt(path: string, page: ReactNode, shell: { fresh?: boolean; loaded?: boolean } = {}) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <Shell {...shell}>
            <nav>
              <Link to="/dashboard">Overview</Link>
              <Link to="/backups">Backups</Link>
              <Link to="/settings">Settings</Link>
            </nav>
            <main id="bv-main">{page}</main>
            <Where />
          </Shell>
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

const dots = () => [...document.querySelectorAll<HTMLElement>("[data-new-dot]")];
const dotOf = (id: string) => document.querySelector<HTMLElement>(`[data-new-dot="${id}"]`);
const wayDots = () => dots().filter((dot) => dot.dataset.newDot === "way");

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  desktop = true;
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  // jsdom lays nothing out, so every element reports a box and the frame the
  // dots wait for runs at once.
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 20)] as unknown as DOMRectList);
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 100, 20));
  vi.spyOn(window, "requestAnimationFrame").mockImplementation((run) => {
    run(0);
    return 0;
  });
  beginWhatsNew("v9.9.1");
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const overview = (
  <>
    <span data-new="overview-head">ring</span>
    <span data-new="overview-customize">Customize</span>
  </>
);

describe("a change on the page", () => {
  it("carries a dot that says what changed", async () => {
    await renderAt("/dashboard", overview);
    expect(dotOf("overview-head")?.getAttribute("aria-label")).toBe(text("overview-head"));
    expect(dotOf("overview-customize")).not.toBeNull();
  });

  it("loses its dot at a click, and stays without it", async () => {
    await renderAt("/dashboard", overview);
    await press(dotOf("overview-head")!);
    expect(dotOf("overview-head")).toBeNull();
    expect(dotOf("overview-customize")).not.toBeNull();

    cleanup();
    await renderAt("/dashboard", overview);
    expect(dotOf("overview-head")).toBeNull();
  });

  it("is reached by the keyboard like a button", async () => {
    await renderAt("/dashboard", overview);
    const dot = dotOf("overview-head")!;
    expect(dot.tagName).toBe("BUTTON");
    dot.focus();
    expect(document.activeElement).toBe(dot);
  });

  it("gets its dot when the page draws it later", async () => {
    await renderAt("/dashboard", <div id="late" />);
    expect(dotOf("overview-head")).toBeNull();
    await act(async () => {
      document.getElementById("late")!.innerHTML = '<span data-new="overview-head">ring</span>';
      await Promise.resolve();
    });
    expect(dotOf("overview-head")).not.toBeNull();
  });

  it("has no dot for an element only a wide window shows, on a phone", async () => {
    desktop = false;
    await renderAt("/dashboard", overview);
    expect(dotOf("overview-head")).not.toBeNull();
    expect(dotOf("overview-customize")).toBeNull();
  });
});

describe("a change elsewhere", () => {
  it("puts a dot on the link that leads there, which counts and lists the changes", async () => {
    await renderAt("/dashboard", overview);
    const backups = wayDots().find((dot) => dot.getAttribute("aria-label")?.includes(text("backups-tiles")));
    expect(backups).toBeDefined();
    const label = backups!.getAttribute("aria-label")!;
    expect(label.startsWith(en["whatsNew.way"].split("|other=")[1].replace("{n}", String(ON_BACKUPS.length)))).toBe(true);
    for (const id of ON_BACKUPS) expect(label).toContain(text(id));
  });

  it("follows the link at a click and marks nothing as seen", async () => {
    await renderAt("/dashboard", overview);
    await press(wayDots().find((dot) => dot.getAttribute("aria-label")?.includes(text("backups-tiles")))!);
    expect(screen.getByTestId("where").textContent).toBe("/backups");
    expect(JSON.parse(localStorage.getItem(`bv-whats-new@${WHATS_NEW_RELEASE}`)!).seen).toEqual([]);
  });

  it("has no dot on the link to the page that is open", async () => {
    await renderAt("/dashboard", <div />);
    for (const dot of wayDots()) {
      for (const id of ON_OVERVIEW) expect(dot.getAttribute("aria-label")).not.toContain(text(id));
    }
  });

  it("drops the dot on the way once everything behind it has been seen", async () => {
    await renderAt("/dashboard", overview);
    const toBackups = () => wayDots().find((dot) => dot.getAttribute("aria-label")?.includes(text(ON_BACKUPS[ON_BACKUPS.length - 1])));
    await act(async () => markSeen(ON_BACKUPS.slice(0, -1)));
    expect(toBackups()?.getAttribute("aria-label")).toContain(en["whatsNew.way"].split("|")[0].replace("one=", "").replace("{n}", "1"));
    await act(async () => markSeen(ON_BACKUPS));
    expect(toBackups()).toBeUndefined();
  });

  it("marks the row and the tab on the way to a change deeper down", async () => {
    await renderAt(
      "/backups",
      <ul>
        <li data-new-way="entry">plex</li>
        <li data-new-way="entry">sonarr</li>
      </ul>
    );
    const row = wayDots().filter((dot) => dot.getAttribute("aria-label")?.includes(text("entry-page")));
    expect(row).toHaveLength(1);

    cleanup();
    await renderAt(
      "/backups/container/plex",
      <div data-new-way="entry-tabs">
        <button data-sel-id="backups">Backups</button>
        <button data-sel-id="settings">Settings</button>
      </div>
    );
    const tab = wayDots().filter((dot) => dot.getAttribute("aria-label")?.includes(text("entry-defaults")));
    expect(tab).toHaveLength(1);
    expect(tab[0].getAttribute("aria-label")).not.toContain(text("entry-page"));
  });

  it("marks More on a phone for a page its bar has no room for", async () => {
    desktop = false;
    await renderAt("/dashboard", <button data-new-way="more">More</button>);
    const more = wayDots().find((dot) => dot.getAttribute("aria-label")?.includes(text("anomalies-tiles")));
    expect(more).toBeDefined();
    expect(more!.getAttribute("aria-label")).not.toContain(text("backups-tiles"));
  });
});

describe("where nothing is new", () => {
  it("draws no dot during the first start", async () => {
    await renderAt("/dashboard", overview, { fresh: true });
    expect(dots()).toEqual([]);
  });

  it("draws no dot before it knows whether this is a first start", async () => {
    await renderAt("/dashboard", overview, { loaded: false });
    expect(dots()).toEqual([]);
  });

  it("draws no dot on the welcome page", async () => {
    await renderAt("/welcome", overview);
    expect(dots()).toEqual([]);
  });

  it("draws no dot in a browser that never saw an older release", async () => {
    localStorage.clear();
    await renderAt("/dashboard", overview);
    expect(dots()).toEqual([]);
  });

  it("draws no dot while they are switched off, and all of them again when switched on", async () => {
    showDots(false);
    await renderAt("/dashboard", overview);
    expect(dots()).toEqual([]);
    await act(async () => showDots(true));
    expect(dotOf("overview-head")).not.toBeNull();
  });

  it("hides the dots of what a window covers", async () => {
    await renderAt(
      "/dashboard",
      <>
        {overview}
        <div role="dialog" aria-modal="true">
          window
        </div>
      </>
    );
    expect(dots()).toEqual([]);
  });
});
