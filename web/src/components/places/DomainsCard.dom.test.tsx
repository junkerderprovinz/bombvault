// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { CopiesPreview, DomainRow, HomePreview, Place, UnplacedRow } from "../../lib/places";

type Answer = Record<string, unknown>;

let rows: DomainRow[] = [];
let places: Place[] = [];
let unplaced: UnplacedRow[] = [];
let domainReads = 0;
const homePreviews: Answer[] = [];
const homeWrites: Answer[] = [];
const homeBodies: unknown[] = [];
const copiesPreviews: Answer[] = [];
const copiesWrites: Answer[] = [];
const copiesBodies: unknown[] = [];
const confirmed: [string, string[]][] = [];
const copyNow: string[] = [];
let copyAnswer: Answer = { ok: true };

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    getStorageDomains: () => {
      domainReads++;
      return Promise.resolve({ ok: true, domains: rows });
    },
    listPlaces: () => Promise.resolve({ ok: true, places, unplaced }),
    previewDomainHome: () => Promise.resolve(homePreviews.shift() ?? { ok: false, error: "no preview queued" }),
    setDomainHome: (_d: string, body: unknown) => {
      homeBodies.push(body);
      return Promise.resolve(homeWrites.shift() ?? { ok: true, reset: [], kept: [] });
    },
    previewDomainCopies: () => Promise.resolve(copiesPreviews.shift() ?? { ok: false, error: "no preview queued" }),
    setDomainCopies: (_d: string, body: unknown) => {
      copiesBodies.push(body);
      return Promise.resolve(copiesWrites.shift() ?? { ok: true });
    },
  };
});

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getConfirmPreview: () =>
    Promise.resolve({
      ok: true,
      paused: true,
      targets: [
        {
          targetId: "t-b2",
          name: "B2",
          preview: { items: 3, formerlyExcluded: [], defaultExcludes: false, snapshots: 12, bytes: null, unreadable: [] },
        },
      ],
      unmatched: [{ identity: "container:old", snapshots: 2 }],
    }),
  confirmPlacementDefault: (domain: string, exclude: string[]) => {
    confirmed.push([domain, exclude]);
    return Promise.resolve({ ok: true });
  },
  replicateOffsite: (domain: string) => {
    copyNow.push(domain);
    return Promise.resolve(copyAnswer);
  },
}));

vi.mock("../placeMarks", () => ({
  PlaceMark: ({ provider, onFill }: { provider: string; onFill?: boolean }) => (
    <span data-testid="mark" data-provider={provider} data-on-fill={String(!!onFill)} />
  ),
}));

const { DomainsCard } = await import("./DomainsCard");
const { placesChanged } = await import("../../lib/places");

function place(id: string, name: string, provider: string, over: Partial<Place> = {}): Place {
  return {
    id,
    name,
    provider,
    kind: "local",
    base: "user/bombvault",
    folders: { containers: "container", vms: "vms", flash: "flash", config: "config", files: "files" },
    offPremises: false,
    storageClass: "",
    immutable: false,
    retentionKeepLast: 0,
    retentionKeepDaily: 0,
    retentionKeepWeekly: 0,
    retentionKeepMonthly: 0,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    sortOrder: 0,
    usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0 },
    locked: {},
    repository: false,
    creds: { shared: false, fields: {}, set: [] },
    ...over,
  };
}

function row(domain: string, over: Partial<DomainRow> = {}): DomainRow {
  return {
    domain,
    homePlace: "p-unraid",
    storedIn: "p-unraid",
    chips: [],
    exceptions: [],
    paused: false,
    schedule: "daily 02:00",
    unreadable: false,
    ...over,
  };
}

function preview(over: Partial<HomePreview> = {}): HomePreview {
  return { mode: "home-place", placeId: "p-nas", homePlace: "p-unraid", homeHasBackups: false, backups: 0, ...over };
}

