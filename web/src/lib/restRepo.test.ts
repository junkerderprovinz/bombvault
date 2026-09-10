// The one character that cost issue #194 its reporter three rounds of
// screenshots: a credential set signing in as "bombvault_containers" against a
// URL beginning "bombvault-containers".
//
// These checks pin the comparison to the case it can prove, and keep it quiet
// everywhere it would be guessing. A hint that fires when nothing is wrong is
// worse than none: it sends somebody off to rename a value that was already
// right.
import { expect, it } from "vitest";
import { restPathUserMismatch, restRepoUserSegment } from "./restRepo";

it("reads the user segment only when a repository follows it", () => {
  expect(restRepoUserSegment("rest:http://box:8000/tower/containers")).toBe("tower");
  expect(restRepoUserSegment("rest:https://box:8000/tower/containers/")).toBe("tower");
  expect(restRepoUserSegment("rest:http://user:pw@box:8000/tower/flash")).toBe("tower");
  expect(restRepoUserSegment("  rest:http://box:8000/tower/containers  ")).toBe("tower");
  // One segment is an ordinary path on a server WITHOUT --private-repos, where
  // a 401 means a wrong password and this hint would be a wrong steer.
  expect(restRepoUserSegment("rest:http://box:8000/containers")).toBe("");
  expect(restRepoUserSegment("rest:http://box:8000/")).toBe("");
  expect(restRepoUserSegment("rest:http://box:8000")).toBe("");
  // Other backends carry no htpasswd user at all.
  expect(restRepoUserSegment("s3:s3.amazonaws.com/bucket/containers")).toBe("");
  expect(restRepoUserSegment("sftp:box:/mnt/user/backups/containers")).toBe("");
  expect(restRepoUserSegment("/mnt/user/backups/containers")).toBe("");
  expect(restRepoUserSegment("rest:not a url at all")).toBe("");
  expect(restRepoUserSegment("")).toBe("");
});

it("names both words when they disagree", () => {
  const hit = restPathUserMismatch(
    "rest:http://192.168.100.29:8000/bombvault-containers/containers",
    "bombvault_containers",
  );
  expect(hit).toEqual({ segment: "bombvault-containers", user: "bombvault_containers" });
});

it("says nothing when there is nothing to say", () => {
  const url = "rest:http://box:8000/bombvault-containers/containers";
  // The happy case, and the same word with stray whitespace around it.
  expect(restPathUserMismatch(url, "bombvault-containers")).toBeNull();
  expect(restPathUserMismatch(url, "  bombvault-containers  ")).toBeNull();
  // Nothing to compare against: no credentials filled in yet.
  expect(restPathUserMismatch(url, "")).toBeNull();
  // Nothing to compare with: not a rest repository, or no user segment.
  expect(restPathUserMismatch("s3:s3.amazonaws.com/bucket/x", "tower")).toBeNull();
  expect(restPathUserMismatch("rest:http://box:8000/containers", "tower")).toBeNull();
});
