// @vitest-environment jsdom
// Most tests drive SelectionTree through FoldersEditor with a mocked api
// module, so a toggle can be checked against the request it sends. The
// remount and id tests render SelectionTree directly with a shared browse
// cache standing in for the editor's; a fresh cache per mount would hide a
// refetch.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { BrowseResponse, ContainerMountsResponse, OkEnvelope } from "../lib/api";

const browseCalls: string[] = [];
const patches: { name: string; paths: string[]; opts?: { selectionSource?: string } }[] = [];
let mountsReply: ContainerMountsResponse;
// Replies may be promises: a pending one keeps the loading row on screen, and
// a deferred save reply holds the queue mid-flight.
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
    // Records the paths and selectionSource from the request body.
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

// Imported after vi.mock so the components pick up the mocked client.
const { FoldersEditor } = await import("../pages/Containers");
const { SelectionTree } = await import("./SelectionTree");

const HOST_ROOT = "/mnt";
const MOUNT = "/mnt/user/appdata/plex";
const PLEX2 = "/mnt/user/appdata/plex2";
const OTHER = "/mnt/user/appdata/other";
const CUSTOM = "/mnt/user/backups";

/** Two selected mounts and one custom include, so unchecking one mount never
 *  empties the selection. */
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

// Stands in for the cache FoldersEditor owns; replaced in beforeEach.
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

describe("SelectionTree lazy expansion", () => {
  it("browses nothing on open, once on first expand and not again on re-expand", async () => {
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

describe("SelectionTree toggle save", () => {
  it("unchecking a subfolder saves the mount plus a !child exclusion and leaves the root mixed", async () => {
    browseReplies = [listing()];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });
    const child = screen.getByRole("treeitem", { name: /transcoding/ });
    // The checkbox is aria-hidden, so getByRole needs hidden: true.
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

    // Updated optimistically: the root is mixed, the child excluded and
    // muted, its sibling still selected.
    expect(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }).getAttribute("aria-checked")).toBe("mixed");
    const excludedChild = screen.getByRole("treeitem", { name: /transcoding/ });
    expect(excludedChild.getAttribute("aria-checked")).toBe("false");
    expect(excludedChild.className).toContain("text-carbon-textMuted");
    expect(screen.getByRole("treeitem", { name: /library/ }).getAttribute("aria-checked")).toBe("true");
  });
});

describe("SelectionTree remount", () => {
  it("restores mixed and excluded states from the lists without browsing again", async () => {
    // Expansion comes back from localStorage, the listing from the shared
    // cache.
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

    // Same includes, exclusions and cache: nothing is fetched again.
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
    // Deselecting everything is stored as exclusions only.
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

describe("SelectionTree listing states", () => {
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
    // A promise that never settles keeps the loading row on screen.
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
    // A refused read shows as an error, not as an empty directory.
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

describe("SelectionTree listing outcome rows", () => {
  it("renders the couldNotRead fallback when ok:false carries no error text, outside the treeitem structure", async () => {
    browseReplies = [{ ok: false, status: "error" }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    // Without an error from the server, the translated fallback shows in a
    // plain row.
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
    // The failed entry was evicted, so the retry browses again and shows the
    // loading row.
    expect(browseCalls).toEqual(["user/appdata/plex", "user/appdata/plex"]);
    expect(screen.getByText("Loading…")).toBeTruthy();

    await act(async () => {
      release(listing());
    });
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
  });

  it("truncated listing renders the served children then one non-interactive notice row", async () => {
    browseReplies = [{ ...listing(), truncated: true }];
    await renderEditor();

    await act(async () => {
      fireEvent.click(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ }));
    });

    expect(screen.getByRole("treeitem", { name: /transcoding/ })).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /library/ })).toBeTruthy();
    // aria-setsize counts only the real children.
    expect(screen.getByRole("treeitem", { name: /transcoding/ }).getAttribute("aria-setsize")).toBe("2");

    // One notice, a plain <p> outside any treeitem.
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

    // Only the failed row offers a retry.
    expect(screen.getAllByRole("button", { name: "Try again" }).length).toBe(1);
  });

  it("a rejected browse promise settles to the error row with no spinner left", async () => {
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

    expect(screen.getByText("Could not read directory")).toBeTruthy();
    expect(document.querySelectorAll(".animate-spin").length).toBe(0);
  });

  it("a node whose listing failed can still be toggled and saved", async () => {
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

    // Only the listing failed; the folder's own include still toggles.
    const box = within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });
    expect(patches).toEqual([{ name: "plex", paths: [PLEX2], opts: { selectionSource: "tree" } }]);
  });
});

