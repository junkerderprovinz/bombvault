// @vitest-environment jsdom
// A place's details save themselves: a switch at once and back with a shake
// when refused, a typed field 800 ms after the last key, and a field still
// waiting when the details close.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { HUE_OFFSET } from "../Selector";
import type { CatalogProvider, PatchPlaceBody, Place, PlaceRefusal, TamperVerdict } from "../../lib/places";

type Answer = PlaceRefusal & { place?: Place };
type TamperAnswer = PlaceRefusal & TamperVerdict;

const patches: Partial<PatchPlaceBody>[] = [];
let answer: (body: Partial<PatchPlaceBody>) => Answer | Promise<Answer> = () => ({ ok: true });
const tamperTested: string[] = [];
let tamperAnswer: TamperAnswer = { ok: true };

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    patchPlace: (_id: string, body: Partial<PatchPlaceBody>) => {
      patches.push(body);
      return Promise.resolve(answer(body));
    },
    tamperTestPlace: (id: string) => {
      tamperTested.push(id);
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
  tamperTested.length = 0;
  tamperAnswer = { ok: true, testable: true, protected: true };
  answer = (body) => ({ ok: true, place: { ...place(), ...(body as Partial<Place>) } });
  vi.useFakeTimers();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function details(p: Place = place(), provider: CatalogProvider | undefined = B2, onSaved = vi.fn()) {
  const tree = (shown: Place) => (
    <I18nProvider>
      <ToastProvider>
        <PlaceDetails place={shown} provider={provider} hostMountRoot="/mnt" onSaved={onSaved} />
      </ToastProvider>
    </I18nProvider>
  );
  const view = render(tree(p));
  return { ...view, onSaved, update: (next: Place) => view.rerender(tree(next)) };
}

async function settle(ms = 0) {
  await act(async () => {
    vi.advanceTimersByTime(ms);
  });
}

/** Details whose saved answers come back as the new place, the way the card
 *  hands them down, and whose first answer waits for release(). */
function savingDetails(p: Place, provider: CatalogProvider | undefined, saved: (body: Partial<PatchPlaceBody>) => Place) {
  let release = () => {};
  answer = (body) => {
    answer = (next) => ({ ok: true, place: saved(next) });
    return new Promise((resolve) => {
      release = () => resolve({ ok: true, place: saved(body) });
    });
  };
  const view = details(p, provider, vi.fn((next: Place) => view.update(next)));
  return {
    release: () =>
      act(async () => {
        release();
      }),
  };
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

  it("keeps a refused name as typed, in one field, with a shake", async () => {
    answer = () => ({ ok: false, code: "place-name-taken", error: "taken" });
    details();
    fireEvent.change(input(en["places.form.name"]), { target: { value: "B2 EU" } });
    await settle(800);
    expect(screen.getByText(en["places.error.nameTaken"])).toBeTruthy();
    expect(screen.getAllByDisplayValue("B2 EU")).toHaveLength(1);
    expect(input(en["places.form.name"]).closest(".glim-shake")).toBeTruthy();
    fireEvent.change(input(en["places.form.name"]), { target: { value: "B2 EUR" } });
    expect(screen.queryByDisplayValue("B2 EU")).toBeNull();
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

  it("asks once for retention changed in several fields and saves them together", async () => {
    details(place({ usage: { homeDomains: [], defaults: [], copyDomains: [], items: 3, copies: 0 } }));
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "3" } });
    await settle(300);
    fireEvent.change(input(en["places.details.keepDaily"]), { target: { value: "5" } });
    await settle(800);
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.confirm"] }));
    });
    expect(patches).toEqual([{ retentionKeepLast: 3, retentionKeepDaily: 5 }]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("takes back every retention field when the question is declined", async () => {
    details(place({ usage: { homeDomains: [], defaults: [], copyDomains: [], items: 3, copies: 0 } }));
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "3" } });
    await settle(300);
    fireEvent.change(input(en["places.details.keepDaily"]), { target: { value: "9" } });
    await settle(800);
    await act(async () => {
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(patches).toEqual([]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(input(en["places.details.keepLast"]).value).toBe("10");
    expect(input(en["places.details.keepDaily"]).value).toBe("7");
  });

  it("keeps refused retention as typed and shakes the retention fields", async () => {
    answer = () => ({ ok: false, error: "no" });
    details();
    fireEvent.change(input(en["places.details.keepWeekly"]), { target: { value: "8" } });
    await settle(800);
    expect(patches).toEqual([{ retentionKeepWeekly: 8 }]);
    expect(input(en["places.details.keepWeekly"]).value).toBe("8");
    expect(input(en["places.details.keepMonthly"]).closest(".glim-shake")).toBeTruthy();
  });

  it("keeps retention still being typed when another field's save comes back", async () => {
    const { update } = details();
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "12" } });
    fireEvent.change(input(en["places.details.keepDaily"]), { target: { value: "9" } });
    update(place({ name: "B2 EU" }));
    expect(input(en["places.details.keepLast"]).value).toBe("12");
    expect(input(en["places.details.keepDaily"]).value).toBe("9");
    await settle(800);
    expect(patches).toEqual([{ retentionKeepLast: 12, retentionKeepDaily: 9 }]);
  });

  it("saves retention typed back while the change before it is still answering", async () => {
    const { release } = savingDetails(place(), B2, (body) => ({ ...place(), ...(body as Partial<Place>) }));
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "12" } });
    await settle(800);
    fireEvent.change(input(en["places.details.keepLast"]), { target: { value: "10" } });
    await release();
    await settle(800);
    expect(patches).toEqual([{ retentionKeepLast: 12 }, { retentionKeepLast: 10 }]);
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
    expect(tamperTested).toEqual(["p1"]);
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

  it("says in a toast when the tamper test cannot run, shakes the button and drops the last verdict", async () => {
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
    const test = () =>
      act(async () => {
        fireEvent.click(screen.getByRole("button", { name: en["places.details.tamperTest"] }));
      });
    await test();
    expect(screen.getByText(en["places.details.tamperProtected"])).toBeTruthy();
    tamperAnswer = { ok: false, code: "place-off", error: "this place is switched off" };
    await test();
    expect(screen.getByText(en["places.error.off"])).toBeTruthy();
    expect(screen.getByRole("button", { name: en["places.details.tamperTest"] }).className).toContain("glim-shake");
    expect(screen.queryByText(en["places.details.tamperProtected"])).toBeNull();
  });

  it("tests the place once and shows one verdict, whatever domains back up or copy to it", async () => {
    tamperAnswer = { ok: true, testable: true, protected: false, detail: "the server accepted a delete (HTTP 200)" };
    details(
      place({
        kind: "rest",
        provider: "rest-server",
        immutable: true,
        usage: { homeDomains: ["vms"], defaults: [], copyDomains: ["containers"], items: 1, copies: 1 },
        creds: { shared: false, fields: { user: "tower" }, set: ["password"] },
      }),
      undefined
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["places.details.tamperTest"] }));
    });
    expect(tamperTested).toEqual(["p1"]);
    expect(screen.getAllByText(`${en["places.details.tamperOpen"]}: the server accepted a delete (HTTP 200)`)).toHaveLength(1);
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

  it("keeps refused credentials as typed, in one set of fields", async () => {
    answer = () => ({ ok: false, error: "bad key" });
    details();
    fireEvent.change(input(en["places.field.keyId"]), { target: { value: "k2" } });
    await settle(800);
    expect(patches).toEqual([{ fields: { keyId: "k2" } }]);
    expect(screen.getAllByDisplayValue("k2")).toHaveLength(1);
    expect(input(en["places.field.keyId"]).closest(".glim-shake")).toBeTruthy();
  });

  it("saves a credential typed back while the change before it is still answering", async () => {
    const { release } = savingDetails(place(), B2, (body) => {
      const p = place();
      return { ...p, creds: { ...p.creds, fields: { ...p.creds.fields, ...body.fields } } };
    });
    fireEvent.change(input(en["places.field.keyId"]), { target: { value: "k2" } });
    await settle(800);
    fireEvent.change(input(en["places.field.keyId"]), { target: { value: "k1" } });
    await release();
    await settle(800);
    expect(patches).toEqual([{ fields: { keyId: "k2" } }, { fields: { keyId: "k1" } }]);
  });

  it("says a place on the shared credentials gets its own set on the first change", () => {
    details(place({ creds: { shared: true, fields: { keyId: "shared" }, set: ["secret"] } }));
    expect(screen.getByLabelText(en["places.details.sharedCreds"])).toBeTruthy();
  });

  it("locks the folder that holds backups, and every folder of a place that is a repository", () => {
    details(place({ locked: { containers: true, vms: false } }));
    const folder = (d: string) => screen.getByRole("textbox", { name: en["places.details.folderOf"].replace("{domain}", d) });
    expect(folder("Containers")).toHaveProperty("disabled", true);
    expect(folder("VMs")).toHaveProperty("disabled", false);
    expect(screen.getByLabelText(en["places.details.folderLocked"])).toBeTruthy();
    cleanup();

    details(place({ repository: true, folders: { containers: "" } }));
    for (const d of ["Containers", "VMs", "Flash"]) expect(folder(d)).toHaveProperty("disabled", true);
    expect(screen.getByLabelText(en["places.details.isRepository"])).toBeTruthy();
  });

  it("writes each domain's name on the switch that offers it", () => {
    details();
    for (const d of ["Containers", "VMs", "Flash"]) {
      const offer = screen.getByRole("switch", { name: d });
      expect(within(offer.parentElement!).getByText(d)).toBeTruthy();
    }
  });

  it("offers another domain with its default folder", async () => {
    details();
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: "Flash" }));
    });
    expect(patches).toEqual([{ folders: { containers: "container", vms: "vms", flash: "flash" } }]);
  });

  it("keeps a domain offered while a typed folder still waits to be saved", async () => {
    details();
    fireEvent.change(screen.getByRole("textbox", { name: en["places.details.folderOf"].replace("{domain}", "VMs") }), {
      target: { value: "vms2" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: "Flash" }));
    });
    await settle(800);
    expect(patches).toEqual([{ folders: { containers: "container", vms: "vms2", flash: "flash" } }]);
  });

  it("takes back only the refused domain and keeps a folder name being typed", async () => {
    answer = () => ({ ok: false, error: "no" });
    details();
    const folder = (d: string) => screen.getByRole("textbox", { name: en["places.details.folderOf"].replace("{domain}", d) }) as HTMLInputElement;
    const offer = (d: string) => screen.getByRole("switch", { name: d });
    fireEvent.change(folder("VMs"), { target: { value: "vms2" } });
    await act(async () => {
      fireEvent.click(offer("Flash"));
    });
    expect(patches).toEqual([{ folders: { containers: "container", vms: "vms2", flash: "flash" } }]);
    expect(offer("Flash").getAttribute("aria-checked")).toBe("false");
    expect(folder("VMs").value).toBe("vms2");
  });

  it("still saves a folder name being typed when the offer that carried it is refused", async () => {
    answer = (body) =>
      body.folders && "flash" in body.folders
        ? { ok: false, error: "no" }
        : { ok: true, place: { ...place(), ...(body as Partial<Place>) } };
    details();
    fireEvent.change(screen.getByRole("textbox", { name: en["places.details.folderOf"].replace("{domain}", "VMs") }), {
      target: { value: "vms2" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: "Flash" }));
    });
    await settle(800);
    expect(patches).toEqual([
      { folders: { containers: "container", vms: "vms2", flash: "flash" } },
      { folders: { containers: "container", vms: "vms2" } },
    ]);
  });
});

