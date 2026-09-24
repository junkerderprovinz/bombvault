import { describe, expect, it } from "vitest";
import type { PlacementView } from "./api";
import { en } from "./i18n";
import {
  addsTargets,
  chipTicked,
  defaultSegment,
  defaultView,
  draftView,
  followLine,
  formatList,
  homeOptionLabel,
  lastChipLocked,
  lockedSegments,
  lockHint,
  noCopyNow,
  observedLine,
  planLines,
  segmentItems,
  sendToLabel,
  stackNoteText,
  stepForChip,
  stepForDefaultSegment,
  stepForHome,
  stepForReset,
  stepForSegment,
  stepForSendTo,
  viewHomeLabel,
} from "./placement";
import {
  defaultRow,
  homeOption,
  observedPlace,
  placementObserved,
  placementOptions,
  placementPlan,
  placementView,
  sendToOption,
  stackNote,
  targetOption,
} from "./placement.testsupport";
import { formatTs } from "./reltime";

const t = ((key: string) => en[key as keyof typeof en] ?? key) as never;
const LRI = "⁦";
const PDI = "⁩";

const opts = placementOptions();
const two = placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })] });
const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox", location: "sftp:u1@box.example:/bv" });
const onBox: PlacementView = placementView({ segment: "offsite-only", repo: "repo-box", repoLabel: "Storagebox", repoKind: "remote", skip: ["*"] });

describe("labels", () => {
  it("names the domain path with the host and keeps its path left to right", () => {
    expect(homeOptionLabel(t, "Unraid", homeOption())).toBe(`Unraid · domain repository · ${LRI}backups/containers${PDI}`);
    expect(homeOptionLabel(t, "Unraid", homeOption({ kind: "domain-remote", scheme: "s3", location: "s3:https://s3.example.com/c" }))).toBe(
      "Domain repository · remote · s3"
    );
    expect(homeOptionLabel(t, "Unraid", homeOption({ id: "repo-nas", name: "NAS Keller", kind: "local" }))).toBe("NAS Keller · mounted");
  });

  it("marks a direct repository that does not exist yet", () => {
    expect(sendToLabel(t, sendToOption())).toBe("B2 · direct · created when first chosen");
    expect(sendToLabel(t, sendToOption({ repoId: "repo-direct" }))).toBe("B2 · direct");
    expect(sendToLabel(t, box)).toBe("Storagebox · remote");
  });

  it("names a stored home from the lists, and marks one that is off or unknown", () => {
    expect(viewHomeLabel(t, "Unraid", placementView({ repo: "repo-nas", repoKind: "local" }), opts)).toBe("NAS Keller · mounted");
    expect(viewHomeLabel(t, "Unraid", placementView({ repo: "repo-cold", repoLabel: "Cold", repoKind: "local", repoOff: true }), opts)).toBe("Cold (off)");
    expect(viewHomeLabel(t, "Unraid", placementView({ repo: "abc", repoLabel: "abc", repoKind: "missing" }), opts)).toBe("abc (unknown)");
  });

  // Send to lists only the targets that are switched on, and a direct
  // repository goes on taking backups while its target is off.
  it("names a direct repository whose target the lists leave out by its own name", () => {
    const direct = placementView({ segment: "offsite-only", repo: "repo-b2-direct", repoLabel: "B2 direct", repoKind: "direct", skip: ["*"] });
    expect(viewHomeLabel(t, "Unraid", direct, placementOptions({ sendTo: [] }))).toBe("B2 direct");
  });

  it("gives each locked segment its reason", () => {
    const items = segmentItems(t, { "offsite-only": "home-fixed" }, "NAS Keller · mounted");
    expect(items.map((i) => [i.id, i.label, i.disabled, i.title])).toEqual([
      ["local", "Local", false, undefined],
      ["local-offsite", "Local + off-site", false, undefined],
      ["offsite-only", "Off-site only", true, "Fixed since the first backup: NAS Keller · mounted"],
    ]);
    expect(lockHint(t, "no-target", "")).toBe("No off-site target set up");
    expect(lockHint(t, "own-credentials", "")).toBe("Has its own credentials and is already off the premises");
    expect(lockHint(t, "at-target", "")).toBe("Already lies at the target");
  });

  it("joins names the way the language does", () => {
    expect(formatList("en", ["B2", "Hetzner"])).toBe("B2 and Hetzner");
  });
});

