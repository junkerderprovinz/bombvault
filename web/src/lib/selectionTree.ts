// The pure half of the container panel's folder tree: list arithmetic over the
// flat selection encoding, mirroring internal/api/selection.go.
//
// A node's state (checked, mixed, excluded, unchecked) is computed only from
// the includes I and the exclusions E, both in host path space, never from
// lazily loaded children. Laziness decides which rows render and (I, E) decides
// what they look like, so a reopened panel restores every state, collapsed
// subtrees included, without a single browse call.
//
// Invariants shared with the Go side:
//   - Prefix tests are segment-aligned: "/c/plex" never matches "/c/plex2/x"
//     (selection.go isStrictDescendant).
//   - The canonical flat order is sorted includes, then sorted "!"-prefixed
//     exclusions, so identical sets serialize identically.
//   - Orphan exclusions are kept, not repaired. Unchecking a parent leaves the
//     exclusions below it stored, because they are the only memory of the
//     node's partial state.
//
// Expansion state is kept in localStorage (bv-tree-expanded-{containerName})
// for comfort only and never consulted for selection, which always round-trips
// the server.

/** Wire prefix marking an exclusion entry. */
export const EXCLUSION_PREFIX = "!";

/** Per-node visual state, derived from (I, E) alone. */
export type NodeState = "checked" | "mixed" | "excluded" | "unchecked";

/** The two entry classes of the flat selection, split and cleaned. */
export interface FlatSets {
  includes: Set<string>;
  exclusions: Set<string>;
}

/** Cap on persisted expansion entries per container. */
const MAX_EXPANDED = 64;

const EXPANDED_KEY_PREFIX = "bv-tree-expanded-";

/** Go's path.Clean on the string alone: collapses slashes, drops a trailing
 *  "/" and resolves "." and ".." lexically. */
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

/** True when `path` lives strictly under `ancestor` (segment-aligned). */
export function isStrictlyUnder(path: string, ancestor: string): boolean {
  const p = cleanPath(path);
  const a = cleanPath(ancestor);
  return p !== a && isAtOrUnder(p, a);
}

/** Host path to browse-relative path. The root itself maps to "", and a path
 *  outside the root passes through untranslated, as the manual custom-path
 *  entry does (Containers.tsx addCustom). */
export function hostToBrowseRel(host: string, hostSourceRoot: string): string {
  const h = cleanPath(host);
  const root = cleanPath(hostSourceRoot);
  if (isAtOrUnder(h, root)) {
    const base = root === "/" ? "" : root;
    return h === root ? "" : h.slice(base.length + 1);
  }
  return h;
}

/** Browse-relative path to host path; an absolute input passes through
 *  untranslated. */
export function browseRelToHost(rel: string, hostSourceRoot: string): string {
  const r = cleanPath(rel);
  if (r.startsWith("/")) return r;
  const root = cleanPath(hostSourceRoot);
  if (r === ".") return root;
  const base = root === "/" ? "" : root;
  return `${base}/${r}`;
}

/** Splits a stored flat list into its two classes, prefix stripped and paths
 *  cleaned. A bare "!" is skipped as selection.go does. An exclusions-only
 *  result is an explicit "nothing selected" and is kept as is: the client
 *  shows it and never repairs it. */
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

/** Serializes the two classes back to the canonical wire form: sorted bare
 *  includes, then sorted "!"-prefixed exclusions. Identical sets produce
 *  identical arrays, so the server's re-normalization is a no-op. */
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

/** Splits custom-row paths into those under a mount, which the tree absorbs,
 *  and standalone ones.
 *
 *  The server files a stored include as custom whenever it is not exactly a
 *  mount root (service.go ContainerMounts), so a sub-include of a reachable
 *  mount arrives as a custom row and would otherwise appear twice on screen.
 *  It stays in the (I, E) mirror, which already renders its mount mixed, and
 *  is left out of the custom rows. An exact mount root is the mount row itself,
 *  not a sub-include. Only reachable mounts absorb, so the caller passes their
 *  sources; an unreachable mount cannot be browsed and its sub-includes keep
 *  their own rows. */
export function partitionCustomPaths(
  customPaths: readonly string[],
  mountSources: readonly string[],
): { underMount: string[]; standalone: string[] } {
  const underMount: string[] = [];
  const standalone: string[] = [];
  for (const raw of customPaths) {
    const host = cleanPath(raw);
    if (host === ".") continue;
    if (mountSources.some((m) => isStrictlyUnder(host, m))) underMount.push(host);
    else standalone.push(host);
  }
  return { underMount, standalone };
}

