// The stored selection is one flat list: bare entries are includes and
// "!"-prefixed entries are exclusions (internal/api/selection.go). These tables
// pin what the Go side guarantees: segment-aligned prefix matching ("/c/plex"
// never matches "/c/plex2/x"), a canonical order (sorted includes, then sorted
// exclusions, identical output for identical sets), and that an exclusion whose
// include is gone stays stored, since the exclusions below a node are the only
// memory of its partial state.
//
// Node environment. The persistence tests stub globalThis.localStorage, which
// the module reads lazily.
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
  rootExclusions,
  rootIncludeCount,
  saveExpanded,
  splitFlatSet,
  toFlatList,
} from "./selectionTree";

describe("isAtOrUnder / isStrictlyUnder", () => {
  const cases: { path: string; ancestor: string; atOrUnder: boolean; strictlyUnder: boolean }[] = [
    { path: "/c/plex", ancestor: "/c", atOrUnder: true, strictlyUnder: true },
    { path: "/c/plex", ancestor: "/c/plex", atOrUnder: true, strictlyUnder: false },
    // A sibling sharing the string prefix is not a descendant.
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
    { name: "sibling sharing the prefix is not covered (/c/plex vs /c/plex2)", includes: ["/c/plex"], exclusions: [], node: "/c/plex2", want: "unchecked" },
    { name: "deep sibling stays uncovered (/c/plex2/x)", includes: ["/c/plex"], exclusions: [], node: "/c/plex2/x", want: "unchecked" },
    { name: "own exclusion -> excluded", includes: ["/a"], exclusions: ["/a/b"], node: "/a/b", want: "excluded" },
    { name: "ancestor exclusion covers -> excluded", includes: ["/a"], exclusions: ["/a/b"], node: "/a/b/c", want: "excluded" },
    { name: "include applies with an exclusion strictly below -> mixed (carve-out)", includes: ["/a"], exclusions: ["/a/b"], node: "/a", want: "mixed" },
    { name: "include strictly below -> mixed (whitelist start-state)", includes: ["/a/b"], exclusions: [], node: "/a", want: "mixed" },
    // The reducer never stores an equal include/exclude pair, but the
    // classifier must stay total: exclusion wins.
    { name: "equal include+exclusion -> excluded (total, exclusion wins)", includes: ["/a"], exclusions: ["/a"], node: "/a", want: "excluded" },
  ];
  it.each(cases)("$name", ({ includes, exclusions, node, want }) => {
    expect(classifyNode(node, new Set(includes), new Set(exclusions))).toBe(want);
  });
});

