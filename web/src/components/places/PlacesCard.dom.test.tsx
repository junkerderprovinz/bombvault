// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { placesChanged, type Place, type UnplacedRow } from "../../lib/places";

let listed: { places: Place[]; unplaced: UnplacedRow[] } = { places: [], unplaced: [] };
let listFails = false;
let lists = 0;
const adopted: [string, string, string][] = [];

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
  };
});

const { PlacesCard } = await import("./PlacesCard");

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
    usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0 },
    locked: {},
    repository: false,
    creds: { shared: false, fields: {}, set: [] },
  };
}

beforeEach(() => {
  lists = 0;
  listFails = false;
  adopted.length = 0;
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
      { rowId: "", domain: "flash", role: "path", name: "", repo: "user/old/flash" },
      { rowId: "t9", domain: "vms", role: "target", name: "Old NAS", repo: "remotes/oldnas/vms" },
      { rowId: "r3", domain: "", role: "repository", name: "Archive", repo: "remotes/archive" },
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
    listed.unplaced = [{ rowId: "r3", domain: "", role: "repository", name: "Archive", repo: "remotes/archive" }];
    await card();
    const row = screen.getByText("remotes/archive").closest("div.rounded-card") as HTMLElement;
    for (const key of ["places.unplaced.place", "places.unplaced.domain"] as const) {
      const label = within(row).getByText(en[key]);
      expect(label.tagName).toBe("LABEL");
      expect(document.getElementById(label.getAttribute("for") ?? "")?.getAttribute("role")).toBe("combobox");
    }
  });
});
