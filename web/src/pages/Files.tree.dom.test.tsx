// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Files-page selection-tree integration tests (Phase 4, plan 03 — INTEG-02
// criterion 1). Same import-after-mock harness as Containers.tree.dom.test
// .tsx: the mocked api client serves browse listings and captures PATCH bodies
// so the wire contract is observable step by step.
//
// Task 2 pins the mount contract: the Choose folders disclosure, exactly ONE
// level-1 treeitem (the set's resolved path, aria-setsize=1) — no custom rows,
// no CACHEDIR switch, no dest←source arrow — the NULL mirror seed (a set never
// touched by the tree renders its root CHECKED at "1 path", UI-SPEC item 9),
// the argv-matching preview count, the no-Path gating, and lazy browse through
// the hostMountRoot prefix with the editor-lifetime cache.
//
// Task 3 adds the live-save pipeline pins (serialized queue, client/server D-06
// refusal, revert-from-live-mirror, exact reopen) over the same harness.
//
// Plan 04 adds the audit surfaces: the per-root exclusions review list and the
// dialog's path-change disclosure. The list itself is NOT reimplemented here —
// it rides the shared SelectionTree disclosure (rootExclusions, the Phase 3
// implementation) which the plan 03 mount already carries, so those pins are
// characterization pins locking the FILES-page rendering (relative mono muted
// rows, no controls, collapsed on reopen, dormant-root parity,
// existence-unfiltered); the genuinely new implementation the RED gate drives
// is the files.pathChangeHint caption in FileSetDialog.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, FileSetView } from "../lib/api";

// jsdom has no EventSource, and rendering the full card (FileSetRow) opens the
// progress stream on mount via useProgress. A no-op stand-in keeps these tests
// about the selection tree rather than about SSE (Config.autosave precedent).
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
// No-overlap proof (T-04-11): the mock counts calls in flight; two overlapping
// PATCHes would push maxConcurrentPatches to 2.
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
  };
});

// Imported AFTER vi.mock so the components pick up the mocked client. The
// namespace import additionally reaches FileSetDialog, whose export lands with
// the plan 04 GREEN (the FoldersEditor harness precedent: exported for this
// page's dom harness only).
const FilesModule = await import("./Files");
const { FileSetFoldersEditor, FileSetRow } = FilesModule;
// Bound from the namespace like FileSetDialog: the reseed pins below depend on
// this call-site key helper (review CR-01).
const fileSetEditorKey = FilesModule.fileSetEditorKey;
// Bound from the namespace rather than a destructured ESM import: a missing
// named export fails the whole file at link time, while a missing namespace
// property fails only the dialog tests — the honest RED before the plan 04
// GREEN adds the export (the FoldersEditor harness precedent: exported for
// this page's dom harness only).
const FileSetDialog = FilesModule.FileSetDialog;

const HOST_MOUNT_ROOT = "/host/user";
const REL = "documents";
const ROOT = "/host/user/documents";
const SUB = "/host/user/documents/sub";

/** A file-set view as GET /api/files serves it. selectedPaths left absent by
 *  default — the NULL column case every new/legacy set starts in. */
function setView(overrides?: Partial<FileSetView>): FileSetView {
  return {
    id: "set1",
    name: "Documents",
    path: REL,
    excludes: [],
    enabled: true,
    lastBackup: 0,
    pathExists: true,
    ...overrides,
  };
}

/** Children of the documents root, browse-relative (host root "/host/user"
 *  swapped). */
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
  // The call site's exact keying (FileSetRow in Files.tsx): the CR-01 reseed
  // pin only means something if the harness remounts the way the page does.
  return <FileSetFoldersEditor key={fileSetEditorKey(set)} set={set} hostMountRoot={hostMountRoot} t={t} />;
}

