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
let browseReplies: (BrowseResponse | Promise<BrowseResponse>)[] = [];

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
    setBackupPaths: (name: string, paths: string[], opts?: { selectionSource?: string }) => {
      patches.push({ name, paths, opts });
      return Promise.resolve({ ok: true });
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

function mountsResponse(overrides?: Partial<ContainerMountsResponse>): ContainerMountsResponse {
  return {
    ok: true,
    mounts: [{ source: MOUNT, dest: "/config", selected: true, isAppdata: false, reachable: true }],
    custom: [],
    excluded: [],
    hostMountRoot: "/host/user",
    hostSourceRoot: HOST_ROOT,
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

function EditorHarness({ open }: { open: boolean }) {
  const { t } = useT();
  return <FoldersEditor name="tree" stack="" open={open} t={t} />;
}

function Providers({ children }: { children: React.ReactNode }) {
  return (
    <I18nProvider>
      <ToastProvider>{children}</ToastProvider>
    </I18nProvider>
  );
}

async function renderEditor(open = true) {
  const view = render(
    <Providers>
      <EditorHarness open={open} />
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
  patches.length = 0;
  browseReplies = [];
  mountsReply = mountsResponse();
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
        "At least one folder must stay selected. To back up none of this container, turn off Include in schedule.",
      ),
    ).toBeTruthy();
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
    // is honest and cheap.
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
  });
});