describe("steps", () => {
  it("Local writes only the copies of an item on a copy source", () => {
    expect(stepForSegment("local", placementView(), opts, t, "Unraid")).toEqual({
      kind: "save",
      change: { copies: { skip: ["*"] } },
      confirmHome: null,
      optimistic: { segment: "local", skip: ["*"], copiesFollow: false },
    });
  });

  it("Local brings an item off a remote repository back to a default that is a copy source", () => {
    const nasDefault = placementOptions({ default: defaultRow({ home: "repo-nas", homeKind: "local" }), sendTo: [box] });
    expect(stepForSegment("local", onBox, nasDefault, t, "Unraid")).toMatchObject({
      kind: "save",
      change: { home: { follow: true }, copies: { skip: ["*"] } },
      confirmHome: "NAS Keller · mounted",
      optimistic: { repo: "repo-nas", repoKind: "local", homeFollows: true, segment: "local" },
    });
  });

  it("Local + off-site chooses the domain path when the default is not a copy source", () => {
    const remoteDefault = placementOptions({ default: defaultRow({ home: "repo-box", homeKind: "remote" }), sendTo: [box] });
    expect(stepForSegment("local-offsite", onBox, remoteDefault, t, "Unraid")).toMatchObject({
      change: { home: { repo: "" }, copies: { skip: [] } },
      confirmHome: `Unraid · domain repository · ${LRI}backups/containers${PDI}`,
    });
  });

  it("leaves the home alone once the first backup fixed it", () => {
    const step = stepForSegment("local", { ...onBox, locked: true }, placementOptions({ sendTo: [box] }), t, "Unraid");
    expect(step).toEqual({
      kind: "save",
      change: { copies: { skip: ["*"] } },
      confirmHome: null,
      optimistic: { segment: "local", skip: ["*"], copiesFollow: false },
    });
  });

  it("Off-site only opens the window while the direct repository does not exist", () => {
    expect(stepForSegment("offsite-only", placementView(), opts, t, "Unraid")).toEqual({ kind: "direct", target: opts.sendTo[0] });
  });

  it("Off-site only sends to the first place and takes no copies", () => {
    expect(stepForSegment("offsite-only", placementView(), placementOptions({ sendTo: [box] }), t, "Unraid")).toMatchObject({
      change: { home: { repo: "repo-box" }, copies: { skip: ["*"] } },
      confirmHome: "Storagebox · remote",
    });
  });

  it("locks Off-site only with nothing to send to, and a click that slips through changes nothing", () => {
    const empty = placementOptions({ sendTo: [] });
    expect(lockedSegments(placementView(), empty)).toEqual({ "offsite-only": "no-target" });
    expect(lockedSegments(placementView(), opts)).toEqual({});
    expect(stepForSegment("offsite-only", placementView(), empty, t, "Unraid")).toEqual({ kind: "none" });
    expect(stepForDefaultSegment("offsite-only", defaultRow(), empty)).toEqual({ kind: "none" });
  });

  it("Stored on asks with the new home, and a chosen home picked again sends nothing", () => {
    expect(stepForHome("repo-nas", placementView(), opts, t, "Unraid")).toMatchObject({
      change: { home: { repo: "repo-nas" } },
      confirmHome: "NAS Keller · mounted",
    });
    expect(stepForHome("", placementView(), opts, t, "Unraid")).toEqual({ kind: "none" });
    expect(stepForHome("", placementView({ homeFollows: true }), opts, t, "Unraid")).toMatchObject({ change: { home: { repo: "" } } });
  });

  it("Send to opens the window for a direct repository that does not exist yet", () => {
    expect(stepForSendTo(sendToOption(), onBox, t)).toEqual({ kind: "direct", target: sendToOption() });
    expect(stepForSendTo(box, onBox, t)).toEqual({ kind: "none" });
  });

  it("a chip writes the unticked targets and drops ids of deleted ones", () => {
    expect(stepForChip("t-hz", false, placementView({ skip: ["t-gone"] }), two)).toMatchObject({
      change: { copies: { skip: ["t-hz"] } },
      confirmHome: null,
    });
    expect(stepForChip("t-b2", true, placementView({ skip: ["t-b2", "t-gone"] }), two)).toMatchObject({ change: { copies: { skip: [] } } });
  });

  it("keeps the last ticked enabled target, since Local is how to copy nowhere", () => {
    expect(stepForChip("t-b2", false, placementView(), opts)).toEqual({ kind: "none" });
    expect(lastChipLocked(placementView(), opts, "t-b2")).toBe(true);
    expect(lastChipLocked(placementView(), two, "t-b2")).toBe(false);
  });

  it("resets both axes before the first backup and only the copies after it", () => {
    expect(stepForReset(placementView())).toMatchObject({ change: { home: { follow: true }, copies: { follow: true } } });
    expect(stepForReset(placementView({ locked: true }))).toEqual({
      kind: "save",
      change: { copies: { follow: true } },
      confirmHome: null,
      optimistic: { copiesFollow: true },
    });
  });

  it("knows which changes can hand an item to another target", () => {
    expect(addsTargets(placementView({ skip: ["t-b2"] }), { copies: { skip: [] } })).toBe(true);
    expect(addsTargets(placementView({ skip: ["*"] }), { copies: { skip: ["t-b2"] } })).toBe(true);
    expect(addsTargets(placementView(), { copies: { skip: ["t-b2"] } })).toBe(false);
    expect(addsTargets(placementView(), { copies: { skip: ["*"] } })).toBe(false);
    expect(addsTargets(placementView(), { copies: { follow: true } })).toBe(true);
    expect(addsTargets(placementView(), { home: { repo: "repo-nas" } })).toBe(false);
  });
});

