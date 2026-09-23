// @vitest-environment jsdom
// FoldersEditor with the selection tree, against a mocked api client: the PATCH
// bodies it sends, the floor that keeps at least one include, stored
// sub-includes shown under their mount, and expansion restored from
// localStorage without refetching.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, ContainerMountsResponse, MountInfo } from "../lib/api";

const browseCalls: string[] = [];
const patches: { name: string; paths: string[]; opts?: { selectionSource?: string } }[] = [];
let mountsReply: ContainerMountsResponse;
// Whether each browse call opted into hidden entries, parallel to browseCalls.
const browseHidden: boolean[] = [];
let browseReplies: (BrowseResponse | Promise<BrowseResponse>)[] = [];
// A pending promise holds a save in flight, so the queue's serialize and drain
// steps can be observed one at a time.
let patchReplies: ({ ok: boolean; error?: string } | Promise<{ ok: boolean; error?: string }>)[] = [];
let mountsCalls = 0;
// Every setContainerTargets body, verbatim. maxConcurrentPatches counts
// requests in flight; two overlapping PATCHes would push it to 2.
const patchBodies: { name: string; body: Record<string, unknown> }[] = [];
let activePatches = 0;
let maxConcurrentPatches = 0;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getContainerMounts: () => {
      mountsCalls += 1;
      return Promise.resolve(mountsReply);
    },
    browse: (path: string, hidden?: boolean) => {
      browseCalls.push(path);
      browseHidden.push(hidden === true);
      const reply = browseReplies.shift() ?? { ok: true, dirs: [], status: "ok", truncated: false };
      return Promise.resolve(reply);
    },
    setContainerTargets: (name: string, body: Record<string, unknown>) => {
      activePatches += 1;
      maxConcurrentPatches = Math.max(maxConcurrentPatches, activePatches);
      patchBodies.push({ name, body });
      // The flat paths/selectionSource view of the same body, which most tests
      // assert on.
      patches.push({
        name,
        paths: (body.backupPaths as string[] | undefined) ?? [],
        opts: body.selectionSource ? { selectionSource: body.selectionSource as string } : undefined,
      });
      const reply = patchReplies.shift() ?? { ok: true };
      return Promise.resolve(reply).finally(() => {
        activePatches -= 1;
      });
    },
  };
});

// Imported after vi.mock so the components get the mocked client.
const { FoldersEditor } = await import("./Containers");
const { SelectionTree } = await import("../components/SelectionTree");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";
const SUB_INCLUDE = "/mnt/user/appdata/plex/Media";
const STANDALONE = "/mnt/user/backups";
const SECOND_MOUNT = "/mnt/user/appdata/sonarr";

function mountsResponse(overrides?: Partial<ContainerMountsResponse>): ContainerMountsResponse {
  return {
    ok: true,
    mounts: [{ source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true }],
    custom: [],
    excluded: [],
    hostMountRoot: "/host/user",
    hostSourceRoot: HOST_ROOT,
    // The server always sends the CACHEDIR map; empty means nothing is skipped.
    excludeCaches: {},
    ...overrides,
  };
}

/** Children of the plex mount, browse-relative (host root "/mnt" swapped). */
function plexListing(): BrowseResponse {
  return {
    ok: true,
    status: "ok",
    truncated: false,
    dirs: [
      { name: "transcoding", path: "user/appdata/plex/transcoding" },
      { name: "Media", path: "user/appdata/plex/Media" },
    ],
  };
}

function EditorHarness({ open, lastBackup }: { open: boolean; lastBackup?: number | null }) {
  const { t } = useT();
  return <FoldersEditor name="tree" stack="" open={open} t={t} lastBackup={lastBackup ?? null} />;
}

function Providers({ children }: { children: React.ReactNode }) {
  return (
    <I18nProvider>
      <ToastProvider>{children}</ToastProvider>
    </I18nProvider>
  );
}

async function renderEditor(open = true, lastBackup: number | null = null) {
  const view = render(
    <Providers>
      <EditorHarness open={open} lastBackup={lastBackup} />
    </Providers>,
  );
  // Flush the getContainerMounts load effect.
  await act(async () => {});
  return view;
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls.length = 0;
  browseHidden.length = 0;
  patches.length = 0;
  browseReplies = [];
  patchReplies = [];
  mountsCalls = 0;
  mountsReply = mountsResponse();
  patchBodies.length = 0;
  activePatches = 0;
  maxConcurrentPatches = 0;
});

afterEach(cleanup);

