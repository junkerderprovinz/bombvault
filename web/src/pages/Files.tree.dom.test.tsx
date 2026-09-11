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

// Imported AFTER vi.mock so the components pick up the mocked client.
const { FileSetFoldersEditor, FileSetRow } = await import("./Files");

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
  return <FileSetFoldersEditor set={set} hostMountRoot={hostMountRoot} t={t} />;
}

function RowHarness({ set, hostMountRoot = HOST_MOUNT_ROOT, index = 0 }: { set: FileSetView; hostMountRoot?: string; index?: number }) {
  const { t } = useT();
  return <FileSetRow set={set} hostMountRoot={hostMountRoot} restoreFolder="/restore" t={t} onRefresh={() => {}} onEdit={() => {}} index={index} />;
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
// Task 3: the live-save pipeline (Containers.tsx Pattern 4, single owed class)
// ---------------------------------------------------------------------------

describe("FileSetFoldersEditor live-save pipeline (T-04-11 queue, D-06 refusal, exact reopen)", () => {
  it("applies a toggle optimistically and PATCHes the full flat list once, with no selectionSource field anywhere", async () => {
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = within(root).getByRole("treeitem", { name: /^media$/ });
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
    const first = deferred<{ ok: boolean }>();
    patchReplies = [first.promise];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = within(root).getByRole("treeitem", { name: /^media$/ });
    const sub = within(root).getByRole("treeitem", { name: /^sub$/ });

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
    // The mirror is untouched: the root still reads checked.
    expect(root.getAttribute("aria-checked")).toBe("true");
    // The inline warn line routes under the blocked row, orienting to Remove
    // set (the files domain has no Reset; D-06), and the row shakes.
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Remove set if you no longer want this set.",
      ),
    ).toBeTruthy();
    expect(root.className).toContain("glim-shake");
  });

  it("routes a server code empty-selection refusal to the same warn line plus a fail toast and reverts from the LIVE mirror (a stacked toggle survives)", async () => {
    patchReplies = [{ ok: false, error: "selection refused", code: "empty-selection" }];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = within(root).getByRole("treeitem", { name: /^media$/ });

    await act(async () => {
      fireEvent.click(within(media).getByRole("checkbox", { hidden: true }));
    });

    // The refusal lands as the fail toast (server text verbatim) AND the same
    // inline warn line the client block uses, under the failed row.
    expect(screen.getByText("selection refused")).toBeTruthy();
    expect(
      screen.getByText(
        "A set needs at least one folder, so the last tick cannot be removed. Use Remove set if you no longer want this set.",
      ),
    ).toBeTruthy();
    // The revert re-derives from the live mirror: media is back covered by
    // the root include (checked), the row shook.
    expect(media.getAttribute("aria-checked")).toBe("true");
    expect(media.className).toContain("glim-shake");
  });

  it("on a generic failure toasts and reverts by set-difference inverse, clearing the row's busy state", async () => {
    const first = deferred<{ ok: boolean; error?: string }>();
    patchReplies = [first.promise];
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = within(root).getByRole("treeitem", { name: /^media$/ });
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
    // Reverted to checked, busy state cleared, row shook.
    expect(box.disabled).toBe(false);
    expect(media.getAttribute("aria-checked")).toBe("true");
    expect(media.className).toContain("glim-shake");
  });

  it("reopens exactly: the toggled selection reconstructs identically after a remount, with zero refetch on plain close/reopen", async () => {
    browseReplies = [documentsListing()];
    const view = await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const media = within(root).getByRole("treeitem", { name: /^media$/ });
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
    expect(within(reopened).getByRole("treeitem", { name: /^media$/ }).getAttribute("aria-checked")).toBe("false");
    expect(JSON.parse(localStorage.getItem("bv-tree-expanded-fileset-set1") ?? "[]")).toEqual([ROOT]);

    // Full remount with the SAVED view (what a page revisit serves): states
    // reconstruct identically from FileSetView.selectedPaths.
    cleanup();
    view.unmount();
    await renderEditor(setView({ selectedPaths: saved }));
    await openDisclosure();
    const remounted = screen.getByRole("treeitem", { name: /documents/ });
    expect(remounted.getAttribute("aria-checked")).toBe("mixed");
    await act(async () => {
      fireEvent.click(remounted);
    });
    expect(within(remounted).getByRole("treeitem", { name: /^media$/ }).getAttribute("aria-checked")).toBe("false");
  });

  it("routes Space through the same onToggle pipeline as clicks (one toggle semantics, T-02-10)", async () => {
    await renderEditor(setView());
    await openDisclosure();
    const root = screen.getByRole("treeitem", { name: /documents/ });
    await act(async () => {
      fireEvent.click(root);
    });
    const sub = within(root).getByRole("treeitem", { name: /^sub$/ });
    // Space on the EXCLUDED child re-includes it — converging to the root's
    // maximal include, exactly what a click on the checkbox produces.
    await act(async () => {
      fireEvent.keyDown(sub, { key: " " });
    });
    expect(patches).toHaveLength(1);
    expect(patches[0].body.selectedPaths).toEqual([ROOT]);
    expect(sub.getAttribute("aria-checked")).toBe("true");
  });
});
