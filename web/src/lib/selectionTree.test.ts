// Pure selection-tree logic (Phase 2, plan 01): classifier, D-01 toggle
// reducer, path translation, flat-list (de)serialization, expansion
// persistence.
//
// The semantic source of truth is internal/api/selection.go (Phase 1): the
// stored selection is ONE flat list where bare entries are includes and "!"-
// prefixed entries are exclusions. Everything this module does is list
// arithmetic over those two classes, so these tables pin the invariants the
// Go side already guarantees — segment-aligned prefix matching ("/c/plex"
// never matches "/c/plex2/x", mirroring isStrictDescendant), per-class
// maximal-prune-free canonical ordering (sorted includes, then sorted "!"-
// prefixed exclusions — byte-identical output for identical sets), and
// orphan-exclusion preservation (an exclusion whose include is gone stays
// stored DORMANT; deleting it would destroy D-01's remembered partial state,
// which IS the exclusion list below the node — there is no second UI-side
// memory).
//
// Node-env file: no jsdom pragma (same convention as i18n.parity.test.ts).
// The persistence tests stub globalThis.localStorage because Node has no
// storage global; the module reads it lazily inside the functions.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  EXCLUSION_PREFIX,
  applyToggle,
  browseRelToHost,
  classifyNode,
  hostToBrowseRel,
  isAtOrUnder,
  isStrictlyUnder,
  loadExpanded,
  partitionCustomPaths,
  rootIncludeCount,
  saveExpanded,
  splitFlatSet,
  toFlatList,
} from "./selectionTree";

// ---------------------------------------------------------------------------
// Segment-aligned prefix arithmetic (mirrors selection.go isStrictDescendant)
// ---------------------------------------------------------------------------

describe("isAtOrUnder / isStrictlyUnder", () => {
  const cases: { path: string; ancestor: string; atOrUnder: boolean; strictlyUnder: boolean }[] = [
    { path: "/c/plex", ancestor: "/c", atOrUnder: true, strictlyUnder: true },
    { path: "/c/plex", ancestor: "/c/plex", atOrUnder: true, strictlyUnder: false },
    // The case that punishes naive string HasPrefix: a sibling sharing the
    // prefix is NOT a descendant. selection.go:45-51 aligns on path segments.
    { path: "/c/plex2/x", ancestor: "/c/plex", atOrUnder: false, strictlyUnder: false },
    { path: "/c", ancestor: "/c/plex", atOrUnder: false, strictlyUnder: false },
    { path: "/c/plex/meta", ancestor: "/c/plex", atOrUnder: true, strictlyUnder: true },
  ];
  it.each(cases)(
    "$path vs $ancestor -> atOrUnder=$atOrUnder strictlyUnder=$strictlyUnder",
    ({ path, ancestor, atOrUnder, strictlyUnder }) => {
      expect(isAtOrUnder(path, ancestor)).toBe(atOrUnder);
      expect(isStrictlyUnder(path, ancestor)).toBe(strictlyUnder);
    },
  );
});

// ---------------------------------------------------------------------------
// classifyNode — the four per-node states, pure (I, E) arithmetic (TREE-03/04)
// ---------------------------------------------------------------------------