beforeEach(() => {
  domainReads = 0;
  homePreviews.length = 0;
  homeWrites.length = 0;
  homeBodies.length = 0;
  copiesPreviews.length = 0;
  copiesWrites.length = 0;
  copiesBodies.length = 0;
  confirmed.length = 0;
  copyNow.length = 0;
  copyAnswer = { ok: true };
  places = [
    place("p-unraid", "Unraid", "unraid-folder"),
    place("p-nas", "NAS Keller", "synology"),
    place("p-b2", "B2", "b2", { folders: { vms: "vms" } }),
    place("p-old", "Old disk", "unraid-folder", { enabled: false }),
  ];
  rows = [row("containers"), row("flash", { schedule: "off" })];
  unplaced = [];
});
afterEach(cleanup);

async function card() {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <DomainsCard />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

const rowOf = (name: string) => screen.getByRole("region", { name });
const storedIn = (name: string) => within(rowOf(name)).getByRole("combobox", { name: en["storageDomains.storedIn"] });
const shown = (name: string) => storedIn(name).querySelector('span[aria-hidden="false"]')?.textContent;
const dialog = () => screen.getByRole("dialog");

async function pick(name: string, option: string) {
  fireEvent.click(storedIn(name));
  fireEvent.click(screen.getByRole("option", { name: option }));
  await act(async () => {
    fireEvent.click(within(rowOf(name)).getByRole("button", { name: en["placement.saveHome"] }));
  });
}

async function answer(button: string) {
  await act(async () => {
    fireEvent.click(within(dialog()).getByRole("button", { name: button }));
  });
}

describe("DomainsCard", () => {
  it("shows a row per domain with its schedule, linked to the schedules", async () => {
    await card();
    expect(screen.getByRole("heading", { name: new RegExp(`^${en["storageDomains.title"]}`) })).toBeTruthy();
    const link = within(rowOf("Containers")).getByRole("link", { name: "Daily at 02:00" });
    expect(link.getAttribute("href")).toBe("/settings#schedules");
    expect(within(rowOf("Flash")).getByRole("link", { name: en["jobs.notScheduled"] })).toBeTruthy();
  });

  it("offers every switched-on place with the domain's folder, each with its mark", async () => {
    await card();
    expect(shown("Containers")).toBe("Unraid");
    fireEvent.click(storedIn("Containers"));
    const options = screen.getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["Unraid", "NAS Keller"]);
    expect(options.map((o) => o.querySelector('[data-testid="mark"]')?.getAttribute("data-provider"))).toEqual([
      "unraid-folder",
      "synology",
    ]);
  });

  it("keeps the stored place in the field, dimmed, when it is switched off", async () => {
    rows = [row("containers", { storedIn: "p-old" })];
    await card();
    expect(shown("Containers")).toBe("Old disk");
    fireEvent.click(storedIn("Containers"));
    const off = screen.getByRole("option", { name: "Old disk" });
    expect((off as HTMLButtonElement).disabled).toBe(true);
  });

  it("sets its field off the row's own surface", async () => {
    await card();
    expect(storedIn("Containers").classList.contains("bg-carbon-surface3")).toBe(true);
  });

  it("says a row it cannot read instead of offering places", async () => {
    rows = [row("containers", { unreadable: true })];
    await card();
    expect(within(rowOf("Containers")).getByText(en["placement.unreadable"])).toBeTruthy();
    expect(within(rowOf("Containers")).queryByRole("combobox")).toBeNull();
  });

  it("makes a place the home of a domain without backups once asked, and reads the rows again", async () => {
    await card();
    homePreviews.push({ ok: true, ...preview() });
    await pick("Containers", "NAS Keller");
    expect(
      within(dialog()).getByText(
        en["storageDomains.homePlaceAsk"].replace("{domain}", "Containers").replace("{place}", "NAS Keller")
      )
    ).toBeTruthy();
    const reads = domainReads;
    await answer(en["placement.saveHome"]);
    expect(homeBodies).toEqual([
      {
        placeId: "p-nas",
        expect: { mode: "home-place", placeId: "p-nas", homePlace: "p-unraid", homeHasBackups: false, backups: 0 },
        applyToOpen: false,
      },
    ]);
    expect(domainReads).toBeGreaterThan(reads);
  });

  it("points the default there with the switch for items without backups", async () => {
    await card();
    const impact = { dropped: [], added: [], openTakeHome: 2, home: "", skip: [] };
    homePreviews.push({ ok: true, ...preview({ mode: "default", homeHasBackups: true, creates: "repository", impact }) });
    await pick("Containers", "NAS Keller");
    expect(
      within(dialog()).getByText(
        en["storageDomains.createsRepository"].replace("{domain}", "Containers").replace("{place}", "NAS Keller")
      )
    ).toBeTruthy();
    expect(
      within(dialog()).getByText(en["placementDefaults.openTakeHome"].replace("{home}", "NAS Keller").replace("{n}", "2"))
    ).toBeTruthy();
    fireEvent.click(within(dialog()).getByRole("switch", { name: en["placementDefaults.apply"] }));
    await answer(en["placement.saveHome"]);
    expect(homeBodies).toHaveLength(1);
    expect((homeBodies[0] as { applyToOpen: boolean }).applyToOpen).toBe(true);
  });

  it("moves the home of flash and says how many snapshots stay behind", async () => {
    await card();
    homePreviews.push({ ok: true, ...preview({ mode: "home-move", homeHasBackups: true, backups: 3 }) });
    await pick("Flash", "NAS Keller");
    expect(
      within(dialog()).getByText(en["storageDomains.homeMoveAsk"].replace("{domain}", "Flash").replace("{place}", "NAS Keller"))
    ).toBeTruthy();
    expect(within(dialog()).getByText(/^3 snapshots stay at the old place/)).toBeTruthy();
  });

  it("asks again with the numbers of a stale answer", async () => {
    await card();
    const impact = (n: number) => ({ dropped: [], added: [], openTakeHome: n, home: "", skip: [] });
    homePreviews.push({ ok: true, ...preview({ mode: "default", homeHasBackups: true, impact: impact(1) }) });
    homeWrites.push({
      ok: false,
      code: "stale",
      error: "stale",
      preview: preview({ mode: "default", homeHasBackups: true, impact: impact(5) }),
    });
    await pick("Containers", "NAS Keller");
    await answer(en["placement.saveHome"]);
    expect(
      within(dialog()).getByText(en["placementDefaults.openTakeHome"].replace("{home}", "NAS Keller").replace("{n}", "5"))
    ).toBeTruthy();
    await answer(en["placement.saveHome"]);
    expect(homeBodies).toHaveLength(2);
    expect((homeBodies[1] as { expect: HomePreview }).expect.impact?.openTakeHome).toBe(5);
  });

  it("keeps the picked place while a list read lands during its question, and takes it back on Cancel", async () => {
    await card();
    homePreviews.push({ ok: true, ...preview() });
    await pick("Containers", "NAS Keller");
    await act(async () => {
      placesChanged();
    });
    expect(shown("Containers")).toBe("NAS Keller");
    expect(within(rowOf("Containers")).queryByRole("button", { name: en["placement.saveHome"] })).toBeNull();
    await answer(en["common.cancel"]);
    expect(shown("Containers")).toBe("Unraid");
    expect(homeBodies).toEqual([]);
  });

  it("takes the choice back and shakes when the write is refused", async () => {
    await card();
    homePreviews.push({ ok: true, ...preview() });
    homeWrites.push({ ok: false, code: "place-off", error: "place is switched off" });
    await pick("Containers", "NAS Keller");
    await answer(en["placement.saveHome"]);
    expect(shown("Containers")).toBe("Unraid");
    expect(storedIn("Containers").closest(".glim-shake")).toBeTruthy();
    expect(screen.getByText(en["places.error.off"])).toBeTruthy();
  });
});

