// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { OkEnvelope } from "../../lib/api";
import { PLACES_CHANGED, type CatalogProvider, type CreatePlaceBody, type ProbeResult } from "../../lib/places";

const CATALOG: CatalogProvider[] = [
  { id: "b2", group: "cloud", kind: "s3", offPremises: true, fields: [{ key: "keyId" }, { key: "secret", secret: true }] },
  {
    id: "wasabi",
    group: "cloud",
    kind: "s3",
    offPremises: true,
    fields: [{ key: "keyId" }, { key: "secret", secret: true }, { key: "bucket", optional: true }],
  },
  { id: "minio", group: "self", kind: "s3", fields: [{ key: "endpoint" }, { key: "keyId" }, { key: "secret", secret: true }] },
  { id: "unraid-folder", group: "here", kind: "local", offPremises: false, pickRoots: ["user", ""], fields: [{ key: "path" }] },
  { id: "bombvault", group: "self", kind: "rest", fields: [{ key: "url" }, { key: "user" }, { key: "password", secret: true }] },
];
const creates: CreatePlaceBody[] = [];
let catalog: () => Promise<OkEnvelope & { providers?: CatalogProvider[] }>;
let probeAnswer: OkEnvelope & ProbeResult;

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    getPlacesCatalog: () => catalog(),
    probePlace: () => Promise.resolve(probeAnswer),
    createPlace: (body: CreatePlaceBody) => {
      creates.push(body);
      return Promise.resolve({ ok: true, place: { id: "p1", name: body.name } });
    },
  };
});

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  listMeshOffers: () =>
    Promise.resolve({
      ok: true,
      offers: [
        { id: "o1", from: "DXP480T", suggestedDomain: "vms", repo: "rest:http://192.0.2.5:8000/bv/vms", restUser: "bv", status: "pending", receivedAt: 1_758_170_400 },
      ],
    }),
  getNewTargetPreview: () =>
    Promise.resolve({
      ok: true,
      preview: { items: 4, formerlyExcluded: [{ identity: "vm:win11", skip: [] }], defaultExcludes: true, snapshots: 30, bytes: null, unreadable: [] },
    }),
  acceptMeshOffer: () => Promise.resolve({ ok: true, target: { id: "t9" }, place: { id: "p9", name: "mesh: DXP480T" } }),
}));

const { AddPlaceDialog } = await import("./AddPlaceDialog");

beforeEach(() => {
  creates.length = 0;
  catalog = () => Promise.resolve({ ok: true, providers: CATALOG });
  probeAnswer = { ok: true, base: "s3:https://s3.eu-central-003.backblazeb2.com/bv" };
});
afterEach(() => {
  cleanup();
  localStorage.clear();
});

async function open(onClose = vi.fn(), onAdded = vi.fn()) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <AddPlaceDialog hostMountRoot="/mnt" onClose={onClose} onAdded={onAdded} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  return { onClose, onAdded };
}

