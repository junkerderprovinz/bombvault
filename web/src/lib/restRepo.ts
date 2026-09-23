// A rest-server started with --private-repos, as every recipe the wizard prints
// is, gives each htpasswd user only the tree under its own name: the first path
// segment of the URL is the user. Get that word wrong and the server answers
// 401, the same as for a wrong password. The server-side check
// (internal/api/rest_auth_user.go) reports it after a connection test; this
// shows both words side by side while the field is still being filled in.

/**
 * The first path segment of a `rest:` repository URL, but only when a second
 * segment follows it. `rest://box:8000/containers` is an ordinary repository
 * path on a server without --private-repos, and calling that a mismatch would
 * send somebody off to rename something already right. Returns "" for anything
 * else, including a non-rest backend.
 */
export function restRepoUserSegment(repo: string): string {
  const body = repo.trim().replace(/^rest:/i, "");
  if (body === repo.trim()) return "";
  let path: string;
  try {
    path = new URL(body).pathname;
  } catch {
    return "";
  }
  const segments = path.split("/").filter(Boolean);
  if (segments.length < 2) return "";
  return decodeURIComponent(segments[0] ?? "");
}

/**
 * The two words that disagree, or null when there is nothing to say: a
 * non-rest URL, a URL with no user segment, no username to compare against, or
 * the happy case where they match.
 */
export function restPathUserMismatch(
  repo: string,
  // Undefined when no REST credentials were filled in yet or the row is still
  // loading; with nothing to compare there is nothing to say.
  user: string | undefined,
): { segment: string; user: string } | null {
  const segment = restRepoUserSegment(repo);
  const trimmed = (user ?? "").trim();
  if (!segment || !trimmed || segment === trimmed) return null;
  return { segment, user: trimmed };
}
