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
import { keepRegistryAuths, markRegistryTokensStored } from "./Settings";
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

// ---------------------------------------------------------------------------
// markRegistryTokensStored — what stays on SCREEN after a registry save, as
// opposed to what keepRegistryAuths sends to the server.
//
// The two used to be one function, and that was correct while the card had a
// Save button: clicking it meant "I am finished", so dropping the blank rows and
// re-rendering the list from the trimmed result were the same act. Once every
// keystroke arms an 800ms auto-save, they stopped being the same act — the trim
// then ran mid-interaction and deleted the row the user had added seconds
// earlier and was about to fill in, because they had gone back to fix a typo two
// rows up first. A blank row is worth nothing to the server and everything to
// the person typing into it.
// ---------------------------------------------------------------------------
describe("markRegistryTokensStored", () => {
  it("keeps a blank row, which is exactly what keepRegistryAuths drops", () => {
    const rows = [entry({ host: "ghcr.io" }), entry()];
    expect(markRegistryTokensStored(rows)).toHaveLength(2);
    expect(keepRegistryAuths(rows, ["r1", "r2"]).auths).toHaveLength(1);
  });

  it("keeps every row in its original order, so the row ids stay index-aligned", () => {
    const rows = [entry({ host: "a" }), entry(), entry({ host: "c" })];
    expect(markRegistryTokensStored(rows).map((a) => a.host)).toEqual(["a", "", "c"]);
  });

  it("marks a freshly typed token as stored, so the field shows its kept-placeholder", () => {
    expect(markRegistryTokensStored([entry({ host: "ghcr.io", token: "shhh" })])[0].tokenSet).toBe(true);
  });

  it("keeps an already-stored token stored when the field is left blank", () => {
    expect(markRegistryTokensStored([entry({ host: "ghcr.io", tokenSet: true })])[0].tokenSet).toBe(true);
  });

  it("leaves tokenSet false for a row with no token and none stored", () => {
    expect(markRegistryTokensStored([entry({ host: "ghcr.io" })])[0].tokenSet).toBe(false);
  });

  it("does not mutate the input array/entries", () => {
    const original = [entry({ host: "ghcr.io", token: "shhh" }), entry()];
    const snapshot = JSON.parse(JSON.stringify(original));
    markRegistryTokensStored(original);
    expect(original).toEqual(snapshot);
  });
});
