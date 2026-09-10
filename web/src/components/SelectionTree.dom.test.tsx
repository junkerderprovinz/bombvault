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
import type { BrowseResponse, ContainerMountsResponse } from "../lib/api";

const browseCalls: string[] = [];
const patches: { name: string; paths: string[]; opts?: { selectionSource?: string } }[] = [];
let mountsReply: ContainerMountsResponse;
// Replies may be plain values or promises (a pending promise pins the loading
// row, which an immediately-resolving mock can never show).
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
const { FoldersEditor } = await import("../pages/Containers");
const { SelectionTree } = await import("./SelectionTree");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";

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
    />
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls.length = 0;
  patches.length = 0;
  browseReplies = [];
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
    const box = within(child).getByRole("checkbox");
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

describe("empty-selection guard (D-04, pulled forward from plan 02)", () => {
  it("blocks the toggle that would leave zero includes before any PATCH, with the inline warn line", async () => {
    mountsReply = mountsResponse();
    browseReplies = [listing()];
    await renderEditor();

    // The single selected mount root IS the only include: unchecking it
    // would empty the item's selection.
    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    const box = within(root).getByRole("checkbox");
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
      "At least one folder must stay selected. To back up none of this container, turn off Include in schedule.",
    );
    expect(warn.className).toContain("text-statusWarn");
  });
});