/** Counts the stored includes at or under one root, for the per-root
 *  "{n} paths" preview.
 *
 *  What toFlatList serializes is what the next backup hands restic as
 *  positional sources, so the count comes from the includes alone. It does not
 *  look at checked nodes on screen, which would miss collapsed and unloaded
 *  subtrees, and it does not filter by existence: a stale or unreachable
 *  include still goes to restic, and its row already warns about it. An include
 *  above the root is not this root's to count. */
export function rootIncludeCount(root: string, includes: ReadonlySet<string>): number {
  let n = 0;
  for (const p of includes) {
    if (isAtOrUnder(p, root)) n++;
  }
  return n;
}

/** Lists the stored exclusions strictly under one root as relative paths,
 *  sorted, for the per-root "{n} exclusions" list.
 *
 *  It walks the full stored set, dormant entries included, rather than the
 *  loaded tree nodes, so a collapsed or never-expanded root still lists all of
 *  its exclusions. An exclusion equal to the root is left out, since it would
 *  be an empty relative path. Callers only render the result. */
export function rootExclusions(root: string, exclusions: ReadonlySet<string>): string[] {
  const r = cleanPath(root);
  const base = r === "/" ? "" : r;
  const out: string[] = [];
  for (const raw of exclusions) {
    const p = cleanPath(raw);
    if (!isStrictlyUnder(p, r)) continue;
    out.push(p.slice(base.length + 1));
  }
  return out.sort();
}

/** Classifies a node from (I, E) alone. Excluded when at or under an E entry
 *  (exclusion wins, so the classifier stays total even for an equal
 *  include/exclude pair the reducer never produces); mixed when an include
 *  applies and an E lies strictly below (carve-out) or when an I lies strictly
 *  below (whitelist); checked when an include applies with nothing carved out
 *  and no deeper include; unchecked otherwise. */
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

/** Applies one checkbox toggle to (I, E), dispatching on the node's current
 *  state and list membership:
 *
 *    own include present        -> drop it and every include strictly below;
 *                                  the exclusions below stay stored.
 *    checked via ancestor only  -> carve out: add the node to E.
 *    mixed, no own include      -> drop the includes strictly below if there
 *                                  are any (whitelist). Otherwise the node is
 *                                  mixed because of an exclusion below an
 *                                  ancestor include, and the click deselects
 *                                  the branch by adding the node to E.
 *    excluded                   -> delete every covering exclusion (deeper
 *                                  ones stay, so the branch comes back in the
 *                                  same partial state) and add an own include
 *                                  only when nothing covers the node.
 *    unchecked                  -> add the node to I; exclusions below apply
 *                                  again at once. */
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
    case "mixed": {
      // Whitelist flavor (an include strictly below): deselect everything
      // below without inventing coverage-level exclusions.
      let dropped = false;
      for (const i of includes) {
        if (isStrictlyUnder(i, node)) {
          next.includes.delete(i);
          dropped = true;
        }
      }
      if (!dropped) {
        // Carve-out flavor: an ancestor include applies and the node is mixed
        // because of an exclusion below it, so there is nothing of ours to
        // drop. The box reads checked, so the click deselects the whole
        // branch. The deeper exclusion stays stored like every orphan and
        // applies again if the branch is re-included.
        next.exclusions.add(node);
      }
      return next;
    }
    default:
      next.includes.add(node);
      return next;
  }
}

/** Lazy, guarded localStorage access: Node's test environment has no storage
 *  global until a test installs one, and a private browser window can throw
 *  on access or write. */
function storage(): Storage | null {
  try {
    return (globalThis as { localStorage?: Storage }).localStorage ?? null;
  } catch {
    return null;
  }
}

/** Reads the persisted expansion list for a container. Corrupt payloads and
 *  throwing storage read as nothing expanded. */
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

/** Persists the expansion list (newest last), keeping only the last
 *  MAX_EXPANDED entries so the key stays bounded. Never throws. */
export function saveExpanded(containerName: string, paths: readonly string[]): void {
  const s = storage();
  if (!s) return;
  try {
    s.setItem(EXPANDED_KEY_PREFIX + containerName, JSON.stringify(paths.slice(-MAX_EXPANDED)));
  } catch {
    // Expansion is comfort state; a failed write is not the user's problem.
  }
}