describe("classifyNode", () => {
  const cases: {
    name: string;
    includes: string[];
    exclusions: string[];
    node: string;
    want: "checked" | "mixed" | "excluded" | "unchecked";
  }[] = [
    { name: "nothing stored -> unchecked", includes: [], exclusions: [], node: "/a", want: "unchecked" },
    { name: "own include -> checked", includes: ["/a"], exclusions: [], node: "/a", want: "checked" },
    { name: "ancestor include covers -> checked", includes: ["/a"], exclusions: [], node: "/a/b", want: "checked" },
    // Segment alignment through the classifier: the Phase 1 example verbatim.
    { name: "sibling sharing the prefix is NOT covered (/c/plex vs /c/plex2)", includes: ["/c/plex"], exclusions: [], node: "/c/plex2", want: "unchecked" },
    { name: "deep sibling stays uncovered (/c/plex2/x)", includes: ["/c/plex"], exclusions: [], node: "/c/plex2/x", want: "unchecked" },
    { name: "own exclusion -> excluded", includes: ["/a"], exclusions: ["/a/b"], node: "/a/b", want: "excluded" },
    { name: "ancestor exclusion covers -> excluded", includes: ["/a"], exclusions: ["/a/b"], node: "/a/b/c", want: "excluded" },
    { name: "include applies with an exclusion strictly below -> mixed (carve-out)", includes: ["/a"], exclusions: ["/a/b"], node: "/a", want: "mixed" },
    { name: "include strictly below -> mixed (whitelist start-state)", includes: ["/a/b"], exclusions: [], node: "/a", want: "mixed" },
    // An equal include/exclude pair is never stored (the reducer cannot
    // produce one), but the classifier must stay total: exclusion dominates.
    { name: "equal include+exclusion -> excluded (total, exclusion wins)", includes: ["/a"], exclusions: ["/a"], node: "/a", want: "excluded" },
  ];
  it.each(cases)("$name", ({ includes, exclusions, node, want }) => {
    expect(classifyNode(node, new Set(includes), new Set(exclusions))).toBe(want);
  });
});

// ---------------------------------------------------------------------------
// applyToggle — the D-01 remembered-partial cycle (TREE-02/03)
// ---------------------------------------------------------------------------

