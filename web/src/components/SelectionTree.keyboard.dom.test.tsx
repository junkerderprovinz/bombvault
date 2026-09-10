// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The selection tree is fully operable from the keyboard (TREE-05): the APG
// TreeView checkbox-variant key map, pinned through document.activeElement.
//
// The map's easy-to-get-wrong halves (RESEARCH Pattern 4): Right EXPANDS a
// closed node with focus staying on the parent and only moves to the first
// child on a SECOND Right — and while children are still loading there is no
// child to move to, so that second Right is a no-op; Left collapses an open
// node, walks UP from a closed child, and does nothing on a closed level-1
// root; Down/Up/Home/End move focus without ever expanding anything; Enter is
// the expansion default action; Space is the ONLY selection toggler and rides
// the exact same onToggle path as checkbox clicks (threat T-02-10: one toggle
// semantics, so the D-04 guard and the save queue cannot be bypassed by key).
//
// Geometry (Pitfall 7): aria-level/setsize/posinset computed from the flat
// model on EVERY node uniformly — including lazy-loaded deep nodes — and the
// notice rows (loading, truncated, empty) are plain rows outside both the
// treeitem role and the setsize arithmetic. Selection stays aria-checked only
// (never aria-selected), and exactly one treeitem is tabbable at a time.
//
// Harness: the FoldersEditor mock-api shape from SelectionTree.dom.test.tsx
// (so Space assertions can read the PATCH bodies), driven line-for-line in the
// DropdownListbox.keyboard.dom.test.tsx style — scrollIntoView stubbed in
// beforeEach, act-wrapped keyDown on the tree element, activeElement asserts.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, ContainerMountsResponse, OkEnvelope } from "../lib/api";

const browseCalls: string[] = [];
const patches: { name: string; paths: string[]; opts?: { selectionSource?: string } }[] = [];
let mountsReply: ContainerMountsResponse;
// Plain values or promises: a deferred promise pins the per-node loading row,
// which an immediately-resolving mock can never show.
let browseReplies: (BrowseResponse | Promise<BrowseResponse>)[] = [];
let patchReplies: (OkEnvelope | Promise<OkEnvelope>)[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getContainerMounts: () => Promise.resolve(mountsReply),
    browse: (path: string) => {
      browseCalls.push(path);
      const reply = browseReplies.shift() ?? { ok: true, dirs: [], status: "ok", truncated: false };
      return Promise.resolve(reply);
    },
    // The composed PATCH endpoint (plan 03 Task 2) with the flat paths/source
    // view the keyboard Space assertions pin, projected out of the body — the
    // same wire contract the retired setBackupPaths mock captured.
    setContainerTargets: (name: string, body: Record<string, unknown>) => {
      patches.push({
        name,
        paths: (body.backupPaths as string[] | undefined) ?? [],
        opts: body.selectionSource ? { selectionSource: body.selectionSource as string } : undefined,
      });
      const reply = patchReplies.shift() ?? { ok: true };
      return Promise.resolve(reply);
    },
  };
});

// Imported AFTER vi.mock so the components pick up the mocked client.
const { FoldersEditor } = await import("../pages/Containers");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";
const OTHER = "/mnt/user/appdata/other";
const CUSTOM = "/mnt/user/backups";
// A custom path outside the served root: server containment means it can never
// be browsed, so it is the tree's honest LEAF (aria-expanded omitted, Enter and
// Right are no-ops on it).
const OUTSIDE = "/srv/data";

/** A controllable promise: `resolve` releases it from inside act(). */
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

/** Two selected mounts plus two custom includes: every key walk below stays
 *  above the D-04 zero-include floor, and the custom list ends in one true
 *  leaf so end-node behavior is pinnable in the same tree. */
