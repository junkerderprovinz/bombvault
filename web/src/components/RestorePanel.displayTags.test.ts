import { describe, expect, it } from "vitest";
import { displayTags } from "./RestorePanel";

describe("displayTags", () => {
  it("shows neither the owner nor the markers BombVault sets itself", () => {
    const tags = ["container:web", "p1", "bv:direct", "before-upgrade"];
    expect(displayTags(tags, "web")).toEqual(["before-upgrade"]);
  });
});
