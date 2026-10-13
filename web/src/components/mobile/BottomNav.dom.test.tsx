// @vitest-environment jsdom
// BottomNav's dom contract; the bar-card and the More trigger.
//
// The geometry half of the bar-card (insets, radius, safe areas) is CSS and
// jsdom computes no layout, so the observable form here is the class token:
// the host paints nothing (transparent, no border anywhere on the bar) and
// the card carries the sidebar surface token and the card radius; the same
// targeted-token discipline BottomSheet.dom.test.tsx states for its own
// styling contracts. What is behavioural here:
//   - the More trigger is a disclosure: aria-haspopup="dialog" and
//     aria-expanded that tracks the sheet's open state;
//   - the More trigger reads as active (filled accent) exactly while the
//     current route belongs to one of the sheet's entries (Anomalies, the
//     storage locations, Instances) and never while a bar destination is
//     current;
//   - Overview carries the count of what needs the person, in grey, and in the
//     ink of the fill on the current slot;
//   - every slot carries the colour engine (glim-hue + its own --item-hue)
//     with the active/filled slot additionally carrying glim-active;
//   - in glyph mode the slot's name lives in aria-label and opens the app's
//     tip bubble (lib/useTipBubble), the sidebar's mechanism; the native
//     title attribute is retired.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { BottomNav } from "./BottomNav";
import { I18nProvider } from "../../lib/i18n";
import { hueVars } from "../../lib/appearance";
import { AttentionProvider } from "../../lib/attentionProvider";
import { AnomalyProvider } from "../../lib/useAnomalies";
import type { AnomalySummary, BackupItem, Settings } from "../../lib/api";

const getAnomalySummary = vi.fn();
const listItems = vi.fn();
vi.mock("../../lib/api", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  getAnomalySummary: () => getAnomalySummary(),
  listRuns: async () => ({ ok: true, runs: [] }),
  listItems: () => listItems(),
}));

const PINS_KEY = "bombvault.navPins";

function draw(path: string, settings: Settings | null = null) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nProvider>
        <BottomNav settings={settings} authEnabled={false} scrollMainToTop={() => undefined} />
      </I18nProvider>
    </MemoryRouter>,
  );
}