describe("applyToggle D-01 transitions", () => {
  const cases: {
    name: string;
    before: { includes: string[]; exclusions: string[] };
    node: string;
    after: { includes: string[]; exclusions: string[] };
  }[] = [
    {
      name: "unchecked node gains an include",
      before: { includes: [], exclusions: [] },
      node: "/a",
      after: { includes: ["/a"], exclusions: [] },
    },
    {
      // The remembered-partial wake-up: the dormant exclusion below is
      // PRESERVED and immediately applies again, with zero UI-side memory.
      name: "unchecked node over dormant exclusions wakes them as mixed",
      before: { includes: [], exclusions: ["/a/b"] },
      node: "/a",
      after: { includes: ["/a"], exclusions: ["/a/b"] },
    },
    {
      // Unchecking a parent NEVER deletes the exclusions strictly below it —
      // they stay stored dormant so the partial state round-trips (D-01).
      name: "own include removed; exclusions below stay stored (dormant)",
      before: { includes: ["/a"], exclusions: ["/a/b"] },
      node: "/a",
      after: { includes: [], exclusions: ["/a/b"] },
    },
    {
      name: "checked-via-ancestor node carves itself out as a ! exclusion",
      before: { includes: ["/a"], exclusions: [] },
      node: "/a/b",
      after: { includes: ["/a"], exclusions: ["/a/b"] },
    },
    {
      // Mixed click (whitelist flavor included): own AND strictly-below
      // includes go; the E list below IS the remembered partial and stays.
      name: "mixed node loses own and strictly-below includes; E stays",
      before: { includes: ["/a", "/a/x"], exclusions: ["/a/b"] },
      node: "/a",
      after: { includes: [], exclusions: ["/a/b"] },
    },
    {
      // Carve-out flavor of mixed (review CR-01): an ancestor include
      // applies, the exclusion is strictly below, and nothing of ours lives
      // under the node to drop — the whitelist loop alone deleted zero
      // entries, making the toggle a silent no-op that the editor still
      // PATCHed and toasted "Saved" over. The click must be a state change:
      // it deselects the branch exactly like the checked-via-ancestor case,
      // and the deeper exclusion stays stored (redundant under ours, but
      // preserved like every orphan E entry).
      name: "carve-out mixed node deselects its branch as a ! exclusion",
      before: { includes: ["/a"], exclusions: ["/a/b/c"] },
      node: "/a/b",
      after: { includes: ["/a"], exclusions: ["/a/b", "/a/b/c"] },
    },
    {
      name: "excluded node loses the covering exclusion and needs no new include (already covered)",
      before: { includes: ["/a"], exclusions: ["/a/b"] },
      node: "/a/b",
      after: { includes: ["/a"], exclusions: [] },
    },
    {
      name: "excluded node with nothing covering it gains its own include",
      before: { includes: ["/a/x"], exclusions: ["/a/b"] },
      node: "/a/b",
      after: { includes: ["/a/b", "/a/x"], exclusions: [] },
    },
    {
      // Only COVERING exclusions go; a deeper exclusion stays so the branch
      // returns as the same partial state, not as fully checked.
      name: "re-including an excluded branch keeps deeper exclusions (partial restore)",
      before: { includes: ["/a"], exclusions: ["/a/b", "/a/b/c"] },
      node: "/a/b",
      after: { includes: ["/a"], exclusions: ["/a/b/c"] },
    },
  ];
  it.each(cases)("$name", ({ before, node, after }) => {
    const next = applyToggle(node, new Set(before.includes), new Set(before.exclusions));
    expect([...next.includes].sort()).toEqual([...after.includes].sort());
    expect([...next.exclusions].sort()).toEqual([...after.exclusions].sort());
  });

  it("runs the full D-01 remembered-partial cycle: check, carve out, uncheck parent, recheck parent", () => {
    // Start: mount root selected whole.
    let inc = new Set(["/mnt/user/appdata/plex"]);
    let exc = new Set<string>([]);
    // Uncheck one subfolder -> exclusion carved out, parent goes mixed.
    ({ includes: inc, exclusions: exc } = applyToggle("/mnt/user/appdata/plex/transcoding", inc, exc));
    expect(classifyNode("/mnt/user/appdata/plex", inc, exc)).toBe("mixed");
    // Uncheck the parent -> include removed, exclusion dormant below.
    ({ includes: inc, exclusions: exc } = applyToggle("/mnt/user/appdata/plex", inc, exc));
    expect(classifyNode("/mnt/user/appdata/plex", inc, exc)).toBe("unchecked");
    expect([...exc]).toEqual(["/mnt/user/appdata/plex/transcoding"]);
    // Re-check the parent -> EXACTLY the prior partial state returns.
    ({ includes: inc, exclusions: exc } = applyToggle("/mnt/user/appdata/plex", inc, exc));
    expect([...inc]).toEqual(["/mnt/user/appdata/plex"]);
    expect([...exc]).toEqual(["/mnt/user/appdata/plex/transcoding"]);
    expect(classifyNode("/mnt/user/appdata/plex", inc, exc)).toBe("mixed");
    expect(classifyNode("/mnt/user/appdata/plex/transcoding", inc, exc)).toBe("excluded");
  });
});

// ---------------------------------------------------------------------------
// splitFlatSet / toFlatList — flat wire form <-> class sets
// ---------------------------------------------------------------------------

describe("splitFlatSet", () => {
  it("splits a mixed flat list into the two classes, prefix stripped", () => {
    const { includes, exclusions } = splitFlatSet(["b", "!x", "a", "!a/b"]);
    expect([...includes].sort()).toEqual(["a", "b"]);
    expect([...exclusions].sort()).toEqual(["a/b", "x"]);
  });

  it("returns two empty sets for an empty list", () => {
    const { includes, exclusions } = splitFlatSet([]);
    expect(includes.size).toBe(0);
    expect(exclusions.size).toBe(0);
  });

  it("splits an exclusions-only list to zero includes (explicit-none carrier)", () => {
    // Phase 1 stores exclusions-only as a deliberate explicit-none choice;
    // the client displays it and never repairs it (storedDataIsGone decision).
    const { includes, exclusions } = splitFlatSet(["!e1", "!e2"]);
    expect(includes.size).toBe(0);
    expect([...exclusions].sort()).toEqual(["e1", "e2"]);
  });

  it("skips a bare '!' entry (exclusion prefix with no path) like selection.go", () => {
    const { includes, exclusions } = splitFlatSet(["!", "a"]);
    expect([...includes]).toEqual(["a"]);
    expect(exclusions.size).toBe(0);
  });

  it("cleans entries like path.Clean (collapse slashes, drop trailing slash)", () => {
    const { includes, exclusions } = splitFlatSet(["/a//b/", "!/x/"]);
    expect([...includes]).toEqual(["/a/b"]);
    expect([...exclusions]).toEqual(["/x"]);
  });
});

