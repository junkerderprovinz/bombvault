// ---------------------------------------------------------------------------
// Selection-tree logic (Phase 2, plan 01): the pure half of the container
// panel's folder tree. No React, no network — everything here is list
// arithmetic over the Phase 1 flat selection encoding, so it is node-env
// table-testable and mirrors the Go semantic source of truth
// (internal/api/selection.go) segment-for-segment.
//
// THE LOAD-BEARING RULE: per-node state (checked/mixed/excluded/unchecked) is
// computed ONLY from the two entry classes — includes I and exclusions E, both
// held in HOST path space — never from lazily loaded children. Laziness
// decides which rows render; (I, E) decides what they look like. That is what
// makes TREE-04 exact: a reopened panel reconstructs every state without a
// single browse call, including collapsed and never-expanded subtrees.
//
// Mirrored Go invariants:
//   - Segment-aligned prefix tests ("/c/plex" never matches "/c/plex2/x") —
//     selection.go isStrictDescendant.
//   - Canonical flat order: sorted bare includes, then sorted "!"-prefixed
//     exclusions — byte-identical output for identical sets, like
//     NormalizeSelection's deterministic ordering.
//   - Orphan exclusions are PRESERVED, not repaired: unchecking a parent
//     never deletes the exclusions strictly below it. They stay stored
//     DORMANT so D-01's remembered partial round-trips — the exclusion list
//     below a node IS the remembered partial state; there is no second
//     UI-side memory to drift.
//
// Expansion persistence (D-05) is comfort state ONLY: localStorage
// bv-tree-expanded-{containerName}, capped, never consulted for selection.
// Selection ALWAYS round-trips the server (D-03).
// ---------------------------------------------------------------------------

/** Phase 1 wire prefix marking an exclusion entry ("!"). */
export const EXCLUSION_PREFIX = "!";

/** Per-node visual state, derived purely from (I, E) — TREE-03. */
export type NodeState = "checked" | "mixed" | "excluded" | "unchecked";

/** The two entry classes of the flat selection, split and cleaned. */
export interface FlatSets {
  includes: Set<string>;
  exclusions: Set<string>;
}

/** Cap on persisted expansion entries per container (D-05: bounded keys). */
const MAX_EXPANDED = 64;

const EXPANDED_KEY_PREFIX = "bv-tree-expanded-";

/** POSIX path.Clean mirror (pure string form — no filesystem, no host calls):
 *  collapse slashes, drop trailing "/", resolve "." and ".." lexically. */
function cleanPath(p: string): string {
  const isAbs = p.startsWith("/");
  const parts: string[] = [];
  for (const seg of p.split("/")) {
    if (seg === "" || seg === ".") continue;
    if (seg === "..") {
      if (parts.length > 0 && parts[parts.length - 1] !== "..") parts.pop();
      else if (!isAbs) parts.push("..");
      continue;
    }
    parts.push(seg);
  }
  const joined = parts.join("/");
  if (isAbs) return `/${joined}`;
  return joined || ".";
}

/** True when `path` equals or lives under `ancestor` (segment-aligned). */
export function isAtOrUnder(path: string, ancestor: string): boolean {
  const p = cleanPath(path);
  const a = cleanPath(ancestor);
  if (p === a) return true;
  const base = a === "/" ? "" : a;
  return p.startsWith(`${base}/`);
}

/** True when `path` lives STRICTLY under `ancestor` (segment-aligned). */
export function isStrictlyUnder(path: string, ancestor: string): boolean {
  const p = cleanPath(path);
  const a = cleanPath(ancestor);
  return p !== a && isAtOrUnder(p, a);
}

/** Host path -> browse-relative path (swap the hostSourceRoot prefix).
 *  The root itself maps to "" (browse of the top level). An already-absolute
 *  path elsewhere passes through untranslated — the same precedent as the
 *  manual custom-path entry (Containers.tsx addCustom). */
export function hostToBrowseRel(host: string, hostSourceRoot: string): string {
  const h = cleanPath(host);
  const root = cleanPath(hostSourceRoot);
  if (isAtOrUnder(h, root)) {
    const base = root === "/" ? "" : root;
    return h === root ? "" : h.slice(base.length + 1);
  }
  return h;
}

/** Browse-relative path -> host path (the inverse prefix swap; an
 *  already-absolute input passes through untranslated). */
export function browseRelToHost(rel: string, hostSourceRoot: string): string {
  const r = cleanPath(rel);
  if (r.startsWith("/")) return r;
  const root = cleanPath(hostSourceRoot);
  if (r === ".") return root;
  const base = root === "/" ? "" : root;
  return `${base}/${r}`;
}

/** Split a stored flat list into its two classes, prefix stripped and paths
 *  cleaned. A bare "!" (prefix with no path) is skipped like selection.go;
 *  an exclusions-only result is the deliberate explicit-none carrier and is
 *  kept as-is — the client displays it, never repairs it. */
export function splitFlatSet(entries: readonly string[]): FlatSets {
  const includes = new Set<string>();
  const exclusions = new Set<string>();
  for (const raw of entries) {
    const bare = raw.startsWith(EXCLUSION_PREFIX) ? raw.slice(EXCLUSION_PREFIX.length) : raw;
    const p = cleanPath(bare);
    if (p === ".") continue;
    if (raw.startsWith(EXCLUSION_PREFIX)) exclusions.add(p);
    else includes.add(p);
  }
  return { includes, exclusions };
}

