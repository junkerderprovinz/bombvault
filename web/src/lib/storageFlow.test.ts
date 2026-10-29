// The lines of the flow picture: which section is backed up where, and what a
// click on each line switches.
import { describe, expect, it } from "vitest";
import type {
  DefaultRow,
  OffsiteDomain,
  PlacementOptions,
  SendToOption,
  StorageLocation,
  StorageLocationSection,
  TargetOption,
} from "./api";
import { defaultView } from "./placement";
import { flowCounts, flowDomains, flowRows, HOME_NOWHERE, type FlowDefault, type FlowLine, type FlowRow } from "./storageFlow";

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(domain: OffsiteDomain, over: Partial<StorageLocationSection> = {}): StorageLocationSection {
  return {
    domain,
    use: "home",
    where: "",
    enabled: true,
    immutable: false,
    retention: KEEP,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

function location(over: Partial<StorageLocation>): StorageLocation {
  return {
    id: "path:local",
    object: "path",
    kind: "local",
    provider: "",
    backend: "local",
    name: "bombvault",
    where: "/mnt/user/backups/bombvault",
    enabled: true,
    offPremises: false,
    sections: [],
    protection: { immutable: false, testable: false },
    capacity: {},
    ...over,
  };
}

const LOCAL = location({ sections: [section("containers"), section("vms"), section("flash")] });
const BOX = location({
  id: "destination:box",
  object: "destination",
  kind: "offsite",
  name: "Storage Box",
  offPremises: true,
  sections: [
    section("containers", { use: "copy", targetId: "t-box" }),
    section("flash", { use: "copy", targetId: "t-box-flash" }),
  ],
});
const B2 = location({ id: "destination:b2", object: "destination", kind: "offsite", name: "B2", offPremises: true });

function row(domain: "containers" | "vms" | "files", over: Partial<DefaultRow> = {}): DefaultRow {
  return {
    domain,
    exists: true,
    home: "",
    homeKind: "domain",
    homeOff: false,
    skip: [],
    paused: false,
    confirmedAt: 1,
    counts: { follow: 0, own: 0, open: 0, chosenNoRun: 0 },
    unreadable: false,
    ...over,
  };
}

function target(id: string, over: Partial<TargetOption> = {}): TargetOption {
  return { id, name: id, enabled: true, primary: false, appendOnly: false, hint: "", ...over };
}

function direct(targetId: string, repoId = ""): SendToOption {
  return { kind: "direct", repoId, targetId, name: targetId, location: "" };
}

function placed(default_: DefaultRow, over: Partial<PlacementOptions> = {}): FlowDefault {
  const options: PlacementOptions = {
    domain: default_.domain,
    unreadable: false,
    paused: false,
    homes: [{ id: "", name: "", location: "/mnt/user/backups/bombvault", kind: "domain", scheme: "" }],
    targets: [],
    sendTo: [],
    segmentLocks: {},
    default: default_,
    ...over,
  };
  return { options, view: defaultView(default_, options) };
}

function rows(locations: StorageLocation[], defaults: Parameters<typeof flowRows>[1] = {}): FlowRow[] {
  return flowRows(locations, defaults);
}

function line(found: FlowRow[], domain: OffsiteDomain, location: StorageLocation): FlowLine {
  const hit = found.find((r) => r.domain === domain)?.lines.find((l) => l.location.id === location.id);
  if (!hit) throw new Error(`no line from ${domain} to ${location.name}`);
  return hit;
}

describe("the sections of the picture", () => {
  it("are the ones whose own folder is in use, in the order of the pages", () => {
    const local = location({ sections: [section("flash"), section("zfs", { enabled: false }), section("containers")] });
    expect(flowDomains([local, BOX])).toEqual(["containers", "flash"]);
  });

  it("count a section whose own folder lies in a storage location, with nothing on this host", () => {
    const dxp = location({
      id: "destination:dxp",
      object: "destination",
      kind: "offsite",
      name: "DXP6800",
      sections: [section("containers"), section("files", { use: "home", targetId: "t-x", repoId: "r-x" })],
    });
    expect(flowDomains([dxp, B2])).toEqual(["containers"]);
  });
});

describe("a section whose own folder lies in a storage location", () => {
  const DXP = location({ id: "destination:dxp", object: "destination", kind: "offsite", name: "DXP6800", sections: [section("containers")] });
  const containers = placed(row("containers"));

  it("draws that location as its home, lit, and offers no copy onto it", () => {
    const found = rows([DXP, B2], { containers });
    const home = line(found, "containers", DXP);
    expect(home.on).toBe(true);
    expect(home.action.kind).not.toBe("destination");
    expect(line(found, "containers", B2).action).toEqual({ kind: "destination", destinationId: "b2" });
  });
});

describe("a section with placement", () => {
  const containers = placed(row("containers"), { targets: [target("t-box")], sendTo: [direct("t-box")] });

  it("reads a line to a destination off the default: on while the target is not skipped", () => {
    const on = line(rows([LOCAL, BOX], { containers }), "containers", BOX);
    expect(on).toMatchObject({ on: true, action: { kind: "default", change: { skip: ["*"] } } });

    const skipped = placed(row("containers", { skip: ["*"] }), { targets: [target("t-box")] });
    const off = line(rows([LOCAL, BOX], { containers: skipped }), "containers", BOX);
    expect(off).toMatchObject({ on: false, action: { kind: "default", change: { skip: [] } } });
  });

  it("offers a destination without a target for the section, and creates the target on the click", () => {
    const fresh = line(rows([LOCAL, BOX, B2], { containers }), "containers", B2);
    expect(fresh.on).toBe(false);
    expect(fresh.action).toEqual({ kind: "destination", destinationId: "b2" });
  });

  it("lets Local go off while nothing else is on, which leaves the section without a place", () => {
    const alone = placed(row("containers", { skip: ["*"] }), { targets: [target("t-box")] });
    const local = line(rows([LOCAL, BOX], { containers: alone }), "containers", LOCAL);
    expect(local).toMatchObject({ on: true, action: { kind: "default", change: { home: HOME_NOWHERE } } });
  });

  it("hands Local over to the first lit target, which writes the backups itself from then on", () => {
    expect(line(rows([LOCAL, BOX], { containers }), "containers", LOCAL).action).toEqual({ kind: "direct", targetId: "t-box", skip: ["*"] });

    const made = placed(row("containers"), { targets: [target("t-box")], sendTo: [direct("t-box", "r-box")] });
    expect(line(rows([LOCAL, BOX], { containers: made }), "containers", LOCAL).action).toEqual({
      kind: "default",
      change: { home: "r-box", skip: ["*"] },
    });
  });

  it("brings Local back with every lit target as a copy, and moves the place on when its target goes off", () => {
    const B2_COPY = { ...B2, sections: [section("containers", { use: "copy", targetId: "t-b2" })] };
    const atBox = placed(row("containers", { home: "r-box", homeKind: "direct", homeTarget: "t-box" }), {
      targets: [target("t-box"), target("t-b2")],
      sendTo: [direct("t-box", "r-box"), direct("t-b2")],
    });
    const found = rows([LOCAL, BOX, B2_COPY], { containers: atBox });

    expect(line(found, "containers", LOCAL)).toMatchObject({ on: false, action: { kind: "default", change: { home: "", skip: [] } } });
    expect(line(found, "containers", BOX)).toMatchObject({ on: true, action: { kind: "direct", targetId: "t-b2", skip: ["*"] } });
  });

  it("marks a section with every line off and offers each line again", () => {
    const none = placed(row("containers", { home: HOME_NOWHERE, homeKind: "none" }), {
      targets: [target("t-box")],
      sendTo: [direct("t-box")],
    });
    const found = rows([LOCAL, BOX], { containers: none });

    expect(found[0].nowhere).toBe(true);
    expect(line(found, "containers", LOCAL)).toMatchObject({ on: false, action: { kind: "default", change: { home: "", skip: ["*"] } } });
    expect(line(found, "containers", BOX)).toMatchObject({ on: false, action: { kind: "direct", targetId: "t-box", skip: ["*"] } });
  });

  it("switches on a target that is switched off together with its line", () => {
    const skipped = placed(row("containers", { skip: ["*"] }), { targets: [target("t-box", { enabled: false })] });
    expect(line(rows([LOCAL, BOX], { containers: skipped }), "containers", BOX)).toMatchObject({
      on: false,
      action: { kind: "default", change: { skip: [] }, enable: "t-box" },
    });

    const ticked = placed(row("containers"), { targets: [target("t-box", { enabled: false })] });
    expect(line(rows([LOCAL, BOX], { containers: ticked }), "containers", BOX)).toMatchObject({
      on: false,
      action: { kind: "enable", targetId: "t-box", on: true },
    });
  });

  it("makes a target the place to write to where the section is stored on a repository with its own credentials", () => {
    const remote = placed(row("containers", { home: "r1", homeKind: "remote" }), {
      targets: [target("t-box")],
      sendTo: [direct("t-box"), { kind: "remote", repoId: "r1", targetId: "", name: "Hetzner", location: "" }],
    });
    expect(line(rows([LOCAL, BOX], { containers: remote }), "containers", BOX).action).toEqual({ kind: "direct", targetId: "t-box", skip: ["*"] });
  });

  it("offers a named repository as the place to write to, and only one place at a time", () => {
    const archive = location({ id: "repo:archive", object: "repo", name: "Archiv" });
    const two = placed(row("containers"), {
      homes: [
        { id: "", name: "", location: "/mnt/user/backups/bombvault", kind: "domain", scheme: "" },
        { id: "archive", name: "Archiv", location: "/mnt/disks/archiv", kind: "local", scheme: "" },
      ],
    });
    const found = rows([LOCAL, archive], { containers: two });
    expect(line(found, "containers", LOCAL).on).toBe(true);
    expect(line(found, "containers", archive)).toMatchObject({ on: false, action: { kind: "default", change: { home: "archive", skip: ["*"] } } });
  });

  it("draws no line to a repository the section cannot be written to", () => {
    const archive = location({ id: "repo:archive", object: "repo", name: "Archiv" });
    const found = rows([LOCAL, archive], { containers });
    expect(found[0].lines.map((l) => l.location.name)).toEqual(["bombvault"]);
  });

  it("shows no lines for a section whose default cannot be read", () => {
    const found = rows([LOCAL, BOX], { containers: "unreadable" });
    expect(found[0]).toMatchObject({ domain: "containers", unreadable: true, nowhere: false, lines: [] });
  });
});

describe("flash, ZFS and the self-backup", () => {
  it("switch their own folder", () => {
    expect(line(rows([LOCAL, BOX]), "flash", LOCAL)).toMatchObject({ on: true, action: { kind: "local", on: false } });

    const off = { ...LOCAL, sections: [section("flash", { off: true })] };
    expect(line(rows([off, BOX]), "flash", off)).toMatchObject({ on: false, action: { kind: "local", on: true } });
  });

  it("switch a copy with the target's own switch, the one their off-site settings describe as well", () => {
    expect(line(rows([LOCAL, BOX]), "flash", BOX)).toMatchObject({
      on: true,
      action: { kind: "enable", targetId: "t-box-flash", on: false },
    });

    const primary = { ...BOX, sections: [section("flash", { use: "copy", targetId: "t-box-flash", primary: true, enabled: false })] };
    expect(line(rows([LOCAL, primary]), "flash", primary)).toMatchObject({ on: false, action: { kind: "enable", on: true } });
  });

  it("create the target under a destination that has none for them", () => {
    expect(line(rows([LOCAL, B2]), "flash", B2).action).toEqual({ kind: "create", destinationId: "b2" });
  });

  it("have no place while their folder and every copy are off", () => {
    const off = { ...LOCAL, sections: [section("flash", { off: true })] };
    const copyOff = { ...BOX, sections: [section("flash", { use: "copy", targetId: "t-box-flash", enabled: false })] };
    expect(rows([off, copyOff])[0]).toMatchObject({ domain: "flash", nowhere: true, counts: { copies: 1, media: 1, offsite: 0 } });
  });
});

describe("the 3-2-1 count", () => {
  const containers = placed(row("containers"), { targets: [target("t-box")], sendTo: [direct("t-box")] });

  it("counts the original, every place and every medium away from this host", () => {
    const [first] = rows([LOCAL, BOX], { containers });
    expect(first.counts).toEqual({ copies: 3, media: 2, offsite: 1 });
  });

  it("leaves out a place that is switched off and one that stands in the same building", () => {
    const lines = rows([LOCAL, BOX], { containers })[0].lines;
    const nearby = lines.map((l) => (l.location === BOX ? { ...l, location: { ...BOX, offPremises: false } } : l));
    expect(flowCounts(nearby)).toEqual({ copies: 3, media: 2, offsite: 0 });
    const down = lines.map((l) => (l.location === BOX ? { ...l, location: { ...BOX, enabled: false } } : l));
    expect(flowCounts(down)).toEqual({ copies: 2, media: 1, offsite: 0 });
  });

  it("names the destinations at which a section holds rules of its own", () => {
    const own = { ...BOX, sections: [section("containers", { use: "copy", targetId: "t-box", own: ["retention"] })] };
    expect(rows([LOCAL, own], { containers })[0].ownRules).toEqual([own]);
    expect(rows([LOCAL, BOX], { containers })[0].ownRules).toEqual([]);
  });
});