function RowHarness({ set, hostMountRoot = HOST_MOUNT_ROOT, index = 0 }: { set: FileSetView; hostMountRoot?: string; index?: number }) {
  const { t } = useT();
  return <FileSetRow set={set} hostMountRoot={hostMountRoot} restoreFolder="/restore" t={t} onRefresh={() => {}} onEdit={() => {}} index={index} />;
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

/** A reply the test holds back, pinning a PATCH in flight so the queue's
 *  serialize/drain behavior is observable step by step (the Containers
 *  harness's deferred-reply machinery). */
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

describe("FileSetFoldersEditor tree mount (INTEG-02 criterion 1, UI-SPEC items 1-9)", () => {
  it("discloses exactly ONE level-1 treeitem: the resolved path, aria-setsize=1, mono root label", async () => {
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
    // The path renders ltr mono with a long-path title (house pattern) — and
    // NO dest ← source arrow (the label is the path alone).
    expect(root.textContent).not.toContain("←");
    // The aria-label discloses the selection context (folders.treeLabel).
    expect(screen.getByRole("tree", { name: "Backup folder selection" })).toBeTruthy();
  });

  it("NULL seed (UI-SPEC item 9): a never-tree-edited set renders its root CHECKED at 1 path, and no write happens", async () => {
    await renderEditor(setView({ selectedPaths: undefined }));
    await openDisclosure();

    const root = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1")[0];
    expect(root.getAttribute("aria-checked")).toBe("true");
    // The visible count equals the bare positionals the next backup hands
    // restic — the legacy [SourceDir] argv is ONE entry (Phase 3 pinned rule).
    // The reused key's copy is "{n} paths" (byte-identical, no pluralization),
    // so one entry renders "1 paths".
    expect(within(root).getByText("1 paths")).toBeTruthy();
    // Rendering honesty is read-only: NULL stays NULL until the first toggle.
    expect(patches).toEqual([]);
  });

  it("stored partial selection [root, !sub]: root mixed, sub excluded, preview counts only the maximal root (TREE-04)", async () => {
    browseReplies = [documentsListing()];
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();

    // Root state derives from (I, E) alone — mixed WITHOUT any child loaded.
    const root = screen.getAllByRole("treeitem").filter((el) => el.getAttribute("aria-level") === "1")[0];
    expect(root.getAttribute("aria-checked")).toBe("mixed");
    // Exactly one maximal include at-or-under the root: the visible number is
    // what toFlatList serializes as bare positionals (T-04-10).
    expect(within(root).getByText("1 paths")).toBeTruthy();

    // Expand: the carved-out child renders excluded, honest aria-expanded.
    await act(async () => {
      fireEvent.click(root);
    });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(sub.getAttribute("aria-checked")).toBe("false");
    expect(root.getAttribute("aria-expanded")).toBe("true");
  });

  it("a set without a Path renders no disclosure and no tree (FileSetRow gating, D-02)", async () => {
    render(
      <Providers>
        <RowHarness set={setView({ path: "" })} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.queryByRole("button", { name: "Choose folders" })).toBeNull();
    expect(screen.queryByRole("tree")).toBeNull();
    // The existing stand-in line is untouched.
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
    // The browse call carries the browse-relative path (hostMountRoot swapped).
    expect(browseCalls).toEqual([REL]);
    expect(screen.getByRole("treeitem", { name: /^sub$/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /^media$/ })).toBeTruthy();

    // Collapse and re-expand: the editor-lifetime cache serves the listing —
    // zero refetch.
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

// ---------------------------------------------------------------------------
// Review CR-01: the mirror-reseed contract. The editor seeds from mount-time
// props only, so the call site keys it by fileSetEditorKey — an anchor or
// selection-presence change remounts it onto the fresh view.
// ---------------------------------------------------------------------------

describe("FileSetFoldersEditor mirror reseed (review CR-01)", () => {
  it("a refetched view after a dialog path edit (new anchor, cleared selection) remounts to the honest post-clear seed — no stale mirror, no wedge", async () => {
    browseReplies = [documentsListing()];
    const view = await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    expect(root.getAttribute("aria-checked")).toBe("mixed");

    // What FileSetDialog's save + loadSets() serve back: a DEEPER anchor and
    // a CLEARED selection (the server-side A3 clear stored NULL; the view
    // omits the key). Without the remount the mounted editor kept the old
    // anchor's mixed mirror here.
    view.rerender(
      <Providers>
        <EditorHarness set={setView({ path: "documents/docs", selectedPaths: undefined })} />
      </Providers>,
    );
    // The remount reset the disclosure; reopen onto the new root.
    await openDisclosure();
    await act(async () => {});
    const moved = screen.getByRole("treeitem", { name: /documents\/docs/ });
    // Honest NULL seed (UI-SPEC item 9), not the old anchor's stale mirror:
    // the root reads CHECKED at 1 paths — what the server actually has.
    expect(moved.getAttribute("aria-checked")).toBe("true");
    expect(within(moved).getByText("1 paths")).toBeTruthy();

    // The wedge pin: a toggle under the new anchor PATCHes ONLY fresh-anchor
    // entries — no stale old-root entry rides along to be atomically refused
    // against the new root (or, on a deeper move, to resurrect the cleared
    // selection).
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
    // A refetch that serves the same id/path/selection-presence (e.g. a
    // finished backup's onRefresh) must NOT remount: the disclosure stays
    // open — a remount would reset it to closed — and the mirror state is
    // preserved.
    view.rerender(
      <Providers>
        <EditorHarness set={setView({ selectedPaths: [ROOT, `!${SUB}`] })} />
      </Providers>,
    );
    await act(async () => {});
    expect(screen.getByRole("treeitem", { name: /documents/ }).getAttribute("aria-checked")).toBe("mixed");
  });
});

// ---------------------------------------------------------------------------
// Task 3: the live-save pipeline (Containers.tsx Pattern 4, single owed class)
// ---------------------------------------------------------------------------

describe("FileSetFoldersEditor live-save pipeline (T-04-11 queue, D-06 refusal, exact reopen)", () => {
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
    // ONE PATCH of the full list — canonical toFlatList order, bare root plus
    // the "!"-prefixed host carve-out — and NO selectionSource field: the
    // file-set PATCH has no legacy client to protect (plan 02 presence gate).
    expect(patches).toHaveLength(1);
    expect(patches[0].id).toBe("set1");
    expect(patches[0].body.selectedPaths).toEqual([ROOT, `!${ROOT}/media`]);
    expect(patches[0].body.selectionSource).toBeUndefined();
    expect(Object.keys(patches[0].body)).toEqual(["selectedPaths"]);
  });

  it("collapses a rapid burst to maxConcurrentPatches === 1 and one drain carrying the LIVE mirror's latest state", async () => {
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

    // Burst: two toggles land while PATCH #1 is still in flight.
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
      fireEvent.click(within(sub).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toHaveLength(1);

    // The in-flight attempt resolves; the single drain sends the LIVE mirror.
    await act(async () => {
      first.resolve({ ok: true });
    });
    expect(patches).toHaveLength(2);
    expect(patches[1].body.selectedPaths).toEqual([ROOT, `!${ROOT}/media`, `!${ROOT}/sub`]);
    // Never two concurrent PATCHes (T-04-11 / T-02-08 discipline).
    expect(maxConcurrentPatches).toBe(1);
  });

  it("refuses the last untick BEFORE any request: warn line, glim-shake, mirror unchanged, zero PATCHes (client D-06)", async () => {
    await renderEditor(setView()); // NULL seed: the root itself is the one include
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(within(root).getByRole("checkbox", { hidden: true }));
    });

    expect(patches).toEqual([]);
    // The inline warn line routes under the blocked row, orienting to Remove
    // set (the files domain has no Reset; D-06).
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Remove set if you no longer want this set.",
      ),
    ).toBeTruthy();
    // Re-query: the shake nonce remounts the row (keyed Fragment), so the
    // pre-toggle reference is detached. The fresh row still reads checked
    // (mirror untouched) and carries the replayed shake class.
    const shakenRoot = screen.getByRole("treeitem", { name: /documents/ });
    expect(shakenRoot.getAttribute("aria-checked")).toBe("true");
    expect(shakenRoot.className).toContain("glim-shake");
  });

  it("routes a server code empty-selection refusal to the same warn line plus a fail toast and reverts from the LIVE mirror (a stacked toggle survives)", async () => {
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

    // Toggle media (PATCH #1 goes in flight), then toggle sub: it stacks as
    // dirty while #1 is unresolved.
    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });
    await act(async () => {
      fireEvent.click(within(sub).getByRole("checkbox", { hidden: true }));
    });
    expect(patches).toHaveLength(1);

    // PATCH #1 comes back as a coded empty-selection refusal.
    await act(async () => {
      first.resolve({ ok: false, error: "selection refused", code: "empty-selection" });
    });

    // The refusal lands as the fail toast (server text verbatim) AND the same
    // inline warn line the client block uses, under the failed row.
    expect(screen.getByText("selection refused")).toBeTruthy();
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Remove set if you no longer want this set.",
      ),
    ).toBeTruthy();
    // The revert re-derives from the LIVE mirror by set-difference inverse of
    // the FAILED mutation: media returns to covered (checked), while sub —
    // the newer toggle stacked behind the failing save — survives unchecked.
    // Re-queried: the revert's shake nonce remounts media's row.
    const revertedMedia = screen.getByRole("treeitem", { name: /^media$/ });
    const revertedSub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(revertedMedia.getAttribute("aria-checked")).toBe("true");
    expect(revertedSub.getAttribute("aria-checked")).toBe("false");
    expect(revertedMedia.className).toContain("glim-shake");
    // The drain re-sends the live post-revert list: the root include plus
    // sub's carve-out, nothing else.
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
    // Re-queried: the revert's shake nonce remounts the row, so the captured
    // checkbox belongs to the detached node. The fresh row reads reverted to
    // checked with the busy state cleared and the shake replayed.
    const revertedMedia = screen.getByRole("treeitem", { name: /^media$/ });
    const revertedBox = within(revertedMedia).getByRole("checkbox", { hidden: true });
    expect(revertedBox.disabled).toBe(false);
    expect(revertedMedia.getAttribute("aria-checked")).toBe("true");
    expect(revertedMedia.className).toContain("glim-shake");
  });

  it("reopens exactly: the toggled selection reconstructs identically after a remount, with zero refetch on plain close/reopen", async () => {
    // Two listings: one for the first expansion, one for the remount's
    // expansion (the mock's default reply is an EMPTY listing).
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

    // Reopen: expansion memory restores the root expanded (zero refetch), and
    // every state reconstructs from the (I, E) mirror — mixed root, excluded
    // media, unchecked... never a DOM or localStorage selection.
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

    // Full remount with the SAVED view (what a page revisit serves): states
    // reconstruct identically from FileSetView.selectedPaths. The expansion
    // memory also survives (localStorage), so the root renders pre-expanded
    // — the mount browse consumes the second listing; no manual expansion.
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

  it("routes Space through the same onToggle pipeline as clicks (one toggle semantics, T-02-10)", async () => {
    // Stored [root, !sub]: sub renders EXCLUDED, so Space re-includes it.
    browseReplies = [documentsListing()];
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const sub = screen.getByRole("treeitem", { name: /^sub$/ });
    expect(sub.getAttribute("aria-checked")).toBe("false");
    // Roving tabindex: focusing the row is what aims the tree's key map at it
    // (the handler acts on the roving-focus node, per the component's own
    // keyboard harness); the keyDown then fires on the tree element as a
    // real keyboard user's bubbling event.
    await act(async () => {
      sub.focus();
    });
    expect(sub.getAttribute("tabindex")).toBe("0");
    const tree = screen.getByRole("tree");
    await act(async () => {
      fireEvent.keyDown(tree, { key: " " });
    });
    // Space on the EXCLUDED child re-includes it — converging to the root's
    // maximal include (applyToggle re-adds no own include when an ancestor
    // covers it), exactly what a click on the checkbox produces.
    expect(patches).toHaveLength(1);
    expect(patches[0].body.selectedPaths).toEqual([ROOT]);
    expect(screen.getByRole("treeitem", { name: /^sub$/ }).getAttribute("aria-checked")).toBe("true");
  });
});

