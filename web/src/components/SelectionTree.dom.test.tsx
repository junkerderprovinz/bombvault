// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// SelectionTree render/state/interaction tests (Phase 2, plan 01 tracer).
//
// These ride the FoldersEditor harness (vi.mock of the api module, component
// imported AFTER the mock — same shape as Containers.excludesAssistant.dom
// .test.tsx) so the whole tracer slice is exercised end to end: the mounts
// response derives the (I, E) mirror, expanding a mount root lazily browses
// once through the browse-relative path, a subfolder uncheck PATCHes the flat
// set (bare mount include + "!"+child exclusion, selectionSource "tree"),
// and a remount reconstructs checked/mixed/excluded from the lists alone.
//
// The remount scenario drives SelectionTree directly with a shared
// editor-lifetime cache: the real cache owner is FoldersEditor (a useRef that
// survives section close, dies with the page — Pitfall 3), and a fresh
// component-local cache on every remount would hide exactly the "cache loss"
// refetch this assertion exists to catch.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, ContainerMountsResponse, OkEnvelope } from "../lib/api";

const browseCalls: string[] = [];
const patches: { name: string; paths: string[]; opts?: { selectionSource?: string } }[] = [];
let mountsReply: ContainerMountsResponse;
// Replies may be plain values or promises (a pending promise pins the loading
// row, which an immediately-resolving mock can never show).
let browseReplies: (BrowseResponse | Promise<BrowseResponse>)[] = [];
// Same shape for setContainerTargets: a deferred reply holds a save in flight
// so the queue's serialize/drain behavior is observable step by step.
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
    // view the Phase 2 assertions pin, projected out of the body — the same
    // wire contract the retired setBackupPaths mock captured.
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
const { SelectionTree } = await import("./SelectionTree");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";
const PLEX2 = "/mnt/user/appdata/plex2";
const OTHER = "/mnt/user/appdata/other";
const CUSTOM = "/mnt/user/backups";

/** Two selected mounts plus one custom include: unchecking either mount alone
 *  stays above the D-04 zero-include floor, so toggle bursts are observable
 *  without tripping the guard. */
