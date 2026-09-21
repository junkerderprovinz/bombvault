// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// FoldersEditor tree integration tests (Phase 2, plan 03 — INTEG-01 closure).
//
// This file pins the editor's END-TO-END wire contract against the mocked api
// client (same import-after-mock harness as Containers.excludesAssistant.dom
// .test.tsx): a subfolder uncheck PATCHes the flat list with the bare mount
// include and the "!"+subfolder HOST path under selectionSource "tree"; the
// last-include toggle fires NO request (D-04, through the real editor — not
// just the tree); sub-includes that the server serves as custom rows (they are
// includes that are not EXACTLY a mount root, service.go:3755-3782) are
// ABSORBED into the tree under their mount — one presentation per path (D-02,
// RESEARCH Q1) — and closing/reopening the section restores expansion from the
// bv-tree-expanded-{name} key while children come back from the editor-lifetime
// cache with zero refetch (D-05: expansion is comfort, selection is truth).
//
// The expansion cap (64) is also proven through the component path here: 66
// live expansions persist only the 64 most recent.
// ---------------------------------------------------------------------------
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
// Same deferred-reply machinery the SelectionTree dom harness uses: a pending
// promise pins a save in flight so the queue's serialize/drain behavior (and
// the Phase 3 reset/narrowing contracts over it) is observable step by step.
let patchReplies: ({ ok: boolean; error?: string } | Promise<{ ok: boolean; error?: string }>)[] = [];
let mountsCalls = 0;
// Composed-body capture (plan 03 Task 2): one entry per setContainerTargets
// call, verbatim body included, so tests can pin EXACTLY which classes a
// drain carried. maxConcurrent is the no-overlap proof (T-03-07): the mock
// counts calls in flight; two overlapping PATCHes would push it to 2.
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
      // Legacy derived view (Phase 2 assertions): the flat paths/source shape
      // the retired setBackupPaths mock captured, projected out of the
      // composed body so the old expectations keep pinning the same wire
      // contract through the single Task 2 endpoint.
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

// Imported AFTER vi.mock so the components pick up the mocked client.
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
    // The server always serves the CACHEDIR map as an object (RESTIC-01 read
    // side, plan 03-01); empty = nothing skipped yet.
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

