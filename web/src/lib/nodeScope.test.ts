import { describe, expect, it } from "vitest";
import { currentNode, inNode, nodeTarget, parseRoutes } from "./nodeScope";

const ROUTES = parseRoutes([
  "GET /api/status",
  "GET /api/containers",
  "GET /api/containers/{name}/snapshots",
  "PATCH /api/containers/{name}",
  "POST /api/containers/{name}/backup",
  "POST /api/files/sets/{id}/restore",
]);

describe("node scope", () => {
  it("holds the node only while the call runs", () => {
    expect(currentNode()).toBeNull();
    const seen = inNode("n1", () => currentNode());
    expect(seen).toBe("n1");
    expect(currentNode()).toBeNull();
  });

  it("gives the scope back when the call throws", () => {
    expect(() =>
      inNode("n1", () => {
        throw new Error("boom");
      }),
    ).toThrow("boom");
    expect(currentNode()).toBeNull();
  });

  it("runs a call without a node as it is", () => {
    expect(inNode(null, () => currentNode())).toBeNull();
  });

  it("restores the outer node after a nested call", () => {
    const seen = inNode("outer", () => [inNode("inner", () => currentNode()), currentNode()]);
    expect(seen).toEqual(["inner", "outer"]);
  });
});

describe("nodeTarget", () => {
  it("sends a listed route to the node with its query", () => {
    expect(nodeTarget(ROUTES, "n1", "GET", "/api/containers/plex%20tv/snapshots?source=offsite")).toBe(
      "/api/nodes/n1/api/containers/plex%20tv/snapshots?source=offsite",
    );
    expect(nodeTarget(ROUTES, "n1", "POST", "/api/files/sets/s1/restore")).toBe("/api/nodes/n1/api/files/sets/s1/restore");
  });

  it("leaves a route the node does not take at home", () => {
    expect(nodeTarget(ROUTES, "n1", "GET", "/api/settings")).toBeNull();
    expect(nodeTarget(ROUTES, "n1", "DELETE", "/api/containers/plex")).toBeNull();
    expect(nodeTarget(ROUTES, "n1", "GET", "/api/containers/plex/dbdumps")).toBeNull();
  });

  it("matches the method as well as the path", () => {
    expect(nodeTarget(ROUTES, "n1", "PATCH", "/api/containers/plex")).toBe("/api/nodes/n1/api/containers/plex");
    expect(nodeTarget(ROUTES, "n1", "GET", "/api/containers/plex")).toBeNull();
  });

  it("refuses an empty segment where a route takes a name", () => {
    expect(nodeTarget(ROUTES, "n1", "POST", "/api/containers//backup")).toBeNull();
  });

  it("escapes the node id", () => {
    expect(nodeTarget(ROUTES, "a/b", "GET", "/api/status")).toBe("/api/nodes/a%2Fb/api/status");
  });
});