describe("toFlatList", () => {
  it("emits sorted bare includes then sorted !-prefixed exclusions", () => {
    expect(toFlatList(new Set(["b", "a"]), new Set(["y", "x"]))).toEqual(["a", "b", "!x", "!y"]);
  });

  it("is byte-identical for identical sets regardless of Set insertion order", () => {
    // Set iteration follows insertion order; the output must not.
    const one = toFlatList(new Set(["b", "a", "c"]), new Set(["z", "m"]));
    const two = toFlatList(new Set(["c", "b", "a"]), new Set(["m", "z"]));
    expect(one).toEqual(two);
    expect(one).toEqual(["a", "b", "c", "!m", "!z"]);
  });

  it("round-trips through splitFlatSet", () => {
    const flat = ["a", "!x"];
    const { includes, exclusions } = splitFlatSet(flat);
    expect(toFlatList(includes, exclusions)).toEqual(flat);
  });

  it("returns an empty array for two empty sets", () => {
    expect(toFlatList(new Set(), new Set())).toEqual([]);
  });

  it("emits a single-entry list for one include and no exclusions", () => {
    expect(toFlatList(new Set(["/a"]), new Set())).toEqual(["/a"]);
  });
});

// ---------------------------------------------------------------------------
// host <-> browse-relative translation (the two served roots)
// ---------------------------------------------------------------------------

describe("path translation", () => {
  it("strips the hostSourceRoot prefix for browse-relative paths", () => {
    expect(hostToBrowseRel("/mnt/user/appdata/plex", "/mnt")).toBe("user/appdata/plex");
  });

  it("maps the host source root itself to the empty relative path", () => {
    expect(hostToBrowseRel("/mnt", "/mnt")).toBe("");
  });

  it("passes an already-absolute-elsewhere path through untranslated (Containers.tsx:802 precedent)", () => {
    expect(hostToBrowseRel("/srv/other", "/mnt")).toBe("/srv/other");
  });

  it("prefixes the hostSourceRoot for host form", () => {
    expect(browseRelToHost("user/appdata/plex", "/mnt")).toBe("/mnt/user/appdata/plex");
  });

  it("passes an already-absolute relative through untranslated (same precedent, other direction)", () => {
    expect(browseRelToHost("/srv/other", "/mnt")).toBe("/srv/other");
  });

  it("round-trips host paths under the root", () => {
    for (const host of ["/mnt/user/appdata/plex", "/mnt/cache/x", "/mnt"]) {
      expect(browseRelToHost(hostToBrowseRel(host, "/mnt"), "/mnt")).toBe(host === "/mnt" ? "/mnt" : host);
    }
  });
});

// ---------------------------------------------------------------------------
// Expansion persistence (D-05): comfort state only, NEVER selection
// ---------------------------------------------------------------------------

// Node has no localStorage global; install a minimal backing store the module
// can find lazily on globalThis (displayPrefs.ts-style guarded access).
function installStorage(backing: Map<string, string>): void {
  (globalThis as unknown as Record<string, unknown>).localStorage = {
    getItem: (k: string) => (backing.has(k) ? backing.get(k)! : null),
    setItem: (k: string, v: string) => void backing.set(k, v),
    removeItem: (k: string) => void backing.delete(k),
    clear: () => backing.clear(),
  };
}