describe("FoldersEditor tree", () => {
  it("unchecking a subfolder PATCHes the bare mount plus the !-prefixed host path with selectionSource tree", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    // hidden: true because the input is aria-hidden, which ByRole filters out
    // by default.
    const child = within(screen.getByRole("treeitem", { name: /transcoding/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(child);
    });

    expect(patches).toEqual([
      {
        name: "tree",
        paths: [MOUNT, `!${MOUNT}/transcoding`],
        opts: { selectionSource: "tree" },
      },
    ]);
    // The carve-out is visible immediately in the tree states.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    expect(screen.getByRole("treeitem", { name: /transcoding/ }).getAttribute("aria-checked")).toBe("false");
  });

  it("unchecking the item's last include fires no request", async () => {
    await renderEditor();

    const box = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    expect(patches).toEqual([]); // blocked before any PATCH was built
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("true");
    expect(
      screen.getByText(
        "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
      ),
    ).toBeTruthy();
  });

  it("removing the last custom row is blocked by the same floor as the checkbox (no exclusions-only save)", async () => {
    // Zero includes plus one exclusion is not an empty list, so the server's
    // empty-selection guard would store it, and every later backup of this
    // container would succeed while capturing nothing.
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: false, reachable: true }],
      excluded: [`${MOUNT}/transcoding`],
      custom: [{ path: STANDALONE, exists: true }],
    });
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /user\/backups/ });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    });

    expect(patches).toEqual([]); // blocked before any PATCH was built
    // The row is still there, and the refusal says why.
    expect(screen.getByRole("treeitem", { name: /user\/backups/ })).toBeTruthy();
    expect(
      screen.getByText(
        "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
      ),
    ).toBeTruthy();
  });

  it("a failed save whose revert would empty the selection falls back to server truth instead of saving an exclusions-only list", async () => {
    // The second toggle is checked against a mirror that still holds the first
    // toggle's optimistic include. When that save fails and the include is
    // taken back, the revert leaves zero includes, and the queued drain must
    // not send that exclusions-only list.
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: SECOND_MOUNT, dest: "/media", selected: false, isAppdata: false, reachable: true },
      ],
      excluded: [`${MOUNT}/transcoding`],
    });
    // The first PATCH is held so the second toggle lands while it is in
    // flight; then it fails.
    await renderEditor();
    let failFirst!: (r: { ok: boolean; error?: string }) => void;
    patchReplies = [new Promise((res) => (failFirst = res))];

    const second = within(screen.getByRole("treeitem", { name: /user\/appdata\/sonarr/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(second); // tick the second mount -> save in flight
    });
    const first = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(first); // untick the first while that save is in flight
    });
    await act(async () => {
      failFirst({ ok: false, error: "boom" });
    });

    // The queue still drains the stacked toggle, but it sends server truth
    // rather than the exclusions-only list.
    expect(patches.length).toBe(2);
    expect(patches[0].paths).toEqual([MOUNT, SECOND_MOUNT, `!${MOUNT}/transcoding`]);
    expect(patches[1].paths).toEqual([MOUNT, `!${MOUNT}/transcoding`]);
    // Back on server truth, and the refused newer toggle says so.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/sonarr/ }).getAttribute("aria-checked")).toBe("false");
    expect(
      screen.getByText(
        "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
      ),
    ).toBeTruthy();
  });

  it("removing a custom root takes its CACHEDIR.TAG entry with it", async () => {
    // Nothing else prunes the map, and the switch only renders for a root that
    // still has a row, so a left-over entry would keep --exclude-caches on with
    // no way to turn it off.
    mountsReply = mountsResponse({
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [STANDALONE]: true, [MOUNT]: false },
    });
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /user\/backups/ });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    });

    // Two drains through the same queue: the smaller selection, then the map
    // without the removed root. The second is scheduled from the first one's
    // success, so a failed removal keeps the entry.
    expect(patchBodies.length).toBe(2);
    expect(patchBodies[0].body.backupPaths).toEqual([MOUNT]);
    expect(patchBodies[0].body.excludeCaches).toBeUndefined();
    expect(patchBodies[1].body.excludeCaches).toEqual({ [MOUNT]: false });
    expect(maxConcurrentPatches).toBe(1);
  });

  it("a failed removal leaves the CACHEDIR entry alone", async () => {
    // A structural save is never reverted, so a cleanup sent after a failed
    // removal would stick.
    mountsReply = mountsResponse({
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [STANDALONE]: true, [MOUNT]: true },
    });
    patchReplies = [{ ok: false, error: "boom" }];
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /user\/backups/ });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    });

    // Exactly one request, the one that failed. No caches PATCH behind it.
    expect(patchBodies.length).toBe(1);
    expect(patchBodies[0].body.excludeCaches).toBeUndefined();
  });

  it("a vanished sub-include under a reachable mount keeps its own row and its warning", async () => {
    // A sub-include is normally shown by the tree under its mount, but the tree
    // builds children from browse listings, and a folder that is gone from disk
    // is in none of them.
    mountsReply = mountsResponse({
      custom: [{ path: `${MOUNT}/Library`, exists: false }],
    });
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /Library/ });
    expect(within(row).getByText("no data folder detected (nothing to back up here)")).toBeTruthy();
  });

  it("adding a folder inside an excluded branch clears the covering exclusion instead of saving a no-op", async () => {
    // With the exclusion kept, the server would prune the redundant include and
    // the backup would still exclude the branch. The tree checkbox on an
    // excluded node clears the covering exclusion, and Add does the same.
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`] });
    await renderEditor();

    const input = screen.getByPlaceholderText("user/appdata");
    await act(async () => {
      fireEvent.change(input, { target: { value: "user/appdata/plex/transcoding/keepme" } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    // The exclusion is gone and no include is added: the mount already covers
    // the path, and the server would prune a redundant descendant include.
    expect(patches).toEqual([
      { name: "tree", paths: [MOUNT], opts: { selectionSource: "tree" } },
    ]);
  });

  it("adding a folder that is the parent of an existing include saves it", async () => {
    // classifyNode answers "mixed" when an include sits below the added path.
    // That is not redundant: nothing covers the path from above.
    mountsReply = mountsResponse({
      custom: [{ path: `${STANDALONE}/daily`, exists: true }],
    });
    await renderEditor();

    const input = screen.getByPlaceholderText("user/appdata");
    await act(async () => {
      fireEvent.change(input, { target: { value: "user/backups" } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    // The parent goes out; the server prunes the child as redundant.
    expect(patches).toEqual([
      {
        name: "tree",
        paths: [MOUNT, STANDALONE, `${STANDALONE}/daily`],
        opts: { selectionSource: "tree" },
      },
    ]);
  });

  it("adding a folder already covered by an included mount changes nothing and keeps the staged pick", async () => {
    // The server would prune such an include, so the duplicate check has to
    // look at included mounts, not only the custom list and exact includes.
    await renderEditor();

    const input = screen.getByPlaceholderText("user/appdata");
    await act(async () => {
      fireEvent.change(input, { target: { value: "user/appdata/plex/Media" } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    expect(patches).toEqual([]);
    // The pick stays in the field as feedback, as for a literal duplicate.
    expect((input as HTMLInputElement).value).toBe("user/appdata/plex/Media");
  });

  it("absorbs a stored sub-include into its mount: mixed root, checked child, no duplicate custom row", async () => {
    // The server sends an include below a mount root as a custom row; the tree
    // shows it once, under its mount.
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: false, reachable: true }],
      custom: [{ path: SUB_INCLUDE, exists: true }],
    });
    browseReplies = [plexListing()];
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });

    // The include below renders the mount mixed without the mount itself being
    // selected.
    expect(root.getAttribute("aria-checked")).toBe("mixed");

    // The sub-include is not a level-1 custom row, and the root group counts
    // one real root.
    expect(screen.queryByRole("treeitem", { name: /^\/mnt\/user\/appdata\/plex\/Media$/ })).toBeNull();
    expect(root.getAttribute("aria-setsize")).toBe("1");
    expect(root.getAttribute("aria-posinset")).toBe("1");

    // Expanded, the sub-include renders once, as a checked child of its mount.
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^Media$/ });
    expect(media.getAttribute("aria-checked")).toBe("true");
    // Still exactly one row presenting that path.
    expect(screen.getAllByRole("treeitem", { name: /Media/ }).length).toBe(1);
  });

  it("keeps standalone custom paths as expandable level-1 lazy roots beside the mounts", async () => {
    mountsReply = mountsResponse({
      custom: [{ path: STANDALONE, exists: true }],
    });
    browseReplies = [
      plexListing(),
      { ok: true, status: "ok", truncated: false, dirs: [{ name: "docs", path: "user/backups/docs" }] },
    ];
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    // A custom row's accessible name includes its remove chip text, so match
    // on the path fragment.
    const custom = screen.getByRole("treeitem", { name: /user\/backups/ });
    for (const row of [root, custom]) {
      expect(row.getAttribute("aria-expanded")).toBe("false");
      expect(row.getAttribute("aria-level")).toBe("1");
    }
    expect(root.getAttribute("aria-setsize")).toBe("2");

    // Both kinds of root lazily browse, in browse-relative form.
    await act(async () => {
      fireEvent.click(custom);
    });
    await act(async () => {
      fireEvent.click(root);
    });
    expect(browseCalls).toEqual(["user/backups", "user/appdata/plex"]);
    expect(screen.getByRole("treeitem", { name: /^docs$/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /^Media$/ })).toBeTruthy();
  });

  it("browses with the hidden opt-in, so dot-directories have a row to tick", async () => {
    // Without it the server leaves dot-directories out of the listing without
    // saying so. Children are a whitelist, so a folder with no row is left out
    // of the backup, and in appdata these are often the ones that matter
    // (.storage holds Home Assistant's config).
    browseReplies = [
      {
        ok: true,
        status: "ok",
        truncated: false,
        dirs: [
          { name: ".storage", path: "user/appdata/plex/.storage" },
          { name: "Media", path: "user/appdata/plex/Media" },
        ],
      },
    ];
    await renderEditor();
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    expect(browseCalls).toEqual(["user/appdata/plex"]);
    expect(browseHidden).toEqual([true]);
    const dot = screen.getByRole("treeitem", { name: /\.storage/ });
    await act(async () => {
      fireEvent.click(within(dot).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toEqual([
      { name: "tree", paths: [MOUNT, `!${MOUNT}/.storage`], opts: { selectionSource: "tree" } },
    ]);
  });

  it("restores expansion from the localStorage key and listings from the cache on section reopen, without refetch", async () => {
    browseReplies = [plexListing()];
    const view = await renderEditor(true);

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    await act(async () => {
      fireEvent.click(root);
    });
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    // The expanded host path is stored under the per-container key.
    expect(JSON.parse(localStorage.getItem("bv-tree-expanded-tree") ?? "[]")).toEqual([MOUNT]);

    // Close the section: the tree unmounts, editor state survives.
    view.rerender(
      <Providers>
        <EditorHarness open={false} />
      </Providers>,
    );
    expect(screen.queryByRole("tree")).toBeNull();

    // Reopen: expansion comes from the key, children from the editor's cache
    // (no second browse), and check states from the stored selection.
    view.rerender(
      <Providers>
        <EditorHarness open />
      </Providers>,
    );
    await act(async () => {});
    expect(browseCalls).toEqual(["user/appdata/plex"]);
    const reopened = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(reopened.getAttribute("aria-expanded")).toBe("true");
    expect(reopened.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("treeitem", { name: /^Media$/ }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("treeitem", { name: /transcoding/ }).getAttribute("aria-checked")).toBe("true");
  });

  it("persists only the 64 most recent expansions", async () => {
    // 66 mounts expanded by real clicks; every browse reply defaults to an
    // empty listing. That many clicks outlast the default budget while the
    // rest of the suite runs alongside, hence the raised timeout below.
    const mounts: MountInfo[] = Array.from({ length: 66 }, (_, i) => ({
      source: `${HOST_ROOT}/user/appdata/d${String(i).padStart(2, "0")}`,
      dest: `/d${i}`,
      selected: false,
      isAppdata: false,
      reachable: true,
    }));
    const sharedCache = new Map<string, Promise<BrowseResponse>>();
    render(
      <Providers>
        <SelectionTree
          mounts={mounts}
          customPaths={[]}
          includes={new Set()}
          exclusions={new Set()}
          hostSourceRoot={HOST_ROOT}
          containerName="cap"
          browseCache={sharedCache}
          onToggle={() => {}}
          onRemoveCustom={() => {}}
          excludeCaches={{}}
          onToggleCaches={() => {}}
        />
      </Providers>,
    );
    // All 66 clicks in one act: the functional expandedOrder update chains
    // across batched events, and a single flush keeps the test fast.
    await act(async () => {
      for (const m of mounts) {
        fireEvent.click(screen.getByRole("treeitem", { name: new RegExp(m.source) }));
      }
    });

    const stored: string[] = JSON.parse(localStorage.getItem("bv-tree-expanded-cap") ?? "[]");
    expect(stored).toHaveLength(64);
    expect(stored).not.toContain(mounts[0].source); // oldest expanded evicted
    expect(stored).not.toContain(mounts[1].source);
    expect(stored).toContain(mounts[2].source);
    expect(stored[63]).toBe(mounts[65].source); // newest last (recency order)
    expect(browseCalls).toHaveLength(66);
  }, 15000);
});

// Each root row shows a muted "{n} paths" line computed from the stored
// includes and exclusions, not from what is checked on screen, so it matches
// what the next PATCH sends. A fully deselected root shows "0 paths".
describe("per-root path count", () => {
  it("announces '{n} paths' inside every root row: included mount, deselected mount (0), unreachable mount (0), standalone custom", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: false, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/srv9/elsewhere`, dest: "/gone", selected: false, isAppdata: false, reachable: false },
      ],
      custom: [{ path: STANDALONE, exists: true }],
    });
    await renderEditor();

    // The line sits inside the root treeitem's label column, so it is part of
    // the row's accessible name.
    const plex = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("1 path");
    expect(plex.className).toContain("text-carbon-textMuted");

    // A fully deselected mount shows its 0.
    within(screen.getByRole("treeitem", { name: /\/media ← \/mnt\/user\/media/ })).getByText("0 paths");

    // Unreachable mount: its own 0 case renders too.
    within(screen.getByRole("treeitem", { name: /\/mnt\/srv9\/elsewhere/ })).getByText("0 paths");

    // Standalone custom row: included custom paths count as their own root
    // include on load, so the row announces "1 path".
    within(screen.getByRole("treeitem", { name: /user\/backups/ })).getByText("1 path");
  });

  it("a carve-out toggle leaves the count unchanged; narrowing the parent to child includes changes it", async () => {
    // Two selected mounts, so the last-include floor never fires while plex is
    // narrowed below.
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    browseReplies = [plexListing(), plexListing()];
    await renderEditor();

    // Unchecking a child of an included parent adds an exclusion and keeps the
    // parent include, so the count stays.
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    within(root).getByText("1 path");
    const child = within(screen.getByRole("treeitem", { name: /transcoding/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(child);
    });
    within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("1 path");
    // The saved list carries both mounts plus the "!" entry.
    expect(patches[patches.length - 1]?.paths).toEqual([
      MOUNT,
      `${HOST_ROOT}/user/media`,
      `!${MOUNT}/transcoding`,
    ]);

    // Unchecking the parent drops its include (media keeps the item
    // non-empty); checking two children then counts them.
    const parentBox = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(parentBox);
    });
    within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("0 paths");
    // The exclusion below it is remembered: bare includes first, then the "!"
    // entry.
    expect(patches[patches.length - 1]?.paths).toEqual([
      `${HOST_ROOT}/user/media`,
      `!${MOUNT}/transcoding`,
    ]);

    for (const name of [/^transcoding$/, /^Media$/]) {
      const box = within(screen.getByRole("treeitem", { name })).getByRole("checkbox", { hidden: true });
      await act(async () => {
        fireEvent.click(box);
      });
    }
    within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("2 paths");
  });
});

