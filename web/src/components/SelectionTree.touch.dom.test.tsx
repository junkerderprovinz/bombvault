// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// SelectionTree TOUCH-MODE twins — the interaction-layer
// contract of `interactionMode="touch"`, pinned against the SAME component
// the desktop suites exercise (never a fork: the whole point is that this
// mode is a prop on the ONE tree).
//
// What these tests own that the desktop suites do not:
//   - tap-to-toggle: a row click toggles through onToggle (the ONE toggle
//     pipeline) instead of expanding — and two taps ROUND-TRIP the check,
//     the executable refutation of the checkbox double-fire hazard (research
//     Pitfall 1: a second, live toggle surface under the row would make two
//     taps a no-op).
//   - the chevron is a real, labelled >=44x44 button that expands WITHOUT
//     toggling — the dedicated-zone half of the touch contract (expand and
//     check never share a hit area).
//   - the checkbox is purely presentational: pointer-events-none so real
//     taps fall through to the row, and no onChange — clicking the input
//     itself toggles nothing.
//   - the pointer default is UNTOUCHED: without the prop the row click
//     expands (never toggles) and the checkbox stays the live control —
//     the byte-identical-desktop guarantee in its most observable form.
//
// Harness: the direct-render TreeHarness shape from SelectionTree.dom
// .test.tsx, but with a STATEFUL includes mirror on the harness side so a
// tap's optimistic round-trip is real state arithmetic, not a spy count.
// jsdom has no scrollIntoView — stubbed, same as the keyboard suite
// (focusNode calls it on every tap).
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { I18nProvider } from "../lib/i18n";
import type { BrowseResponse, MountInfo } from "../lib/api";

let browseCalls: string[] = [];
// Paths the mocked server refuses to read — drives the error-notice branch
// (the retry Button's >=44px touch sizing) without a second mock shape.
const rejectPaths = new Set<string>();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    browse: (path: string) => {
      browseCalls.push(path);
      if (rejectPaths.has(path)) return Promise.reject(new Error("refused by test"));
      const reply: BrowseResponse = {
        ok: true,
        status: "ok",
        truncated: false,
        dirs: [{ name: "library", path: "user/appdata/plex/library" }],
      };
      return Promise.resolve(reply);
    },
  };
});

// Imported AFTER vi.mock so the tree picks up the mocked client.
const { SelectionTree } = await import("./SelectionTree");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";
// A second mount so the roving-tabindex test has a NON-tab-target row to tap
// (with one mount, the tap target and the tabindex home are the same row).
const MOUNT_B = "/mnt/user/appdata/jellyfin";

const MOUNTS: MountInfo[] = [
  { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
];

const TWO_MOUNTS: MountInfo[] = [
  ...MOUNTS,
  { source: MOUNT_B, dest: "/data", selected: true, isAppdata: false, reachable: true },
];

/** Stateful tree: `includes` mirrors what the taps mutate, exactly the way
 *  FoldersEditor's mirror does, so aria-checked round-trips are observable
 *  through the REAL classifyNode path. */
function TouchHarness({
  taps,
  mounts = MOUNTS,
  initial = [MOUNT],
}: {
  taps: string[];
  mounts?: MountInfo[];
  initial?: string[];
}) {
  const [includes, setIncludes] = useState<Set<string>>(() => new Set(initial));
  const toggle = (hostPath: string) => {
    taps.push(hostPath);
    setIncludes((prev) => {
      const next = new Set(prev);
      if (next.has(hostPath)) next.delete(hostPath);
      else next.add(hostPath);
      return next;
    });
  };
  return (
    <SelectionTree
      mounts={mounts}
      customPaths={[]}
      includes={includes}
      exclusions={new Set()}
      hostSourceRoot={HOST_ROOT}
      containerName="touch"
      browseCache={new Map()}
      onToggle={toggle}
      onRemoveCustom={() => {}}
      interactionMode="touch"
    />
  );
}

/** The DESKTOP control harness: no interactionMode prop at all — the default
 *  the Files-page mount (and every existing caller) rides. */
function PointerHarness({ taps }: { taps: string[] }) {
  return (
    <SelectionTree
      mounts={MOUNTS}
      customPaths={[]}
      includes={new Set([MOUNT])}
      exclusions={new Set()}
      hostSourceRoot={HOST_ROOT}
      containerName="pointer"
      browseCache={new Map()}
      onToggle={(p) => taps.push(p)}
      onRemoveCustom={() => {}}
    />
  );
}

function renderTouch(taps: string[]): void {
  render(
    <I18nProvider>
      <TouchHarness taps={taps} />
    </I18nProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls = [];
  rejectPaths.clear();
  // focusNode scrollIntoViews on every tap; jsdom does not implement it.
  Element.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  cleanup();
});

describe("SelectionTree touch mode: tap-to-toggle", () => {
  it("tapping the row toggles the check through onToggle", async () => {
    const taps: string[] = [];
    renderTouch(taps);

    const row = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    expect(row.getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.click(row);
    });

    expect(taps).toEqual([MOUNT]);
    expect(screen.getByRole("treeitem", { name: /appdata\/plex/ }).getAttribute("aria-checked")).toBe("false");
  });

  it("tapping twice round-trips the check (never a zero-change double-fire)", async () => {
    const taps: string[] = [];
    renderTouch(taps);
    const name = /appdata\/plex/;

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name }));
    });
    expect(screen.getByRole("treeitem", { name }).getAttribute("aria-checked")).toBe("false");

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name }));
    });
    expect(screen.getByRole("treeitem", { name }).getAttribute("aria-checked")).toBe("true");

    // Two taps, two toggles — the double-fire hazard (Pitfall 1) would land
    // back on "true" after the FIRST tap, i.e. zero net change per tap.
    expect(taps).toEqual([MOUNT, MOUNT]);
  });

  it("guarded rows cannot toggle: the exact Space guard gates the tap too", async () => {
    const taps: string[] = [];
    render(
      <I18nProvider>
        <SelectionTree
          mounts={[{ ...MOUNTS[0], reachable: false }]}
          customPaths={[]}
          includes={new Set()}
          exclusions={new Set()}
          hostSourceRoot={HOST_ROOT}
          containerName="touch-guarded"
          browseCache={new Map()}
          onToggle={(p) => taps.push(p)}
          onRemoveCustom={() => {}}
          interactionMode="touch"
        />
      </I18nProvider>,
    );

    // An unreachable mount renders with NO tap handler at all — the same
    // rule Space obeys (`!spec.unreachable && !busyPaths?.has(...)`) is the
    // rule the finger obeys (one guard, one toggle semantics).
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /appdata\/plex/ }));
    });
    expect(taps).toEqual([]);
  });
});

