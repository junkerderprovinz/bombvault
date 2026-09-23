// @vitest-environment jsdom
// The folder selection tree on the Files page, with the same harness as
// Containers.tree.dom.test.tsx: the mocked api client serves browse listings
// and records PATCH bodies, so what goes over the wire can be checked step by
// step.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, FileSetView } from "../lib/api";
import { placementOptions, placementView } from "../lib/placement.testsupport";

// jsdom has no EventSource, and FileSetRow opens the progress stream on mount.
class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

const browseCalls: string[] = [];
const patches: { id: string; body: Record<string, unknown> }[] = [];
let browseReplies: (BrowseResponse | Promise<BrowseResponse>)[] = [];
let patchReplies: ({ ok: boolean; error?: string; code?: string } | Promise<{ ok: boolean; error?: string; code?: string }>)[] = [];
// Two overlapping PATCHes would push maxConcurrentPatches to 2.
let activePatches = 0;
let maxConcurrentPatches = 0;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    browse: (path: string) => {
      browseCalls.push(path);
      const reply = browseReplies.shift() ?? { ok: true, dirs: [], status: "ok", truncated: false };
      return Promise.resolve(reply);
    },
    patchFileSet: (id: string, body: Record<string, unknown>) => {
      activePatches += 1;
      maxConcurrentPatches = Math.max(maxConcurrentPatches, activePatches);
      patches.push({ id, body });
      const reply = patchReplies.shift() ?? { ok: true };
      return Promise.resolve(reply).finally(() => {
        activePatches -= 1;
      });
    },
    getPlacementOptions: () => Promise.resolve({ ok: true, options: placementOptions({ domain: "files" }) }),
    getSettings: () => Promise.resolve({ ok: true, platform: "unraid", hostMountRoot: "/host/user" }),
  };
});

// Imported after vi.mock so the components get the mocked client.
const FilesModule = await import("./Files");
const { FileSetFoldersEditor, FileSetRow } = FilesModule;
const fileSetEditorKey = FilesModule.fileSetEditorKey;
const FileSetDialog = FilesModule.FileSetDialog;

const HOST_MOUNT_ROOT = "/host/user";
const REL = "documents";
const ROOT = "/host/user/documents";
const SUB = "/host/user/documents/sub";

/** A file set as GET /api/files serves it. selectedPaths is absent by default,
 *  as for every set the tree has not touched yet. */
function setView(overrides?: Partial<FileSetView>): FileSetView {
  return {
    id: "set1",
    name: "Documents",
    path: REL,
    excludes: [],
    enabled: true,
    lastBackup: 0,
    pathExists: true,
    placement: placementView(),
    ...overrides,
  };
}

/** Children of the documents root, relative to the host mount root. */
function documentsListing(): BrowseResponse {
  return {
    ok: true,
    status: "ok",
    truncated: false,
    dirs: [
      { name: "sub", path: "documents/sub" },
      { name: "media", path: "documents/media" },
    ],
  };
}

function EditorHarness({ set, hostMountRoot = HOST_MOUNT_ROOT }: { set: FileSetView; hostMountRoot?: string }) {
  const { t } = useT();
  // Keyed as FileSetRow keys it, so the harness remounts when the page would.
  return <FileSetFoldersEditor key={fileSetEditorKey(set)} set={set} hostMountRoot={hostMountRoot} t={t} />;
}

function RowHarness({ set, hostMountRoot = HOST_MOUNT_ROOT, index = 0 }: { set: FileSetView; hostMountRoot?: string; index?: number }) {
  const { t } = useT();
  return (
    <FileSetRow
      set={set}
      hostMountRoot={hostMountRoot}
      restoreFolder="/restore"
      t={t}
      onRefresh={() => {}}
      onEdit={() => {}}
      onPlacement={() => {}}
      index={index}
    />
  );
}

