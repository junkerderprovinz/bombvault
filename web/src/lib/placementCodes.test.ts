import { describe, expect, it } from "vitest";
import type { PlacementDomain } from "./api";
import { en, type TranslationKey } from "./i18n";
import { placementErrorText, pushSaveWarnings, saveWarningText } from "./placementCodes";

const t = (key: TranslationKey) => en[key];

describe("placementErrorText", () => {
  it("has a sentence for every refusal code of the placement routes", () => {
    const codes = [
      "placement-unreadable", "invalid-placement", "copies-not-allowed", "unknown-target", "stack-rule",
      "copy-rule-taken", "domain-busy", "has-backups", "stale", "repo-invalid", "default-repo-missing",
      "nested-location", "foreign-domain", "mirrored-field", "companion-taken", "exclusion-unsaved",
      "home-uncheckable", "rest-path-too-deep", "repo-wrong-key", "repo-unopened", "repo-gone",
      "destination-holds-backups", "backups-not-there", "last-copies", "nothing-to-erase",
      "move-same-place", "move-section-there", "move-direct-repo", "move-running",
      "own-folder-there",
      "copy-on-own-folder",
      "own-folder-on-copy",
      "own-folder-on-typed-copy",
      "place-unchecked",
      "replication-paused",
      "nothing-to-copy",
      "named-repo-overlap",
      "unprefixed-remote",
      "location-needed",
      "restore-folder-remote",
      "path-absolute",
      "path-outside",
      "own-folder-unreadable",
    ];
    for (const code of codes) {
      const text = placementErrorText(t, "en", { ok: false, error: "server text", code }, "settings.error");
      expect(text, code).not.toBe("server text");
      expect(text, code).not.toBe("");
    }
  });

  it("names the section and the task a busy refusal is about", () => {
    const busy = (busyWith?: string) => ({ ok: false, error: "x", code: "domain-busy", domain: "containers", busyWith });
    expect(placementErrorText(t, "en", busy("backup"), "settings.error")).toBe(
      "Containers is being backed up right now. Try again when the backup has finished."
    );
    expect(placementErrorText(t, "en", busy("check"), "settings.error")).toBe(
      "Containers is being checked right now. Try again when the check has finished."
    );
    expect(placementErrorText(t, "en", busy(), "settings.error")).toBe(en["placementCode.domainBusy"]);
  });

  it("names the defaults that hold a repository", () => {
    const refusal = (defaultDomains: PlacementDomain[]) => ({ ok: false, error: "x", code: "repo-in-use", defaultDomains });
    expect(placementErrorText(t, "en", refusal(["containers", "vms"]), "settings.error")).toBe(
      "The default for Containers and VMs points at this repository. Change the default first."
    );
    expect(placementErrorText(t, "en", refusal([]), "settings.error")).toBe(en["repos.deleteBlocked"]);
  });

  it("counts the items on a target's direct repository before its defaults", () => {
    const refusal = (items: number, defaultDomains: PlacementDomain[]) => ({
      ok: false, error: "x", code: "target-in-use", use: { directRepoId: "d", items, defaultDomains },
    });
    expect(placementErrorText(t, "en", refusal(3, ["files"]), "settings.error")).toBe(
      "Items still back up to the direct repository of this target: 3. Point them somewhere else first."
    );
    expect(placementErrorText(t, "en", refusal(0, ["files"]), "settings.error")).toBe(
      "The default for Folders points at the direct repository of this target. Change the default first."
    );
  });

  it("names the target a direct repository goes with", () => {
    const refusal = { ok: false, error: "x", code: "direct-repo", target: { id: "t-b2", name: "B2" } };
    expect(placementErrorText(t, "en", refusal, "settings.error")).toBe(
      "This repository goes with B2. Remove that target instead."
    );
  });

  it("shows the place a nested location runs into", () => {
    const refusal = { ok: false, error: "x", code: "nested-location", at: "s3:http://minio:9000/bv/vms" };
    expect(placementErrorText(t, "en", refusal, "settings.error")).toBe(
      `${en["placementCode.nestedLocation"]} ⁦s3:http://minio:9000/bv/vms⁩`
    );
  });

  it("gives the relative path to enter instead of a full host path", () => {
    const refusal = { ok: false, error: "x", code: "path-absolute", suggest: "user/backups" };
    expect(placementErrorText(t, "en", refusal, "settings.error")).toBe(
      en["placementCode.pathAbsoluteTo"].replace("{path}", "⁦user/backups⁩")
    );
    expect(placementErrorText(t, "en", { ok: false, code: "path-absolute" }, "settings.error")).toBe(en["placementCode.pathAbsolute"]);
  });

  it("falls back to the server's text, then to the given key", () => {
    expect(placementErrorText(t, "en", { ok: false, error: "server text", code: "no-such-code" }, "settings.error")).toBe("server text");
    expect(placementErrorText(t, "en", { ok: false }, "settings.error")).toBe(en["settings.error"]);
  });
});

describe("placementErrorText for deleting at a target", () => {
  it.each([
    ["append-only", "The target is append-only. Nothing here may delete from it."],
    ["removal-grown", "More snapshots now exist only there than were shown. Check the list again."],
    ["name-mismatch", "The typed name does not match."],
    ["home-unreadable", "The item's location could not be read, so nothing was deleted."],
    ["snapshot-missing", "This backup is no longer at the chosen place."],
  ])("translates %s instead of showing the server's sentence", (code, text) => {
    expect(placementErrorText(t, "en", { ok: false, code, error: "server sentence" }, "settings.error")).toBe(text);
  });
});

describe("save warnings", () => {
  it("fill in the target and the count and go out as warnings", () => {
    const w = { code: "direct-retention-lowered" as const, targetId: "t", targetName: "B2", items: 3 };
    expect(saveWarningText(t, w)).toBe("B2 direct now keeps less. Items whose only copy is there: 3.");
    const pushed: [string, string | undefined][] = [];
    pushSaveWarnings((m, s) => pushed.push([m, s]), t, [w]);
    pushSaveWarnings((m, s) => pushed.push([m, s]), t, undefined);
    expect(pushed).toEqual([["B2 direct now keeps less. Items whose only copy is there: 3.", "warn"]]);
  });
});