describe("FoldersEditor tree integration (INTEG-01, D-02, D-04, D-05)", () => {
  it("unchecking a subfolder PATCHes the bare mount plus the !-prefixed host path with selectionSource tree", async () => {
    browseReplies = [plexListing()];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    // hidden: true — the input carries aria-hidden (WR-02), so ByRole's
    // default a11y-tree filter would not return it.
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

  it("unchecking the item's LAST include fires no request (D-04 through the real FoldersEditor)", async () => {
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

  it("removing the LAST custom row is blocked by the same floor as the checkbox (no exclusions-only save)", async () => {
    // The end state the Remove chip used to reach silently: zero includes plus
    // one left-over exclusion. That list is not EMPTY, so the server's
    // empty-selection guard passes it, it gets stored, and from then on every
    // backup of this container succeeds while capturing nothing - the first one
    // overwriting AppdataPaths, the last record of where the data was. Through
    // the checkbox the identical state is refused; through this chip it was not.
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
    // The sideways route through the floor. The second toggle was checked
    // against a mirror still carrying the first toggle's optimistic include;
    // when that save fails and the include is taken back, the revert lands on
    // zero includes - and the chained drain then PATCHes exactly the
    // exclusions-only list the floor exists to prevent. The user would see a
    // fail toast followed by a green "Saved".
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: SECOND_MOUNT, dest: "/media", selected: false, isAppdata: false, reachable: true },
      ],
      excluded: [`${MOUNT}/transcoding`],
    });
    // The first PATCH is held pending so the second toggle can land while it is
    // in flight; it then fails. A second request must never be built.
    await renderEditor();
    let failFirst!: (r: { ok: boolean; error?: string }) => void;
    patchReplies = [new Promise((res) => (failFirst = res))];

    const second = within(screen.getByRole("treeitem", { name: /user\/appdata\/sonarr/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(second); // tick the second mount -> save in flight
    });
    const first = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(first); // untick the first WHILE that save is in flight
    });
    await act(async () => {
      failFirst({ ok: false, error: "boom" });
    });

    // The queue still drains the stacked toggle - that is by design, latest
    // mirror wins. What matters is WHAT it sends: server truth, never the
    // exclusions-only list. Before the fix the second entry was
    // ["!/mnt/user/appdata/plex/transcoding"] alone.
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
    // Nothing prunes that map - not the server's setter, not the selection
    // save - and the switch only renders for a root that still has a row. The
    // entry therefore stayed ON with no control left to turn it off: every
    // later backup ran with --exclude-caches, skipping every tagged directory
    // under the roots that DID remain, while every switch on screen read off.
    mountsReply = mountsResponse({
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [STANDALONE]: true, [MOUNT]: false },
    });
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /user\/backups/ });
    await act(async () => {
      fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    });

    // Two drains, serialized by the same one-deep queue as everything else:
    // the shrunken selection, then the map without the removed root. The second
    // one is scheduled from the FIRST one's success branch, so a failed removal
    // never takes the CACHEDIR entry with it. Only the paths half toasts, a
    // caches-only save is quiet by design.
    expect(patchBodies.length).toBe(2);
    expect(patchBodies[0].body.backupPaths).toEqual([MOUNT]);
    expect(patchBodies[0].body.excludeCaches).toBeUndefined();
    expect(patchBodies[1].body.excludeCaches).toEqual({ [MOUNT]: false });
    expect(maxConcurrentPatches).toBe(1);
  });

  it("a FAILED removal leaves the CACHEDIR entry alone", async () => {
    // The cleanup used to be scheduled next to the removal rather than after
    // it, so it went out as its own PATCH and landed even when the removal in
    // front of it had failed. A structural save is never reverted, so nothing
    // undid it: the row came back on the next reload with its switch silently
    // flipped off, untouched by the user.
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
    // The mount would normally absorb a sub-include and let the tree present
    // it instead - but the tree builds its children from browse listings, and a
    // folder that is no longer on disk appears in none. It had no row, no
    // child and no warning anywhere while the mount row still counted it. At
    // run time it is dropped from the positionals and the backup is recorded a
    // success; the empty-backup guard only speaks up once EVERY include is
    // gone.
    mountsReply = mountsResponse({
      custom: [{ path: `${MOUNT}/Library`, exists: false }],
    });
    await renderEditor();

    const row = screen.getByRole("treeitem", { name: /Library/ });
    expect(within(row).getByText("no data folder detected (nothing to back up here)")).toBeTruthy();
  });

  it("adding a folder inside an excluded branch clears the covering exclusion instead of saving a no-op", async () => {
    // Add used to pass the exclusions through untouched, so the server pruned
    // the redundant include, kept the exclusion, and the backup argv still
    // carried --exclude for the branch - which swallows the added folder. The
    // stored selection came out byte-identical to before the click while the UI
    // toasted Saved and the row count went up by one. The tree's own checkbox
    // on an excluded node does the opposite (it clears the covering exclusion),
    // and the two routes now agree.
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`] });
    await renderEditor();

    const input = screen.getByPlaceholderText("user/appdata");
    await act(async () => {
      fireEvent.change(input, { target: { value: "user/appdata/plex/transcoding/keepme" } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    // The exclusion is gone. No own include is added: the mount above already
    // covers the path, and a redundant descendant include is what the server
    // prunes (which is how the no-op arose in the first place).
    expect(patches).toEqual([
      { name: "tree", paths: [MOUNT], opts: { selectionSource: "tree" } },
    ]);
  });

  it("adding a folder that is the PARENT of an existing include really saves it", async () => {
    // The narrow version of the guard skipped this one: classifyNode answers
    // "mixed" when an include sits strictly BELOW the added path, and that is
    // the opposite of redundant - nothing covers it from above, so the server
    // keeps the parent and prunes the child. Skipping it sent the list out
    // unchanged, cleared the input, added a row and toasted Saved, while the
    // folder was never backed up.
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

    // The parent goes out. The server prunes the child as redundant, which is
    // exactly the right direction.
    expect(patches).toEqual([
      {
        name: "tree",
        paths: [MOUNT, STANDALONE, `${STANDALONE}/daily`],
        opts: { selectionSource: "tree" },
      },
    ]);
  });

  it("adding a folder already covered by an included mount changes nothing and keeps the staged pick", async () => {
    // Same silent-prune shape without any exclusion involved: the duplicate
    // guard only compared against the custom list and the exact includes, so a
    // path under an included mount passed it, counted toward the mount's path
    // total, and vanished on the next reload.
    await renderEditor();

    const input = screen.getByPlaceholderText("user/appdata");
    await act(async () => {
      fireEvent.change(input, { target: { value: "user/appdata/plex/Media" } });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    expect(patches).toEqual([]);
    // The pick stays in the field - that is the feedback, same as a literal
    // duplicate.
    expect((input as HTMLInputElement).value).toBe("user/appdata/plex/Media");
  });

  it("absorbs a stored sub-include into its mount: mixed root, checked child, no duplicate custom row", async () => {
    // The server serves a sub-mount include as a custom row (exact-match
    // rule); the tree must absorb it — D-02: one presentation per path.
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: false, reachable: true }],
      custom: [{ path: SUB_INCLUDE, exists: true }],
    });
    browseReplies = [plexListing()];
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });

    // Whitelist start-state: the include strictly below renders the mount
    // mixed WITHOUT the mount itself being selected.
    expect(root.getAttribute("aria-checked")).toBe("mixed");

    // No second presentation: the sub-include is NOT a level-1 custom row,
    // and the root group counts one real root.
    expect(screen.queryByRole("treeitem", { name: /^\/mnt\/user\/appdata\/plex\/Media$/ })).toBeNull();
    expect(root.getAttribute("aria-setsize")).toBe("1");
    expect(root.getAttribute("aria-posinset")).toBe("1");

    // Once expanded, the sub-include renders exactly once — as a checked
    // child treeitem under its mount.
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
    // Custom rows carry their remove chip inside the row, so the accessible
    // name is the path plus the chip text — match on the path fragment.
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

  it("browses WITH the hidden opt-in, so dot-directories have a row to tick", async () => {
    // Without it the server filters every dot-prefixed directory out of the
    // listing and the 200 envelope carries no hint that it did, so the tree has
    // neither a row nor anything to warn from. Ticking children is a whitelist,
    // so a folder with no row is simply left out of the backup - and in appdata
    // the dot-directories are the ones that matter (.storage holds Home
    // Assistant's config, auth and device registry).
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
    // And the row is really there to be ticked.
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
    // D-05 write side: the expanded HOST path lands in the per-container key.
    expect(JSON.parse(localStorage.getItem("bv-tree-expanded-tree") ?? "[]")).toEqual([MOUNT]);

    // Close the section: the tree unmounts, editor state survives.
    view.rerender(
      <Providers>
        <EditorHarness open={false} />
      </Providers>,
    );
    expect(screen.queryByRole("tree")).toBeNull();

    // Reopen: expansion restored from the key, children served by the
    // editor-lifetime cache (no second browse), states re-derived from the
    // (I, E) mirror — the server round-trip, never the localStorage key.
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

  it("persists only the 64 most recent expansions through the component path (D-05 cap)", async () => {
    // 66 reachable mounts, expanded one by one through real clicks; every
    // browse reply defaults to an empty listing (dirs: []) so each expansion
    // is honest and cheap. That many clicks outlast the default budget while
    // the rest of the suite runs alongside.
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
    // All 66 expansions in one act: the functional expandedOrder reducer
    // chains correctly across batched events, and a single flush keeps the
    // jsdom round-trips well under the test timeout.
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

// ---------------------------------------------------------------------------
// Per-root effective-selection preview (Phase 3, plan 02 — SELECT-03 first
// half, D-01, INTEG-04).
//
// Every root row (each mount AND each standalone custom row) carries a muted
// "{n} paths" line derived from the stored (includes, exclusions) mirror —
// never from checked nodes on screen — so the count is exactly what the next
// backup PATCH serializes as bare positionals. Zero is information: a fully
// deselected root renders "0 paths", never a hidden line.
// ---------------------------------------------------------------------------

describe("per-root effective-selection preview (SELECT-03 first half, D-01)", () => {
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

    // The preview line renders INSIDE the root treeitem's label column, so it
    // is announced with the row's accessible name (the appdataDefault /
    // notReachable placement precedent) — not a detached node beside it.
    const plex = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("1 paths");
    expect(plex.className).toContain("text-carbon-textMuted");

    // Fully deselected mount: the 0 case renders (INTEG-04 — zero is
    // information, not an empty state to hide).
    within(screen.getByRole("treeitem", { name: /\/media ← \/mnt\/user\/media/ })).getByText("0 paths");

    // Unreachable mount: its own 0 case renders too.
    within(screen.getByRole("treeitem", { name: /\/mnt\/srv9\/elsewhere/ })).getByText("0 paths");

    // Standalone custom row: included custom paths count as their own root
    // include on load, so the row announces "1 paths".
    within(screen.getByRole("treeitem", { name: /user\/backups/ })).getByText("1 paths");
  });

  it("a carve-out toggle leaves the count unchanged; narrowing the parent to child includes changes it", async () => {
    // Two selected mounts so the D-04 last-include guard never fires while
    // the plex root is narrowed below.
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: `${HOST_ROOT}/user/media`, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
    });
    browseReplies = [plexListing(), plexListing()];
    await renderEditor();

    // 1. Carve-out: unchecking a child of an included parent creates an
    //    exclusion and leaves the parent include untouched — the count of
    //    includes at-or-under the root is unchanged.
    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    within(root).getByText("1 paths");
    const child = within(screen.getByRole("treeitem", { name: /transcoding/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(child);
    });
    within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("1 paths");
    // The carve-out really happened (exclusion stored, mount mixed): the
    // serialized list carries both mounts' bare includes plus the "!" entry.
    expect(patches[patches.length - 1]?.paths).toEqual([
      MOUNT,
      `${HOST_ROOT}/user/media`,
      `!${MOUNT}/transcoding`,
    ]);

    // 2. Narrowing: uncheck the parent (its include drops; the media mount
    //    keeps the item non-empty), then check two children — the preview now
    //    counts the child includes (0 paths, then 1, then 2).
    const parentBox = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(parentBox);
    });
    within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByText("0 paths");
    // The dormant carve-out exclusion is still stored below it (D-01 memory):
    // bare includes first, then the "!" entry.
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

// ---------------------------------------------------------------------------
// Per-root reviewable exclusions (Phase 3, plan 02 — INTEG-03, D-03, D-04).
//
// Under every root carrying at least one exclusion STRICTLY below it a
// collapsible "{n} exclusions" section renders: a plain disclosure button
// plus, when open, a muted non-interactive list of relative paths. The list
// is complete by construction — derived from the full stored exclusion set
// (dormant entries included), never from which tree nodes are loaded — and
// nothing in it can mutate the selection (one toggle pipeline, T-02-10).
// ---------------------------------------------------------------------------

describe("per-root reviewable exclusions (INTEG-03, D-03, D-04)", () => {
  it("renders the disclosure for a root with exclusions, expands to relative mono rows, and renders no section at zero", async () => {
    mountsReply = mountsResponse({ excluded: [`${MOUNT}/transcoding`, `${MOUNT}/Media`] });
    await renderEditor();

    // Zero-exclusion roots do not exist in this fixture beyond the mount
    // itself (it carries both), so the negative case is pinned in the
    // dedicated zero test below. Here: the button exists, collapsed first.
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
      // title carries the full relative path; the house mono ltr long-path
      // pattern renders the row.
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

  it("D-04 dormant case: fully-deselected root renders unchecked box, '0 paths' and '{n} exclusions' together, never hidden", async () => {
    mountsReply = mountsResponse({
      mounts: [{ source: MOUNT, dest: "/config", selected: false, isAppdata: false, reachable: true }],
      excluded: [`${MOUNT}/transcoding`],
    });
    await renderEditor();

    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(root.getAttribute("aria-checked")).toBe("false");
    within(root).getByText("0 paths");
    expect(screen.getByRole("button", { name: /1 exclusions/ })).toBeTruthy();

    // Expanding the dormant section lists the remembered exclusion.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /1 exclusions/ }));
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

    // No mutation affordance anywhere in the list body, and rows are not
    // focusable (no tabindex; li is not focusable by default and must not
    // become so).
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

    // The tree node is NEVER expanded — zero browse calls — yet the
    // disclosure knows and lists both exclusions.
    expect(browseCalls).toEqual([]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /2 exclusions/ }));
    });
    expect(browseCalls).toEqual([]);
    expect(screen.getAllByRole("listitem").map((li) => li.textContent)).toEqual(["Media", "transcoding"]);
  });
});

// ---------------------------------------------------------------------------
// Reset selection and the narrowing note (Phase 3, plan 03 — INTEG-04 D-05,
// SELECT-03 D-02).
//
// The reset is the ONE sanctioned exit back to auto-detection: confirmed
// (fail tone, every consequence named), serialized through the same one-deep
// queue as toggles, body exactly {backupPaths: [], excludeCaches: {}} with
// NO selectionSource (the Phase 1 guard is strictly tree-source-gated, so
// the reset passes it by design — while tree toggles keep the guard live;
// the cleared caches map is review WR-04: a stored toggle keyed by a root
// the reset removes must not survive as an orphaned --exclude-caches with
// no switch left to turn it off), and non-optimistic: the editor refetches
// on ok and the served auto-detected state replaces everything, remembered
// exclusions and cache-folder settings gone. The narrowing note is
// event-driven (D-02): a successful save whose attempted include count is
// lower than the last-SAVED count, gated on container.lastBackup, announced
// as a polite role="status" warn line, transient for the editor session
// only.
// ---------------------------------------------------------------------------

const NARROWED_NOTE_EN =
  "The selection now covers fewer folders than before. From the next backup on, snapshots will contain only the selected folders. Existing snapshots are unchanged.";
const RESET_CONFIRM_EN =
  "Reset the folder selection? The container returns to automatic detection (appdata default) and all remembered exclusions and cache-folder settings are removed.";

describe("Reset selection, narrowing note, guard and hint copy (INTEG-04 D-05, SELECT-03 D-02)", () => {
  it("Reset selection: fail-tone confirm naming every consequence, one serialized PATCH clearing selection AND the caches map, refetch renders the auto-detected selection", async () => {
    // The orphan scenario WR-04 exists for: the stored caches map carries a
    // true keyed by a STANDALONE CUSTOM root — exactly the kind of row the
    // reset removes (the selection becomes auto-detected), so a reset that
    // cleared only backupPaths would leave the key firing --exclude-caches
    // for every future backup with no switch left anywhere to turn it off.
    mountsReply = mountsResponse({
      excluded: [`${MOUNT}/transcoding`],
      custom: [{ path: STANDALONE, exists: true }],
      excludeCaches: { [STANDALONE]: true },
    });
    await renderEditor(true, 1700000000);
    // Pre-reset: the remembered exclusion is visible, the mount included, and
    // the orphan-to-be's switch is ON.
    expect(screen.getByRole("button", { name: /1 exclusions/ })).toBeTruthy();
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
    // No status colour on the commit button (GlimStone 1.12.0). The window
    // carries the weight in WORDS - the sentence above names every consequence
    // - and the button looks like its sibling on purpose. It used to be
    // bg-statusFailSolid, and this assertion is what that change had to walk
    // through, which is the point of pinning it the other way round now.
    const confirmBtn = within(dialog).getByRole("button", { name: "Confirm" });
    expect(confirmBtn.className).not.toContain("bg-statusFailSolid");

    // The server's post-reset state: auto-detected default, exclusions gone,
    // custom row gone, caches map CLEARED (the reset PATCH below is what
    // clears it — the fixture models the fixed server; the pre-fix server
    // kept the stored map, which is the orphan the review flagged).
    mountsReply = mountsResponse();
    await act(async () => {
      fireEvent.click(confirmBtn);
    });

    // Exactly one PATCH for the reset, empty list, NO selection source: the
    // Phase 1 guard is strictly gated on the literal "tree", so only this
    // shape reaches auto-detection (the api helper omits the key when no
    // source is passed — pinned here by the helper call carrying no opts).
    expect(patches).toHaveLength(1);
    expect(patches[0]).toEqual({ name: "tree", paths: [] });
    expect(patches[0].opts).toBeUndefined();
    // WR-04: the same body clears the caches map — the composed reset PATCH
    // carries both cleared classes in one serialized request.
    expect(patchBodies[0]).toEqual({ name: "tree", body: { backupPaths: [], excludeCaches: {} } });
    // On ok the editor refetches mounts (non-optimistic: state comes from the
    // served response, never a local emptying).
    expect(mountsCalls).toBe(2);
    expect(screen.queryByRole("button", { name: /1 exclusions/ })).toBeNull();
    // The custom root is gone (auto-detected mounts only) and no switch
    // renders ON anywhere: the orphaned true did not survive the reset.
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
    // Non-optimistic failure: nothing was ever mutated locally. The selected
    // mount classifies MIXED (not "true") — the fixture carries a remembered
    // exclusion strictly below it, which is exactly the D-01 remembered-partial
    // classification — and "untouched" means it still does after the failure.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    expect(screen.getByRole("button", { name: /1 exclusions/ })).toBeTruthy();
    expect(mountsCalls).toBe(1); // no refetch on failure
    expect(screen.getByRole("button", { name: "Reset selection" }).className).toContain("glim-shake");
  });

  it("a toggle stacked behind a PENDING reset never displaces it: the drain sends the reset body and the reload re-derives the editor (WR-01)", async () => {
    // Three selected mounts so both stacked toggles stay above the D-04
    // zero-include floor, plus a remembered exclusion for the reset to clear.
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

    // 2. The user confirms the reset while toggle A is in flight: the reset
    //    descriptor is PENDING behind it — stacked, never concurrent.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reset selection" }));
    });
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    });
    expect(patchBodies).toHaveLength(1);

    // 3. Toggle B (uncheck w2) lands while the reset is still pending (w1's
    //    row is busy from toggle A, w2's is not; w0 keeps the item non-empty,
    //    so the D-04 floor allows it). Pre-WR-01 this overwrote the pending
    //    reset desc; it must not.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /appdata\/w2/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patchBodies).toHaveLength(1);

    // The served post-reset state: all three mounts auto-detected, the
    // remembered exclusion gone.
    mountsReply = mountsResponse({ mounts });
    // 4. Toggle A settles ok: the drain carries the RESET body — the empty
    //    no-source list plus the cleared caches map — NOT the live pre-reset
    //    list under "tree".
    await act(async () => {
      resolveFirst({ ok: true });
    });
    expect(patchBodies).toHaveLength(2);
    expect(patchBodies[1]).toEqual({ name: "tree", body: { backupPaths: [], excludeCaches: {} } });
    expect(maxConcurrentPatches).toBe(1);

    // 5. The reload re-derives everything from the served auto-detected
    //    state: toggle B's optimistic uncheck is superseded (w2 back to
    //    checked), the exclusion disclosure is gone.
    expect(mountsCalls).toBe(2);
    expect(screen.getByRole("treeitem", { name: /appdata\/w2/ }).getAttribute("aria-checked")).toBe("true");
    expect(screen.queryByRole("button", { name: /1 exclusions/ })).toBeNull();
  });

  it("the post-reset refetch waits for a mutation stacked during the reset PATCH's flight (WR-02)", async () => {
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

    // 2. A CACHEDIR flip stacks DURING the reset PATCH's flight — the same
    //    window a tree toggle or another flip could land in.
    await act(async () => {
      fireEvent.click(within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch"));
    });
    expect(patchBodies).toHaveLength(1); // stacked, never concurrent
    expect(
      (within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch") as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    // The served post-reset state: auto-detected mounts, and the caches map
    // the server holds after the reset cleared it and the flip's whole-map
    // drain re-persisted the flipped entry (the fixed-server model).
    mountsReply = mountsResponse({
      mounts: [
        { source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true },
        { source: MEDIA_MOUNT, dest: "/media", selected: true, isAppdata: false, reachable: true },
      ],
      excludeCaches: { [MOUNT]: true },
    });

    // 3. The reset settles ok: the flip drains as one further PATCH (held) —
    //    and the reload GET must NOT leave while that drain is in flight,
    //    or its response could reflect pre-drain server state and clobber
    //    the flip's optimistic apply locally.
    await act(async () => {
      resolveReset({ ok: true });
    });
    expect(patchBodies).toHaveLength(2);
    expect(patchBodies[1]).toEqual({ name: "tree", body: { excludeCaches: { [MOUNT]: true } } });
    expect(maxConcurrentPatches).toBe(1);
    expect(mountsCalls).toBe(1); // WR-02: refetch deferred behind the drain

    // 4. The flip's drain settles: only now does the reload GET leave, and
    //    the served (post-drain) state re-seeds the editor — the flip's
    //    effect is what the UI ends up showing.
    await act(async () => {
      resolveFlip({ ok: true });
    });
    expect(mountsCalls).toBe(2);
    expect(
      within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch").getAttribute("aria-checked"),
    ).toBe("true");
    expect(screen.queryByRole("button", { name: /1 exclusions/ })).toBeNull();
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
    // The note sits under the tree, before the Add row (D-02 placement).
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
    // While in flight: uncheck d1 (3), then re-check d1 (4). The burst
    // collapses into the next drain; the ATTEMPTED count of the first
    // attempt (4) is already lower than the load-time 5, so the note fires
    // on that attempt's ok — comparing against last-SAVED, not pre-mutation,
    // is what nets stacked toggles correctly (RESEARCH Pitfall 3).
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
    // The burst drained as ONE further fetch carrying the live 4-include list.
    expect(patches).toHaveLength(2);
    expect(patches[1].paths).toEqual(
      Array.from({ length: 5 }, (_, i) => `${HOST_ROOT}/user/appdata/d${String(i)}`).filter((p) => !p.endsWith("d0")),
    );
    expect(patches[1].opts).toEqual({ selectionSource: "tree" });
  });

  it("the hint teaches the reset and no longer claims unticking everything reverts to the automatic default", async () => {
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

// ---------------------------------------------------------------------------
// Per-root CACHEDIR.TAG toggle (Phase 3, plan 03 Task 2 — D-06, RESTIC-01,
// T-03-07).
//
// Every root row (mounts AND standalone customs) carries a switch that flips
// that root's entry in the stored excludeCaches map. The switch renders in a
// presentation-wrapped sub-row immediately after its treeitem, regardless of
// expand state, with the caller-drawn label and an InfoBubble disclosing the
// ITEM-WIDE scope (the flag applies to the whole backup, not the folder).
// Unreachable mounts render it disabled. Flips ride the SAME one-deep
// serialized PATCH queue as selection saves: the drain composes one body
// carrying only the classes it owes, so a caches flip stacked behind an
// in-flight selection save never overlaps it — the mock's concurrency
// counter pins that at 1. Failure reverts to the stored value (a newer flip
// on the same key survives), toasts the verbatim error, shakes the switch;
// success is quiet.
// ---------------------------------------------------------------------------

const CACHEDIR_TOGGLE_EN = "Skip cache folders (CACHEDIR.TAG)";
const CACHEDIR_SCOPE_EN = "Applies to the entire backup of this container, not only this folder.";
const MEDIA_MOUNT = `${HOST_ROOT}/user/media`;

/** The CACHEDIR sub-row that belongs to a root: the presentation wrapper
 *  carrying that root's switch. Since the UAT 2026-09-10 reposition it
 *  renders AFTER the root's expanded children group (a 16px-indented row
 *  wedged between the root and its first child read as that subtree's first
 *  subfolder), so it is the treeitem's immediate next sibling only while the
 *  root is collapsed — scan forward within the root's own fragment (up to
 *  the next treeitem) instead of assuming adjacency. The exclusions
 *  disclosure wrapper is role="presentation" too but carries a button, not
 *  a switch, so the switch query discriminates. */
function cachedirRow(row: HTMLElement): HTMLElement {
  let el: HTMLElement | null = row.nextElementSibling;
  while (el && el.getAttribute("role") !== "treeitem") {
    if (el.getAttribute("role") === "presentation" && el.querySelector('[role="switch"]')) return el;
    el = el.nextElementSibling;
  }
  throw new Error("the root's CACHEDIR sub-row must render inside its own fragment, before the next treeitem");
}

describe("per-root CACHEDIR.TAG toggle (D-06, RESTIC-01, T-03-07)", () => {
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

    // The switch's accessible name is the label (Toggle sets aria-label);
    // the caller-drawn visible label sits beside it in the tree's 12px
    // register (hideLabel is the sanctioned shape for exactly this).
    for (const sub of [plex, gone, custom]) {
      const sw = within(sub).getByRole("switch", { name: CACHEDIR_TOGGLE_EN });
      const label = within(sub).getByText(CACHEDIR_TOGGLE_EN);
      expect(label.className).toContain("text-carbon-textSub");
      expect(label.className).toContain("text-xs");
      // The scope disclosure rides an InfoBubble beside the label.
      expect(within(sub).getByLabelText(CACHEDIR_SCOPE_EN)).toBeTruthy();
      expect(sw.tagName).toBe("BUTTON");
    }

    // Stored truth renders: the fixture marks plex skipped only.
    expect(within(plex).getByRole("switch").getAttribute("aria-checked")).toBe("true");
    expect(within(gone).getByRole("switch").getAttribute("aria-checked")).toBe("false");
    expect(within(custom).getByRole("switch").getAttribute("aria-checked")).toBe("false");

    // Unreachable mounts cannot back up — their switch is disabled; the
    // reachable mount's and the custom root's are not. (Plain property
    // checks: no @testing-library/jest-dom in this repo.)
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

    // ONE fetch, caches-only: nothing about backupPaths is owed, so the
    // composed body carries exactly the excludeCaches class.
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

    // 2. Flip the plex CACHEDIR switch while that save is pending: stacked,
    //    not concurrent — no second fetch yet, and the switch disables while
    //    its flip is unacknowledged (in-flight disable).
    const plexSub = cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    await act(async () => {
      fireEvent.click(within(plexSub).getByRole("switch"));
    });
    expect(patchBodies).toHaveLength(1);
    expect(
      (within(cachedirRow(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }))).getByRole("switch") as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    // 3. The selection save settles; the queue drains the caches flip as ONE
    //    further fetch carrying the live map — never two PATCHes at once.
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
    // The shake lands on the switch's own row — the inner flex div inside the
    // classless presentation wrapper (keyed nonce remount replays it).
    expect((sub.firstElementChild as HTMLElement).className).toContain("glim-shake");
  });
});