function DialogHarness({ initial }: { initial: FileSetView | null }) {
  const { t } = useT();
  return (
    <FileSetDialog
      initial={initial}
      presetSeed={null}
      hostMountRoot={HOST_MOUNT_ROOT}
      t={t}
      onClose={() => {}}
      onSaved={() => {}}
    />
  );
}

function Providers({ children }: { children: React.ReactNode }) {
  return (
    <I18nProvider>
      <ToastProvider>{children}</ToastProvider>
    </I18nProvider>
  );
}

/** A reply the test resolves by hand, to hold a PATCH in flight. */
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

async function renderEditor(set: FileSetView) {
  const view = render(
    <Providers>
      <EditorHarness set={set} />
    </Providers>,
  );
  await act(async () => {});
  return view;
}

/** Expand the Choose folders disclosure (the editor's own button). */
async function openDisclosure() {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Choose folders" }));
  });
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  browseCalls.length = 0;
  patches.length = 0;
  browseReplies = [];
  patchReplies = [];
  activePatches = 0;
  maxConcurrentPatches = 0;
});

afterEach(cleanup);

describe("FileSetFoldersEditor tree mount", () => {
  it("shows one level-1 treeitem: the resolved path, aria-setsize=1, mono root label", async () => {
    browseReplies = [documentsListing()];
    await renderEditor(setView());
    await openDisclosure();

    // One level-1 treeitem, and it is the set's resolved host path.
    const roots = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1");
    expect(roots).toHaveLength(1);
    const root = roots[0];
    expect(root.getAttribute("aria-setsize")).toBe("1");
    expect(root.getAttribute("aria-posinset")).toBe("1");
    expect(root.getAttribute("aria-expanded")).toBe("false");
    expect(within(root).getByText(ROOT)).toBeTruthy();
    // The label is the path alone, without a dest/source arrow.
    expect(root.textContent).not.toContain("←");
    // The aria-label discloses the selection context (folders.treeLabel).
    expect(screen.getByRole("tree", { name: "Backup folder selection" })).toBeTruthy();
  });

  it("renders a set the tree never edited with its root checked at 1 path, without writing", async () => {
    await renderEditor(setView({ selectedPaths: undefined }));
    await openDisclosure();

    const root = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1")[0];
    expect(root.getAttribute("aria-checked")).toBe("true");
    // The count is the number of paths the next backup hands restic, and such
    // a set backs up its one source dir. The key has no plural form.
    expect(within(root).getByText("1 path")).toBeTruthy();
    expect(patches).toEqual([]);
  });

  it("stored partial selection [root, !sub]: root mixed, sub excluded, preview counts only the maximal root", async () => {
    browseReplies = [documentsListing()];
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();

    // The root state comes from the stored lists alone, before any child loads.
    const root = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1")[0];
    expect(root.getAttribute("aria-checked")).toBe("mixed");
    // One maximal include under the root, which toFlatList sends as a
    // positional.
    expect(within(root).getByText("1 path")).toBeTruthy();

    // Expanded, the carved-out child reads excluded.
    await act(async () => {
      fireEvent.click(root);
    });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(sub.getAttribute("aria-checked")).toBe("false");
    expect(root.getAttribute("aria-expanded")).toBe("true");
  });

  it("a set without a path renders no disclosure and no tree", async () => {
    render(
      <Providers>
        <RowHarness set={setView({ path: "" })} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.queryByRole("button", { name: "Choose folders" })).toBeNull();
    expect(screen.queryByRole("tree")).toBeNull();
    expect(
      screen.getByText(
        "Rebuilt from backups without a folder. Set a folder to back it up again. Restoring to a folder already works.",
      ),
    ).toBeTruthy();
  });

  it("lazy expansion browses through the hostMountRoot prefix and caches for the editor lifetime", async () => {
    browseReplies = [documentsListing()];
    const view = await renderEditor(setView());
    await openDisclosure();

    const root = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1")[0];
    await act(async () => {
      fireEvent.click(root);
    });
    // Browsing uses the path relative to the host mount root.
    expect(browseCalls).toEqual([REL]);
    expect(screen.getByRole("treeitem", { name: /^sub$/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /^media$/ })).toBeTruthy();

    // Collapse and expand again: the cache serves the listing.
    await act(async () => {
      fireEvent.click(root);
    });
    await act(async () => {
      fireEvent.click(root);
    });
    expect(browseCalls).toEqual([REL]);
    expect(screen.getByRole("treeitem", { name: /^media$/ })).toBeTruthy();
    expect(view).toBeTruthy();
  });
});