// A root with exclusions below it gets a collapsible "{n} exclusions" section:
// a disclosure button and, when open, a read-only list of relative paths. The
// list comes from the stored exclusion set, remembered entries included, not
// from whichever tree nodes are loaded.
describe("per-root exclusion list", () => {
  it("renders the disclosure for a root with exclusions and expands to relative mono rows", async () => {
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`, `${MOUNT}/Media`] });
    await renderEditor();

    const btn = screen.getByRole("button", { name: /2 exclusions/ });
    expect(btn.getAttribute("aria-expanded")).toBe("false");
    const listId = btn.getAttribute("aria-controls");
    expect(listId).toBeTruthy();
    // Collapsed body is not rendered.
    expect(document.getElementById(listId ?? "")).toBeNull();

    await act(async () => {
      fireEvent.click(btn);
    });
    expect(btn.getAttribute("aria-expanded")).toBe("true");
    const list = document.getElementById(listId ?? "");
    expect(list).not.toBeNull();
    expect(list?.tagName).toBe("UL");
    const items = within(list as HTMLElement).getAllByRole("listitem");
    // Relative, lexically sorted, no root prefix, no leading slash.
    expect(items.map((li) => li.textContent)).toEqual(["Media", "transcoding"]);
    for (const li of items) {
      // title carries the full relative path; the row uses the mono ltr
      // long-path style.
      expect(li.getAttribute("title")).toBe(li.textContent);
      expect(li.getAttribute("dir")).toBe("ltr");
      expect(li.className).toContain("font-mono");
      expect(li.className).toContain("break-all");
    }
  });

  it("renders no section at all for a root with zero exclusions (no empty-list copy)", async () => {
    await renderEditor();
    expect(screen.queryByRole("button", { name: /exclusions/ })).toBeNull();
    expect(screen.queryByText(/exclusions/i)).toBeNull();
  });

  it("a fully deselected root shows an unchecked box, '0 paths' and '{n} exclusions' together", async () => {
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: false, reachable: true }],
      excluded: [`${MOUNT}/transcoding`],
    });
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(root.getAttribute("aria-checked")).toBe("false");
    within(root).getByText("0 paths");
    expect(screen.getByRole("button", { name: /1 exclusion/ })).toBeTruthy();

    // Expanding the dormant section lists the remembered exclusion.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /1 exclusion/ }));
    });
    expect(screen.getAllByRole("listitem").map((li) => li.textContent)).toEqual(["transcoding"]);
  });

  it("the expanded list is non-interactive: no controls inside the body, rows not focusable, treeitem counts unchanged", async () => {
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`, `${MOUNT}/Media`] });
    await renderEditor();

    // Disclosure rows are excluded from treeitem counts: still exactly one
    // root treeitem (aria-setsize of the level-1 set unchanged).
    expect(screen.getAllByRole("treeitem")).toHaveLength(1);
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-setsize")).toBe("1");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /2 exclusions/ }));
    });
    expect(screen.getAllByRole("treeitem")).toHaveLength(1);

    // No controls in the list, and no tabindex on the rows.
    for (const li of screen.getAllByRole("listitem")) {
      expect(li.getAttribute("tabindex")).toBeNull();
      expect(within(li).queryByRole("checkbox")).toBeNull();
      expect(within(li).queryByRole("button")).toBeNull();
      expect(within(li).queryByRole("link")).toBeNull();
    }
  });

  it("a never-expanded root still lists its exclusions (derived from the set, not from loaded children)", async () => {
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`, `${MOUNT}/Media`] });
    await renderEditor();

    // The node is never expanded, so there are no browse calls, yet both
    // exclusions are listed.
    expect(browseCalls).toEqual([]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /2 exclusions/ }));
    });
    expect(browseCalls).toEqual([]);
    expect(screen.getAllByRole("listitem").map((li) => li.textContent)).toEqual(["Media", "transcoding"]);
  });
});

// Reset selection returns the container to automatic detection. It is
// confirmed, goes through the same one-deep queue as toggles, and sends
// {backupPaths: [], excludeCaches: {}} without selectionSource, since the
// server's empty-selection guard only applies to "tree" saves. Clearing the
// caches map as well means no --exclude-caches survives for a root the reset
// removes. It is not optimistic: on success the editor refetches.
//
// The narrowing note follows a successful save that includes fewer folders
// than the last saved one, only once the container has a backup, and lasts
// for the editor session.

const NARROWED_NOTE_EN =
  "The selection now covers fewer folders than before. From the next backup on, snapshots will contain only the selected folders. Existing snapshots are unchanged.";
const RESET_CONFIRM_EN =
  "Reset the folder selection? The container returns to automatic detection (appdata default) and all remembered exclusions and cache-folder settings are removed.";

describe("Reset selection, narrowing note and hint copy", () => {
  it("Reset selection confirms with every consequence named, clears the selection and the caches map in one PATCH, and reloads the auto-detected selection", async () => {
    // The caches map holds a true for a custom root, which the reset removes.
    // Clearing only backupPaths would leave --exclude-caches on with no switch
    // left to turn it off.
    mountsReply = mountsResponse({
      excluded: [`${MOUNT}/transcoding`],
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [STANDALONE]: true },
    });
    await renderEditor(true, 1700000000);
    // Before the reset: the exclusion is listed and the custom root's switch
    // is on.
    expect(screen.getByRole("button", { name: /1 exclusion/ })).toBeTruthy();
    expect(
      within(cachedirRow(screen.getByRole("treeitem", { name: /user\/backups/ })))
        .getByRole("switch")
        .getAttribute("aria-checked"),
    ).toBe("true");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(RESET_CONFIRM_EN)).toBeTruthy();
    // The confirm button carries no status colour; the sentence above names
    // the consequences.
    const confirmBtn = within(dialog).getByRole("button", { name: "Confirm" });
    expect(confirmBtn.className).not.toContain("bg-statusFailSolid");

    // The server's state after the reset: auto-detected default, no
    // exclusions, no custom row, empty caches map.
    mountsReply = mountsResponse();
    await act(async () => {
      fireEvent.click(confirmBtn);
    });

    // One PATCH with an empty list and no selection source; the api helper
    // omits the key when no source is passed.
    expect(patches).toHaveLength(1);
    expect(patches[0]).toEqual({ name: "tree", paths: [] });
    expect(patches[0].opts).toBeUndefined();
    // The same request clears the caches map.
    expect(patchBodies[0]).toEqual({ name: "tree", body: { backupPaths: [], excludeCaches: {} } });
    // On success the editor refetches mounts instead of emptying its state
    // locally.
    expect(mountsCalls).toBe(2);
    expect(screen.queryByRole("button", { name: /1 exclusion/ })).toBeNull();
    // The custom root is gone and no switch is on anywhere.
    expect(screen.queryByRole("treeitem", { name: /user\/backups/ })).toBeNull();
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("true");
    expect(
      within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })))
        .getByRole("switch")
        .getAttribute("aria-checked"),
    ).toBe("false");
  });

  it("the reset save does not fire the narrowing note (auto-detection is not a narrowed selection)", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excluded: [`${MOUNT}/transcoding`],
    });
    await renderEditor(true, 1700000000);
    mountsReply = mountsResponse(); // post-reset: one auto-detected include
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    });
    expect(screen.queryByText(NARROWED_NOTE_EN)).toBeNull();
  });

  it("a failed reset PATCH toasts the verbatim server error, shakes the button, and leaves selection state untouched", async () => {
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`] });
    await renderEditor(true, 1700000000);
    patchReplies = [{ ok: false, error: "scrubbed failure" }];
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    });
    expect(screen.getByText("scrubbed failure")).toBeTruthy();
    expect(patches).toHaveLength(1);
    // Nothing was changed locally. The mount stays mixed because of the
    // remembered exclusion below it.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    expect(screen.getByRole("button", { name: /1 exclusion/ })).toBeTruthy();
    expect(mountsCalls).toBe(1); // no refetch on failure
    expect(screen.getByRole("button", { name: "Reset selection" }).className).toContain("glim-shake");
  });

  it("a toggle stacked behind a pending reset does not displace it: the drain sends the reset body and the reload re-derives the editor", async () => {
    // Three selected mounts so both stacked toggles stay above the
    // zero-include floor, plus an exclusion for the reset to clear.
    const mounts: MountInfo[] = Array.from({ length: 3 }, (_, i) => ({
      source: `${HOST_ROOT}/user/appdata/w${i}`,
      dest: `/w${i}`,
      selected: true,
      isAppdata: false,
      reachable: true,
    }));
    mountsReply = mountsResponse({ mounts, excluded: [`${HOST_ROOT}/user/appdata/w0/x`] });
    await renderEditor(true, 1700000000);
    let resolveFirst!: (r: { ok: boolean }) => void;
    patchReplies = [new Promise((res) => (resolveFirst = res))];

    // 1. Toggle A (uncheck w1) goes in flight; its PATCH is held pending.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /appdata\/w1/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patchBodies).toHaveLength(1);

    // 2. The reset is confirmed while toggle A is in flight and waits behind
    //    it.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    });
    expect(patchBodies).toHaveLength(1);

    // 3. Toggle B (uncheck w2) lands while the reset is pending. w0 keeps the
    //    item non-empty, so the floor allows it, and it must not replace the
    //    pending reset.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /appdata\/w2/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patchBodies).toHaveLength(1);

    // The served post-reset state: all three mounts auto-detected, the
    // remembered exclusion gone.
    mountsReply = mountsResponse({ mounts });
    // 4. Toggle A succeeds and the drain sends the reset body, not the live
    //    list under "tree".
    await act(async () => {
      resolveFirst({ ok: true });
    });
    expect(patchBodies).toHaveLength(2);
    expect(patchBodies[1]).toEqual({ name: "tree", body: { backupPaths: [], excludeCaches: {} } });
    expect(maxConcurrentPatches).toBe(1);

    // 5. The reload replaces everything: w2 is checked again and the exclusion
    //    list is gone.
    expect(mountsCalls).toBe(2);
    expect(screen.getByRole("treeitem", { name: /appdata\/w2/ }).getAttribute("aria-checked")).toBe("true");
    expect(screen.queryByRole("button", { name: /1 exclusion/ })).toBeNull();
  });

  it("the post-reset refetch waits for a mutation stacked during the reset PATCH's flight", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excluded: [`${MOUNT}/transcoding`],
      excludeCaches: { [MOUNT]: false },
    });
    await renderEditor(true, 1700000000);
    let resolveReset!: (r: { ok: boolean }) => void;
    let resolveFlip!: (r: { ok: boolean }) => void;
    patchReplies = [new Promise((res) => (resolveReset = res)), new Promise((res) => (resolveFlip = res))];

    // 1. The confirmed reset PATCH goes in flight (held pending).
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    });
    expect(patchBodies).toHaveLength(1);
    expect(patchBodies[0]).toEqual({ name: "tree", body: { backupPaths: [], excludeCaches: {} } });

    // 2. A CACHEDIR flip is queued while the reset PATCH is in flight.
    await act(async () => {
      fireEvent.click(within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch"));
    });
    expect(patchBodies).toHaveLength(1); // stacked, never concurrent
    expect(
      (within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch") as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    // What the server holds once the reset has cleared the map and the flip
    // has stored its entry again.
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excludeCaches: { [MOUNT]: true },
    });

    // 3. The reset succeeds and the flip drains as one further PATCH (held).
    //    The reload has to wait for it, or its response could predate the flip
    //    and overwrite it locally.
    await act(async () => {
      resolveReset({ ok: true });
    });
    expect(patchBodies).toHaveLength(2);
    expect(patchBodies[1]).toEqual({ name: "tree", body: { excludeCaches: { [MOUNT]: true } } });
    expect(maxConcurrentPatches).toBe(1);
    expect(mountsCalls).toBe(1); // refetch deferred behind the drain

    // 4. Only once the flip settles does the reload run, so the UI ends up
    //    showing the flip.
    await act(async () => {
      resolveFlip({ ok: true });
    });
    expect(mountsCalls).toBe(2);
    expect(
      within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch").getAttribute("aria-checked"),
    ).toBe("true");
    expect(screen.queryByRole("button", { name: /1 exclusion/ })).toBeNull();
  });

  it("a successful narrowing save with lastBackup non-null renders the role=status warn note under the tree", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    await renderEditor(true, 1700000000);
    // 2 includes at load; unchecking one mount narrows the attempted save 2 -> 1.
    const box = within(screen.getByRole("treeitem", { name: /\/media ← / })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    // Queried by text (the success toast is a role="status" live region too);
    // the role/token are then asserted on the element itself.
    const note = screen.getByText(NARROWED_NOTE_EN);
    expect(note.tagName).toBe("P");
    expect(note.getAttribute("role")).toBe("status");
    expect(note.className).toContain("text-statusWarn");
    // The note sits under the tree, before the Add row.
    const add = screen.getByRole("button", { name: "Add" });
    expect(note.compareDocumentPosition(add) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("no narrowing note when lastBackup is null (never backed up: nothing narrowed relative to a snapshot)", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    await renderEditor(true, null);
    const box = within(screen.getByRole("treeitem", { name: /\/media ← / })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    expect(screen.queryByText(NARROWED_NOTE_EN)).toBeNull();
  });

  it("no narrowing note on a net-widening save", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: false, isAppdata: false, reachable: true },
      ],
    });
    await renderEditor(true, 1700000000);
    const box = within(screen.getByRole("treeitem", { name: /\/media ← / })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box); // 1 -> 2: widening
    });
    expect(screen.queryByText(NARROWED_NOTE_EN)).toBeNull();
  });

  it("a queue-collapsed burst nets to narrowing: 5 -> 3 -> 4 during one in-flight save fires the note and drains once", async () => {
    const mounts: MountInfo[] = Array.from({ length: 5 }, (_, i) => ({
      source: `${HOST_ROOT}/user/appdata/d${String(i)}`,
      dest: `/d${i}`,
      selected: true,
      isAppdata: false,
      reachable: true,
    }));
    mountsReply = mountsResponse({ mounts });
    await renderEditor(true, 1700000000);
    let resolveFirst!: (r: { ok: boolean }) => void;
    patchReplies = [new Promise((res) => (resolveFirst = res))];
    // First toggle starts the in-flight attempt (live list: 4 includes).
    const d0 = within(screen.getByRole("treeitem", { name: /appdata\/d0/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(d0);
    });
    // While it is in flight, uncheck and re-check d1; the burst collapses into
    // the next drain. The first attempt (4) is already below the saved 5, so
    // the note fires when it succeeds. Comparing against the last saved count,
    // not the count before the click, is what nets stacked toggles correctly.
    const d1 = within(screen.getByRole("treeitem", { name: /appdata\/d1/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(d1);
    });
    await act(async () => {
      fireEvent.click(d1);
    });
    await act(async () => {
      resolveFirst({ ok: true });
    });
    expect(screen.getByText(NARROWED_NOTE_EN)).toBeTruthy();
    // The burst drained as one further fetch carrying the live 4-include list.
    expect(patches).toHaveLength(2);
    expect(patches[1].paths).toEqual(
      Array.from({ length: 5 }, (_, i) => `${HOST_ROOT}/user/appdata/d${String(i)}`).filter((p) => !p.endsWith("d0")),
    );
    expect(patches[1].opts).toEqual({ selectionSource: "tree" });
  });

  it("the hint points to Reset selection instead of claiming that unticking everything reverts to the default", async () => {
    await renderEditor();
    expect(screen.getByText(/Unticking everything is blocked; use Reset selection to return to the automatic appdata default\.$/)).toBeTruthy();
    expect(screen.queryByText(/reverts to the automatic appdata default/)).toBeNull();
  });

  it("the narrowing note is transient: closing and reopening the section clears it (no server-side include-count history)", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    const view = await renderEditor(true, 1700000000);
    const box = within(screen.getByRole("treeitem", { name: /\/media ← / })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    expect(screen.getByText(NARROWED_NOTE_EN)).toBeTruthy();

    view.rerender(
      <Providers>
        <EditorHarness open={false} lastBackup={1700000000} />
      </Providers>,
    );
    view.rerender(
      <Providers>
        <EditorHarness open lastBackup={1700000000} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.queryByText(NARROWED_NOTE_EN)).toBeNull();
  });
});