describe("chips and lines", () => {
  it("shows a switched-off target with its stored tick and warns when nothing is copied", () => {
    const off = placementOptions({ targets: [targetOption({ enabled: false })] });
    expect(chipTicked(placementView(), "t-b2")).toBe(true);
    expect(chipTicked(placementView({ skip: ["*"] }), "t-b2")).toBe(false);
    expect(noCopyNow(placementView(), off)).toEqual(["B2"]);
    expect(noCopyNow(placementView({ segment: "local", skip: ["*"] }), off)).toEqual([]);
    expect(noCopyNow(placementView(), opts)).toEqual([]);
  });

  it("says what the card follows", () => {
    expect(followLine(placementView({ homeFollows: true }))).toBe("follows");
    expect(followLine(placementView({ homeFollows: true, copiesFollow: false }))).toBe("own-copies");
    expect(followLine(placementView())).toBe("home-set");
    expect(followLine(placementView({ locked: true }))).toBe("copies-follow");
    expect(followLine(placementView({ locked: true, copiesFollow: false }))).toBe("own-copies");
  });
});

describe("defaults", () => {
  it("reads the segment of a default", () => {
    expect(defaultSegment(defaultRow(), opts)).toBe("local-offsite");
    expect(defaultSegment(defaultRow({ skip: ["*"] }), opts)).toBe("local");
    expect(defaultSegment(defaultRow({ home: "repo-direct", homeKind: "direct" }), opts)).toBe("offsite-only");
  });

  it("gives a domain without a target the segment the bar shows for it", () => {
    const none = placementOptions({ targets: [], sendTo: [] });
    expect(defaultSegment(defaultRow(), none)).toBe("local");
    expect(defaultView(defaultRow(), none).segment).toBe("local");
    expect(stepForDefaultSegment("local", defaultRow(), none)).toEqual({ kind: "none" });
  });

  it("changes only the home of a default for Off-site only", () => {
    expect(stepForDefaultSegment("offsite-only", defaultRow({ skip: ["t-b2"] }), placementOptions({ sendTo: [box] }))).toEqual({
      kind: "change",
      change: { home: "repo-box" },
    });
    expect(stepForDefaultSegment("offsite-only", defaultRow(), opts)).toEqual({ kind: "direct", target: sendToOption() });
  });

  it("sets the skip for Local and Local + off-site and brings a remote home back", () => {
    expect(stepForDefaultSegment("local", defaultRow(), opts)).toEqual({ kind: "change", change: { skip: ["*"] } });
    expect(stepForDefaultSegment("local-offsite", defaultRow({ home: "repo-box", homeKind: "remote", skip: ["*"] }), opts)).toEqual({
      kind: "change",
      change: { skip: [], home: "" },
    });
  });

  it("also brings back a home that points at a deleted repository, not only a remote or direct one", () => {
    expect(stepForDefaultSegment("local", defaultRow({ home: "repo-gone", homeKind: "missing" }), opts)).toEqual({
      kind: "change",
      change: { skip: ["*"], home: "" },
    });
  });

  it("starts a draft at the default, following both axes", () => {
    const view = draftView(placementOptions({ default: defaultRow({ home: "repo-nas", homeKind: "local" }) }));
    expect(view).toMatchObject({ repo: "repo-nas", repoLabel: "NAS Keller", homeFollows: true, copiesFollow: true, segment: "local-offsite" });
  });
});

