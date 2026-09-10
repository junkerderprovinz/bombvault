// The one thing a rest-server URL and its credentials have to agree on (#194).
//
// A server started with --private-repos, which every recipe this wizard prints
// does, hands each htpasswd user only the tree under its own name: the first
// path segment of the URL IS the user. Get that one word wrong and the answer
// is 401, the same 401 a wrong password gives, and nothing in it says which of
// the two you hit.
//
// The reporter of #194 had every other piece right and still could not get a
// backup out: the credential set signed in as "bombvault_containers" while the
// URL began "bombvault-containers". One character, and it survived three rounds
// of screenshots because nothing on screen ever put the two words side by side.
//
// The server says the same thing once a connection test has run
// (internal/api/rest_auth_user.go). This says it while the field is being
// filled in, which is a round earlier.

/**
 * The first path segment of a `rest:` repository URL, but only when a second
 * segment follows it.
 *
 * That condition is the difference between "the wrong name" and "no name", and
 * only the first is safe to state as a fact. `rest://box:8000/containers` is an
 * ordinary repository path on a server running WITHOUT --private-repos, and
 * calling that a mismatch would send somebody off to rename something already
 * right. Returns "" for anything else, including a non-rest backend.
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
  // Undefined on purpose: a credential set that has never had REST credentials
  // filled in carries no username at all, and neither does a reader holding a
  // partially loaded row. Nothing to compare is not an error, it is silence.
  user: string | undefined,
): { segment: string; user: string } | null {
  const segment = restRepoUserSegment(repo);
  const trimmed = (user ?? "").trim();
  if (!segment || !trimmed || segment === trimmed) return null;
  return { segment, user: trimmed };
}