// The editor seeds from its mount-time props, so the call site keys it by
// fileSetEditorKey: a new anchor, or a selection appearing or disappearing,
// remounts it onto the fresh view.
describe("FileSetFoldersEditor reseed", () => {
  it("remounts onto the refetched view after a path edit, with the new anchor and no selection", async () => {
    browseReplies = [documentsListing()];
    const view = await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    expect(root.getAttribute("aria-checked")).toBe("mixed");

    // What the dialog's save and loadSets() serve back: a deeper anchor and no
    // selection, since the server clears it when the path changes.
    view.rerender(
      <Providers>
        <EditorHarness set={setView({ path: "documents/docs", selectedPaths: undefined })} />
      </Providers>,
    );
    // The remount reset the disclosure; reopen onto the new root.
    await openDisclosure();
    await act(async () => {});
    const moved = screen.getByRole("treeitem", { name: /documents\/docs/ });
    // Checked at 1 path, as the server has it, not the old anchor's state.
    expect(moved.getAttribute("aria-checked")).toBe("true");
    expect(within(moved).getByText("1 path")).toBeTruthy();

    // A toggle sends only entries under the new anchor. A leftover entry for
    // the old root would get the whole PATCH refused, or bring the cleared
    // selection back after a move deeper.
    browseReplies = [
      { ok: true, status: "ok", truncated: false, dirs: [{ name: "media", path: "documents/docs/media" }] },
    ];
    await act(async () => {
      fireEvent.click(moved);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toHaveLength(1);
    expect(patches[0].body.selectedPaths).toEqual(["/host/user/documents/docs", "!/host/user/documents/docs/media"]);
  });

  it("a same-anchor refetch keeps the editor instance mounted: open disclosure and mirror survive", async () => {
    browseReplies = [documentsListing()];
    const view = await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    // A refetch with the same id, path and selection presence, such as after
    // a finished backup, keeps the disclosure open and the state as it was.
    view.rerender(
      <Providers>
        <EditorHarness set={setView({ selectedPaths: [ROOT, `!${SUB}`] })} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.getByRole("treeitem", { name: /documents/ }).getAttribute("aria-checked")).toBe("mixed");
  });
});

describe("FileSetFoldersEditor live save", () => {
  it("applies a toggle optimistically and PATCHes the full flat list once, with no selectionSource field anywhere", async () => {
    browseReplies = [documentsListing()];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });

    // Optimistic: the carved-out child reads unchecked immediately.
    expect(media.getAttribute("aria-checked")).toBe("false");
    expect(root.getAttribute("aria-checked")).toBe("mixed");
    // One PATCH with the full list in toFlatList order, the root plus the "!"
    // carve-out, and no selectionSource field: file sets have no older client
    // to tell apart.
    expect(patches).toHaveLength(1);
    expect(patches[0].id).toBe("set1");
    expect(patches[0].body.selectedPaths).toEqual([ROOT, `!${ROOT}/media`]);
    expect(patches[0].body.selectionSource).toBeUndefined();
    expect(Object.keys(patches[0].body)).toEqual(["selectedPaths"]);
  });

  it("sends a rapid burst as one follow-up PATCH with the latest state, never two at once", async () => {
    browseReplies = [documentsListing()];
    const first = deferred<{ ok: boolean }>();
    patchReplies = [first.promise];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });

    // Two toggles land while the first PATCH is in flight.
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
      fireEvent.click(within(sub).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toHaveLength(1);

    // Once it resolves, a single follow-up sends the current state.
    await act(async () => {
      first.resolve({ ok: true });
    });
    expect(patches).toHaveLength(2);
    expect(patches[1].body.selectedPaths).toEqual([ROOT, `!${ROOT}/media`, `!${ROOT}/sub`]);
    expect(maxConcurrentPatches).toBe(1);
  });

  it("refuses the last untick before any request: warn line, shake, state unchanged, no PATCH", async () => {
    await renderEditor(setView()); // no selection: the root is the only include
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(within(root).getByRole("checkbox", { hidden: true }));
    });

    expect(patches).toEqual([]);
    // The warn line under the row points to deleting the set, since file sets
    // have no reset.
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Delete folder set if you no longer want this set.",
      ),
    ).toBeTruthy();
    // The shake nonce remounts the row, so it is queried again. It still
    // reads checked and carries the shake class.
    const shakenRoot = screen.getByRole("treeitem", { name: /documents/ });
    expect(shakenRoot.getAttribute("aria-checked")).toBe("true");
    expect(shakenRoot.className).toContain("glim-shake");
  });

  it("shows a server empty-selection refusal as the same warn line plus a toast, and reverts only the failed toggle", async () => {
    browseReplies = [documentsListing()];
    const first = deferred<{ ok: boolean; error?: string; code?: string }>();
    patchReplies = [first.promise, { ok: true }];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });

    // Toggle media, whose PATCH stays in flight, then sub, which waits.
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });
    await act(async () => {
      fireEvent.click(within(sub).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toHaveLength(1);

    // The first PATCH comes back as a coded empty-selection refusal.
    await act(async () => {
      first.resolve({ ok: false, error: "selection refused", code: "empty-selection" });
    });

    // The server text shows as a toast, and the client check's warn line
    // appears under the row.
    expect(screen.getByText("selection refused")).toBeTruthy();
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Delete folder set if you no longer want this set.",
      ),
    ).toBeTruthy();
    // Only the failed toggle is undone on the current state: media is checked
    // again, while sub, toggled after it, stays unchecked. The shake remounts
    // media's row, so it is queried again.
    const revertedMedia = screen.getByRole("treeitem", { name: /^media$/ });
    const revertedSub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(revertedMedia.getAttribute("aria-checked")).toBe("true");
    expect(revertedSub.getAttribute("aria-checked")).toBe("false");
    expect(revertedMedia.className).toContain("glim-shake");
    // The follow-up sends the reverted list: the root plus sub's carve-out.
    expect(patches).toHaveLength(2);
    expect(patches[1].body.selectedPaths).toEqual([ROOT, `!${ROOT}/sub`]);
  });

  it("on a generic failure toasts and reverts by set-difference inverse, clearing the row's busy state", async () => {
    browseReplies = [documentsListing()];
    const first = deferred<{ ok: boolean; error?: string }>();
    patchReplies = [first.promise];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    const box = within(media).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    // In flight: the row's checkbox disables (busyPaths).
    expect(box.disabled).toBe(true);

    await act(async () => {
      first.resolve({ ok: false, error: "disk on fire" });
    });
    expect(screen.getByText("disk on fire")).toBeTruthy();
    // The shake remounts the row, so the captured checkbox is detached. The
    // new row is checked again, not busy, and shaking.
    const revertedMedia = screen.getByRole("treeitem", { name: /^media$/ });
    const revertedBox = within(revertedMedia).getByRole("checkbox", { hidden: true });
    expect(revertedBox.disabled).toBe(false);
    expect(revertedMedia.getAttribute("aria-checked")).toBe("true");
    expect(revertedMedia.className).toContain("glim-shake");
  });

  it("reopens exactly: the toggled selection reconstructs identically after a remount, with zero refetch on plain close/reopen", async () => {
    // One listing per expansion; the mock's default reply is an empty listing.
    browseReplies = [documentsListing(), documentsListing()];
    const view = await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = screen.getByRole("treeitem", { name: /^media$/ });
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });
    // The save must land before the remount: what the PATCH carried is what
    // the server now serves (the harness's stand-in for the list refetch).
    expect(patches).toHaveLength(1);
    const saved = patches[0].body.selectedPaths as string[];
    expect(saved).toEqual([ROOT, `!${ROOT}/media`]);

    // Close the disclosure (the tree unmounts; the editor and its cache stay).
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Choose folders" }));
    });
    expect(screen.queryByRole("tree")).toBeNull();

    // Reopened, the root is expanded again without a refetch, and every state
    // is rebuilt from the stored lists rather than from the DOM or
    // localStorage.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Choose folders" }));
    });
    await act(async () => {});
    expect(browseCalls).toEqual([REL]);
    const reopened = screen.getByRole("treeitem", { name: /documents/ });
    expect(reopened.getAttribute("aria-expanded")).toBe("true");
    expect(reopened.getAttribute("aria-checked")).toBe("mixed");
    // Child treeitems are DOM siblings of the root (role="group" wrapper), so
    // the state query is screen-level.
    expect(screen.getByRole("treeitem", { name: /^media$/ }).getAttribute("aria-checked")).toBe("false");
    expect(JSON.parse(localStorage.getItem("bv-tree-expanded-fileset-set1") ?? "[]")).toEqual([ROOT]);

    // A full remount with the saved view, as a page revisit serves it,
    // rebuilds the same states. The expansion survives in localStorage, so
    // the mount's own browse takes the second listing.
    cleanup();
    view.unmount();
    await renderEditor(setView({ selectedPaths: saved }));
    await openDisclosure();
    await act(async () => {});
    const remounted = screen.getByRole("treeitem", { name: /documents/ });
    expect(remounted.getAttribute("aria-expanded")).toBe("true");
    expect(remounted.getAttribute("aria-checked")).toBe("mixed");
    expect(screen.getByRole("treeitem", { name: /^media$/ }).getAttribute("aria-checked")).toBe("false");
  });

  it("routes Space through the same toggle path as a click", async () => {
    // Stored [root, !sub]: sub is excluded, so Space includes it again.
    browseReplies = [documentsListing()];
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(sub.getAttribute("aria-checked")).toBe("false");
    // Focusing the row points the roving tabindex at it; the keyDown then
    // fires on the tree element, where a real key press bubbles to.
    await act(async () => {
      sub.focus();
    });
    expect(sub.getAttribute("tabindex")).toBe("0");
    const tree = screen.getByRole("tree");
    await act(async () => {
      fireEvent.keyDown(tree, { key: " " });
    });
    // Including sub again leaves the root as the only include, since
    // applyToggle adds none under a covering ancestor, as a click would.
    expect(patches).toHaveLength(1);
    expect(patches[0].body.selectedPaths).toEqual([ROOT]);
    expect(screen.getByRole("treeitem", { name: /^sub$/ }).getAttribute("aria-checked")).toBe("true");
  });
});