const tEn = ((k: string) => en[k as keyof typeof en] ?? k) as never;

describe("planLines", () => {
  it("names the home and the targets in two sentences", () => {
    expect(
      planLines(tEn, "en", "Unraid", "vms", placementPlan({ home: "NAS Keller", targets: ["B2", "Hetzner"] }), false)
    ).toEqual([
      { text: "On NAS Keller.", tone: "normal" },
      { text: "Copied to B2 and Hetzner.", tone: "normal" },
    ]);
  });

  it("puts the server's name in for the domain path and warns when nothing leaves", () => {
    expect(planLines(tEn, "en", "Unraid", "vms", placementPlan({ targets: [], warn: true, noCopy: true }), false)).toEqual([
      { text: "On Unraid.", tone: "warn" },
      { text: "No copy off the premises.", tone: "warn" },
    ]);
  });

  it("says when the domain has no off-site copy set up at all", () => {
    const lines = planLines(tEn, "en", "Unraid", "vms", placementPlan({ targets: [], warn: true, noCopy: true }), true);
    expect(lines[1]).toEqual({ text: "No off-site copy is set up for VMs.", tone: "warn" });
  });

  it("tells an open item's default from its first-backup question", () => {
    expect(
      planLines(tEn, "en", "Unraid", "vms", placementPlan({ kind: "default-home", home: "NAS Keller" }), false)[0].text
    ).toBe("On NAS Keller (the default, applies from the first backup).");
    expect(planLines(tEn, "en", "Unraid", "vms", placementPlan({ kind: "stays-domain" }), false)[0].text).toBe(
      "Stays on Unraid: backups of this name are there."
    );
    expect(
      planLines(tEn, "en", "Unraid", "vms", placementPlan({ kind: "decides-at-first-backup", targets: [] }), false)
    ).toEqual([{ text: "The location is decided at the first backup.", tone: "normal" }]);
  });

  it("says only the pause while the domain is paused", () => {
    expect(
      planLines(tEn, "en", "Unraid", "vms", placementPlan({ kind: "paused", targets: [], warn: true }), false)
    ).toEqual([{ text: "Off-site paused until the default is confirmed.", tone: "warn" }]);
  });

  it("names a default that cannot be used", () => {
    const off = placementPlan({ kind: "not-backed-up", home: "NAS Keller", targets: [], warn: true, reason: "default-off" });
    expect(planLines(tEn, "en", "Unraid", "vms", off, false)).toEqual([
      { text: "Not backed up: the default points at NAS Keller, which is switched off.", tone: "warn" },
    ]);
    const missing = placementPlan({ kind: "not-backed-up", targets: [], warn: true, reason: "default-missing" });
    expect(planLines(tEn, "en", "Unraid", "vms", missing, false)[0].text).toBe(
      "Not backed up: the default points at a repository that no longer exists."
    );
  });
});