// ---------------------------------------------------------------------------
// Plan 04: the audit surfaces — the per-root exclusions review list (D-07)
// and the dialog's path-change disclosure (A3).
// ---------------------------------------------------------------------------

describe("FileSetFoldersEditor exclusions audit list (D-07, INTEG-03 pattern parity)", () => {
  it("renders the per-root audit disclosure: count line, relative mono muted rows, zero interactive controls", async () => {
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`, `!${ROOT}/media`] }));
    await openDisclosure();

    // The count line is the shared folders.exclusions key over rootExclusions —
    // the same classification the compile consumes (never a DOM count).
    const disc = screen.getByRole("button", { name: "2 exclusions" });
    expect(disc.getAttribute("aria-expanded")).toBe("false");
    // A plain tabbable control OUTSIDE the roving set: no tabindex attribute of
    // its own (backupOrder precedent), unlike the treeitems' 0/-1 roving.
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
    // Audit-only prohibition (T-04-14): the rows render NO controls — no
    // per-row remove, no ExcludesEditor fanout. The only toggle path is the
    // tree's one checkbox pipeline.
    expect(within(list as HTMLElement).queryByRole("button")).toBeNull();
    expect(within(list as HTMLElement).queryByRole("checkbox")).toBeNull();
  });

  it("collapses on every reopen: the expansion is component state, deliberately not persisted", async () => {
    await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const disc = screen.getByRole("button", { name: "1 exclusions" });
    await act(async () => {
      fireEvent.click(disc);
    });
    expect(screen.getByTitle("sub")).toBeTruthy();

    // Close the whole Choose folders disclosure (the tree unmounts) and
    // reopen: the audit section is collapsed again — an audit view, not
    // navigation comfort (STATE.md Phase 3 decision), so nothing about it
    // reaches localStorage.
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Choose folders" }));
    });
    await openDisclosure();
    const reopened = screen.getByRole("button", { name: "1 exclusions" });
    expect(reopened.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("listitem")).toBeNull();
    // No audit-state key leaks into storage (only the tree's expansion comfort
    // key may exist).
    expect(localStorage.getItem("bv-tree-expanded-fileset-set1")).not.toBeNull();
    const bvKeys = Object.keys(localStorage).filter((k) => k.startsWith("bv-"));
    expect(bvKeys.every((k) => k === "bv-lang" || k.startsWith("bv-tree-expanded-"))).toBe(true);
  });

  it("lists stored exclusions existence-unfiltered and renders a dormant root identically to an active one", async () => {
    // The listing no longer contains "sub" (gone on disk since the selection
    // was stored) — the audit list still names it (Phase 3 A3 rule: the list
    // is existence-unfiltered; row-level warnings are the container panel's
    // concern and are not re-implemented here).
    browseReplies = [
      { ok: true, status: "ok", truncated: false, dirs: [{ name: "media", path: "documents/media" }] },
    ];
    const active = await renderEditor(setView({ selectedPaths: [ROOT, `!${SUB}`] }));
    await openDisclosure();
    const disc = screen.getByRole("button", { name: "1 exclusions" });
    await act(async () => {
      fireEvent.click(disc);
    });
    const activeRowClass = screen.getByTitle("sub").className;
    active.unmount();

    // Dormant-root parity: a stored exclusions-only list (root include gone —
    // not reachable through this UI, where D-06 refuses the last untick on
    // both halves, but the classifier is total) renders the IDENTICAL section
    // for the same root: same count line, same rows, and the row carries the
    // exact same classes as the active case (no extra gating, no different
    // tone for a dormant root).
    const dormant = await renderEditor(setView({ selectedPaths: [`!${SUB}`] }));
    await openDisclosure();
    const dormantDisc = screen.getByRole("button", { name: "1 exclusions" });
    await act(async () => {
      fireEvent.click(dormantDisc);
    });
    expect(screen.getByTitle("sub").className).toBe(activeRowClass);
    expect(screen.getByTitle("sub").className).toContain("text-carbon-textMuted");
    expect(screen.getByTitle("sub").className).toContain("font-mono");
    dormant.unmount();
  });
});

describe("FileSetDialog path-change disclosure (A3, plan 02 PATCH-time clear rule)", () => {
  it("discloses the clear consequence under the FolderBrowser, unconditionally", async () => {
    render(
      <Providers>
        <DialogHarness initial={setView()} />
      </Providers>,
    );
    await act(async () => {});
    const hint = screen.getByText("Changing the folder clears the ticked sub-folder selection.");
    // "Under the FolderBrowser": the caption follows the path input in DOM
    // order, inside the same field block.
    const pathInput = screen.getByPlaceholderText("user/appdata");
    expect(pathInput.compareDocumentPosition(hint) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // It states the consequence, not a condition: it renders even though this
    // fixture stores no selection at all. The generic path hint is untouched.
    expect(
      screen.getByText("The folder to back up, a relative subpath under the host mount root."),
    ).toBeTruthy();
  });

  it("keeps the existing hint behavior for a path-less set (both captions render, nothing conditioned on the selection)", async () => {
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