describe("SelectionTree touch mode: the chevron zone", () => {
  it("expands through the labelled chevron button without toggling", async () => {
    const taps: string[] = [];
    renderTouch(taps);

    const row = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    // (4) The chevron EXISTS as a real control with an accessible name —
    // the desktop glyph is aria-hidden decoration; the touch button is
    // announced. The carried 06-UI-REVIEW fix 3b: the name composes the
    // row's HOST PATH with the action word ("{path} Expand", the wave-1
    // common.expand key) — the action alone names no row.
    const chevron = within(row).getByRole("button", { name: `${MOUNT} Expand` });
    expect(row.getAttribute("aria-expanded")).toBe("false");

    await act(async () => {
      fireEvent.click(chevron);
    });

    // Expansion happened through the lazy browse…
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /appdata\/plex/ }).getAttribute("aria-expanded")).toBe("true");
    // …and the check did NOT move: the two gestures never share a pipeline.
    expect(taps).toEqual([]);
    expect(screen.getByRole("treeitem", { name: /appdata\/plex/ }).getAttribute("aria-checked")).toBe("true");
    // The name follows the state ("{path} Collapse" once open) — same key
    // pair the chevron is labelled from.
    expect(
      within(screen.getByRole("treeitem", { name: /appdata\/plex/ })).getByRole("button", { name: `${MOUNT} Collapse` }),
    ).toBeTruthy();
  });
});

describe("SelectionTree touch mode: the checkbox is purely presentational (Pitfall 1)", () => {
  it("carries pointer-events-none and toggles nothing when clicked itself", async () => {
    const taps: string[] = [];
    renderTouch(taps);

    const row = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    const box = within(row).getByRole("checkbox", { hidden: true });
    // pointer-events-none is the half that makes REAL taps fall through to
    // the row (the one toggle surface).
    expect(box.className).toContain("pointer-events-none");

    // …and the DOM-level half: with onChange unwired, even a synthetic click
    // on the input (stopPropagation kept) toggles nothing — no dead zone and
    // no second pipeline.
    await act(async () => {
      fireEvent.click(box);
    });
    expect(taps).toEqual([]);
    expect(row.getAttribute("aria-checked")).toBe("true");
  });
});

describe("SelectionTree pointer default is untouched", () => {
  it("row click expands (never toggles); the checkbox stays the live control", async () => {
    const taps: string[] = [];
    render(
      <I18nProvider>
        <PointerHarness taps={taps} />
      </I18nProvider>,
    );

    const row = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    // No chevron button on the desktop tree — the glyph is aria-hidden.
    expect(within(row).queryByRole("button", { name: /expand/i })).toBeNull();

    await act(async () => {
      fireEvent.click(row);
    });
    // Desktop semantics: the row click EXPANDED…
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /appdata\/plex/ }).getAttribute("aria-expanded")).toBe("true");
    // …and did NOT toggle.
    expect(taps).toEqual([]);

    // The checkbox is still the live toggle (no pointer-events-none, real
    // onChange): clicking it flips the check.
    const box = within(screen.getByRole("treeitem", { name: /appdata\/plex/ })).getByRole("checkbox", {
      hidden: true,
    });
    expect(box.className).not.toContain("pointer-events-none");
    await act(async () => {
      fireEvent.click(box);
    });
    expect(taps).toEqual([MOUNT]);
  });
});