// Roots whose paths differ only in punctuation must get distinct list ids, or
// every disclosure's aria-controls points at the first list.
describe("SelectionTree exclusion list ids", () => {
  it("roots that differ only in punctuation get distinct ids, and each button controls its own list", async () => {
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
    const btns = screen.getAllByRole("button", { name: /1 exclusion/ });
    expect(btns).toHaveLength(2);
    const ids = btns.map((b) => b.getAttribute("aria-controls"));
    expect(ids[0]).toBeTruthy();
    expect(new Set(ids).size).toBe(2);

    // Once open, each button's aria-controls resolves to its own list.
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

describe("empty-selection guard", () => {
  it("blocks the toggle that would leave zero includes before any PATCH, with the inline warn line", async () => {
    mountsReply = mountsResponse();
    browseReplies = [listing()];
    await renderEditor();

    // The mount root is the only include, so unchecking it would empty the
    // selection.
    const root = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    const box = within(root).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    // Blocked before any request: the include stays and the warning says why.
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

// One save in flight at a time; toggles made meanwhile go out as one
// follow-up save.
describe("FoldersEditor save queue and empty-selection guard", () => {
  it("a blocked uncheck of the last include shakes the row and shows the warning", async () => {
    // One selected mount, no custom: unchecking it would empty the item.
    await renderEditor();

    const box = within(screen.getByRole("treeitem", { name: /user\/appdata\/plex/ })).getByRole("checkbox", { hidden: true });
    await act(async () => {
      fireEvent.click(box);
    });

    expect(patches).toEqual([]);
    const row = screen.getByRole("treeitem", { name: /user\/appdata\/plex/ });
    expect(row.getAttribute("aria-checked")).toBe("true");
    expect(row.className).toContain("glim-shake");
    const warn = screen.getByText(
      "At least one folder must stay selected. To back up none of this container, turn off Include in schedule. To return to automatic detection, use Reset selection.",
    );
    expect(warn.className).toContain("text-xs");
    expect(warn.className).toContain("text-statusWarn");
  });

  it("unchecking the last mount still saves while a custom include remains", async () => {
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

    // Uncheck plex; the first save is held in flight.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/config/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patches.length).toBe(1);
    expect(patches[0].paths).toEqual([PLEX2, CUSTOM]);

    // A second toggle meanwhile updates the view but is not sent yet.
    await act(async () => {
      fireEvent.click(within(screen.getByRole("treeitem", { name: /\/data/ })).getByRole("checkbox", { hidden: true }));
    });
    expect(patches.length).toBe(1);

    // The first save fails. The revert is applied to the current state, so
    // plex comes back and plex2's uncheck survives; then the queued save sends
    // the final list once.
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

  it("collapses a burst of toggles into one follow-up save", async () => {
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
    // One follow-up save carrying the latest list.
    expect(patches.length).toBe(2);
    expect(patches[0].paths).toEqual([PLEX2, CUSTOM]);
    expect(patches[1].paths).toEqual([CUSTOM]);
  });

  it("toasts a coded empty-selection refusal and reverts the change", async () => {
    // The client guard should prevent this, but the server's refusal is
    // still handled.
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

    expect(patches.length).toBe(1); // the refusal settles the queue
    expect(screen.getByText("Selection would be empty")).toBeTruthy();
    expect(screen.getByRole("treeitem", { name: /\/config/ }).getAttribute("aria-checked")).toBe("true");
  });
});
