import { describe, expect, it } from "vitest";
import type { PlacementView } from "./api";
import { de, en } from "./i18n";
import {
  addsTargets,
  chipTicked,
  defaultChangeOf,
  defaultSegment,
  defaultView,
  draftView,
  followLine,
  formatList,
  homeOptionLabel,
  lockHint,
  noCopyNow,
  observedLine,
  placementButtons,
  planLines,
  sendToLabel,
  stackNoteText,
  stepForHome,
  stepForLocal,
  stepForNewTarget,
  stepForReset,
  stepForTarget,
  viewHomeLabel,
} from "./placement";
import { blockedText } from "./placementButtons";
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
const DOMAIN_PATH = `Unraid · domain repository · ${LRI}backups/containers${PDI}`;

const opts = placementOptions();
const two = placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })] });
const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox", location: "sftp:u1@box.example:/bv" });
const onBox: PlacementView = placementView({ segment: "offsite-only", repo: "repo-box", repoLabel: "Storagebox", repoKind: "remote", skip: ["*"] });
// B2 has its direct repository, Hetzner does not have one yet.
const direct = placementOptions({
  targets: two.targets,
  sendTo: [sendToOption({ repoId: "repo-b2-direct" }), sendToOption({ targetId: "t-hz", name: "Hetzner" })],
});
const bothDirect = placementOptions({
  targets: two.targets,
  sendTo: [sendToOption({ repoId: "repo-b2-direct" }), sendToOption({ targetId: "t-hz", name: "Hetzner", repoId: "repo-hz-direct" })],
});
const onB2: PlacementView = placementView({
  segment: "offsite-only",
  repo: "repo-b2-direct",
  repoLabel: "B2 direct",
  repoKind: "direct",
  repoTarget: "t-b2",
  skip: ["*"],
});