/** Serialize the two classes back to the canonical wire form: sorted bare
 *  includes, then sorted "!"-prefixed exclusions. Identical sets produce
 *  byte-identical arrays, so the server's re-normalization is a no-op. */
export function toFlatList(includes: ReadonlySet<string>, exclusions: ReadonlySet<string>): string[] {
  const inc = [...includes]
    .map(cleanPath)
    .filter((p) => p !== ".")
    .sort();
  const exc = [...exclusions]
    .map(cleanPath)
    .filter((p) => p !== ".")
    .sort();
  return [...inc, ...exc.map((e) => EXCLUSION_PREFIX + e)];
}

/** Classify a node from (I, E) alone (RESEARCH Pattern 1, the pinned shape):
 *  excluded when at/under an E entry (exclusion dominates — the classifier
 *  stays total even for an equal include/exclude pair the reducer can never
 *  produce); mixed when an include applies AND an E lies strictly below
 *  (carve-out) or an I lies strictly below (whitelist start-state); checked
 *  when an include applies with nothing carved out and no deeper include;
 *  unchecked otherwise. */
export function classifyNode(
  hostPath: string,
  includes: ReadonlySet<string>,
  exclusions: ReadonlySet<string>,
): NodeState {
  const node = cleanPath(hostPath);
  for (const e of exclusions) {
    if (isAtOrUnder(node, e)) return "excluded";
  }
  let incApplies = false;
  for (const i of includes) {
    if (isAtOrUnder(node, i)) {
      incApplies = true;
      break;
    }
  }
  let exclBelow = false;
  for (const e of exclusions) {
    if (isStrictlyUnder(e, node)) {
      exclBelow = true;
      break;
    }
  }
  let incBelow = false;
  for (const i of includes) {
    if (isStrictlyUnder(i, node)) {
      incBelow = true;
      break;
    }
  }
  if ((incApplies && exclBelow) || incBelow) return "mixed";
  if (incApplies) return "checked";
  return "unchecked";
}

/** Apply one checkbox toggle to (I, E) — the D-01 remembered-partial cycle.
 *  Dispatch keys off the node's CURRENT state plus list membership:
 *
 *    own include present        -> unselect: drop the own include and every
 *                                   strictly-below include; E below stays
 *                                   stored dormant (the remembered partial).
 *    checked via ancestor only  -> carve-out: add "!"+node to E; the parent
 *                                   include stays (TREE-02).
 *    mixed, no own include      -> whitelist unselect: drop every
 *                                   strictly-below include (pinned research
 *                                   reading of D-01, assumption A2).
 *    excluded                   -> re-include: delete every COVERING exclusion
 *                                   (deeper ones stay, so the branch returns
 *                                   as the same partial state); add an own
 *                                   include only when nothing covers it.
 *    unchecked                  -> select: add node to I. Dormant E below
 *                                   wakes immediately (the partial state
 *                                   applies again with zero UI memory). */
export function applyToggle(
  hostPath: string,
  includes: ReadonlySet<string>,
  exclusions: ReadonlySet<string>,
): FlatSets {
  const node = cleanPath(hostPath);
  const state = classifyNode(node, includes, exclusions);
  const next: FlatSets = { includes: new Set(includes), exclusions: new Set(exclusions) };
  if (includes.has(node)) {
    next.includes.delete(node);
    for (const i of includes) {
      if (isStrictlyUnder(i, node)) next.includes.delete(i);
    }
    return next;
  }
  switch (state) {
    case "excluded": {
      for (const e of exclusions) {
        if (isAtOrUnder(node, e)) next.exclusions.delete(e);
      }
      if (classifyNode(node, next.includes, next.exclusions) === "unchecked") {
        next.includes.add(node);
      }
      return next;
    }
    case "checked":
      // Covered by an ancestor include and not excluded: the click carves
      // this node out of that coverage.
      next.exclusions.add(node);
      return next;
    case "mixed":
      // Whitelist flavor (include strictly below, none at/under): deselect
      // everything below without inventing coverage-level exclusions.
      for (const i of includes) {
        if (isStrictlyUnder(i, node)) next.includes.delete(i);
      }
      return next;
    default:
      next.includes.add(node);
      return next;
  }
}

/** localStorage access is lazy and guarded both directions (displayPrefs.ts
 *  discipline): Node's test env has no storage global until a test installs
 *  one, and a browser's private window throws on access or write. */
function storage(): Storage | null {
  try {
    return (globalThis as { localStorage?: Storage }).localStorage ?? null;
  } catch {
    return null;
  }
}

/** Read the persisted expansion list for a container (D-05). Comfort state
 *  only — selection is NEVER read from or written here. Corrupted payloads
 *  and throwing storage degrade to "nothing expanded". */
export function loadExpanded(containerName: string): string[] {
  const s = storage();
  if (!s) return [];
  try {
    const raw = s.getItem(EXPANDED_KEY_PREFIX + containerName);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((p): p is string => typeof p === "string");
  } catch {
    return [];
  }
}

/** Persist the expansion list (write order = recency, newest last), capped
 *  at MAX_EXPANDED with the oldest-expanded evicted first so the key stays
 *  bounded (D-05). Never throws. */
export function saveExpanded(containerName: string, paths: readonly string[]): void {
  const s = storage();
  if (!s) return;
  try {
    s.setItem(EXPANDED_KEY_PREFIX + containerName, JSON.stringify(paths.slice(-MAX_EXPANDED)));
  } catch {
    // Expansion is comfort state; a failed write is not the user's problem.
  }
}