describe("FileSetFoldersEditor exclusions list", () => {
  it("renders the per-root audit disclosure: count line, relative mono muted rows, zero interactive controls", async () => {
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`, `!${ROOT}/media`] }));
    await openDisclosure();

    // The count comes from rootExclusions, the classification the backup uses,
    // not from the DOM.
    const disc = screen.getByRole("button", { name: "2 exclusions" });
    expect(disc.getAttribute("aria-expanded")).toBe("false");
    // A plain tabbable control outside the roving set, so no tabindex of its
    // own.
    expect(disc.getAttribute("tabindex")).toBeNull();
    await act(async () => {
      fireEvent.click(disc);
    });
    expect(disc.getAttribute("aria-expanded")).toBe("true");

    // Relative display paths, lexically sorted, mono ltr break-all muted with
    // the house long-path title.
    const list = document.getElementById(disc.getAttribute("aria-controls") ?? "");
    expect(list).toBeTruthy();
    const rows = within(list as HTMLElement).getAllByRole("listitem");
    expect(rows.map((r) => r.textContent)).toEqual(["media", "sub"]);
    for (const row of rows) {
      expect(row.className).toContain("font-mono");
      expect(row.className).toContain("break-all");
      expect(row.className).toContain("text-carbon-textMuted");
      expect(row.getAttribute("dir")).toBe("ltr");
      expect(row.getAttribute("title")).toBe(row.textContent);
    }
    // The list is read-only; toggling only happens in the tree.
    expect(within(list as HTMLElement).queryByRole("button")).toBeNull();
    expect(within(list as HTMLElement).queryByRole("checkbox")).toBeNull();
  });

  it("collapses on every reopen, since its expansion is not persisted", async () => {
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const disc = screen.getByRole("button", { name: "1 exclusion" });
    await act(async () => {
      fireEvent.click(disc);
    });
    expect(screen.getByTitle("sub")).toBeTruthy();

    // Closing and reopening Choose folders unmounts the tree; the list comes
    // back collapsed and left nothing in localStorage.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Choose folders" }));
    });
    await openDisclosure();
    const reopened = screen.getByRole("button", { name: "1 exclusion" });
    expect(reopened.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("listitem")).toBeNull();
    // Only the tree's expansion key may exist.
    expect(localStorage.getItem("bv-tree-expanded-fileset-set1")).not.toBeNull();
    const bvKeys = Object.keys(localStorage).filter((k) => k.startsWith("bv-"));
    expect(bvKeys.every((k) => k === "bv-lang" || k.startsWith("bv-tree-expanded-"))).toBe(true);
  });

  it("lists stored exclusions existence-unfiltered and renders a dormant root identically to an active one", async () => {
    // "sub" is gone from disk, but the list shows what is stored whether it
    // exists or not.
    browseReplies = [
      { ok: true, status: "ok", truncated: false, dirs: [{ name: "media", path: "documents/media" }] },
    ];
    const active = await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const disc = screen.getByRole("button", { name: "1 exclusion" });
    await act(async () => {
      fireEvent.click(disc);
    });
    const activeRowClass = screen.getByTitle("sub").className;
    active.unmount();

    // A stored list of exclusions only cannot come from this UI, which refuses
    // the last untick, but it renders the same count and rows with the same
    // classes.
    const dormant = await renderEditor(setView({ selectedPaths: [`!${SUB}`] }));
    await openDisclosure();
    const dormantDisc = screen.getByRole("button", { name: "1 exclusion" });
    await act(async () => {
      fireEvent.click(dormantDisc);
    });
    expect(screen.getByTitle("sub").className).toBe(activeRowClass);
    expect(screen.getByTitle("sub").className).toContain("text-carbon-textMuted");
    expect(screen.getByTitle("sub").className).toContain("font-mono");
    dormant.unmount();
  });
});

describe("FileSetDialog path-change hint", () => {
  it("always says under the folder picker that a new path clears the selection", async () => {
    render(
      <Providers>
        <DialogHarness initial={setView()} />
      </Providers>,
    );
    await act(async () => {});
    const hint = screen.getByText("Changing the folder clears the ticked sub-folder selection.");
    // The caption follows the path input inside the same field block.
    const pathInput = screen.getByPlaceholderText("user/appdata");
    expect(pathInput.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // It shows even though this fixture stores no selection, next to the
    // generic path hint.
    expect(
      screen.getByText("The folder to back up, a relative subpath under the host mount root."),
    ).toBeTruthy();
  });

  it("shows both captions for a set without a path", async () => {
    render(
      <Providers>
        <DialogHarness initial={setView({ path: "", selectedPaths: undefined })} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.getByText("Changing the folder clears the ticked sub-folder selection.")).toBeTruthy();
    expect(
      screen.getByText("The folder to back up, a relative subpath under the host mount root."),
    ).toBeTruthy();
  });
});

describe("FileSetRow placement", () => {
  it("carries the placement bar, the same row the container card has", async () => {
    render(
      <Providers>
        <RowHarness set={setView()} />
      </Providers>,
    );
    expect(await screen.findByRole("toolbar", { name: "Placement" })).toBeTruthy();
  });
});