describe("expansion persistence (D-05)", () => {
  beforeEach(() => {
    installStorage(new Map());
  });
  afterEach(() => {
    delete (globalThis as unknown as Record<string, unknown>).localStorage;
  });

  it("returns an empty array when nothing is stored", () => {
    expect(loadExpanded("plex")).toEqual([]);
  });

  it("round-trips an expansion list under the per-container bv- key", () => {
    saveExpanded("plex", ["/mnt/user/appdata/plex", "/mnt/user/appdata"]);
    expect(loadExpanded("plex")).toEqual(["/mnt/user/appdata/plex", "/mnt/user/appdata"]);
    // Scoped per container: a different name sees nothing.
    expect(loadExpanded("jellyfin")).toEqual([]);
  });

  it("caps the stored list at 64, keeping the most recently expanded (write order = recency)", () => {
    const many = Array.from({ length: 70 }, (_, i) => `/mnt/user/appdata/d${i}`);
    saveExpanded("plex", many);
    const stored = loadExpanded("plex");
    expect(stored).toHaveLength(64);
    // The oldest 6 were evicted; the tail (latest writes) survived.
    expect(stored[0]).toBe("/mnt/user/appdata/d6");
    expect(stored[63]).toBe("/mnt/user/appdata/d69");
  });

  it("ignores a corrupted payload (non-array JSON) and returns an empty array", () => {
    (globalThis as unknown as Record<string, unknown>).localStorage = {
      getItem: () => "{not json",
      setItem: () => {},
      removeItem: () => {},
      clear: () => {},
    };
    expect(loadExpanded("plex")).toEqual([]);
  });
});

// The exclusion prefix is part of the wire contract (Phase 1 selection.go);
// pinning the literal keeps the constant honest.
describe("EXCLUSION_PREFIX", () => {
  it("is the Phase 1 wire prefix", () => {
    expect(EXCLUSION_PREFIX).toBe("!");
  });
});

// ---------------------------------------------------------------------------
// Determinism and set semantics (plan 02 edge tables)
// ---------------------------------------------------------------------------

describe("toggle-sequence determinism", () => {
  it("produces byte-identical flat lists when the same sequence runs twice from the same start", () => {
    // Three toggles: carve out a subfolder, uncheck its parent (dormant),
    // re-check the parent (dormant wakes) — the D-01 heart of the cycle.
    const run = () => {
      let inc = new Set(["/a"]);
      let exc = new Set<string>([]);
      ({ includes: inc, exclusions: exc } = applyToggle("/a/b/c", inc, exc));
      ({ includes: inc, exclusions: exc } = applyToggle("/a/b", inc, exc));
      ({ includes: inc, exclusions: exc } = applyToggle("/a/b", inc, exc));
      return toFlatList(inc, exc);
    };
    const first = run();
    const second = run();
    expect(first).toEqual(second);
    // And the end state itself is pinned: parent include restored, exclusion
    // awake below it — exactly the pre-carve-out partial state.
    expect(first).toEqual(["/a", "!/a/b/c"]);
  });
});

