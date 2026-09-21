import { describe, expect, it } from "vitest";
import type { Snapshot } from "../lib/api";
import { displayTags } from "./RestorePanel";

describe("displayTags", () => {
  it("shows neither the owner nor the markers BombVault sets itself", () => {
    const snap: Snapshot = {
      id: "a1",
      time: "2026-09-19T02:00:00Z",
      paths: ["/data"],
      tags: ["container:web", "p1", "bv:direct", "before-upgrade"],
      hostname: "tower",
    };
    expect(displayTags(snap, "web")).toEqual(["before-upgrade"]);
  });
});
