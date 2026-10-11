// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";

import type { BackupItem, StorageLocation } from "./api";
import {
  addedLocations,
  forgetFirstSteps,
  foundOnServer,
  foundSentence,
  rememberSectionsChosen,
  sectionsChosen,
  stepStates,
} from "./firstSteps";
import { countText, de, en, type TranslationKey } from "./i18n";

const entry = (over: Partial<BackupItem>) => ({ kind: "container", key: "plex", lastBackup: 0, ...over }) as BackupItem;
const place = (over: Partial<StorageLocation>) => ({ id: "path:1", object: "path", enabled: true, ...over }) as StorageLocation;

describe("stepStates", () => {
  it("starts with the sections next and the rest open", () => {
    expect(stepStates({ sections: false, location: false })).toEqual({ sections: "next", location: "open", backup: "open" });
  });

  it("moves on to the storage location once the sections are chosen", () => {
    expect(stepStates({ sections: true, location: false })).toEqual({ sections: "done", location: "next", backup: "open" });
  });

  it("leaves the first backup as the one step to do when both are done", () => {
    expect(stepStates({ sections: true, location: true })).toEqual({ sections: "done", location: "done", backup: "next" });
  });

  it("keeps the sections next when a location was added first", () => {
    expect(stepStates({ sections: false, location: true })).toEqual({ sections: "next", location: "done", backup: "open" });
  });
});

describe("addedLocations", () => {
  it("leaves out the folder every section starts with", () => {
    expect(addedLocations([place({})])).toEqual([]);
  });

  it("counts a destination, a named repository and a target", () => {
    const added = [
      place({ id: "destination:1", object: "destination" }),
      place({ id: "repo:1", object: "repo" }),
      place({ id: "target:1", object: "target" }),
    ];
    expect(addedLocations([place({}), ...added])).toEqual(added);
  });

  it("leaves out a location that is switched off", () => {
    expect(addedLocations([place({ id: "destination:1", object: "destination", enabled: false })])).toEqual([]);
  });
});

describe("foundOnServer", () => {
  const reported = [
    entry({ key: "plex" }),
    entry({ key: "gitea", kindDisabled: true }),
    entry({ key: "bombvault", self: true }),
    entry({ key: "old-wiki", installed: false }),
    entry({ kind: "vm", key: "win11", kindDisabled: true }),
    entry({ kind: "files", key: "set-1" }),
    entry({ kind: "zfs", key: "zfs-1" }),
    entry({ kind: "zfs", key: "zfs-2" }),
    entry({ kind: "config", key: "config" }),
  ];

  it("counts each kind, also while its section is off", () => {
    expect(foundOnServer(reported, "generic")).toEqual({ container: 2, vm: 1, files: 1, zfs: 2, flash: false });
  });

  it("leaves out BombVault's own container and what is gone from the host", () => {
    expect(foundOnServer([entry({ self: true }), entry({ installed: false })], "generic").container).toBe(0);
  });

  it("knows a flash drive on Unraid alone", () => {
    expect(foundOnServer([], "unraid").flash).toBe(true);
    expect(foundOnServer([], "truenas").flash).toBe(false);
    expect(foundOnServer([], "").flash).toBe(false);
  });
});

describe("foundSentence", () => {
  const say = (table: Record<string, string>, lang: string) => (key: TranslationKey, n?: number) =>
    countText(table[key], lang, n);

  it("names every kind that was found, in one sentence", () => {
    const found = { container: 9, vm: 2, files: 3, zfs: 2, flash: true };
    expect(foundSentence(found, say(en, "en"), "en")).toBe(
      "BombVault found 9 containers, 2 VMs, 3 folders, 2 ZFS datasets, and the flash drive."
    );
    expect(foundSentence(found, say(de, "de"), "de")).toBe(
      "BombVault hat 9 Container, 2 VMs, 3 Ordner, 2 ZFS-Datasets und den Flash gefunden."
    );
  });

  it("says nothing about a kind the server does not report", () => {
    const found = { container: 1, vm: 0, files: 0, zfs: 0, flash: false };
    expect(foundSentence(found, say(en, "en"), "en")).toBe("BombVault found 1 container.");
  });

  it("is absent where nothing was found", () => {
    expect(foundSentence({ container: 0, vm: 0, files: 0, zfs: 0, flash: false }, say(en, "en"), "en")).toBeNull();
  });
});

describe("the mark of the first step", () => {
  beforeEach(() => localStorage.clear());

  it("is not set on a new browser", () => {
    expect(sectionsChosen()).toBe(false);
  });

  it("outlives the visit until it is forgotten", () => {
    rememberSectionsChosen();
    expect(sectionsChosen()).toBe(true);
    forgetFirstSteps();
    expect(sectionsChosen()).toBe(false);
  });
});