describe("permutation-insensitive classification", () => {
  // All orderings of a 4-entry list — small enough to exhaust.
  function permutations(items: string[]): string[][] {
    if (items.length <= 1) return [items];
    const out: string[][] = [];
    items.forEach((item, i) => {
      const rest = [...items.slice(0, i), ...items.slice(i + 1)];
      for (const tail of permutations(rest)) out.push([item, ...tail]);
    });
    return out;
  }

  it("classifies a fixed node corpus identically for every permutation of the same flat list", () => {
    const flat = [
      "/mnt/user/appdata/plex",
      "/mnt/user/media",
      "!/mnt/user/appdata/plex/transcoding",
      "!/mnt/user/appdata/plex/cache/tmp",
    ];
    const corpus = [
      "/mnt/user/appdata/plex", // include applies, exclusion strictly below
      "/mnt/user/appdata/plex/transcoding", // own exclusion
      "/mnt/user/appdata/plex/cache/tmp/x", // under an exclusion
      "/mnt/user/appdata/plex/library", // covered by the include, nothing carved out
      "/mnt/user/media", // own include
      "/mnt/user", // includes strictly below (whitelist parent)
      "/mnt/user/other", // nothing stored at/under it
    ];
    const baseline = splitFlatSet(flat);
    const expected = corpus.map((n) => classifyNode(n, baseline.includes, baseline.exclusions));
    // The states themselves are the point — set semantics, not order reading.
    expect(expected).toEqual([
      "mixed",
      "excluded",
      "excluded",
      "checked",
      "checked",
      "mixed",
      "unchecked",
    ]);

    const perms = permutations(flat);
    expect(perms.length).toBe(24);
    for (const perm of perms) {
      const { includes, exclusions } = splitFlatSet(perm);
      expect(corpus.map((n) => classifyNode(n, includes, exclusions))).toEqual(expected);
    }
  });
});

describe("toFlatList totality", () => {
  it("serializes exactly what it is given — exclusions-only input round-trips as the explicit-none carrier", () => {
    // Phase 1's storedDataIsGone decision: an exclusions-only list is a
    // deliberate full deselect, a stored state the client must display and
    // never repair. The serializer has no item-level knowledge (whether some
    // OTHER mount still holds includes) and must not invent any.
    expect(toFlatList(new Set(), new Set(["e2", "e1"]))).toEqual(["!e1", "!e2"]);
    const { includes, exclusions } = splitFlatSet(["!e1", "!e2"]);
    expect(includes.size).toBe(0);
    expect(toFlatList(includes, exclusions)).toEqual(["!e1", "!e2"]);
  });

  it("serializes a single-include input with no exclusion noise", () => {
    expect(toFlatList(new Set(["i"]), new Set())).toEqual(["i"]);
  });
});

describe("expansion persistence bounds (D-05, plan 02 tables)", () => {
  beforeEach(() => {
    installStorage(new Map());
  });
  afterEach(() => {
    delete (globalThis as unknown as Record<string, unknown>).localStorage;
  });

  it("returns [] and never throws under a throwing storage, in both directions", () => {
    // D-05: expansion is comfort state ONLY — a broken private-window
    // storage can never break the panel, let alone leak into selection.
    (globalThis as unknown as Record<string, unknown>).localStorage = {
      getItem: () => {
        throw new Error("unavailable");
      },
      setItem: () => {
        throw new Error("unavailable");
      },
      removeItem: () => {
        throw new Error("unavailable");
      },
      clear: () => {
        throw new Error("unavailable");
      },
    };
    expect(loadExpanded("plex")).toEqual([]);
    expect(() => saveExpanded("plex", ["/a"])).not.toThrow();
  });

  it("evicts the oldest-expanded first across successive saves (write order = recency)", () => {
    // Expanding one node at a time mirrors the real call pattern: every
    // expand appends and saves the whole current list.
    const expanded: string[] = [];
    for (let i = 0; i <= 65; i++) {
      expanded.push(`/mnt/user/appdata/d${i}`);
      saveExpanded("plex", expanded);
    }
    const stored = loadExpanded("plex");
    expect(stored).toHaveLength(64);
    expect(stored[0]).toBe("/mnt/user/appdata/d2"); // d0, d1 evicted
    expect(stored[63]).toBe("/mnt/user/appdata/d65");

    // Re-expanding a long-expanded node refreshes its recency: it moves to
    // the end and the next-oldest becomes the eviction candidate.
    const refreshed = [...stored.filter((p) => p !== "/mnt/user/appdata/d2"), "/mnt/user/appdata/d2"];
    saveExpanded("plex", refreshed);
    const reloaded = loadExpanded("plex");
    expect(reloaded[0]).toBe("/mnt/user/appdata/d3");
    expect(reloaded[63]).toBe("/mnt/user/appdata/d2");
  });
});