function bar() {
  return screen.getByTestId("bottom-nav");
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(cleanup);

describe("BottomNav bar-card", () => {
  it("the host paints nothing and carries no line: transparent ground, zero border tokens", () => {
    // Separation by shade, never by a line; any border/ring class on the
    // host would reintroduce the strip-with-a-line-on-top the bar-card
    // language replaced.
    draw("/dashboard");
    const host = bar();
    expect(host.className).toContain("bg-transparent");
    expect(host.className).not.toMatch(/border-\S+/);
    expect(host.className).not.toMatch(/ring-\S+/);
    // The bottom safe area belongs to the host (the card floats above it).
    expect(host.className).toContain("pb-[var(--safe-area-bottom)]");
  });

  it("the card carries the sidebar surface token and the card radius, still with no line", () => {
    draw("/dashboard");
    const card = bar().querySelector("div.rounded-card");
    expect(card).not.toBeNull();
    expect(card!.className).toContain("bg-carbon-sidebar");
    expect(card!.className).not.toMatch(/border-\S+/);
  });

  it("renders Overview, Backups, Restore and Settings, in that order, plus the More trigger", () => {
    draw("/dashboard");
    const slots = bar().querySelectorAll("div.grid.h-14 > *");
    expect(slots).toHaveLength(5);
    expect(within(bar()).getAllByRole("link").map((slot) => slot.textContent)).toEqual([
      "Overview",
      "Backups",
      "Restore",
      "Settings",
    ]);
    expect(slots[4]).toBe(within(bar()).getByRole("button", { name: "More" }));
  });

  it("keeps the same four slots whatever backup types, modules and pins are on", () => {
    localStorage.setItem(PINS_KEY, JSON.stringify({ storage: true, instances: true }));
    draw("/dashboard", {
      anomalyEnabled: true,
      vmsEnabled: true,
      filesEnabled: true,
      fleetEnabled: true,
    } as Settings);
    expect(within(bar()).getAllByRole("link").map((slot) => slot.getAttribute("href"))).toEqual([
      "/dashboard",
      "/backups",
      "/restore",
      "/settings",
    ]);
  });
});

describe("BottomNav colour engine", () => {
  it("every slot carries glim-hue and its own --item-hue, pairwise distinct", () => {
    draw("/dashboard");
    const slots = Array.from(bar().querySelectorAll("div.grid.h-14 > *")) as HTMLElement[];
    const hues = slots.map((s) => s.style.getPropertyValue("--item-hue"));
    for (const hue of hues) {
      expect(hue).toMatch(/^var\(--rb-[0-7]\)$/);
    }
    expect(new Set(hues).size).toBe(hues.length);
    for (const slot of slots) {
      expect(slot.className).toContain("glim-hue");
    }
  });

  it("the active destination slot fills with the accent and carries glim-active; resting slots do not", () => {
    draw("/dashboard");
    const active = within(bar()).getByRole("link", { name: "Overview" });
    expect(active.className).toContain("bg-accent");
    expect(active.className).toContain("text-accentContrast");
    expect(active.className).toContain("glim-active");
    const resting = within(bar()).getByRole("link", { name: "Backups" });
    expect(resting.className).not.toContain("bg-accent");
    expect(resting.className).not.toContain("glim-active");
  });
});

describe("BottomNav More trigger", () => {
  it("announces a dialog and tracks the sheet's open state in aria-expanded", () => {
    draw("/dashboard");
    const trigger = within(bar()).getByRole("button", { name: "More" });
    expect(trigger.getAttribute("aria-haspopup")).toBe("dialog");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
  });

  it("closes the sheet on Back and stays on the page", async () => {
    // A sheet left open by an earlier test gives its entry back on unmount.
    await new Promise((resolve) => setTimeout(resolve, 20));
    draw("/backups");
    const trigger = within(bar()).getByRole("button", { name: "More" });
    fireEvent.click(trigger);
    expect(screen.getByTestId("more-sheet")).toBeTruthy();
    act(() => window.history.back());
    await waitFor(() => expect(screen.queryByTestId("more-sheet")).toBeNull());
    expect(within(bar()).getByRole("link", { name: "Backups" }).className).toContain("bg-accent");
  });

  it("closes the sheet when the page changes under it", () => {
    draw("/backups");
    fireEvent.click(within(bar()).getByRole("button", { name: "More" }));
    expect(screen.getByTestId("more-sheet")).toBeTruthy();
    fireEvent.click(within(bar()).getByRole("link", { name: "Restore" }));
    expect(screen.queryByTestId("more-sheet")).toBeNull();
  });

  it("is filled while one of the sheet's pages is current, not while a bar destination is", () => {
    draw("/storage");
    const onMoreRoute = within(bar()).getByRole("button", { name: "More" });
    expect(onMoreRoute.className).toContain("bg-accent");
    expect(onMoreRoute.className).toContain("glim-active");
    // The state is announced as well as painted, so it survives a screen
    // reader and forced colours; the destination slots get the same from
    // NavLink.
    expect(onMoreRoute.getAttribute("aria-current")).toBe("true");
    for (const barRoute of ["/dashboard", "/settings"]) {
      cleanup();
      draw(barRoute);
      const onBarRoute = within(bar()).getByRole("button", { name: "More" });
      expect(onBarRoute.className, barRoute).not.toContain("bg-accent");
      expect(onBarRoute.className, barRoute).not.toContain("glim-active");
      expect(onBarRoute.getAttribute("aria-current"), barRoute).toBeNull();
    }
  });

  it("stays filled on a page below one of the sheet's entries", () => {
    draw("/storage/local");
    const trigger = within(bar()).getByRole("button", { name: "More" });
    expect(trigger.className).toContain("bg-accent");
    expect(trigger.getAttribute("aria-current")).toBe("true");
  });

  it("is filled on a gated route exactly while its gate is open: the registry decides, not the path", () => {
    draw("/anomalies", { anomalyEnabled: true } as Settings);
    const detectionOn = within(bar()).getByRole("button", { name: "More" });
    expect(detectionOn.className).toContain("bg-accent");
    expect(detectionOn.getAttribute("aria-current")).toBe("true");
    cleanup();
    draw("/anomalies");
    const detectionOff = within(bar()).getByRole("button", { name: "More" });
    expect(detectionOff.className).not.toContain("bg-accent");
    expect(detectionOff.getAttribute("aria-current")).toBeNull();
    cleanup();

    localStorage.setItem(PINS_KEY, JSON.stringify({ instances: true }));
    draw("/instances");
    expect(within(bar()).getByRole("button", { name: "More" }).getAttribute("aria-current")).toBe("true");
    cleanup();
    localStorage.setItem(PINS_KEY, JSON.stringify({ instances: false }));
    draw("/instances");
    expect(within(bar()).getByRole("button", { name: "More" }).getAttribute("aria-current")).toBeNull();
  });

  it("hands the sheet a rotation position that continues the bar's, not a restart", () => {
    draw("/dashboard");
    const slots = Array.from(bar().querySelectorAll("div.grid.h-14 > *")) as HTMLElement[];
    // The DOM row ends with the More trigger; the destination slots are
    // everything before it, so the trigger's own position is the count of
    // destinations and the sheet's first row is the one after the trigger.
    const triggerHue = slots[slots.length - 1].style.getPropertyValue("--item-hue");
    fireEvent.click(within(bar()).getByRole("button", { name: "More" }));
    const firstRow = screen.getByTestId("more-sheet").querySelector("a") as HTMLElement;
    const firstRowHue = firstRow.style.getPropertyValue("--item-hue");
    // Restarting at the palette's first colour would repeat the bar's first
    // slot's colour on the same screen.
    expect(firstRowHue).not.toBe(triggerHue);
    expect(firstRowHue).toBe(String(hueVars(slots.length)["--item-hue"]));
  });
});

// The bar's own label axis ("bottombar", lib/controls.ts); behaviour, not
// just the source asserts mobileShellSource.test.ts carries. The four modes
// change what a slot's caption is: visible text, or sr-only words behind an
// aria-label, or the reactive at-rest reveal. The contract that matters most:
// a hiding mode never leaves a slot unnamed; the caption's words move into
// aria-label (and open the tip bubble), so a glyph bar is never eleven
// unnamed pictures. jsdom computes no layout, and nothing here needs it: the
// modes' observable surface in a dom environment is exactly classes and
// attributes. The axis preference arrives the way the real app delivers it;
// the localStorage key getLabelMode reads; never by poking the hook.
describe("BottomNav label axis (bottombar)", () => {
  const AXIS_KEY = "bv-labels-bottombar";

  afterEach(() => {
    localStorage.removeItem(AXIS_KEY);
  });

  function drawInMode(mode: string) {
    localStorage.setItem(AXIS_KEY, mode);
    return draw("/dashboard");
  }

  function allSlots(): HTMLElement[] {
    return Array.from(bar().querySelectorAll("div.grid.h-14 > *")) as HTMLElement[];
  }

  it("text mode shows every caption and names slots through their text alone", () => {
    drawInMode("text");
    const slots = allSlots();
    expect(slots).toHaveLength(5);
    for (const slot of slots) {
      expect(slot.querySelector("span.sr-only")).toBeNull();
      expect(slot.querySelector("span.glim-label-reactive")).toBeNull();
      expect(slot.getAttribute("aria-label")).toBeNull();
      expect(slot.getAttribute("title")).toBeNull();
      expect(slot.style.getPropertyValue("--reactive-chars")).toBe("");
    }
    expect(within(bar()).getByRole("link", { name: "Overview" })).toBeTruthy();
  });

  it("glyph mode hides the captions and keeps every slot named (aria-label plus the tip bubble)", () => {
    drawInMode("glyph");
    for (const slot of allSlots()) {
      expect(slot.querySelector("span.sr-only")).not.toBeNull();
      expect(slot.getAttribute("aria-label")).toBeTruthy();
      // The native title is retired here the way the sidebar retired it
      // (lib/useTipBubble): the slot opens the app's own bubble on hover and
      // focus, and the bubble says what the aria-label says.
      expect(slot.getAttribute("title")).toBeNull();
      fireEvent.mouseEnter(slot);
      const bubble = document.body.querySelector(".glim-bubble");
      expect(bubble).not.toBeNull();
      expect(bubble!.textContent).toBe(slot.getAttribute("aria-label"));
      fireEvent.mouseLeave(slot);
      expect(document.body.querySelector(".glim-bubble")).toBeNull();
    }
    // The accessible names survive the hiding: role queries resolve through
    // aria-label exactly as assistive tech reads them.
    for (const name of ["Overview", "Backups", "Restore", "Settings"]) {
      expect(within(bar()).getByRole("link", { name })).toBeTruthy();
    }
    expect(within(bar()).getByRole("button", { name: "More" })).toBeTruthy();
  });

  it("reactive mode arms the coarse at-rest reveal: glim-reactive slots, reactive captions, sized ceilings", () => {
    drawInMode("reactive");
    for (const slot of allSlots()) {
      // Hiding mode, so the naming contract from glyph mode still holds.
      expect(slot.getAttribute("aria-label")).toBeTruthy();
      expect(slot.className).toContain("glim-reactive");
      // The caption is neither shown flat nor sr-only: it is the reactive
      // label the index.css coarse block reveals at rest.
      const caption = slot.querySelector("span.glim-label-reactive");
      expect(caption).not.toBeNull();
      expect(slot.querySelector("span.sr-only")).toBeNull();
      // The reveal's ceiling comes from the label's own length; the slot
      // carries the per-label width var the CSS formula consumes.
      expect(slot.style.getPropertyValue("--reactive-chars")).not.toBe("");
    }
  });
});

describe("BottomNav's More trigger and open findings", () => {
  function summary(open: AnomalySummary["open"]): AnomalySummary {
    return {
      enabled: true,
      ready: true,
      generation: 1,
      open,
      recoveredCritical: 0,
      learningItems: 0,
      retentionHeld: 0,
      evalErrors: 0,
      notifyMuted: false,
      backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
      unmeasuredVolumes: [],
    };
  }

  async function drawWithFindings(open: AnomalySummary["open"], anomalyEnabled = true) {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary(open) });
    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <I18nProvider>
          <AnomalyProvider>
            <BottomNav settings={{ anomalyEnabled } as Settings} authEnabled={false} scrollMainToTop={() => undefined} />
          </AnomalyProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
    await act(async () => {});
    return within(bar()).getByRole("button", { name: "More" });
  }

  it("marks the trigger in the fail tone while a critical finding is open", async () => {
    const more = await drawWithFindings({ critical: 1, warning: 2, info: 0 });
    const dot = within(more).getByTestId("more-anomaly-dot");
    expect(dot.className).toContain("bg-statusFailSolid");
    expect(dot.getAttribute("aria-hidden")).toBe("true");
  });

  it("marks it in the warning tone when only warnings are open", async () => {
    const more = await drawWithFindings({ critical: 0, warning: 2, info: 0 });
    expect(within(more).getByTestId("more-anomaly-dot").className).toContain("bg-statusWarnSolid");
  });

  it("leaves it unmarked for notes alone", async () => {
    const more = await drawWithFindings({ critical: 0, warning: 0, info: 3 });
    expect(within(more).queryByTestId("more-anomaly-dot")).toBeNull();
  });

  it("leaves it unmarked while detection is off, since the sheet has no Anomalies row then", async () => {
    const more = await drawWithFindings({ critical: 1, warning: 0, info: 0 }, false);
    expect(within(more).queryByTestId("more-anomaly-dot")).toBeNull();
  });

  it("leaves it unmarked before the first backup", async () => {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ critical: 1, warning: 0, info: 0 }) });
    listItems.mockResolvedValue({ ok: true, items: [], unlisted: [] });
    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <I18nProvider>
          <AnomalyProvider>
            <AttentionProvider settings={null}>
              <BottomNav settings={{ anomalyEnabled: true } as Settings} authEnabled={false} scrollMainToTop={() => undefined} />
            </AttentionProvider>
          </AnomalyProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
    await act(async () => {});
    expect(within(bar()).queryByTestId("more-anomaly-dot")).toBeNull();
  });
});

