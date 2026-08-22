// ---------------------------------------------------------------------------
// keepRegistryAuths — the pure filtering logic behind the Image Cleanup &
// Registries card's own auto-save (GlimStone follow-up round, merge A —
// "no Speichern button, every field auto-saves"). Extracted specifically so
// this decision (which rows survive a save, and whether a row's token
// becomes "stored") is testable without mounting the whole SettingsPage —
// same "pure logic, node environment, no DOM" footing as isRemotePath
// (PathModeSwitch.tsx) and Selector.test.ts's own nextFocusIndex/rovedIndex.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { keepRegistryAuths } from "./Settings";
import type { RegistryAuthEntry } from "../lib/api";

function entry(over: Partial<RegistryAuthEntry> = {}): RegistryAuthEntry {
  return { host: "", username: "", token: "", tokenSet: false, ...over };
}

describe("keepRegistryAuths", () => {
  it("keeps a row with any one field filled in", () => {
    const { auths, rowIds } = keepRegistryAuths([entry({ host: "ghcr.io" })], ["r1"]);
    expect(auths).toHaveLength(1);
    expect(rowIds).toEqual(["r1"]);
  });

  it("drops an entirely blank row (all three fields empty)", () => {
    const { auths, rowIds } = keepRegistryAuths([entry()], ["r1"]);
    expect(auths).toHaveLength(0);
    expect(rowIds).toEqual([]);
  });

  it("whitespace-only fields count as blank, same as empty", () => {
    const { auths } = keepRegistryAuths([entry({ host: "   ", username: "\t" })], ["r1"]);
    expect(auths).toHaveLength(0);
  });

  it("keeps rowIds aligned with the surviving auths after dropping a blank row in the middle", () => {
    const { auths, rowIds } = keepRegistryAuths(
      [entry({ host: "ghcr.io" }), entry(), entry({ host: "docker.io" })],
      ["r1", "r2", "r3"]
    );
    expect(auths.map((a) => a.host)).toEqual(["ghcr.io", "docker.io"]);
    expect(rowIds).toEqual(["r1", "r3"]);
  });

  it("marks tokenSet true once a token has actually been typed", () => {
    const { auths } = keepRegistryAuths([entry({ host: "ghcr.io", token: "shhh" })], ["r1"]);
    expect(auths[0].tokenSet).toBe(true);
  });

  it("keeps tokenSet true when it was already stored, even with a blank token field (blank-on-save keeps the stored one)", () => {
    const { auths } = keepRegistryAuths([entry({ host: "ghcr.io", token: "", tokenSet: true })], ["r1"]);
    expect(auths[0].tokenSet).toBe(true);
  });

  it("leaves tokenSet false for a row with no token and none stored", () => {
    const { auths } = keepRegistryAuths([entry({ host: "ghcr.io" })], ["r1"]);
    expect(auths[0].tokenSet).toBe(false);
  });

  it("an empty list in, empty list out", () => {
    expect(keepRegistryAuths([], [])).toEqual({ auths: [], rowIds: [] });
  });

  it("does not mutate the input array/entries", () => {
    const original = [entry({ host: "ghcr.io", token: "shhh" })];
    const snapshot = JSON.parse(JSON.stringify(original));
    keepRegistryAuths(original, ["r1"]);
    expect(original).toEqual(snapshot);
  });
});
