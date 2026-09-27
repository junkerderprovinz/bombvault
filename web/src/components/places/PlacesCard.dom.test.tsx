// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { placesChanged, type Place, type PlaceRefusal, type UnplacedRow } from "../../lib/places";

let listed: { places: Place[]; unplaced: UnplacedRow[] } = { places: [], unplaced: [] };
let listFails = false;
let lists = 0;
const adopted: [string, string, string][] = [];
const switched: [string, string, boolean][] = [];
let switchAnswer: PlaceRefusal = { ok: true };

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    listPlaces: () => {
      lists++;
      return Promise.resolve(listFails ? { ok: false, error: "database is locked" } : { ok: true, ...listed });
    },
    getPlacesCatalog: () => Promise.resolve({ ok: true, providers: [] }),
    adoptRow: (placeId: string, rowId: string, domain: string) => {
      adopted.push([placeId, rowId, domain]);
      return Promise.resolve({ ok: true });
    },
    setUnplacedAppendOnly: (rowId: string, domain: string, immutable: boolean) => {
      switched.push([rowId, domain, immutable]);
      return Promise.resolve(switchAnswer);
    },
  };
});

const { PlacesCard } = await import("./PlacesCard");

function loose(over: Partial<UnplacedRow> & Pick<UnplacedRow, "rowId" | "domain" | "role" | "repo">): UnplacedRow {
  return { name: "", immutable: false, protectable: false, items: 0, ...over };
}

function place(id: string, name: string, provider: string): Place {
  return {
    id,
    name,
    provider,
    kind: "local",
    base: "user/bombvault",
    folders: {},
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
    usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0, repositories: 0 },
    locked: {},
    repository: false,
    creds: { shared: false, fields: {}, set: [] },
  };
}

beforeEach(() => {
  lists = 0;
  listFails = false;
  adopted.length = 0;
  switched.length = 0;
  switchAnswer = { ok: true };
  listed = { places: [place("p1", "Unraid", "unraid-folder"), place("p2", "B2", "b2")], unplaced: [] };
});
afterEach(cleanup);

async function card() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PlacesCard hostMountRoot="/mnt" />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