function richMounts(): ContainerMountsResponse {
  return mountsResponse({
    mounts: [
      { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
      { source: PLEX2, dest: "/data", selected: true, isAppdata: false, reachable: true },
    ],
    custom: [{ path: CUSTOM, exists: true }],
  });
}

/** A controllable promise: `resolve` releases it from inside act(). */
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function mountsResponse(overrides?: Partial<ContainerMountsResponse>): ContainerMountsResponse {
  return {
    ok: true,
    mounts: [{ source: MOUNT, dest: "/config", selected: true, isAppdata: true, reachable: true }],
    custom: [],
    excluded: [],
    hostMountRoot: "/host/user",
    hostSourceRoot: HOST_ROOT,
    ...overrides,
  };
}

function listing(): BrowseResponse {
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

function EditorHarness() {
  const { t } = useT();
  return <FoldersEditor name="plex" stack="" open t={t} />;
}

async function renderEditor(): Promise<void> {
  render(
    <I18nProvider>
      <ToastProvider>
        <EditorHarness />
      </ToastProvider>
    </I18nProvider>,
  );
  // Flush the getContainerMounts load effect.
  await act(async () => {});
}

// Stands in for the editor-lifetime cache FoldersEditor owns and passes down;
// reassigned per test so scenarios never share listings.
let sharedCache = new Map<string, Promise<BrowseResponse>>();

function TreeHarness() {
  return (
    <SelectionTree
      mounts={mountsReply.mounts ?? []}
      customPaths={mountsReply.custom ?? []}
      includes={new Set([MOUNT])}
      exclusions={new Set([`${MOUNT}/transcoding`])}
      hostSourceRoot={HOST_ROOT}
      containerName="plex"
      browseCache={sharedCache}
      onToggle={() => {}}
      onRemoveCustom={() => {}}
      excludeCaches={{}}
      onToggleCaches={() => {}}
    />
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls.length = 0;
  patches.length = 0;
  browseReplies = [];
  patchReplies = [];
  mountsReply = mountsResponse();
  sharedCache = new Map();
});

afterEach(() => {
  cleanup();
});

describe("SelectionTree lazy expansion (TREE-01)", () => {
  it("fires zero browse calls on open, one on first expand (browse-relative), none on re-expand", async () => {
    browseReplies = [listing()];
    await renderEditor();

    // Opening the panel lists roots from the mounts response alone.
    expect(browseCalls).toEqual([]);

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(root.getAttribute("aria-expanded")).toBe("false");
    await act(async () => {
      fireEvent.click(root);
    });

    // The wire path is browse-relative: hostSourceRoot prefix swapped.
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /transcoding/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
    expect(root.getAttribute("aria-expanded")).toBe("true");

    // Collapse, then re-expand: the promise cache answers, no second call.
    await act(async () => {
      fireEvent.click(root);
    });
    expect(screen.queryByRole("treeitem", { name: /transcoding/ })).toBeNull();
    await act(async () => {
      fireEvent.click(root);
    });
    expect(screen.getByRole("treeitem", { name: /transcoding/ })).toBeTruthy();
    expect(browseCalls).toEqual(["user/appdata/plex"]);
  });
});

describe("SelectionTree toggle live-save (TREE-02, D-03)", () => {
  it("unchecking a subfolder PATCHes mount bare + !child with selectionSource tree, root goes mixed", async () => {
    browseReplies = [listing()];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    const child = screen.getByRole("treeitem", { name: /transcoding/ });
    // hidden: true — the input carries aria-hidden (the treeitem's
    // aria-checked is the single announcement; review WR-02), so ByRole's
    // default a11y-tree filter would not return it.
    const box = within(child).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    expect(patches).toEqual([
      {
        name: "plex",
        paths: [MOUNT, `!${MOUNT}/transcoding`],
        opts: { selectionSource: "tree" },
      },
    ]);

    // Optimistic mirror: the carve-out leaves the root mixed and the child
    // excluded (unchecked box, muted tone) while the rest stays selected.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    const excludedChild = screen.getByRole("treeitem", { name: /transcoding/ });
    expect(excludedChild.getAttribute("aria-checked")).toBe("false");
    expect(excludedChild.className).toContain("text-carbon-textMuted");
    expect(screen.getByRole("treeitem", { name: /library/ }).getAttribute("aria-checked")).toBe("true");
  });
});

describe("SelectionTree remount reconstruction (TREE-03/04)", () => {
  it("reconstructs mixed/excluded states with zero new browse calls via the editor-lifetime cache", async () => {
    // D-05: expansion survives remount in localStorage; the listing survives
    // in the cache the editor owns.
    localStorage.setItem("bv-tree-expanded-plex", JSON.stringify([MOUNT]));
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`] });
    browseReplies = [listing()];

    const first = render(
      <I18nProvider>
        <TreeHarness />
      </I18nProvider>,
    );
    await act(async () => {});
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    first.unmount();
    cleanup();

    // Remount: same (I, E), same cache — the state is pure list arithmetic,
    // no refetch, no selection memory anywhere but the server.
    render(
      <I18nProvider>
        <TreeHarness />
      </I18nProvider>,
    );
    await act(async () => {});
    expect(browseCalls).toEqual(["user/appdata/plex"]);

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(root.getAttribute("aria-checked")).toBe("mixed");
    const excludedChild = screen.getByRole("treeitem", { name: /transcoding/ });
    expect(excludedChild.getAttribute("aria-checked")).toBe("false");
    expect(excludedChild.className).toContain("text-carbon-textMuted");
    expect(screen.getByRole("treeitem", { name: /library/ }).getAttribute("aria-checked")).toBe("true");
  });

  it("shows an exclusions-only stored list unchecked and never PATCHes a repair", async () => {
    // Phase 1 explicit-none carrier: a deliberate full deselect is stored as
    // exclusions-only. The client displays it as-is and mutates nothing.
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: true, reachable: true }],
      excluded: [MOUNT],
    });
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(root.getAttribute("aria-checked")).toBe("false");
    expect(patches).toEqual([]);
  });
});

describe("SelectionTree per-node listing states (TREE-06, plan-01 scope)", () => {
  it("renders an empty directory as one muted folder.none row, distinct from loading and failure", async () => {
    browseReplies = [{ ok: true, status: "ok", truncated: false, dirs: [] }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    expect(screen.getByText("No subdirectories")).toBeTruthy();
    expect(screen.queryByRole("treeitem", { name: /transcoding/ })).toBeNull();
  });

  it("shows the loading row while a listing is pending, never empty", async () => {
    // A never-settling promise pins the in-flight state: an immediately
    // resolving mock would skip straight past the spinner row.
    browseReplies = [new Promise<BrowseResponse>(() => {})];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    expect(screen.getByText("Loading…")).toBeTruthy();
    expect(screen.queryByText("No subdirectories")).toBeNull();
    expect(screen.queryByRole("treeitem", { name: /transcoding/ })).toBeNull();
  });

  it("renders a failed listing as inline server text with a retry that refetches", async () => {
    browseReplies = [
      { ok: false, status: "restricted", error: "could not read directory" },
      listing(),
    ];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    // ok:false is its own state — never "empty", never a silent collapse.
    expect(screen.getByText("could not read directory")).toBeTruthy();
    expect(screen.queryByText("No subdirectories")).toBeNull();

    // A refused read is not memoized: retry re-enters the fetch.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    });
    expect(browseCalls).toEqual(["user/appdata/plex", "user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
  });
});

// Plan 02 scope: the five browse outcomes are pinned DISTINCT, the retry
// affordance re-enters the in-flight state, the truncated notice stays outside
// the treeitem structure, a rejected promise settles with no spinner left
// behind, and listing failures never interfere with selection persistence.
// (The rows themselves landed with plan 01's documented deviations — these
// tests are the pin the plan asked for: "verify that holds".)
describe("SelectionTree per-node outcome rows (TREE-06/D-06, plan 02)", () => {
  it("renders the couldNotRead fallback when ok:false carries no error text, outside the treeitem structure", async () => {
    browseReplies = [{ ok: false, status: "error" }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    // No error string on the wire -> the translated fallback, verbatim key
    // text, in a plain row (never a treeitem, never an empty-listing stand-in).
    const row = screen.getByText("Could not read directory");
    expect(row.closest('[role="treeitem"]')).toBeNull();
    expect(screen.queryByText("No subdirectories")).toBeNull();
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });

  it("retry re-enters the in-flight state (spinner row) before the refetched listing lands", async () => {
    let release!: (v: BrowseResponse) => void;
    const refetch = new Promise<BrowseResponse>((res) => {
      release = res;
    });
    browseReplies = [
      { ok: false, status: "restricted", error: "could not read directory" },
      refetch,
    ];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    });
    // The failed cache entry was evicted, so "Try again" is a REAL refetch:
    // a second browse call, passing through the spinner row again.
    expect(browseCalls).toEqual(["user/appdata/plex", "user/appdata/plex"]);
    expect(screen.getByText("Loading…")).toBeTruthy();

    await act(async () => {
      release(listing());
    });
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
  });

  it("truncated listing renders the served children then one non-interactive notice row (D-06)", async () => {
    browseReplies = [{ ...listing(), truncated: true }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    // The served children render as real treeitems...
    expect(screen.getByRole("treeitem", { name: /transcoding/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
    // ...and setsize counts ONLY them: the notice row is outside the arithmetic.
    expect(screen.getByRole("treeitem", { name: /transcoding/ }).getAttribute("aria-setsize")).toBe("2");

    // Exactly one notice, a plain <p>: no treeitem role, no checkbox, nothing
    // to click — never counted, never paginated.
    const notices = screen.getAllByText("First 500 entries shown");
    expect(notices.length).toBe(1);
    expect(notices[0].tagName).toBe("P");
    expect(notices[0].closest('[role="treeitem"]')).toBeNull();
    expect(notices[0].querySelector("input")).toBeNull();
  });

  it("renders loading, empty, and no-access outcomes as three distinct rows in one tree", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: PLEX2, dest: "/data", selected: true, isAppdata: false, reachable: true },
        { source: OTHER, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    browseReplies = [
      { ok: false, status: "restricted", error: "could not read directory" }, // /config
      { ok: true, status: "ok", truncated: false, dirs: [] }, // /data
      new Promise<BrowseResponse>(() => {}), // /media, never settles in this test
    ];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /\/config/ }));
    });
    expect(screen.getByText("could not read directory")).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /\/data/ }));
    });
    expect(screen.getByText("No subdirectories")).toBeTruthy();
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /\/media/ }));
    });
    expect(screen.getByText("Loading…")).toBeTruthy();

    // All three texts coexist and are pairwise distinct; the retry affordance
    // belongs to the no-access row alone (empty and loading are plain text).
    expect(screen.getAllByRole("button", { name: "Try again" }).length).toBe(1);
  });

  it("a rejected browse promise settles to the error row — zero spinner nodes remain", async () => {
    let reject!: (e: Error) => void;
    const boom = new Promise<BrowseResponse>((_, rej) => {
      reject = rej;
    });
    browseReplies = [boom];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    await act(async () => {
      reject(new Error("network down"));
    });

    // Both promise outcomes settle the node: the fallback text shows and no
    // animate-spin element survives anywhere in the document.
    expect(screen.getByText("Could not read directory")).toBeTruthy();
    expect(document.querySelectorAll(".animate-spin").length).toBe(0);
  });

  it("a node whose listing failed still toggles and PATCHes normally (listing errors never block saving)", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: PLEX2, dest: "/data", selected: true, isAppdata: false, reachable: true },
      ],
    });
    browseReplies = [{ ok: false, status: "restricted", error: "could not read directory" }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /\/config/ }));
    });
    expect(screen.getByText("could not read directory")).toBeTruthy();

    // The unreadable folder only failed to LIST: its own include still
    // toggles, the save still leaves — selection and browse stay independent.
    const box = within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    expect(patches).toEqual([{ name: "plex", paths: [PLEX2], opts: { selectionSource: "tree" } }]);
  });
});

// Phase 3 review WR-03: the exclusions-disclosure DOM id must be
// collision-free per root. The retired non-alphanumeric stripping made
// "/mnt/user/app-data" and "/mnt/user/app_data" share one sanitized id, so
// two open disclosures rendered duplicate ids and every button's
// aria-controls resolved to the FIRST list. Rendered directly against
// SelectionTree (the remount tests' precedent): the id derivation is pure
// tree arithmetic, no editor queue involved.
describe("SelectionTree exclusions-disclosure ids (WR-03)", () => {
  it("roots whose sanitized paths agree get distinct ids, and each button controls its own list", async () => {
    const DASH = `${HOST_ROOT}/user/app-data`;
    const UNDER = `${HOST_ROOT}/user/app_data`;
    render(
      <I18nProvider>
        <SelectionTree
          mounts={[
            { source: DASH, dest: "/a", selected: true, isAppdata: false, reachable: true },
            { source: UNDER, dest: "/b", selected: true, isAppdata: false, reachable: true },
          ]}
          customPaths={[]}
          includes={new Set([DASH, UNDER])}
          exclusions={new Set([`${DASH}/x`, `${UNDER}/y`])}
          hostSourceRoot={HOST_ROOT}
          containerName="collide"
          browseCache={sharedCache}
          onToggle={() => {}}
          onRemoveCustom={() => {}}
          excludeCaches={{}}
          onToggleCaches={() => {}}
        />
      </I18nProvider>,
    );

    // Both roots carry exactly one exclusion each: two disclosure buttons.
    const btns = screen.getAllByRole("button", { name: /1 exclusions/ });
    expect(btns).toHaveLength(2);
    // The collision pin: distinct aria-controls values for distinct roots
    // (the stripped derivation yielded one shared id here).
    const ids = btns.map((b) => b.getAttribute("aria-controls"));
    expect(ids[0]).toBeTruthy();
    expect(new Set(ids).size).toBe(2);

    // Open both: each button's aria-controls resolves to ITS OWN list —
    // "x" under app-data, "y" under app_data — never the first list twice.
    await act(async () => {
      for (const b of btns) fireEvent.click(b);
    });
    const expected: [HTMLElement, string][] = [
      [btns[0], "x"],
      [btns[1], "y"],
    ];
    for (const [btn, rel] of expected) {
      const list = document.getElementById(btn.getAttribute("aria-controls") ?? "");
      expect(list, `list for ${rel}`).not.toBeNull();
      expect(list?.tagName).toBe("UL");
      expect(within(list as HTMLElement).getAllByRole("listitem").map((li) => li.textContent)).toEqual([rel]);
    }
  });
});

describe("empty-selection guard (D-04, pulled forward from plan 02)", () => {
  it("blocks the toggle that would leave zero includes before any PATCH, with the inline warn line", async () => {
    mountsReply = mountsResponse();
    browseReplies = [listing()];
    await renderEditor();

    // The single selected mount root IS the only include: unchecking it
    // would empty the item's selection.
    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    const box = within(root).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    // Blocked client-side: no PATCH leaves, the include stays, the warn line
    // (statusWarn text + one shake replay) says why and routes to the
    // schedule toggle instead.
    expect(patches).toEqual([]);
    expect(
      screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked"),
    ).toBe("true");
    const warn = screen.getByText(
      "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
    );
    expect(warn.className).toContain("text-statusWarn");
  });
});

// Plan 02 Task 2: the one-deep serialized PATCH queue (RESEARCH Open Question
// 4, Pitfalls 4/5) plus the full D-04 treatment — shake replay + text-xs warn
// line on the blocked row, whole-item include counting, and the defensive
// handling of the server's coded empty-selection refusal.
describe("FoldersEditor serialized save queue and D-04 guard (plan 02)", () => {
  it("blocked last-include uncheck shakes the row and shows the text-xs warn line, mirror untouched", async () => {
    // One selected mount, no custom: unchecking it would empty the item.
    await renderEditor();

    const box = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    expect(patches).toEqual([]); // no request left
    const row = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(row.getAttribute("aria-checked")).toBe("true"); // mirror never flipped
    expect(row.className).toContain("glim-shake"); // one shake replay via the nonce-in-key technique
    const warn = screen.getByText(
      "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
    );
    expect(warn.className).toContain("text-xs");
    expect(warn.className).toContain("text-statusWarn");
  });

  it("unchecking the last MOUNT include PATCHes while a custom include exists (the count spans the whole item)", async () => {
    mountsReply = richMounts();
    await renderEditor();

    const box = within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    // The custom path keeps the item non-empty, so the toggle is legitimate
    // and the save carries the remaining includes.
    expect(patches).toEqual([{ name: "plex", paths: [PLEX2, CUSTOM], opts: { selectionSource: "tree" } }]);
  });

  it("serializes two rapid toggles: a failing first save never clobbers the second toggle", async () => {
    mountsReply = richMounts();
    const first = deferred<OkEnvelope>();
    patchReplies = [first.promise];
    await renderEditor();

    // Toggle 1: uncheck plex. PATCH 1 leaves and is held in flight.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patches.length).toBe(1);
    expect(patches[0].paths).toEqual([PLEX2, CUSTOM]);

    // Toggle 2 while PATCH 1 is in flight: the mirror updates optimistically,
    // the save is queued dirty — never a concurrent second request.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/data/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patches.length).toBe(1);

    // PATCH 1 fails: the revert is RE-DERIVED from the live mirror (plex
    // returns; plex2's uncheck SURVIVES — a captured snapshot would wipe it),
    // then the dirty drain sends the final full list exactly once.
    await act(async () => {
      first.resolve({ ok: false, error: "save failed" });
    });
    expect(patches.length).toBe(2);
    expect(patches[1].paths).toEqual([MOUNT, CUSTOM]);
    expect(screen.getByText("save failed")).toBeTruthy(); // verbatim toast

    expect(screen.getByRole("treeitem", { name: /\/config/ }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("treeitem", { name: /\/data/ }).getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("treeitem", { name: /user\/backups/ }).getAttribute("aria-checked")).toBe("true");
  });

  it("collapses a burst: exactly one draining PATCH after the in-flight save resolves, bodies ordered", async () => {
    mountsReply = richMounts();
    const first = deferred<OkEnvelope>();
    patchReplies = [first.promise];
    await renderEditor();

    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/data/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patches.length).toBe(1);

    await act(async () => {
      first.resolve({ ok: true });
    });
    // The two-toggle burst collapsed into ONE drain carrying the latest list.
    expect(patches.length).toBe(2);
    expect(patches[0].paths).toEqual([PLEX2, CUSTOM]);
    expect(patches[1].paths).toEqual([CUSTOM]);
  });

  it("defensive backstop: a coded empty-selection refusal toasts verbatim and reverts the mirror", async () => {
    // Unreachable once the D-04 block exists (the client never sends an
    // empty-include list) — pinned anyway: the coded envelope is toasted
    // verbatim and the optimistic mirror rolls back.
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: PLEX2, dest: "/data", selected: true, isAppdata: false, reachable: true },
      ],
    });
    patchReplies = [{ ok: false, error: "Selection would be empty", code: "empty-selection" }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true }));
    });

    expect(patches.length).toBe(1); // no drain: the refusal settles the queue
    expect(screen.getByText("Selection would be empty")).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /\/config/ }).getAttribute("aria-checked")).toBe("true");
  });
});