// ---------------------------------------------------------------------------
// partitionCustomPaths — sub-include absorption (INTEG-01, D-02, RESEARCH Q1)
//
// The server puts every include that is not EXACTLY a mount root into custom[]
// (service.go:3755-3782 — matched[cp] only on exact equality), so a sub-include
// under a reachable mount arrives as a custom row. The tree absorbs it: the
// entry joins the (I, E) mirror under its mount (rendering the mount mixed per
// the whitelist start-state) and is filtered from the custom row list, so every
// path has exactly ONE presentation on screen.
// ---------------------------------------------------------------------------

describe("partitionCustomPaths", () => {
  const cases: {
    name: string;
    customPaths: string[];
    mountSources: string[];
    underMount: string[];
    standalone: string[];
  }[] = [
    {
      name: "the plan's example: sub-include absorbed, foreign path standalone",
      customPaths: ["/mnt/user/appdata/plex/Media", "/mnt/user/other"],
      mountSources: ["/mnt/user/appdata/plex", "/mnt/user/media"],
      underMount: ["/mnt/user/appdata/plex/Media"],
      standalone: ["/mnt/user/other"],
    },
    {
      name: "a sibling sharing the prefix stays standalone (segment alignment)",
      customPaths: ["/mnt/user/appdata/plex2"],
      mountSources: ["/mnt/user/appdata/plex"],
      underMount: [],
      standalone: ["/mnt/user/appdata/plex2"],
    },
    {
      name: "an exact mount-root match is never a sub-include (it IS the mount row)",
      customPaths: ["/mnt/user/appdata/plex"],
      mountSources: ["/mnt/user/appdata/plex"],
      underMount: [],
      standalone: ["/mnt/user/appdata/plex"],
    },
    {
      name: "a deeper path still absorbs under its covering mount",
      customPaths: ["/mnt/user/appdata/plex/Library/Movies"],
      mountSources: ["/mnt/user/appdata/plex", "/mnt/user/media"],
      underMount: ["/mnt/user/appdata/plex/Library/Movies"],
      standalone: [],
    },
    {
      name: "multiple mounts: each sub-include absorbs under its own root only",
      customPaths: [
        "/mnt/user/appdata/plex/Media",
        "/mnt/user/media/Movies",
        "/mnt/user/backups",
      ],
      mountSources: ["/mnt/user/appdata/plex", "/mnt/user/media"],
      underMount: ["/mnt/user/appdata/plex/Media", "/mnt/user/media/Movies"],
      standalone: ["/mnt/user/backups"],
    },
    {
      name: "no mounts: everything is standalone",
      customPaths: ["/mnt/user/other"],
      mountSources: [],
      underMount: [],
      standalone: ["/mnt/user/other"],
    },
    {
      name: "no custom paths: both halves empty",
      customPaths: [],
      mountSources: ["/mnt/user/appdata/plex"],
      underMount: [],
      standalone: [],
    },
  ];

  it.each(cases)("$name", ({ customPaths, mountSources, underMount, standalone }) => {
    expect(partitionCustomPaths(customPaths, mountSources)).toEqual({
      underMount,
      standalone,
    });
  });
});

// ---------------------------------------------------------------------------
// rootIncludeCount — the per-root "{n} paths" preview (Phase 3, plan 02, D-01)
//
// The visible count must equal what the next backup hands restic: stored
// maximal includes ARE the positional sources (Phase 1 flat-set contract), so
// the count is computed ONLY from the includes set — never from checked nodes
// on screen, never from loaded children, and never filtered by existence.
// ---------------------------------------------------------------------------

