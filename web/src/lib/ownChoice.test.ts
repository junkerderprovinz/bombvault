// Which entries count as backed up somewhere other than their section, and
// how the places they use are named.
import { describe, expect, it } from "vitest";
import type { DefaultRow, PlacementOptions, PlacementView } from "./api";
import { en, type TranslationKey } from "./i18n";
import { hasOwnChoice, placeNames } from "./ownChoice";

const t = (key: TranslationKey) => en[key];

const ROW: DefaultRow = {
  domain: "containers",
  exists: true,
  home: "",
  homeKind: "domain",
  homeOff: false,
  skip: [],
  paused: false,
  confirmedAt: 1,
  counts: { follow: 0, own: 0, open: 0, chosenNoRun: 0 },
  unreadable: false,
};

const OPTIONS: PlacementOptions = {
  domain: "containers",
  unreadable: false,
  paused: false,
  homes: [{ id: "", name: "", location: "/mnt/user/backups", kind: "domain", scheme: "" }],
  targets: [
    { id: "t-box", name: "Storage Box", enabled: true, primary: false, appendOnly: false, hint: "" },
    { id: "t-b2", name: "B2", enabled: true, primary: false, appendOnly: false, hint: "" },
  ],
  sendTo: [],
  segmentLocks: {},
  default: ROW,
};

function view(over: Partial<PlacementView> = {}): PlacementView {
  return {
    segment: "local-offsite",
    repo: "",
    repoLabel: "",
    repoKind: "domain",
    repoOff: false,
    homeFollows: true,
    copiesFollow: true,
    skip: [],
    locked: false,
    lockReason: "",
    segmentLocks: {},
    paused: false,
    unreadable: false,
    ...over,
  };
}

describe("hasOwnChoice", () => {
  it("is false for an entry whose first backup fixed it where its section is stored", () => {
    expect(hasOwnChoice(view({ homeFollows: false, locked: true, lockReason: "first-backup" }), ROW)).toBe(false);
  });

  it("is true for an entry that copies by a rule of its own", () => {
    expect(hasOwnChoice(view({ copiesFollow: false, skip: ["*"] }), ROW)).toBe(true);
  });

  it("is true for an entry stored somewhere else than its section", () => {
    expect(hasOwnChoice(view({ homeFollows: false, repo: "archive", repoKind: "local" }), ROW)).toBe(true);
  });

  it("is false for an entry whose placement could not be read", () => {
    expect(hasOwnChoice(view({ unreadable: true, copiesFollow: false }), ROW)).toBe(false);
  });
});

describe("placeNames", () => {
  it("names the local place first and then every copy", () => {
    expect(placeNames(t, view(), OPTIONS)).toEqual(["Local", "Storage Box", "B2"]);
    expect(placeNames(t, view({ skip: ["t-b2"] }), OPTIONS)).toEqual(["Local", "Storage Box"]);
    expect(placeNames(t, view({ skip: ["*"] }), OPTIONS)).toEqual(["Local"]);
  });

  it("names a repository by its own name", () => {
    expect(placeNames(t, view({ repo: "archive", repoKind: "local", repoLabel: "Archiv", skip: ["*"] }), OPTIONS)).toEqual(["Archiv"]);
  });
});