function keyboardMounts(): ContainerMountsResponse {
  return {
    ok: true,
    mounts: [
      { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
      { source: OTHER, dest: "/data", selected: true, isAppdata: false, reachable: true },
    ],
    custom: [
      { path: CUSTOM, exists: true },
      { path: OUTSIDE, exists: true },
    ],
    excluded: [],
    hostMountRoot: "/host/user",
    hostSourceRoot: HOST_ROOT,
  };
}

function plexListing(): BrowseResponse {
  return {
    ok: true,
    status: "ok",
    truncated: false,
    dirs: [
      { name: "transcoding", path: "user/appdata/plex/transcoding" },
      { name: "library", path: "user/appdata/plex/library" },
    ],
  };
}

function transcodingListing(): BrowseResponse {
  return {
    ok: true,
    status: "ok",
    truncated: false,
    dirs: [{ name: "cache", path: "user/appdata/plex/transcoding/cache" }],
  };
}

function otherListing(): BrowseResponse {
  return {
    ok: true,
    status: "ok",
    truncated: false,
    dirs: [{ name: "media", path: "user/appdata/other/media" }],
  };
}

function Harness() {
  const { t } = useT();
  return <FoldersEditor name="keyboard" stack="" open t={t} />;
}

async function renderEditor(): Promise<void> {
  render(
    <I18nProvider>
      <ToastProvider>
        <Harness />
      </ToastProvider>
    </I18nProvider>,
  );
  // Flush the getContainerMounts load effect.
  await act(async () => {});
}

/** The tree element — fetched BY ACCESSIBLE NAME so every test also pins
 *  folders.treeLabel (TREE-05: the tree carries its accessible name). */
function tree(): HTMLElement {
  return screen.getByRole("tree", { name: "Backup folder selection" });
}

function item(name: string | RegExp): HTMLElement {
  return screen.getByRole("treeitem", { name });
}

/** Focus a treeitem the way a keyboard user reaches it. The act() wrapper
 *  matters: focusing fires the row's onFocus (which moves the roving tabindex
 *  in component state), and that commit must land BEFORE the next key press
 *  reads it — exactly the browser's own ordering, where the discrete focus
 *  event is always dispatched before the subsequent keydown. */
async function focusItem(name: string | RegExp): Promise<HTMLElement> {
  const el = item(name);
  await act(async () => {
    el.focus();
  });
  return el;
}

/** One act-wrapped keyDown on the tree element, as a keyboard user fires it
 *  (focus sits on a treeitem; the event bubbles to the tree). */
async function press(key: string): Promise<void> {
  await act(async () => {
    fireEvent.keyDown(tree(), { key });
  });
}

function focused(): Element | null {
  return document.activeElement;
}

beforeEach(() => {
  // jsdom does not implement scrollIntoView; the tree calls it to keep the
  // newly focused row visible. Its absence is not what these tests are about.
  Element.prototype.scrollIntoView = function () {};
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls.length = 0;
  patches.length = 0;
  browseReplies = [];
  patchReplies = [];
  mountsReply = keyboardMounts();
});

afterEach(cleanup);

describe("SelectionTree keyboard map (TREE-05)", () => {
  it("Right expands a closed root with focus staying on the parent, then moves to the first child on a second Right", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight");

    // APG: focus STAYS on the parent while (async) children load.
    expect(root.getAttribute("aria-expanded")).toBe("true");
    expect(focused()).toBe(root);
    expect(item(/transcoding/)).toBeTruthy();

    // Second Right, children now rendered: focus moves to the first child.
    await press("ArrowRight");
    expect(focused()).toBe(item(/transcoding/));
  });

  it("Right while children are loading is a no-op; focus moves once they land", async () => {
    const hold = deferred<BrowseResponse>();
    browseReplies = [hold.promise];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight");
    expect(screen.getByText("Loading…")).toBeTruthy();

    // No child treeitem exists yet — the loading row is NOT focusable.
    await press("ArrowRight");
    expect(focused()).toBe(root);

    await act(async () => {
      hold.resolve(plexListing());
    });
    await press("ArrowRight");
    expect(focused()).toBe(item(/transcoding/));
  });

  it("Left collapses an open node (focus stays), walks up from a closed child, and does nothing on a closed root", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight"); // expand, focus stays
    await press("ArrowRight"); // focus first child
    expect(focused()).toBe(item(/transcoding/));

    // Closed child: Left moves focus to the parent treeitem.
    await press("ArrowLeft");
    expect(focused()).toBe(root);

    // Open root: Left collapses it, focus stays put.
    await press("ArrowLeft");
    expect(root.getAttribute("aria-expanded")).toBe("false");
    expect(focused()).toBe(root);

    // Closed level-1 root: nothing at all.
    const other = await focusItem(/user\/appdata\/other/);
    await press("ArrowLeft");
    expect(focused()).toBe(other);
    expect(other.getAttribute("aria-expanded")).toBe("false");
  });

  it("Down and Up move focus between treeitems in visual order without expanding anything", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight"); // expand plex only

    const transcoding = item(/transcoding/);
    const library = item(/library/);
    const other = item(/user\/appdata\/other/);
    const custom = item(/user\/backups/);

    // Visual order across the one tree: mounts (expanded) then customs.
    await press("ArrowDown");
    expect(focused()).toBe(transcoding);
    await press("ArrowDown");
    expect(focused()).toBe(library);
    await press("ArrowDown");
    expect(focused()).toBe(other);
    await press("ArrowDown");
    expect(focused()).toBe(custom);
    await press("ArrowUp");
    expect(focused()).toBe(other);

    // Arrow moves never expand/collapse: only the deliberate plex expand browsed.
    expect(other.getAttribute("aria-expanded")).toBe("false");
    expect(custom.getAttribute("aria-expanded")).toBe("false");
    expect(browseCalls).toEqual(["user/appdata/plex"]);
  });

  it("Home and End jump to the first and last focusable treeitem across all roots", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight");
    await press("ArrowDown"); // somewhere in the middle

    await press("End");
    expect(focused()).toBe(item(/srv\/data/)); // the last root is a leaf custom
    await press("Home");
    expect(focused()).toBe(root); // the first mount root
  });

  it("Enter toggles expansion on any node; a leaf gets no extra action", async () => {
    browseReplies = [otherListing()];
    await renderEditor();

    const other = await focusItem(/user\/appdata\/other/);
    await press("Enter");
    expect(other.getAttribute("aria-expanded")).toBe("true");
    expect(browseCalls).toEqual(["user/appdata/other"]);
    await press("Enter");
    expect(other.getAttribute("aria-expanded")).toBe("false");

    // The outside-root custom is a leaf: Enter and Right are no-ops, and
    // aria-expanded is honestly OMITTED (APG end-node rule, Pitfall 7).
    const leaf = await focusItem(/srv\/data/);
    await press("Enter");
    expect(focused()).toBe(leaf);
    expect(browseCalls).toEqual(["user/appdata/other"]);
    expect(leaf.hasAttribute("aria-expanded")).toBe(false);
    await press("ArrowRight");
    expect(focused()).toBe(leaf);
    expect(browseCalls).toEqual(["user/appdata/other"]);
  });

  it("Space toggles only the focused node's checkbox and toggling twice returns the exact prior flat list", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight"); // expand

    const before = [OTHER, MOUNT, CUSTOM, OUTSIDE]; // canonical sorted wire order
    const transcoding = await focusItem(/transcoding/);

    // Space = the checkbox: the carve-out PATCH changes for exactly this node
    // (one new "!"+host entry; every bare include untouched).
    await press(" ");
    expect(patches).toEqual([
      {
        name: "keyboard",
        paths: [...before, `!${MOUNT}/transcoding`],
        opts: { selectionSource: "tree" },
      },
    ]);
    expect(transcoding.getAttribute("aria-checked")).toBe("false");
    expect(root.getAttribute("aria-checked")).toBe("mixed");

    // Space again on the same (now excluded) node: the exclusion is released
    // and the wire list is byte-identical to the pre-toggle list.
    await press(" ");
    expect(patches[1].paths).toEqual(before);
    expect(transcoding.getAttribute("aria-checked")).toBe("true");
    expect(root.getAttribute("aria-checked")).toBe("true");
  });

  it("keeps exactly one tabbable treeitem at all times: the focused (or last-focused) node", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    const root = item(/user\/appdata\/plex/);
    // Initially the first root carries the roving tabindex.
    expect(root.tabIndex).toBe(0);
    expect(tree().querySelectorAll('[role="treeitem"][tabindex="0"]').length).toBe(1);

    await act(async () => {
      root.focus();
    });
    await press("ArrowRight");
    await press("ArrowRight"); // focus the first child

    const transcoding = item(/transcoding/);
    expect(focused()).toBe(transcoding);
    expect(transcoding.tabIndex).toBe(0);
    expect(root.tabIndex).toBe(-1);
    expect(tree().querySelectorAll('[role="treeitem"][tabindex="0"]').length).toBe(1);
  });

  it("renders uniform aria geometry on every treeitem, including lazy-loaded deep nodes, and never aria-selected", async () => {
    browseReplies = [plexListing(), transcodingListing()];
    await renderEditor();

    const root = await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight"); // plex children (level 2)
    const transcoding = await focusItem(/transcoding/);
    await press("ArrowRight"); // transcoding children (level 3, lazily loaded)

    // Geometry on the deep lazy node — uniform, not a post-load special case.
    const cache = item(/cache/);
    expect(cache.getAttribute("aria-level")).toBe("3");
    expect(cache.getAttribute("aria-setsize")).toBe("1");
    expect(cache.getAttribute("aria-posinset")).toBe("1");
    expect(cache.getAttribute("aria-checked")).toBe("true");
    expect(cache.getAttribute("aria-expanded")).toBe("false");

    // Every treeitem in the tree: full geometry, checked-only selection.
    const rows = tree().querySelectorAll('[role="treeitem"]');
    expect(rows.length).toBeGreaterThan(4);
    for (const el of rows) {
      expect(el.getAttribute("aria-level")).toMatch(/^[0-9]+$/);
      expect(el.getAttribute("aria-setsize")).toMatch(/^[0-9]+$/);
      expect(el.getAttribute("aria-posinset")).toMatch(/^[0-9]+$/);
      expect(["true", "mixed", "false"]).toContain(el.getAttribute("aria-checked"));
      expect(el.hasAttribute("aria-selected")).toBe(false);
    }

    // Group arithmetic: 4 real level-1 rows (2 mounts + 2 customs); 2 real
    // children under plex.
    expect(root.getAttribute("aria-level")).toBe("1");
    expect(root.getAttribute("aria-setsize")).toBe("4");
    expect(root.getAttribute("aria-posinset")).toBe("1");
    expect(transcoding.getAttribute("aria-level")).toBe("2");
    expect(transcoding.getAttribute("aria-setsize")).toBe("2");
    expect(transcoding.getAttribute("aria-posinset")).toBe("1");
    expect(item(/library/).getAttribute("aria-posinset")).toBe("2");
  });

  it("notice rows (loading, truncated, empty) are not treeitems and stay outside setsize counts", async () => {
    const hold = deferred<BrowseResponse>();
    browseReplies = [{ ...plexListing(), truncated: true }, hold.promise];
    await renderEditor();

    await focusItem(/user\/appdata\/plex/);
    await press("ArrowRight");

    // The truncated notice is a plain row: no treeitem role, not counted.
    expect(item(/transcoding/).getAttribute("aria-setsize")).toBe("2");
    const notice = screen.getByText("First 500 entries shown");
    expect(notice.closest('[role="treeitem"]')).toBeNull();

    // The loading row of a second, still-pending node likewise.
    const other = await focusItem(/user\/appdata\/other/);
    await press("ArrowRight");
    const loading = screen.getByText("Loading…");
    expect(loading.closest('[role="treeitem"]')).toBeNull();
    expect(item(/transcoding/).getAttribute("aria-setsize")).toBe("2");

    // An expanded EMPTY directory honestly reports aria-expanded true with
    // zero children — its folder.none row is a notice, not a treeitem.
    await act(async () => {
      hold.resolve({ ok: true, status: "ok", truncated: false, dirs: [] });
    });
    expect(screen.getByText("No subdirectories").closest('[role="treeitem"]')).toBeNull();
    expect(other.getAttribute("aria-expanded")).toBe("true");
  });
});
