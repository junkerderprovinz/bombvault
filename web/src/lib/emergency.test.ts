import { describe, expect, it } from "vitest";

import type { EmergencyPlace, EmergencyState } from "./api";
import { lossText, minutesLeft, placeAdvice, placeName, refusalText, sessionKeyInUse, settingsOfPlaces, takenOver } from "./emergency";
import { countText, en, type TranslationKey } from "./i18n";

const t = (key: TranslationKey, n?: number) => (n === undefined ? en[key] : countText(en[key], "en", n));

function place(over: Partial<EmergencyPlace>): EmergencyPlace {
  return { kind: "local", location: "/mnt/user/backups/containers", state: "readable", ...over };
}

function state(over: Partial<EmergencyState>): EmergencyState {
  return { backupKey: "own", undoable: false, ...over };
}

const session = (refusal?: "adopt-same-key" | "adopt-own-backups") => ({
  source: "kit" as const,
  encrypted: true,
  places: [],
  expiresAt: 0,
  cloudSignIn: false,
  rcloneConfig: false,
  refusal,
});

describe("refusalText", () => {
  it("is the sentence of a code this client knows", () => {
    expect(refusalText(t, "kit-sealed", "this recovery kit is sealed with your age key")).toBe(en["emergency.code.kit-sealed"]);
    expect(refusalText(t, "settings-other-key", undefined)).toBe(en["emergency.code.settings-other-key"]);
  });

  it("is what the server said for a code it does not know, and a plain failure without either", () => {
    expect(refusalText(t, "kit-from-the-future", "the kit is from the future")).toBe("the kit is from the future");
    expect(refusalText(t, undefined, undefined)).toBe(en["settings.error"]);
  });
});

describe("placeName", () => {
  it("names the section of a place and whether it is the local or the off-site one", () => {
    expect(placeName(t, place({ domain: "containers" }))).toBe(`${en["nav.containers"]} · ${en["source.local"]}`);
    expect(placeName(t, place({ domain: "vms", kind: "offsite", location: "rest:https://box/vms" }))).toBe(
      `${en["nav.vms"]} · ${en["source.offsite"]}`
    );
  });

  it("names a named repository and a destination by what the old install called them", () => {
    expect(placeName(t, place({ kind: "named", name: "Photos on B2", location: "s3:b2/photos" }))).toBe("Photos on B2");
    expect(placeName(t, place({ kind: "destination", name: "Storage Box", domain: "files", location: "rclone:box:files" }))).toBe(
      `Storage Box · ${en["nav.files"]}`
    );
  });

  it("falls back to the address of a place that was typed", () => {
    expect(placeName(t, place({ location: "backups/old" }))).toBe("backups/old");
  });
});

describe("placeAdvice", () => {
  it("has nothing to say about a place that opened", () => {
    expect(placeAdvice(t, place({}))).toBeUndefined();
  });

  it("names the folder that was looked for, or says that there was none to look for", () => {
    expect(placeAdvice(t, place({ state: "missing", folder: "old/vms" }))).toBe(en["emergency.place.missingDo"].replace("{folder}", "old/vms"));
    expect(placeAdvice(t, place({ state: "missing" }))).toBe(en["emergency.place.missingOutside"]);
  });

  it.each([
    ["wrong-key", "emergency.place.wrong-keyDo"],
    ["unreachable", "emergency.place.unreachableDo"],
    ["sign-in", "emergency.place.sign-inDo"],
  ] as const)("says what to do about a place that answered %s", (answered, key) => {
    expect(placeAdvice(t, place({ state: answered }))).toBe(en[key]);
  });
});

describe("settingsOfPlaces", () => {
  it("points each section at the folder and the address its kit names", () => {
    expect(
      settingsOfPlaces([
        place({ domain: "containers", folder: "old/containers" }),
        place({ domain: "containers", kind: "offsite", location: "rest:https://box/containers" }),
        place({ domain: "config", folder: "old/config" }),
        place({ domain: "zfs", kind: "offsite", location: "s3:b2/zfs", state: "sign-in" }),
      ])
    ).toEqual({
      containersPath: "old/containers",
      containersOffsite: "rest:https://box/containers",
      configPath: "old/config",
      zfsOffsite: "s3:b2/zfs",
    });
  });

  it("leaves out a local place outside the mount and what belongs to no section", () => {
    expect(
      settingsOfPlaces([
        place({ domain: "vms", location: "/mnt/disks/usb/vms" }),
        place({ kind: "named", name: "Photos", location: "s3:b2/photos" }),
        place({ kind: "destination", domain: "files", name: "Box", location: "rclone:box:files" }),
        place({ domain: "someday", folder: "old/someday" }),
      ])
    ).toEqual({});
  });
});

describe("minutesLeft", () => {
  it("counts the minutes that have begun and stops at none", () => {
    expect(minutesLeft(1_000 + 1800, 1_000_000)).toBe(30);
    expect(minutesLeft(1_000 + 61, 1_000_000)).toBe(2);
    expect(minutesLeft(1_000 + 1, 1_000_000)).toBe(1);
    expect(minutesLeft(1_000, 1_000_000)).toBe(0);
    expect(minutesLeft(900, 1_000_000)).toBe(0);
  });
});

describe("the key of an emergency", () => {
  it("is undecided while it is only shown", () => {
    expect(sessionKeyInUse(state({ session: session() }))).toBe(false);
    expect(sessionKeyInUse(state({ session: session("adopt-own-backups") }))).toBe(false);
    expect(sessionKeyInUse(null)).toBe(false);
  });

  it("is taken over once the server backs up under it as an adopted key", () => {
    expect(takenOver(state({ backupKey: "adopted", session: session("adopt-same-key") }))).toBe(true);
  });

  it("is in use without a takeover when APP_KEY was set to it by hand", () => {
    const byHand = state({ session: session("adopt-same-key") });
    expect(sessionKeyInUse(byHand)).toBe(true);
    expect(takenOver(byHand)).toBe(false);
  });

  it("is not taken over by a server that adopted a key earlier and has none shown", () => {
    expect(takenOver(state({ backupKey: "adopted" }))).toBe(false);
  });
});

describe("lossText", () => {
  it("says nothing when the restore brought everything over", () => {
    expect(lossText(t, "en", undefined)).toBeUndefined();
    expect(lossText(t, "en", { count: 0, kinds: {} })).toBeUndefined();
  });

  it("counts what was lost and names its kinds", () => {
    const text = lossText(t, "en", { count: 3, kinds: { cloud: 2, peers: 1, registry: 0 } });
    expect(text).toContain(countText(en["emergency.lost"], "en", 3));
    expect(text).toContain(`${en["emergency.lost.cloud"]} and ${en["emergency.lost.peers"]}`);
    expect(text).not.toContain(en["emergency.lost.registry"]);
  });

  it("counts alone where the server names no kind", () => {
    expect(lossText(t, "en", { count: 1, kinds: {} })).toBe(countText(en["emergency.lost"], "en", 1));
  });
});