describe("BottomNav's count on Overview", () => {
  const LABEL = "Things that need you: 2";

  /** A container no schedule reaches and that has no backup. */
  const leftOut = (name: string): BackupItem => ({
    kind: "container",
    key: name,
    name,
    included: false,
    paused: false,
    effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: 1,
    installed: true,
    runs: [],
  });

  const plex = { ...leftOut("plex"), lastBackup: 1_700_000_000, included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } } satisfies BackupItem;

  async function drawWithUnprotected(path: string, mode?: string, entries = [leftOut("gitea"), leftOut("immich"), plex]) {
    if (mode) localStorage.setItem("bv-labels-bottombar", mode);
    listItems.mockResolvedValue({ ok: true, items: entries, unlisted: [] });
    render(
      <MemoryRouter initialEntries={[path]}>
        <I18nProvider>
          <AttentionProvider settings={null}>
            <BottomNav settings={null} authEnabled={false} scrollMainToTop={() => undefined} />
          </AttentionProvider>
        </I18nProvider>
      </MemoryRouter>,
    );
    await act(async () => {});
  }

  it("is grey on a resting slot", async () => {
    await drawWithUnprotected("/backups");
    const count = within(bar()).getByLabelText(LABEL);
    expect(count.textContent).toBe("2");
    expect(count.closest("a")?.getAttribute("href")).toBe("/dashboard");
    // index.css rests `.glim-nav-count--quiet` on grey and gives it the fill's
    // ink under `.glim-active`.
    expect(count.className).toContain("glim-nav-count--quiet");
    expect(count.className).not.toMatch(/status|\bbg-/);
    expect(count.closest(".glim-active")).toBeNull();
  });

  it("stands inside the filled slot on the current tab", async () => {
    await drawWithUnprotected("/dashboard");
    const count = within(bar()).getByLabelText(LABEL);
    expect(count.closest(".glim-active")).toBe(count.closest("a"));
  });

  it("is part of the slot's name when the caption is hidden", async () => {
    await drawWithUnprotected("/backups", "glyph");
    expect(within(bar()).getByRole("link", { name: `Overview, ${LABEL}` })).toBeTruthy();
  });

  it("is absent before the first backup", async () => {
    await drawWithUnprotected("/backups", undefined, [leftOut("gitea"), leftOut("immich")]);
    expect(within(bar()).queryByLabelText(/Things that need you/)).toBeNull();
  });

  it("shows on no other slot and is absent while nothing needs the person", async () => {
    await drawWithUnprotected("/backups");
    expect(within(bar()).getAllByLabelText(/Things that need you/)).toHaveLength(1);
    cleanup();
    draw("/backups");
    expect(within(bar()).queryByLabelText(/Things that need you/)).toBeNull();
  });
});
