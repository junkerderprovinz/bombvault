import { describe, expect, it } from "vitest";
import { displayTags } from "./RestorePanel";

describe("displayTags", () => {
  it("shows neither the owner nor the markers BombVault sets itself", () => {
    const tags = ["container:web", "p1", "bv:direct", "before-upgrade"];
    expect(displayTags(tags, "web")).toEqual(["before-upgrade"]);
  });

  it("hides the ownership tags of the entry's former names as well", () => {
    const tags = ["container:radarr", "container:radarr-movies", "container:radarr-old", "before-upgrade"];
    expect(displayTags(tags, "radarr", ["radarr-movies", "radarr-old"])).toEqual(["before-upgrade"]);
  });

  it("hides the formerly marker a takeover leaves on older backups", () => {
    const tags = ["container:radarr", "formerly:radarr-movies", "before-upgrade"];
    expect(displayTags(tags, "radarr", ["radarr-movies"])).toEqual(["before-upgrade"]);
  });
});