function copies(over: Partial<CopiesPreview> = {}): CopiesPreview {
  return { placeId: "p-nas", on: true, skip: [], enabled: false, ...over };
}

const chip = (rowName: string, name: string) => within(rowOf(rowName)).getByRole("button", { name });

async function tick(rowName: string, name: string) {
  await act(async () => {
    fireEvent.click(chip(rowName, name));
  });
}

describe("DomainsCard copies", () => {
  beforeEach(() => {
    rows = [
      row("containers", {
        chips: [
          { placeId: "p-b2", targetId: "t-b2", on: true, disabled: false },
          { placeId: "p-nas", on: false, disabled: false },
          { placeId: "p-old", targetId: "t-old", on: true, disabled: true, reason: "off" },
        ],
      }),
      row("flash", { chips: [{ placeId: "p-nas", on: false, disabled: false }] }),
      row("config", { chips: [] }),
    ];
  });

  it("shows a chip per place with its mark, in the chip's ink once ticked, and a switched-off place dimmed", async () => {
    await card();
    expect(chip("Containers", "B2").getAttribute("aria-pressed")).toBe("true");
    expect(chip("Containers", "B2").querySelector('[data-testid="mark"]')?.getAttribute("data-on-fill")).toBe("true");
    expect(chip("Containers", "NAS Keller").getAttribute("aria-pressed")).toBe("false");
    expect(chip("Containers", "NAS Keller").querySelector('[data-testid="mark"]')?.getAttribute("data-on-fill")).toBe("false");
    const off = chip("Containers", en["placement.off"].replace("{name}", "Old disk"));
    expect((off as HTMLButtonElement).disabled).toBe(true);
  });

  it("sets an idle chip off the row's own surface", async () => {
    await card();
    expect(chip("Containers", "NAS Keller").classList.contains("bg-carbon-surface3")).toBe(true);
  });

  it("says so when no other place offers the domain", async () => {
    await card();
    expect(within(rowOf("Self-Backup")).getByText(en["storageDomains.noCopyPlace"])).toBeTruthy();
  });

  it("asks what a new target receives, and shows the tick while it asks", async () => {
    await card();
    const newTarget = { items: 7, formerlyExcluded: [], defaultExcludes: false, snapshots: 120, bytes: null, unreadable: [] };
    copiesPreviews.push({ ok: true, ...copies({ newTarget }) });
    await tick("Containers", "NAS Keller");
    expect(within(dialog()).getByText(en["newTarget.intro"].replace("{target}", "NAS Keller"))).toBeTruthy();
    expect(within(dialog()).getByText(en["newTarget.items"].replace("{n}", "7"))).toBeTruthy();
    expect(chip("Containers", "NAS Keller").getAttribute("aria-pressed")).toBe("true");
    await answer(en["common.confirm"]);
    expect(copiesBodies).toEqual([{ placeId: "p-nas", on: true, expect: copies({ newTarget }) }]);
  });

  it("says what stays when a chip is switched off", async () => {
    await card();
    const impact = {
      dropped: [{ targetId: "t-b2", name: "B2", items: 4, snapshots: 30, unknown: false, uncheckable: [] }],
      added: [],
      openTakeHome: 0,
      home: "",
      skip: ["t-b2"],
    };
    copiesPreviews.push({
      ok: true,
      ...copies({ placeId: "p-b2", on: false, targetId: "t-b2", skip: ["t-b2"], enabled: true, impact }),
    });
    await tick("Containers", "B2");
    expect(
      within(dialog()).getByText(en["storageDomains.copiesOffAsk"].replace("{place}", "B2").replace("{domain}", "Containers"))
    ).toBeTruthy();
    expect(
      within(dialog()).getByText(
        en["placementDefaults.dropAsk"].replace("{target}", "B2").replace("{n}", "4").replace("{copies}", "30")
      )
    ).toBeTruthy();
    expect(within(dialog()).getByText(en["storageDomains.copiesOffOwnChoice"])).toBeTruthy();
  });

  it("asks again with the preview of a stale answer", async () => {
    await card();
    const added = (n: number) => ({
      dropped: [],
      added: [{ targetId: "t-nas", name: "NAS Keller", items: n, snapshots: 9, unknown: false, uncheckable: [] }],
      openTakeHome: 0,
      home: "",
      skip: [],
    });
    copiesPreviews.push({ ok: true, ...copies({ targetId: "t-nas", impact: added(2) }) });
    copiesWrites.push({ ok: false, code: "stale", error: "stale", preview: copies({ targetId: "t-nas", impact: added(6) }) });
    await tick("Containers", "NAS Keller");
    await answer(en["common.confirm"]);
    expect(within(dialog()).getByText(/gets from now on: 6\./)).toBeTruthy();
    await answer(en["common.confirm"]);
    expect(copiesBodies).toHaveLength(2);
    expect((copiesBodies[1] as { expect: CopiesPreview }).expect.impact?.added[0]?.items).toBe(6);
  });

  it("keeps the tick through a list read until the write is through", async () => {
    await card();
    copiesPreviews.push({
      ok: true,
      ...copies({ newTarget: { items: 1, formerlyExcluded: [], defaultExcludes: false, snapshots: 1, bytes: null, unreadable: [] } }),
    });
    await tick("Containers", "NAS Keller");
    await act(async () => {
      placesChanged();
    });
    expect(chip("Containers", "NAS Keller").getAttribute("aria-pressed")).toBe("true");
    rows = [row("containers", { chips: [{ placeId: "p-nas", targetId: "t-nas", on: true, disabled: false }] })];
    await answer(en["common.confirm"]);
    expect(chip("Containers", "NAS Keller").getAttribute("aria-pressed")).toBe("true");
  });

  it("takes the tick back and shakes when the write is refused", async () => {
    await card();
    copiesPreviews.push({
      ok: true,
      ...copies({ newTarget: { items: 1, formerlyExcluded: [], defaultExcludes: false, snapshots: 1, bytes: null, unreadable: [] } }),
    });
    copiesWrites.push({ ok: false, code: "place-off", error: "place is switched off" });
    await tick("Containers", "NAS Keller");
    await answer(en["common.confirm"]);
    expect(chip("Containers", "NAS Keller").getAttribute("aria-pressed")).toBe("false");
    expect(chip("Containers", "NAS Keller").closest(".glim-shake")).toBeTruthy();
    expect(screen.getByText(en["places.error.off"])).toBeTruthy();
  });

  it("switches a flash chip without a question", async () => {
    await card();
    copiesPreviews.push({ ok: true, ...copies() });
    await tick("Flash", "NAS Keller");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(copiesBodies).toEqual([{ placeId: "p-nas", on: true, expect: copies() }]);
  });
});