describe("SelectionTree touch mode: roving-on-tap (Pitfall 2)", () => {
  it("tapping a non-tab-target row moves the roving tabindex to it, and Space then acts on it", async () => {
    const taps: string[] = [];
    render(
      <I18nProvider>
        <TouchHarness taps={taps} mounts={TWO_MOUNTS} initial={[MOUNT, MOUNT_B]} />
      </I18nProvider>,
    );

    const rowA = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    const rowB = screen.getByRole("treeitem", { name: /appdata\/jellyfin/ });
    // Home state: the FIRST root holds the roving tabindex…
    expect(rowA.getAttribute("tabindex")).toBe("0");
    expect(rowB.getAttribute("tabindex")).toBe("-1");

    // …a tap on B moves it there — focusNode (setFocusPath + scrollIntoView +
    // focus), the exact primitive the arrow keys use, never a re-implementation.
    await act(async () => {
      fireEvent.click(rowB);
    });
    expect(rowB.getAttribute("tabindex")).toBe("0");
    expect(rowA.getAttribute("tabindex")).toBe("-1");
    expect(rowB.className).not.toContain("pointer-events-none");
    expect(taps).toEqual([MOUNT_B]);

    // …and the keyboard now agrees with the finger: Space acts on B through
    // the UNTOUCHED APG handler (no edits to the key map for this to work —
    // that is the whole roving-tabindex point).
    await act(async () => {
      fireEvent.keyDown(rowB, { key: " " });
    });
    expect(taps).toEqual([MOUNT_B, MOUNT_B]);
    // B round-tripped (checked again) while A was never touched by the key.
    expect(screen.getByRole("treeitem", { name: /appdata\/jellyfin/ }).getAttribute("aria-checked")).toBe("true");
  });
});

describe("SelectionTree touch mode: full-row >=44px targets", () => {
  it("touch rows carry min-h-[2.75rem], touch-manipulation and select-none at the 14px register", () => {
    const taps: string[] = [];
    renderTouch(taps);

    const row = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    expect(row.className).toContain("min-h-[2.75rem]");
    expect(row.className).toContain("touch-manipulation");
    expect(row.className).toContain("select-none");
    expect(row.className).toContain("text-sm");
    // 44px REPLACES the depth min-heights on touch — never stacks with them.
    expect(row.className).not.toContain("min-h-8");
    expect(row.className).not.toContain("min-h-7");
  });

  it("pointer rows keep the depth min-heights and the 12px register", async () => {
    const taps: string[] = [];
    render(
      <I18nProvider>
        <PointerHarness taps={taps} />
      </I18nProvider>,
    );

    const root = screen.getByRole("treeitem", { name: /appdata\/plex/ });
    expect(root.className).toContain("min-h-8");
    expect(root.className).toContain("text-xs");
    expect(root.className).not.toContain("min-h-[2.75rem]");
    expect(root.className).not.toContain("touch-manipulation");
    expect(root.className).not.toContain("select-none");

    // Depth-n register: expand the root (desktop row click) and the child
    // row carries min-h-7 — the desktop ladder is intact.
    await act(async () => {
      fireEvent.click(root);
    });
    const child = await screen.findByRole("treeitem", { name: "library" });
    expect(child.className).toContain("min-h-7");
    expect(child.className).not.toContain("min-h-[2.75rem]");
  });
});

describe("SelectionTree touch mode: the retry notice control is a >=44px tap target", () => {
  it("retry Button grows to min-h-[2.75rem] in touch; pointer keeps the engine-sized control", async () => {
    rejectPaths.add("user/appdata/plex");

    const taps: string[] = [];
    renderTouch(taps);
    // Expand through the chevron; the browse refuses, so the error notice
    // row renders with its retry control. Notice copy stays verbatim.
    await act(async () => {
      fireEvent.click(
        within(screen.getByRole("treeitem", { name: /appdata\/plex/ })).getByRole("button", {
          name: `${MOUNT} Expand`,
        }),
      );
    });
    const retry = await screen.findByRole("button", { name: "Try again" });
    expect(retry.className).toContain("min-h-[2.75rem]");
    expect(screen.getByText("Could not read directory")).toBeTruthy();

    // Pointer control: the SAME failure renders the SAME notice with the
    // control at its engine size — the touch bump is interaction-mode-only.
    cleanup();
    render(
      <I18nProvider>
        <PointerHarness taps={[]} />
      </I18nProvider>,
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /appdata\/plex/ }));
    });
    const retryPointer = await screen.findByRole("button", { name: "Try again" });
    expect(retryPointer.className).not.toContain("min-h-[2.75rem]");
  });
});