// Each root row has a switch for its entry in the stored excludeCaches map. It
// sits in a presentation sub-row after the treeitem, with an InfoBubble saying
// that the flag applies to the whole backup of the container, and is disabled
// for unreachable mounts. Flips go through the same one-deep queue as
// selection saves, so the two never overlap. A failure reverts to the stored
// value, toasts the error and shakes the switch; success is quiet.

const CACHEDIR_TOGGLE_EN = "Skip cache folders (CACHEDIR.TAG)";
const CACHEDIR_SCOPE_EN = "Applies to the entire backup of this container, not only this folder.";
const MEDIA_MOUNT = `${HOST_ROOT}/user/media`;

/** The CACHEDIR sub-row of a root. It renders after the root's expanded
 *  children, so it is the next sibling only while the root is collapsed; scan
 *  forward up to the next treeitem instead. The exclusions wrapper is
 *  role="presentation" too but holds a button, not a switch. */
function cachedirRow(row: HTMLElement): HTMLElement {
  let el: HTMLElement | null = row.nextElementSibling;
  while (el && el.getAttribute("role") !== "treeitem") {
    if (el.getAttribute("role") === "presentation" && el.querySelector('[role="switch"]')) return el;
    el = el.nextElementSibling;
  }
  throw new Error("the root's CACHEDIR sub-row must render inside its own fragment, before the next treeitem");
}