describe("DomainsCard exceptions", () => {
  it("lists the items that chose otherwise as links to their cards", async () => {
    rows = [
      row("containers", {
        exceptions: [
          { identity: "container:nginx", name: "nginx", link: "/containers?item=nginx" },
          { identity: "container:plex", name: "plex", link: "/containers?item=plex" },
        ],
      }),
    ];
    await card();
    expect(within(rowOf("Containers")).queryByRole("link", { name: "nginx" })).toBeNull();
    fireEvent.click(within(rowOf("Containers")).getByRole("button", { name: countText(en["places.row.items"], "en", 2) }));
    expect(within(rowOf("Containers")).getByRole("link", { name: "nginx" }).getAttribute("href")).toBe("/containers?item=nginx");
    expect(within(rowOf("Containers")).getByRole("link", { name: "plex" }).getAttribute("href")).toBe("/containers?item=plex");
  });

  it("says none when every item follows the domain", async () => {
    await card();
    expect(within(rowOf("Containers")).getByText(en["storageDomains.exceptionsNone"])).toBeTruthy();
  });
});

describe("DomainsCard pause and copy now", () => {
  it("confirms the default of a paused row with what the next run copies, and flash has none", async () => {
    rows = [row("containers", { paused: true }), row("flash", { paused: true })];
    await card();
    expect(within(rowOf("Flash")).queryByRole("button", { name: en["placementDefaults.confirm"] })).toBeNull();
    expect(within(rowOf("Containers")).getByText(en["placementDefaults.paused"])).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(rowOf("Containers")).getByRole("button", { name: en["placementDefaults.confirm"] }));
    });
    expect(within(dialog()).getByText(en["placementDefaults.confirmAsk"])).toBeTruthy();
    expect(within(dialog()).getByText("B2")).toBeTruthy();
    fireEvent.click(
      within(dialog()).getByRole("switch", {
        name: en["placementDefaults.unmatchedLine"].replace("{name}", "old").replace("{n}", "2"),
      })
    );
    await answer(en["placementDefaults.confirm"]);
    expect(confirmed).toEqual([["containers", ["container:old"]]]);
    expect(screen.getByText(en["placementDefaults.confirmed"])).toBeTruthy();
  });

  it("copies a domain now while it has a target, ticked or not", async () => {
    rows = [
      row("containers", { chips: [{ placeId: "p-b2", targetId: "t-b2", on: false, disabled: false }] }),
      row("vms", { chips: [{ placeId: "p-b2", on: false, disabled: false }] }),
    ];
    await card();
    expect(within(rowOf("VMs")).queryByRole("button", { name: en["storageDomains.copyNow"] })).toBeNull();
    await act(async () => {
      fireEvent.click(within(rowOf("Containers")).getByRole("button", { name: en["storageDomains.copyNow"] }));
    });
    expect(copyNow).toEqual(["containers"]);
    expect(screen.getByText(en["storageDomains.copyStarted"])).toBeTruthy();
  });

  it("names the targets of a domain that fit no place and copies the domain now to them", async () => {
    rows = [row("containers"), row("flash")];
    unplaced = [
      { rowId: "t-native", domain: "containers", role: "target", name: "B2 native", repo: "b2:bucket:containers", immutable: false, protectable: true, items: 0 },
      { rowId: "t-swift", domain: "containers", role: "target", name: "Swift", repo: "swift:box:containers", immutable: false, protectable: true, items: 0 },
      { rowId: "t-gs", domain: "vms", role: "target", name: "GCS", repo: "gs:bucket:vms", immutable: false, protectable: true, items: 0 },
      { rowId: "", domain: "flash", role: "path", name: "", repo: "b2:bucket:flash", immutable: false, protectable: true, items: 0 },
    ];
    await card();
    const containers = rowOf("Containers");
    expect(within(containers).queryByText(en["storageDomains.noCopyPlace"])).toBeNull();
    expect(within(containers).getByText(en["storageDomains.unplacedTargets"].replace("{list}", "B2 native and Swift"))).toBeTruthy();
    expect(within(rowOf("Flash")).getByText(en["storageDomains.noCopyPlace"])).toBeTruthy();
    expect(within(rowOf("Flash")).queryByRole("button", { name: en["storageDomains.copyNow"] })).toBeNull();
    await act(async () => {
      fireEvent.click(within(containers).getByRole("button", { name: en["storageDomains.copyNow"] }));
    });
    expect(copyNow).toEqual(["containers"]);
  });

  it("offers Copy now beside chips without a target when a target fits no place", async () => {
    rows = [row("vms", { chips: [{ placeId: "p-b2", on: false, disabled: false }] })];
    unplaced = [{ rowId: "t-gs", domain: "vms", role: "target", name: "GCS", repo: "gs:bucket:vms", immutable: false, protectable: true, items: 0 }];
    await card();
    expect(chip("VMs", "B2").getAttribute("aria-pressed")).toBe("false");
    expect(within(rowOf("VMs")).getByRole("button", { name: en["storageDomains.copyNow"] })).toBeTruthy();
  });

  it("offers no Copy now while the row waits for its default to be confirmed", async () => {
    rows = [row("containers", { paused: true, chips: [{ placeId: "p-b2", targetId: "t-b2", on: true, disabled: false }] })];
    await card();
    expect(within(rowOf("Containers")).queryByRole("button", { name: en["storageDomains.copyNow"] })).toBeNull();
  });

  it("shakes Copy now and says why when the copy cannot start", async () => {
    rows = [row("containers", { chips: [{ placeId: "p-b2", targetId: "t-b2", on: true, disabled: false }] })];
    copyAnswer = { ok: false, error: "no target is switched on" };
    await card();
    await act(async () => {
      fireEvent.click(within(rowOf("Containers")).getByRole("button", { name: en["storageDomains.copyNow"] }));
    });
    expect(screen.getByText("no target is switched on")).toBeTruthy();
    expect(within(rowOf("Containers")).getByRole("button", { name: en["storageDomains.copyNow"] }).className).toContain(
      "glim-shake"
    );
  });
});