describe("PlacesCard", () => {
  it("lists every place under its title, and reads them again after a change", async () => {
    await card();
    expect(screen.getByRole("heading", { name: new RegExp(`^${en["places.title"]}`) })).toBeTruthy();
    expect(screen.getByText("Unraid")).toBeTruthy();
    expect(screen.getByText("B2")).toBeTruthy();
    listed = { places: [place("p1", "Unraid", "unraid-folder")], unplaced: [] };
    await act(async () => {
      placesChanged();
    });
    expect(lists).toBe(2);
    expect(screen.queryByText("B2")).toBeNull();
  });

  it("says when there is no place yet", async () => {
    listed = { places: [], unplaced: [] };
    await card();
    expect(screen.getByText(en["places.empty"])).toBeTruthy();
  });

  it("says the places could not be read rather than that there are none", async () => {
    listFails = true;
    await card();
    expect(screen.getByText(en["places.loadFailed"])).toBeTruthy();
    expect(screen.queryByText("database is locked")).toBeNull();
    expect(screen.queryByText(en["places.empty"])).toBeNull();
  });

  it("opens the add window from its key button", async () => {
    await card();
    const add = screen.getByRole("button", { name: en["places.add"] });
    expect(add.className).toContain("glim-btn-key");
    await act(async () => {
      fireEvent.click(add);
    });
    expect(screen.getByRole("dialog", { name: en["places.addTitle"] })).toBeTruthy();
  });

  it("keeps rows without a place in their own group and assigns them to a place", async () => {
    listed.unplaced = [
      loose({ rowId: "", domain: "flash", role: "path", repo: "user/old/flash" }),
      loose({ rowId: "t9", domain: "vms", role: "target", name: "Old NAS", repo: "remotes/oldnas/vms" }),
      loose({ rowId: "r3", domain: "", role: "repository", name: "Archive", repo: "remotes/archive" }),
    ];
    await card();
    const group = screen.getByRole("heading", { name: new RegExp(`^${en["places.unplaced.title"]}`) }).parentElement!;
    const rows = () =>
      within(group)
        .getAllByText(/user\/old\/flash|remotes\/oldnas\/vms|remotes\/archive/)
        .map((r) => r.closest("div.rounded-card") as HTMLElement);
    expect(within(rows()[0]!).getByText(en["places.unplaced.path"].replace("{domain}", "Flash"))).toBeTruthy();

    const assign = async (row: HTMLElement, placeName: string, domain?: string) => {
      const [placePick, domainPick] = within(row).getAllByRole("combobox");
      fireEvent.click(placePick!);
      fireEvent.click(screen.getByRole("option", { name: placeName }));
      if (domain) {
        fireEvent.click(domainPick!);
        fireEvent.click(screen.getByRole("option", { name: domain }));
      }
      await act(async () => {
        fireEvent.click(within(row).getByRole("button", { name: en["places.unplaced.link"] }));
      });
    };
    expect(within(rows()[0]!).getByRole("button", { name: en["places.unplaced.link"] })).toHaveProperty("disabled", true);
    await assign(rows()[0]!, "Unraid");
    await assign(rows()[1]!, "B2");
    await assign(rows()[2]!, "B2", "VMs");
    expect(adopted).toEqual([
      ["p1", "", "flash"],
      ["p2", "t9", ""],
      ["p2", "r3", "vms"],
    ]);
  });

  it("writes what each picker of a row without a place chooses beside it", async () => {
    listed.unplaced = [loose({ rowId: "r3", domain: "", role: "repository", name: "Archive", repo: "remotes/archive" })];
    await card();
    const row = screen.getByText("remotes/archive").closest("div.rounded-card") as HTMLElement;
    for (const key of ["places.unplaced.place", "places.unplaced.domain"] as const) {
      const label = within(row).getByText(en[key]);
      expect(label.tagName).toBe("LABEL");
      expect(document.getElementById(label.getAttribute("for") ?? "")?.getAttribute("role")).toBe("combobox");
    }
  });
  it("gives a row without a place an append-only switch only where the flag means something", async () => {
    listed.unplaced = [
      loose({ rowId: "t1", domain: "vms", role: "target", name: "B2 native", repo: "b2:bucket:vms", protectable: true }),
      loose({ rowId: "t2", domain: "vms", role: "target", name: "Old NAS", repo: "remotes/oldnas/vms" }),
    ];
    await card();
    const row = (repo: string) => screen.getByText(repo).closest("div.rounded-card") as HTMLElement;
    expect(within(row("remotes/oldnas/vms")).queryByRole("switch")).toBeNull();

    const on = within(row("b2:bucket:vms")).getByRole("switch", { name: en["places.details.appendOnly"] });
    expect(on.getAttribute("aria-checked")).toBe("false");
    await act(async () => {
      fireEvent.click(on);
    });
    expect(switched).toEqual([["t1", "vms", true]]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(lists).toBe(2);
  });

  it("names the items whose backups lie there before append-only goes off", async () => {
    listed.unplaced = [
      loose({ rowId: "", domain: "vms", role: "path", repo: "b2:bucket:vms", immutable: true, protectable: true, items: 3 }),
    ];
    await card();
    const toggle = () => screen.getByRole("switch", { name: en["places.details.appendOnly"] });
    const answer = async (button: string) => {
      await act(async () => {
        fireEvent.click(toggle());
      });
      const ask = screen.getByRole("dialog");
      expect(within(ask).getByText(countText(en["places.unplaced.appendOnlyOffAsk"], "en", 3))).toBeTruthy();
      await act(async () => {
        fireEvent.click(within(ask).getByRole("button", { name: button }));
      });
    };
    await answer(en["common.cancel"]);
    expect(switched).toEqual([]);
    expect(toggle().getAttribute("aria-checked")).toBe("true");
    await answer(en["common.confirm"]);
    expect(switched).toEqual([["", "vms", false]]);
  });

  it("puts a refused switch back with a toast and a shake", async () => {
    switchAnswer = { ok: false, code: "place-no-append-only", error: "a folder on this server cannot be kept from deletion" };
    listed.unplaced = [loose({ rowId: "t1", domain: "vms", role: "target", repo: "b2:bucket:vms", protectable: true })];
    await card();
    const toggle = () => screen.getByRole("switch", { name: en["places.details.appendOnly"] });
    await act(async () => {
      fireEvent.click(toggle());
    });
    expect(toggle().getAttribute("aria-checked")).toBe("false");
    expect(screen.getByText(en["places.error.noAppendOnly"])).toBeTruthy();
    expect(toggle().closest(".glim-shake")).toBeTruthy();
    expect(lists).toBe(1);
  });
});
