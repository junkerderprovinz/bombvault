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
import type { BrowseResponse, FileSetView } from "../lib/api";

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
      <ToastStub>{children}</ToastStub>
    </I18nProvider>
  );
}

// Toast stub: the editor only pushes on failures; Task 2 has no failure paths,
// and the stub keeps this harness independent of the toast engine's timers.
function ToastStub({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
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
    // restic — the legacy [SourceDir] argv is ONE path (Phase 3 pinned rule).
    expect(within(root).getByText("1 path")).toBeTruthy();
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
    expect(within(root).getByText("1 path")).toBeTruthy();

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
