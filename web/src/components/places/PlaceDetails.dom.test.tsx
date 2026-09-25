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
const tampered: string[] = [];
let tamperAnswer: { ok: boolean; error?: string; testable?: boolean; protected?: boolean } = { ok: true };

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

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    tamperTest: (domain: string) => {
      tampered.push(domain);
      return Promise.resolve(tamperAnswer);
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
  tampered.length = 0;
  tamperAnswer = { ok: true, testable: true, protected: true };
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

describe("PlaceDetails sections", () => {
  it("offers protection only at a remote place, and the tamper test at a rest-server", async () => {
    details(place({ kind: "local", provider: "unraid-folder" }), undefined);
    expect(screen.queryByRole("switch", { name: en["places.details.appendOnly"] })).toBeNull();
    cleanup();

    details(
      place({
        kind: "rest",
        provider: "rest-server",
        immutable: true,
        usage: { homeDomains: [], defaults: [], copyDomains: ["containers"], items: 0, copies: 4 },
        creds: { shared: false, fields: { user: "tower" }, set: ["password"] },
      }),
      undefined
    );
    expect(screen.getByRole("switch", { name: en["places.details.appendOnly"] }).getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["places.details.tamperTest"] }));
    });
    expect(tampered).toEqual(["containers"]);
    expect(screen.getByText(en["places.details.tamperProtected"])).toBeTruthy();
  });

  it("names the items that back up here before append-only goes off", async () => {
    details(
      place({
        kind: "rest",
        provider: "rest-server",
        immutable: true,
        usage: { homeDomains: ["vms"], defaults: [], copyDomains: [], items: 2, copies: 0 },
        creds: { shared: false, fields: { user: "tower" }, set: ["password"] },
      }),
      undefined
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: en["places.details.appendOnly"] }));
    });
    const ask = screen.getByRole("dialog");
    expect(within(ask).getByText(countText(en["places.details.appendOnlyOffAsk"], "en", 2))).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(ask).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(patches).toEqual([]);
    expect(screen.getByRole("switch", { name: en["places.details.appendOnly"] }).getAttribute("aria-checked")).toBe("true");
  });

  it("says in a toast when a domain's tamper test cannot run, and shakes the button", async () => {
    tamperAnswer = { ok: false, error: "no off-site repository" };
    details(
      place({
        kind: "rest",
        provider: "rest-server",
        immutable: true,
        usage: { homeDomains: [], defaults: [], copyDomains: ["vms"], items: 0, copies: 0 },
        creds: { shared: false, fields: { user: "tower" }, set: ["password"] },
      }),
      undefined
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["places.details.tamperTest"] }));
    });
    expect(tampered).toEqual(["vms"]);
    expect(screen.getByText("VMs: no off-site repository")).toBeTruthy();
    expect(screen.getByRole("button", { name: en["places.details.tamperTest"] }).className).toContain("glim-shake");
  });

  it("sends changed credentials together, never a blank secret", async () => {
    details();
    expect(input(en["places.field.secret"]).placeholder).toBe(en["places.details.secretKept"]);
    expect(input(en["places.field.keyId"]).value).toBe("k1");
    fireEvent.change(input(en["places.field.keyId"]), { target: { value: "k2" } });
    await settle(400);
    fireEvent.change(input(en["places.field.secret"]), { target: { value: "s2" } });
    await settle(800);
    expect(patches).toEqual([{ fields: { keyId: "k2", secret: "s2" } }]);
    expect(input(en["places.field.secret"]).value).toBe("");
  });

  it("says a place on the shared credentials gets its own set on the first change", () => {
    details(place({ creds: { shared: true, fields: { keyId: "shared" }, set: ["secret"] } }));
    expect(screen.getByLabelText(en["places.details.sharedCreds"])).toBeTruthy();
  });
});
