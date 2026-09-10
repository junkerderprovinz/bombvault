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