describe("per-root CACHEDIR.TAG toggle", () => {
  it("renders the switch under every root with the caller-drawn label and scope bubble; unreachable mounts disabled", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/srv9/elsewhere`, dest: "/gone", selected: false, isAppdata: false, reachable: false },
      ],
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [MOUNT]: true },
    });
    await renderEditor();

    const plex = cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    const gone = cachedirRow(screen.getByRole("treeitem", { name: /\/mnt\/srv9\/elsewhere/ }));
    const custom = cachedirRow(screen.getByRole("treeitem", { name: /user\/backups/ }));

    // The switch is named by its aria-label; the visible label beside it is
    // drawn by the caller at the tree's 12px size.
    for (const sub of [plex, gone, custom]) {
      const sw = within(sub).getByRole("switch", { name: CACHEDIR_TOGGLE_EN });
      const label = within(sub).getByText(CACHEDIR_TOGGLE_EN);
      expect(label.className).toContain("text-carbon-textSub");
      expect(label.className).toContain("text-xs");
      // The scope is explained in an InfoBubble beside the label.
      expect(within(sub).getByLabelText(CACHEDIR_SCOPE_EN)).toBeTruthy();
      expect(sw.tagName).toBe("BUTTON");
    }

    // Only plex is marked skipped in the fixture.
    expect(within(plex).getByRole("switch").getAttribute("aria-checked")).toBe("true");
    expect(within(gone).getByRole("switch").getAttribute("aria-checked")).toBe("false");
    expect(within(custom).getByRole("switch").getAttribute("aria-checked")).toBe("false");

    // Unreachable mounts cannot be backed up, so their switch is disabled.
    // (Plain property checks: no @testing-library/jest-dom in this repo.)
    expect((within(gone).getByRole("switch") as HTMLButtonElement).disabled).toBe(true);
    expect((within(plex).getByRole("switch") as HTMLButtonElement).disabled).toBe(false);
    expect((within(custom).getByRole("switch") as HTMLButtonElement).disabled).toBe(false);

    // The sub-rows never enter the treeitem set (presentation wrappers).
    expect(screen.getAllByRole("treeitem")).toHaveLength(3);
  });

  it("flipping a switch PATCHes the full live map as a caches-only body and renders the flip optimistically, quietly", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excludeCaches: { [MOUNT]: false, [MEDIA_MOUNT]: true },
    });
    await renderEditor();

    const plexSub = cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    await act(async () => {
      fireEvent.click(within(plexSub).getByRole("switch"));
    });

    // One caches-only request: nothing about backupPaths changed.
    expect(patchBodies).toHaveLength(1);
    expect(patchBodies[0]).toEqual({
      name: "tree",
      body: { excludeCaches: { [MOUNT]: true, [MEDIA_MOUNT]: true } },
    });
    // Optimistic local flip.
    expect(within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })))
      .getByRole("switch")
      .getAttribute("aria-checked")).toBe("true");
    // Quiet success: no Saved toast for a caches-only save.
    expect(screen.queryByText("Saved")).toBeNull();
  });

  it("a caches flip stacked behind an in-flight selection save drains as one further fetch, never overlapping it", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excludeCaches: { [MOUNT]: false },
    });
    await renderEditor(true, 1700000000);
    let resolveFirst!: (r: { ok: boolean }) => void;
    patchReplies = [new Promise((res) => (resolveFirst = res))];

    // 1. A selection save goes in flight (uncheck the media mount).
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/media ← / })).getByRole("checkbox", { hidden: true }));
    });
    expect(patchBodies).toHaveLength(1);

    // 2. Flip the plex switch while that save is pending: no second request
    //    yet, and the switch is disabled until its flip is acknowledged.
    const plexSub = cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    await act(async () => {
      fireEvent.click(within(plexSub).getByRole("switch"));
    });
    expect(patchBodies).toHaveLength(1);
    expect(
      (within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch") as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    // 3. The selection save settles and the flip drains as one further
    //    request with the live map.
    await act(async () => {
      resolveFirst({ ok: true });
    });
    expect(patchBodies).toHaveLength(2);
    expect(patchBodies[1]).toEqual({ name: "tree", body: { excludeCaches: { [MOUNT]: true } } });
    expect(maxConcurrentPatches).toBe(1);
    // Acknowledged: the busy flag clears and the switch re-enables.
    expect(
      (within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch") as HTMLButtonElement)
        .disabled,
    ).toBe(false);
  });

  it("a failed caches PATCH reverts to the stored value, toasts verbatim, and shakes the switch row", async () => {
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excludeCaches: { [MOUNT]: false, [MEDIA_MOUNT]: true },
    });
    await renderEditor();
    patchReplies = [{ ok: false, error: "scrubbed failure" }];

    await act(async () => {
      fireEvent.click(within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch"));
    });

    expect(screen.getByText("scrubbed failure")).toBeTruthy();
    expect(patchBodies).toHaveLength(1); // no retry
    const sub = cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    expect(within(sub).getByRole("switch").getAttribute("aria-checked")).toBe("false");
    expect((within(sub).getByRole("switch") as HTMLButtonElement).disabled).toBe(false);
    // The shake lands on the inner flex row of the wrapper; a keyed remount
    // replays it.
    expect((sub.firstElementChild as HTMLElement).className).toContain("glim-shake");
  });
});
