// @vitest-environment jsdom
// A place's details save themselves: a switch at once and back with a shake
// when refused, a typed field 800 ms after the last key, and a field still
// waiting when the details close.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { HUE_OFFSET } from "../Selector";
import type { CatalogProvider, PatchPlaceBody, Place } from "../../lib/places";

const patches: Partial<PatchPlaceBody>[] = [];
let answer: (body: Partial<PatchPlaceBody>) => { ok: boolean; code?: string; error?: string; place?: Place } = () => ({ ok: true });

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    patchPlace: (_id: string, body: Partial<PatchPlaceBody>) => {
      patches.push(body);
      return Promise.resolve(answer(body));
    },
  };
});

const { PlaceDetails } = await import("./PlaceDetails");

function place(over: Partial<Place> = {}): Place {
  return {
    id: "p1",
    name: "B2",
    provider: "b2",
    kind: "s3",
    base: "s3:https://s3.example.com/bv",
    folders: { containers: "container", vms: "vms" },
    offPremises: true,
    storageClass: "",
    immutable: false,
    retentionKeepLast: 10,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    sortOrder: 0,
    usage: { homeDomains: [], defaults: [], copyDomains: [], items: 0, copies: 0 },
    locked: { containers: false, vms: false },
    repository: false,
    creds: { shared: false, fields: { keyId: "k1", region: "eu" }, set: ["secret"] },
    ...over,
  };
}

const B2: CatalogProvider = {
  id: "b2",
  group: "cloud",
  kind: "s3",
  offPremises: true,
  fields: [{ key: "keyId" }, { key: "secret", secret: true }, { key: "region", optional: true }],
};

beforeEach(() => {
  patches.length = 0;
  answer = (body) => ({ ok: true, place: { ...place(), ...(body as Partial<Place>) } });
  vi.useFakeTimers();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function details(p: Place = place(), provider: CatalogProvider | undefined = B2, onSaved = vi.fn()) {
  const view = render(
    <I18nProvider>
      <ToastProvider>
        <PlaceDetails place={p} provider={provider} onSaved={onSaved} />
      </ToastProvider>
    </I18nProvider>
  );
  return { ...view, onSaved };
}

async function settle(ms = 0) {
  await act(async () => {
    vi.advanceTimersByTime(ms);
  });
}

const input = (label: string) => screen.getByLabelText(label) as HTMLInputElement;

describe("PlaceDetails saving", () => {
  it("saves a switch at once and takes it back with a shake when refused", async () => {
    answer = () => ({ ok: false, code: "place-home-domain", error: "home" });
    details();
    const toggle = screen.getByRole("switch", { name: en["places.details.enabled"] });
    await act(async () => {
      fireEvent.click(toggle);
    });
    expect(patches).toEqual([{ enabled: false }]);
    expect(screen.getByRole("switch", { name: en["places.details.enabled"] }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("switch", { name: en["places.details.enabled"] }).closest(".glim-shake")).toBeTruthy();
    expect(screen.getByText(en["places.error.homeDomain"])).toBeTruthy();
  });

  it("asks a device where it stands, on a colour of its own, and saves the answer at once", async () => {
    const NAS: CatalogProvider = { id: "synology", group: "here", kind: "local", pickRoots: ["remotes"], fields: [{ key: "path" }] };
    details(place({ kind: "local", provider: "synology", offPremises: false }), NAS);
    const where = screen.getByRole("tablist", { name: en["places.form.where"] });
    expect(within(where).getAllByRole("tab")[0]!.style.getPropertyValue("--item-hue")).toBe(`var(--rb-${HUE_OFFSET.placeWhere})`);
    await act(async () => {
      fireEvent.click(within(where).getByRole("tab", { name: en["places.form.away"] }));
    });
    expect(patches).toEqual([{ offPremises: true }]);
  });

  it("saves a typed field 800 ms after the last key", async () => {
    const { onSaved } = details();
    fireEvent.change(input(en["places.form.name"]), { target: { value: "B2 E" } });
    await settle(500);
    fireEvent.change(input(en["places.form.name"]), { target: { value: "B2 EU" } });
    await settle(799);
    expect(patches).toEqual([]);
    await settle(1);
    expect(patches).toEqual([{ name: "B2 EU" }]);
    expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ name: "B2 EU" }));
  });

  it("saves a field that is still waiting when the details close", async () => {
    const { unmount } = details();
    fireEvent.change(input(en["places.details.limitUpload"]), { target: { value: "800" } });
    unmount();
    await settle();
    expect(patches).toEqual([{ limitUpload: 800 }]);
  });

  it("asks before this place keeps fewer snapshots where items back up to it", async () => {
    details(place({ usage: { homeDomains: [], defaults: [], copyDomains: [], items: 3, copies: 0 } }));
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "3" } });
    await settle(800);
    const ask = screen.getByRole("dialog");
    expect(within(ask).getByText(countText(en["places.details.retentionLowerAsk"], "en", 3))).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(ask).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(patches).toEqual([]);
    expect(input(en["places.details.keepLast"]).value).toBe("10");
  });
});