describe("applyToggle transitions", () => {
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
      // The dormant exclusion below is kept and applies again at once.
      name: "unchecked node over dormant exclusions wakes them as mixed",
      before: { includes: [], exclusions: ["/a/b"] },
      node: "/a",
      after: { includes: ["/a"], exclusions: ["/a/b"] },
    },
    {
      // The exclusions below stay stored so the partial state round-trips.
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
      // Own and strictly-below includes go; the exclusions below are the
      // remembered partial state and stay.
      name: "mixed node loses own and strictly-below includes; E stays",
      before: { includes: ["/a", "/a/x"], exclusions: ["/a/b"] },
      node: "/a",
      after: { includes: [], exclusions: ["/a/b"] },
    },
    {
      // An ancestor include applies and the exclusion is strictly below, so
      // nothing of ours lives under the node to drop. The click still has to
      // change state: it deselects the branch like the checked-via-ancestor
      // case, and the deeper exclusion stays stored.
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
      // Only covering exclusions go; a deeper one stays so the branch comes
      // back in the same partial state, not fully checked.
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

  it("runs the remembered-partial cycle: check, carve out, uncheck parent, recheck parent", () => {
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
    // Re-check the parent -> exactly the prior partial state returns.
    ({ includes: inc, exclusions: exc } = applyToggle("/mnt/user/appdata/plex", inc, exc));
    expect([...inc]).toEqual(["/mnt/user/appdata/plex"]);
    expect([...exc]).toEqual(["/mnt/user/appdata/plex/transcoding"]);
    expect(classifyNode("/mnt/user/appdata/plex", inc, exc)).toBe("mixed");
    expect(classifyNode("/mnt/user/appdata/plex/transcoding", inc, exc)).toBe("excluded");
  });
});

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

  it("splits an exclusions-only list into zero includes", () => {
    // An exclusions-only list is a stored "nothing selected"; the client shows
    // it and never repairs it.
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

describe("path translation", () => {
  it("strips the hostSourceRoot prefix for browse-relative paths", () => {
    expect(hostToBrowseRel("/mnt/user/appdata/plex", "/mnt")).toBe("user/appdata/plex");
  });

  it("maps the host source root itself to the empty relative path", () => {
    expect(hostToBrowseRel("/mnt", "/mnt")).toBe("");
  });

  it("passes a host path outside the root through untranslated", () => {
    expect(hostToBrowseRel("/srv/other", "/mnt")).toBe("/srv/other");
  });

  it("prefixes the hostSourceRoot for host form", () => {
    expect(browseRelToHost("user/appdata/plex", "/mnt")).toBe("/mnt/user/appdata/plex");
  });

  it("passes an already-absolute relative path through untranslated", () => {
    expect(browseRelToHost("/srv/other", "/mnt")).toBe("/srv/other");
  });

  it("round-trips host paths under the root", () => {
    for (const host of ["/mnt/user/appdata/plex", "/mnt/cache/x", "/mnt"]) {
      expect(browseRelToHost(hostToBrowseRel(host, "/mnt"), "/mnt")).toBe(host === "/mnt" ? "/mnt" : host);
    }
  });
});

// Node has no localStorage global, so these tests install a minimal one.
function installStorage(backing: Map<string, string>): void {
  (globalThis as unknown as Record<string, unknown>).localStorage = {
    getItem: (k: string) => (backing.has(k) ? backing.get(k)! : null),
    setItem: (k: string, v: string) => void backing.set(k, v),
    removeItem: (k: string) => void backing.delete(k),
    clear: () => backing.clear(),
  };
}

describe("expansion persistence", () => {
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

describe("EXCLUSION_PREFIX", () => {
  it("is the wire prefix selection.go expects", () => {
    expect(EXCLUSION_PREFIX).toBe("!");
  });
});

describe("toggle-sequence determinism", () => {
  it("produces byte-identical flat lists when the same sequence runs twice from the same start", () => {
    // Carve out a folder, then toggle its parent twice.
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
    // The end state: the include restored and the exclusion awake below it.
    expect(first).toEqual(["/a", "!/a/b/c"]);
  });
});

describe("permutation-insensitive classification", () => {
  // All orderings of a four-entry list.
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
  it("round-trips an exclusions-only list unchanged", () => {
    // The serializer cannot know whether another mount still holds includes,
    // so it must not invent any.
    expect(toFlatList(new Set(), new Set(["e2", "e1"]))).toEqual(["!e1", "!e2"]);
    const { includes, exclusions } = splitFlatSet(["!e1", "!e2"]);
    expect(includes.size).toBe(0);
    expect(toFlatList(includes, exclusions)).toEqual(["!e1", "!e2"]);
  });

  it("serializes a single-include input with no exclusion noise", () => {
    expect(toFlatList(new Set(["i"]), new Set())).toEqual(["i"]);
  });
});

describe("expansion persistence bounds", () => {
  beforeEach(() => {
    installStorage(new Map());
  });
  afterEach(() => {
    delete (globalThis as unknown as Record<string, unknown>).localStorage;
  });

  it("returns [] and never throws under a throwing storage, in both directions", () => {
    // A broken private-window storage must not break the panel.
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

describe("partitionCustomPaths", () => {
  const cases: {
    name: string;
    customPaths: string[];
    mountSources: string[];
    underMount: string[];
    standalone: string[];
  }[] = [
    {
      name: "absorbs a sub-include and keeps a foreign path standalone",
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
      name: "an exact mount-root match is not a sub-include",
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

describe("rootIncludeCount", () => {
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
      name: "an include above the root counts 0 (a parent include is not this root's entry)",
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

  it("agrees with toFlatList: the per-root counts sum to the bare entries the serializer emits", () => {
    // Three kinds of root: a mount counted by its own entry, a mount counted
    // by two entries below it, and a standalone custom row. The sum has to
    // equal the bare entries toFlatList emits, since that list is what restic
    // receives as positional sources.
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

  it("an exclusions-only root counts 0 while its exclusions stay stored", () => {
    // The count reads the includes alone, so a fully deselected root with
    // remembered exclusions counts 0 and its exclusions are left alone.
    const exclusions = new Set(["/mnt/user/appdata/plex/transcoding"]);
    expect(rootIncludeCount("/mnt/user/appdata/plex", new Set())).toBe(0);
    expect([...exclusions]).toEqual(["/mnt/user/appdata/plex/transcoding"]);
  });

  it("still counts stale and unreachable paths", () => {
    // A path whose folder vanished or sits on an unreachable mount already
    // warns on its row, and dropping it here would make the preview disagree
    // with what the next backup carries.
    expect(rootIncludeCount("/mnt/user/appdata/plex", new Set(["/mnt/user/appdata/plex/gone-since-yesterday"]))).toBe(1);
  });
});

describe("rootExclusions", () => {
  const cases: { name: string; root: string; exclusions: string[]; want: string[] }[] = [
    {
      name: "returns only exclusions strictly under the root; one equal to the root is excluded",
      root: "/mnt/user/appdata/plex",
      exclusions: ["/mnt/user/appdata/plex", "/mnt/user/appdata/plex/cache"],
      want: ["cache"],
    },
    {
      name: "strips the root prefix to a relative path with no leading slash",
      root: "/mnt/user/appdata/plex",
      exclusions: ["/mnt/user/appdata/plex/Media/Movies"],
      want: ["Media/Movies"],
    },
    {
      name: "sorts lexically",
      root: "/mnt/user/appdata/plex",
      exclusions: [
        "/mnt/user/appdata/plex/transcoding",
        "/mnt/user/appdata/plex/Media",
        "/mnt/user/appdata/plex/cache",
      ],
      want: ["Media", "cache", "transcoding"],
    },
    {
      name: "returns an empty array when nothing qualifies",
      root: "/mnt/user/appdata/plex",
      exclusions: ["/mnt/user/media"],
      want: [],
    },
    {
      name: "returns dormant exclusions whose root is not included",
      root: "/mnt/user/appdata/plex",
      exclusions: ["/mnt/user/appdata/plex/transcoding"],
      want: ["transcoding"],
    },
    {
      name: "a sibling sharing the prefix never lands in the list (segment alignment)",
      root: "/mnt/user/appdata/plex",
      exclusions: ["/mnt/user/appdata/plex2/cache"],
      want: [],
    },
  ];
  it.each(cases)("$name", ({ root, exclusions, want }) => {
    expect(rootExclusions(root, new Set(exclusions))).toEqual(want);
  });
});