describe("observedLine", () => {
  it("reads sites, each target and the 3-2-1 mark", () => {
    expect(observedLine(tEn, placementObserved())).toEqual([
      { text: "At 2 sites", tone: "normal" },
      { text: `B2 last seen ${formatTs(1_758_170_400)}`, tone: "normal" },
      { text: "3-2-1 met", tone: "normal" },
    ]);
  });

  it("says one site in words of its own", () => {
    expect(observedLine(tEn, placementObserved({ sites: 1, places: [], rule321: "one-copy", tone: "warn" }))).toEqual([
      { text: "At one site", tone: "normal" },
      { text: "3-2-1 not met: one backup", tone: "warn" },
    ]);
  });

  it("warns about a target that could not be reached", () => {
    const lines = observedLine(
      tEn,
      placementObserved({
        places: [observedPlace({ state: "unreachable", since: 1_758_200_000, counts: false })],
        rule321: "one-copy",
      })
    );
    expect(lines[1]).toEqual({
      text: `B2 unreachable since ${formatTs(1_758_200_000)}, last seen ${formatTs(1_758_170_400)}`,
      tone: "warn",
    });
  });

  it("dims a target whose state is unknown and marks 3-2-1 unconfirmed", () => {
    const lines = observedLine(
      tEn,
      placementObserved({
        places: [observedPlace({ state: "unknown", since: 1_758_000_000, stale: true, counts: false })],
        rule321: "unconfirmed",
        tone: "unconfirmed",
      })
    );
    expect(lines.slice(1)).toEqual([
      { text: `B2: state unknown since ${formatTs(1_758_000_000)}`, tone: "muted" },
      { text: "3-2-1 unconfirmed", tone: "unconfirmed" },
    ]);
  });

  it("dates a copy that is too old and names a switched-off target", () => {
    const lines = observedLine(
      tEn,
      placementObserved({
        places: [
          observedPlace({ state: "old-copy", latest: 1_757_000_000, stale: true, counts: false }),
          observedPlace({ place: "offsite:t-hz", label: "Hetzner", state: "off", counts: false }),
        ],
      })
    );
    expect(lines.slice(1, 3)).toEqual([
      { text: `B2: latest copy from ${new Date(1_757_000_000 * 1000).toLocaleDateString()}`, tone: "muted" },
      { text: "Hetzner (off)", tone: "muted" },
    ]);
  });

  it("says a ticked target holds no copy yet and dates nothing", () => {
    const lines = observedLine(
      tEn,
      placementObserved({
        places: [observedPlace({ state: "no-copy", count: 0, latest: 0, seenAt: 0, counts: false })],
        sites: 1,
        rule321: "one-copy",
        tone: "warn",
      })
    );
    expect(lines[1]).toEqual({ text: "B2: no copy yet", tone: "muted" });
  });

  it("says a target has not been listed yet instead of standing there as a bare name", () => {
    const lines = observedLine(
      tEn,
      placementObserved({
        places: [observedPlace({ state: "unknown", since: 0, stale: true, counts: false })],
        sites: 1,
        rule321: "one-copy",
        tone: "warn",
      })
    );
    expect(lines[1]).toEqual({ text: "B2: not listed yet", tone: "muted" });
  });

  it("says no backup yet before the first one", () => {
    expect(observedLine(tEn, placementObserved({ noBackup: true }))).toEqual([
      { text: "No backup yet.", tone: "muted" },
    ]);
  });
});

describe("stackNoteText", () => {
  it("names the project folder, its home and where it is copied", () => {
    expect(stackNoteText(tEn, "en", "Unraid", stackNote())).toBe(
      "Project folder immich: on Unraid, copied to B2 (follows the containers default)"
    );
  });

  it("says when the project folder is not copied", () => {
    expect(stackNoteText(tEn, "en", "Unraid", stackNote({ targets: [] }))).toBe(
      "Project folder immich: on Unraid, not copied (follows the containers default)"
    );
  });
});