const dialog = () => screen.getByRole("dialog", { name: en["places.addTitle"] });
const tile = (name: string) => within(dialog()).getByRole("option", { name });
const button = (key: keyof typeof en) => within(dialog()).getByRole("button", { name: en[key] });
async function click(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

describe("AddPlaceDialog", () => {
  it("is a window with its title fixed above the tiles, which alone scroll", async () => {
    await open();
    const title = within(dialog()).getByRole("heading", { name: en["places.addTitle"] });
    const scroller = within(dialog()).getByRole("listbox").closest(".overflow-y-auto");
    expect(scroller).toBeTruthy();
    expect(scroller!.contains(title)).toBe(false);
    expect(dialog().parentElement!.className).toContain("glim-modal-backdrop");
    expect(within(dialog()).getAllByRole("group").map((g) => g.textContent?.includes(en["places.group.cloud"]))).toEqual([true, false, false]);
  });

  it("says so in the window while the providers load and when they could not be read", async () => {
    let answer: (r: OkEnvelope) => void = () => {};
    catalog = () => new Promise((resolve) => (answer = resolve));
    await open();
    expect(within(dialog()).getByText(en["places.catalogLoading"])).toBeTruthy();
    expect(within(dialog()).queryByRole("listbox")).toBeNull();
    await act(async () => answer({ ok: false, error: "database is locked" }));
    expect(within(dialog()).getByRole("alert").textContent).toBe(en["places.catalogFailed"]);
    expect(within(dialog()).queryByRole("listbox")).toBeNull();
    expect(screen.queryByText("database is locked")).toBeNull();
  });

  it("opens the form of the picked tile in the same window, and Back returns to that tile", async () => {
    await open();
    await click(tile("Backblaze B2"));
    expect(within(dialog()).queryByRole("listbox")).toBeNull();
    expect(within(dialog()).getByText("Backblaze B2")).toBeTruthy();
    await click(button("places.form.back"));
    expect(tile("Backblaze B2").getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(tile("Backblaze B2"));
  });

  it("discards everything on Cancel, Escape and the backdrop", async () => {
    const { onClose } = await open();
    fireEvent.click(button("common.cancel"));
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(dialog().parentElement!);
    expect(onClose).toHaveBeenCalledTimes(3);
    expect(creates).toEqual([]);
  });

  it("closes only the bucket list on Escape and keeps what was typed", async () => {
    probeAnswer = { ok: true, buckets: ["alpha", "beta"] };
    const { onClose } = await open();
    await click(tile("Wasabi"));
    fireEvent.change(within(dialog()).getByLabelText(en["places.field.secret"]), { target: { value: "s3cret" } });
    await click(button("places.form.test"));
    await click(within(dialog()).getByRole("combobox", { name: en["places.field.bucket"] }));
    const list = screen.getByRole("listbox", { name: en["places.field.bucket"] });
    fireEvent.keyDown(within(list).getByRole("option", { name: "alpha" }), { key: "Escape" });
    expect(screen.queryByRole("listbox", { name: en["places.field.bucket"] })).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
    expect(within(dialog()).getByLabelText(en["places.field.secret"])).toHaveProperty("value", "s3cret");
  });

  it("closes an open info bubble on Escape and keeps what was typed", async () => {
    const { onClose } = await open();
    await click(tile("Wasabi"));
    const secret = within(dialog()).getByLabelText(en["places.field.secret"]);
    fireEvent.change(secret, { target: { value: "s3cret" } });
    fireEvent.mouseEnter(within(dialog()).getByLabelText(en["places.intro.s3Cloud"]));
    expect(screen.getByRole("tooltip").textContent).toBe(en["places.intro.s3Cloud"]);

    fireEvent.keyDown(secret, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
    expect(secret).toHaveProperty("value", "s3cret");

    fireEvent.keyDown(secret, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes once the place is added, and says where to choose it", async () => {
    const changed = vi.fn();
    window.addEventListener(PLACES_CHANGED, changed);
    const { onClose, onAdded } = await open();
    await click(tile("Backblaze B2"));
    await click(button("places.form.test"));
    await click(button("places.form.add"));
    expect(creates).toHaveLength(1);
    expect(onAdded).toHaveBeenCalledWith({ id: "p1", name: "Backblaze B2" });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(screen.getByText(en["places.added"].replace("{name}", "Backblaze B2"))).toBeTruthy();
    window.removeEventListener(PLACES_CHANGED, changed);
  });

  it("says where to choose the new place in quiet mode too, under the name as typed", async () => {
    localStorage.setItem("bombvault.quietToasts", "1");
    await open();
    await click(tile("Backblaze B2"));
    await click(button("places.form.test"));
    fireEvent.change(within(dialog()).getByLabelText(en["places.form.name"]), { target: { value: "B2 $& $' co" } });
    await click(button("places.form.add"));
    expect(screen.getByText(en["places.added"].replace("{name}", () => "B2 $& $' co"))).toBeTruthy();
  });
});

describe("AddPlaceDialog offers", () => {
  async function acceptOffer() {
    const opened = await open();
    await click(tile(en["places.provider.bombvault"]));
    await click(await within(dialog()).findByRole("button", { name: en["fleet.mesh.accept"] }));
    return { ...opened, question: screen.getByRole("dialog", { name: en["confirmDialog.title"] }) };
  }

  it("answers Escape in the accept question and keeps the window", async () => {
    const { onClose } = await acceptOffer();
    await act(async () => {
      fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    });
    expect(screen.queryByRole("dialog", { name: en["confirmDialog.title"] })).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
    expect(within(dialog()).getByRole("button", { name: en["fleet.mesh.accept"] })).toBeTruthy();
  });

  it("hands focus back to Accept when the accept question is declined", async () => {
    await acceptOffer();
    await act(async () => {
      fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    });
    expect(document.activeElement).toBe(within(dialog()).getByRole("button", { name: en["fleet.mesh.accept"] }));
  });

  it("leaves the one accent button to the form while an offer is open", async () => {
    await open();
    await click(tile(en["places.provider.bombvault"]));
    await within(dialog()).findByRole("button", { name: en["fleet.mesh.accept"] });
    const accented = within(dialog())
      .getAllByRole("button")
      .filter((b) => /\bbg-accent\b/.test(b.className));
    expect(accented).toEqual([button("places.form.test")]);
  });

  it("closes once an offer is accepted, and says the place keeps its copies already", async () => {
    const changed = vi.fn();
    window.addEventListener(PLACES_CHANGED, changed);
    const { onClose, onAdded, question } = await acceptOffer();
    await click(within(question).getByRole("button", { name: en["common.confirm"] }));
    expect(onAdded).toHaveBeenCalledWith({ id: "p9", name: "mesh: DXP480T" });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(screen.getByText(en["places.offerAccepted"].replace("{name}", "mesh: DXP480T"))).toBeTruthy();
    expect(screen.queryByText(en["places.added"].replace("{name}", "mesh: DXP480T"))).toBeNull();
    window.removeEventListener(PLACES_CHANGED, changed);
  });
});