describe("rootIncludeCount (D-01: the preview mirrors the flat-set positional truth)", () => {
  const cases: { name: string; root: string; includes: string[]; want: number }[] = [
    {
      name: "a root equal to an include counts exactly 1",
      root: "/mnt/user/appdata/plex",
      includes: ["/mnt/user/appdata/plex"],
      want: 1,
    },
    {
      name: "an include strictly under the root counts 1",
      root: "/mnt/user/appdata/plex",
      includes: ["/mnt/user/appdata/plex/Media"],
      want: 1,
    },
    {
      name: "a sibling sharing the prefix counts 0 (segment alignment)",
      root: "/mnt/user/appdata/plex",
      includes: ["/mnt/user/appdata/plex2", "/mnt/user/appdata/plex2/x"],
      want: 0,
    },
    {
      name: "multiple nested includes count once each (stored form is maximal, no ancestor double-count)",
      root: "/mnt/user/appdata/plex",
      includes: ["/mnt/user/appdata/plex/Media", "/mnt/user/appdata/plex/Library"],
      want: 2,
    },
    {
      name: "an include ABOVE the root counts 0 (a parent include is not this root's entry)",
      root: "/mnt/user/appdata/plex",
      includes: ["/mnt/user"],
      want: 0,
    },
    {
      name: "zero includes yields 0",
      root: "/mnt/user/appdata/plex",
      includes: [],
      want: 0,
    },
  ];
  it.each(cases)("$name", ({ root, includes, want }) => {
    expect(rootIncludeCount(root, new Set(includes))).toBe(want);
  });

  it("agrees with toFlatList: the per-root counts sum to the bare entries the serializer emits (D-01)", () => {
    // A representative mirror across the three root kinds: a mount counted by
    // its own entry, a mount counted by two strictly-below entries, and a
    // standalone custom row counted by its own entry. The sum MUST equal the
    // number of bare entries toFlatList serializes — that list is what the
    // next backup PATCH carries and what restic receives as positionals, so
    // any drift here would be the preview lying about the argv.
    const roots = ["/mnt/user/appdata/plex", "/mnt/user/media", "/mnt/user/backups"];
    const includes = new Set([
      "/mnt/user/appdata/plex",
      "/mnt/user/media/Movies",
      "/mnt/user/media/Series",
      "/mnt/user/backups",
    ]);
    const exclusions = new Set(["/mnt/user/appdata/plex/transcoding", "/mnt/user/media/Samples"]);
    const flat = toFlatList(includes, exclusions);
    const bare = flat.filter((e) => !e.startsWith(EXCLUSION_PREFIX)).length;
    const summed = roots.reduce((n, root) => n + rootIncludeCount(root, includes), 0);
    expect(summed).toBe(bare);
    expect(summed).toBe(4);
  });

  it("an exclusions-only root counts 0 while its exclusions stay stored (D-04 dormant)", () => {
    // Fully deselected root with remembered exclusions: the count reads the
    // includes set alone, so it is 0, and the dormant exclusion set is never
    // consulted or mutated by the count.
    const exclusions = new Set(["/mnt/user/appdata/plex/transcoding"]);
    expect(rootIncludeCount("/mnt/user/appdata/plex", new Set())).toBe(0);
    expect([...exclusions]).toEqual(["/mnt/user/appdata/plex/transcoding"]);
  });

  it("is existence-unfiltered BY DESIGN (A3/Pitfall 5): stale and unreachable paths stay counted", () => {
    // Deliberate divergence from run-time existence filtering: the preview
    // derives from the stored flat set ONLY. A path whose folder has since
    // vanished (or sits on an unreachable mount) still counts — those cases
    // are already row-level-warned (folders.customMissing /
    // folders.notReachable), and quietly dropping them here would make the
    // preview disagree with the argv the next backup actually carries.
    expect(rootIncludeCount("/mnt/user/appdata/plex", new Set(["/mnt/user/appdata/plex/gone-since-yesterday"]))).toBe(1);
  });
});