describe("labels", () => {
  it("names the domain path with the host and keeps its path left to right", () => {
    expect(homeOptionLabel(t, "Unraid", homeOption())).toBe(DOMAIN_PATH);
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

  it("names a direct repository without a name of its own after its target", () => {
    const direct = placementView({ segment: "offsite-only", repo: "repo-b2-direct", repoLabel: "", repoDirectOf: "B2", repoKind: "direct", skip: ["*"] });
    expect(viewHomeLabel(t, "Unraid", direct, placementOptions({ sendTo: [] }))).toBe("B2 · direct");
  });

  it("gives each lock its reason", () => {
    expect(lockHint(t, "no-target", "")).toBe("No off-site target set up");
    expect(lockHint(t, "own-credentials", "")).toBe("Has its own credentials and is already off the premises");
    expect(lockHint(t, "at-target", "")).toBe("Already lies at the target");
    expect(lockHint(t, "home-fixed", "NAS Keller · mounted")).toBe("Fixed since the first backup: NAS Keller · mounted");
  });

  it("says why a button cannot move", () => {
    expect(blockedText(t, { kind: "blocked", reason: "home-fixed" }, "NAS Keller · mounted")).toBe(
      "Fixed since the first backup: NAS Keller · mounted"
    );
    expect(blockedText(t, { kind: "blocked", reason: "remote" }, "Storagebox · remote")).toBe(
      "Has its own credentials and is already off the premises"
    );
    expect(blockedText(t, { kind: "blocked", reason: "last" }, "")).toBe(en["placement.lastButton"]);
  });

  it("joins names the way the language does", () => {
    expect(formatList("en", ["B2", "Hetzner"])).toBe("B2 and Hetzner");
  });
});

describe("buttons", () => {
  it("lights Local for a home on the server and every target a copy goes to", () => {
    expect(placementButtons(placementView(), two)).toEqual({ local: true, home: null, ticked: ["t-b2", "t-hz"] });
    expect(placementButtons(placementView({ skip: ["t-hz"] }), two)).toEqual({ local: true, home: null, ticked: ["t-b2"] });
    expect(placementButtons(placementView({ skip: ["*"] }), two)).toEqual({ local: true, home: null, ticked: [] });
    expect(placementButtons(placementView({ repo: "repo-nas", repoKind: "local" }), two).local).toBe(true);
  });

  it("lights a direct home's own target along with the targets it copies to", () => {
    expect(placementButtons(onB2, direct)).toEqual({ local: false, home: "t-b2", ticked: ["t-b2"] });
    expect(placementButtons({ ...onB2, skip: ["t-b2"] }, direct)).toEqual({ local: false, home: "t-b2", ticked: ["t-b2", "t-hz"] });
  });

  it("finds a direct home's target through the send-to list when the view does not name it", () => {
    expect(placementButtons({ ...onB2, repoTarget: undefined }, direct).home).toBe("t-b2");
  });

  it("lights nothing for a home on a remote repository", () => {
    expect(placementButtons(onBox, placementOptions({ sendTo: [box] }))).toEqual({ local: false, home: null, ticked: [] });
  });
});

describe("steps", () => {
  it("turning Local off makes the first lit target the home and copies to the others", () => {
    expect(stepForLocal(false, placementView(), bothDirect, t, "Unraid")).toEqual({
      kind: "save",
      change: { home: { repo: "repo-b2-direct" }, copies: { skip: ["t-b2"] } },
      confirmHome: "B2 · direct",
      optimistic: {
        repo: "repo-b2-direct",
        repoKind: "direct",
        repoTarget: "t-b2",
        repoLabel: "B2",
        homeFollows: false,
        skip: ["t-b2"],
        copiesFollow: false,
      },
    });
  });

  it("opens the window when the new home's direct repository does not exist yet", () => {
    expect(stepForLocal(false, placementView(), two, t, "Unraid")).toEqual({ kind: "direct", target: two.sendTo[0], skip: ["t-b2"] });
    expect(stepForLocal(false, placementView(), opts, t, "Unraid")).toEqual({ kind: "direct", target: opts.sendTo[0], skip: ["*"] });
  });

  it("keeps Local lit while no target is", () => {
    expect(stepForLocal(false, placementView({ skip: ["*"] }), two, t, "Unraid")).toEqual({ kind: "blocked", reason: "last" });
    expect(stepForLocal(false, placementView(), placementOptions({ targets: [], sendTo: [] }), t, "Unraid")).toEqual({
      kind: "blocked",
      reason: "last",
    });
  });

  it("turning Local on brings the item to the domain path and keeps every lit target as a copy", () => {
    expect(stepForLocal(true, { ...onB2, skip: ["t-b2"] }, direct, t, "Unraid")).toEqual({
      kind: "save",
      change: { home: { repo: "" }, copies: { skip: [] } },
      confirmHome: DOMAIN_PATH,
      optimistic: { repo: "", repoKind: "domain", repoTarget: undefined, repoLabel: "", homeFollows: false, skip: [], copiesFollow: false },
    });
  });

  it("turning Local on goes to the default's home when that is on the server", () => {
    const nasDefault = { ...direct, default: defaultRow({ home: "repo-nas", homeKind: "local" }) };
    expect(stepForLocal(true, onB2, nasDefault, t, "Unraid")).toMatchObject({
      change: { home: { repo: "repo-nas" }, copies: { skip: ["t-hz"] } },
      confirmHome: "NAS Keller · mounted",
      optimistic: { repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller" },
    });
  });

  it("turning Local on chooses the domain path when the default is off the premises", () => {
    const remoteDefault = placementOptions({ default: defaultRow({ home: "repo-box", homeKind: "remote" }), sendTo: [box] });
    expect(stepForLocal(true, onBox, remoteDefault, t, "Unraid")).toMatchObject({
      change: { home: { repo: "" }, copies: { skip: ["*"] } },
      confirmHome: DOMAIN_PATH,
    });
  });

  it("leaves the home alone once the first backup fixed it", () => {
    expect(stepForLocal(true, { ...onB2, locked: true }, direct, t, "Unraid")).toEqual({ kind: "blocked", reason: "home-fixed" });
    expect(stepForLocal(false, placementView({ locked: true }), direct, t, "Unraid")).toEqual({ kind: "blocked", reason: "home-fixed" });
    expect(stepForTarget("t-b2", false, { ...onB2, skip: ["t-b2"], locked: true }, bothDirect, t)).toEqual({
      kind: "blocked",
      reason: "home-fixed",
    });
  });

  it("changes nothing for a button that already shows what was asked", () => {
    expect(stepForLocal(true, placementView(), opts, t, "Unraid")).toEqual({ kind: "none" });
    expect(stepForLocal(false, onB2, direct, t, "Unraid")).toEqual({ kind: "none" });
    expect(stepForTarget("t-b2", true, placementView(), opts, t)).toEqual({ kind: "none" });
    expect(stepForTarget("t-hz", false, onB2, direct, t)).toEqual({ kind: "none" });
  });

  it("a target writes the targets left dark and drops ids of deleted ones", () => {
    expect(stepForTarget("t-hz", false, placementView({ skip: ["t-gone"] }), two, t)).toEqual({
      kind: "save",
      change: { copies: { skip: ["t-hz"] } },
      confirmHome: null,
      optimistic: { skip: ["t-hz"], copiesFollow: false },
    });
    expect(stepForTarget("t-b2", true, placementView({ skip: ["t-b2", "t-gone"] }), two, t)).toMatchObject({ change: { copies: { skip: [] } } });
  });

  it("darkening the last copy leaves the item on Local alone", () => {
    expect(stepForTarget("t-b2", false, placementView(), opts, t)).toMatchObject({ change: { copies: { skip: ["*"] } } });
  });

  it("copies from a direct home to every other lit target and never to its own", () => {
    expect(stepForTarget("t-hz", true, onB2, direct, t)).toMatchObject({ change: { copies: { skip: ["t-b2"] } }, confirmHome: null });
    expect(stepForTarget("t-hz", false, { ...onB2, skip: ["t-b2"] }, direct, t)).toMatchObject({ change: { copies: { skip: ["*"] } } });
  });

  it("darkening the home moves it to the next lit target", () => {
    expect(stepForTarget("t-b2", false, { ...onB2, skip: ["t-b2"] }, bothDirect, t)).toMatchObject({
      change: { home: { repo: "repo-hz-direct" }, copies: { skip: ["*"] } },
      confirmHome: "Hetzner · direct",
      optimistic: { repoTarget: "t-hz" },
    });
    expect(stepForTarget("t-b2", false, { ...onB2, skip: ["t-b2"] }, direct, t)).toEqual({
      kind: "direct",
      target: direct.sendTo[1],
      skip: ["*"],
    });
  });

  it("keeps the home lit while no other button is", () => {
    expect(stepForTarget("t-b2", false, onB2, direct, t)).toEqual({ kind: "blocked", reason: "last" });
  });

  it("still changes the copies of a home the first backup fixed", () => {
    expect(stepForTarget("t-hz", false, { ...onB2, skip: ["t-b2"], locked: true }, bothDirect, t)).toMatchObject({
      change: { copies: { skip: ["*"] } },
    });
    expect(stepForTarget("t-hz", false, placementView({ locked: true }), two, t)).toMatchObject({ change: { copies: { skip: ["t-hz"] } } });
  });

  it("makes the first lit target the home of an item that has neither Local nor a known home", () => {
    const elsewhere = placementView({ repo: "repo-gone-direct", repoKind: "direct", skip: ["*"] });
    expect(stepForTarget("t-b2", true, elsewhere, direct, t)).toMatchObject({
      change: { home: { repo: "repo-b2-direct" }, copies: { skip: ["*"] } },
    });
  });

  it("refuses every target for a home on a remote repository", () => {
    expect(stepForTarget("t-b2", true, onBox, placementOptions({ sendTo: [box] }), t)).toEqual({ kind: "blocked", reason: "remote" });
  });

  it("moves a home that is gone to the target clicked", () => {
    const gone = placementView({ repo: "repo-gone", repoKind: "missing" });
    expect(stepForTarget("t-b2", true, gone, direct, t)).toMatchObject({
      change: { home: { repo: "repo-b2-direct" }, copies: { skip: ["*"] } },
    });
  });

  it("lights a target a destination has just made, which the options do not list yet", () => {
    expect(stepForNewTarget("t-new", placementView({ skip: ["*"] }), opts)).toEqual({
      kind: "save",
      change: { copies: { skip: ["t-b2"] } },
      confirmHome: null,
      optimistic: { skip: ["t-b2"], copiesFollow: false },
    });
    expect(stepForNewTarget("t-new", placementView(), opts)).toMatchObject({ change: { copies: { skip: [] } } });
    expect(stepForNewTarget("t-new", onB2, direct)).toMatchObject({ change: { copies: { skip: ["t-b2", "t-hz"] } } });
    expect(stepForNewTarget("t-new", onBox, placementOptions({ sendTo: [box] }))).toEqual({ kind: "blocked", reason: "remote" });
  });

  it("Stored on asks with the new home, and a chosen home picked again sends nothing", () => {
    expect(stepForHome("repo-nas", placementView(), opts, t, "Unraid")).toMatchObject({
      change: { home: { repo: "repo-nas" } },
      confirmHome: "NAS Keller · mounted",
    });
    expect(stepForHome("", placementView(), opts, t, "Unraid")).toEqual({ kind: "none" });
    expect(stepForHome("", placementView({ homeFollows: true }), opts, t, "Unraid")).toMatchObject({ change: { home: { repo: "" } } });
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

  it("gives a domain without a target the segment the bar shows for it, and keeps its Local lit", () => {
    const none = placementOptions({ targets: [], sendTo: [] });
    expect(defaultSegment(defaultRow(), none)).toBe("local");
    expect(defaultView(defaultRow(), none).segment).toBe("local");
    expect(stepForLocal(false, defaultView(defaultRow(), none), none, t, "Unraid")).toEqual({ kind: "blocked", reason: "last" });
  });

  it("lights a default's direct home by the target it belongs to", () => {
    const row = defaultRow({ home: "repo-b2-direct", homeKind: "direct", homeTarget: "t-b2", skip: ["*"] });
    expect(placementButtons(defaultView(row, direct), direct)).toEqual({ local: false, home: "t-b2", ticked: ["t-b2"] });
  });

  it("takes a step's home and skip as a default's change, and nothing that follows", () => {
    expect(defaultChangeOf(stepForLocal(false, defaultView(defaultRow({ skip: ["t-hz"] }), bothDirect), bothDirect, t, "Unraid"))).toEqual({
      home: "repo-b2-direct",
      skip: ["*"],
    });
    expect(defaultChangeOf(stepForTarget("t-hz", false, defaultView(defaultRow(), two), two, t))).toEqual({ skip: ["t-hz"] });
    expect(defaultChangeOf(stepForReset(placementView()))).toEqual({});
    expect(defaultChangeOf({ kind: "blocked", reason: "last" })).toBeNull();
    expect(defaultChangeOf({ kind: "none" })).toBeNull();
  });

  it("brings a direct or remote home back with every lit target as a copy", () => {
    const onDirect = defaultRow({ home: "repo-b2-direct", homeKind: "direct", homeTarget: "t-b2", skip: ["*"] });
    expect(defaultChangeOf(stepForLocal(true, defaultView(onDirect, direct), direct, t, "Unraid"))).toEqual({ home: "", skip: ["t-hz"] });
    const onRemote = defaultRow({ home: "repo-box", homeKind: "remote", skip: ["*"] });
    const withBox = placementOptions({ sendTo: [box], default: onRemote });
    expect(defaultChangeOf(stepForLocal(true, defaultView(onRemote, withBox), withBox, t, "Unraid"))).toEqual({ home: "", skip: ["*"] });
  });

  it("also brings back a home that points at a deleted repository, not only a remote or direct one", () => {
    const gone = defaultRow({ home: "repo-gone", homeKind: "missing" });
    const withGone = placementOptions({ default: gone });
    expect(defaultChangeOf(stepForLocal(true, defaultView(gone, withGone), withGone, t, "Unraid"))).toEqual({ home: "", skip: ["*"] });
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

  it("names a direct home without a name of its own in the reader's language", () => {
    const plan = placementPlan({ home: "", homeDirectOf: "Primary", targets: [] });
    const tDe = ((k: string) => de[k as keyof typeof de] ?? k) as never;
    expect(planLines(tEn, "en", "Unraid", "files", plan, false)[0].text).toBe("On Primary · direct.");
    expect(planLines(tDe, "de", "Unraid", "files", plan, false)[0].text).toBe("Auf Primary · direkt.");
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
