// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { DomainRow, HomePreview, Place } from "../../lib/places";

type Answer = Record<string, unknown>;

let rows: DomainRow[] = [];
let places: Place[] = [];
let domainReads = 0;
const homePreviews: Answer[] = [];
const homeWrites: Answer[] = [];
const homeBodies: unknown[] = [];

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    getStorageDomains: () => {
      domainReads++;
      return Promise.resolve({ ok: true, domains: rows });
    },
    listPlaces: () => Promise.resolve({ ok: true, places, unplaced: [] }),
    previewDomainHome: () => Promise.resolve(homePreviews.shift() ?? { ok: false, error: "no preview queued" }),
    setDomainHome: (_d: string, body: unknown) => {
      homeBodies.push(body);
      return Promise.resolve(homeWrites.shift() ?? { ok: true, reset: [], kept: [] });
    },
  };
});

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
  places = [
    place("p-unraid", "Unraid", "unraid-folder"),
    place("p-nas", "NAS Keller", "synology"),
    place("p-b2", "B2", "b2", { folders: { vms: "vms" } }),
    place("p-old", "Old disk", "unraid-folder", { enabled: false }),
  ];
  rows = [row("containers"), row("flash", { schedule: "off" })];
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
    expect(link.getAttribute("href")).toBe("#schedules");
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
    expect(storedIn("Containers").className).toContain("bg-carbon-surface3");
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