describe("PlaceDetails address", () => {
  const UNRAID: CatalogProvider = {
    id: "unraid-folder",
    group: "here",
    kind: "local",
    offPremises: false,
    pickRoots: ["user", ""],
    fields: [{ key: "path" }],
  };
  const local = () =>
    place({ kind: "local", provider: "unraid-folder", base: "user/bombvault", offPremises: false, creds: { shared: false, fields: {}, set: [] } });

  it("moves a local place to the folder typed, 800 ms after the last key", async () => {
    details(local(), UNRAID);
    fireEvent.change(screen.getByDisplayValue("user/bombvault"), { target: { value: "disk2/bombvault" } });
    await settle(799);
    expect(patches).toEqual([]);
    await settle(1);
    expect(patches).toEqual([{ address: { path: "disk2/bombvault" } }]);
  });

  it("keeps a new folder that is refused, and says why", async () => {
    answer = () => ({ ok: false, code: "place-location-established", error: "moved", snapshots: 9, domains: ["containers"] });
    details(local(), UNRAID);
    fireEvent.change(screen.getByDisplayValue("user/bombvault"), { target: { value: "disk2/bombvault" } });
    await settle(800);
    expect(
      screen.getByText("Backups lie at the old address (9 snapshots of Containers), and the new one does not hold the same repository.")
    ).toBeTruthy();
    expect(screen.getByDisplayValue("disk2/bombvault").closest(".glim-shake")).toBeTruthy();
  });

  it("sends nothing when the folder ends up where it was", async () => {
    details(local(), UNRAID);
    fireEvent.change(screen.getByDisplayValue("user/bombvault"), { target: { value: "user/bombvault2" } });
    await settle(400);
    fireEvent.change(screen.getByDisplayValue("user/bombvault2"), { target: { value: "user/bombvault" } });
    await settle(800);
    expect(patches).toEqual([]);
  });

  it("moves back to the folder it came from while the move away is still answering", async () => {
    const { release } = savingDetails(local(), UNRAID, (body) => ({ ...local(), base: body.address!.path! }));
    fireEvent.change(screen.getByDisplayValue("user/bombvault"), { target: { value: "disk2/bombvault" } });
    await settle(800);
    fireEvent.change(screen.getByDisplayValue("disk2/bombvault"), { target: { value: "user/bombvault" } });
    await release();
    await settle(800);
    expect(patches).toEqual([{ address: { path: "disk2/bombvault" } }, { address: { path: "user/bombvault" } }]);
  });

  it("shows the address of a remote place without a field to change it", () => {
    details();
    expect(screen.getByText("s3:https://s3.example.com/bv")).toBeTruthy();
    expect(screen.queryByDisplayValue("s3:https://s3.example.com/bv")).toBeNull();
  });
});
