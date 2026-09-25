// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { PLACES_CHANGED, type CatalogProvider, type CreatePlaceBody } from "../../lib/places";

const CATALOG: CatalogProvider[] = [
  { id: "b2", group: "cloud", kind: "s3", offPremises: true, fields: [{ key: "keyId" }, { key: "secret", secret: true }] },
  { id: "minio", group: "self", kind: "s3", fields: [{ key: "endpoint" }, { key: "keyId" }, { key: "secret", secret: true }] },
  { id: "unraid-folder", group: "here", kind: "local", offPremises: false, pickRoots: ["user", ""], fields: [{ key: "path" }] },
];
const creates: CreatePlaceBody[] = [];

vi.mock("../../lib/places", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/places")>();
  return {
    ...actual,
    getPlacesCatalog: () => Promise.resolve({ ok: true, providers: CATALOG }),
    probePlace: () => Promise.resolve({ ok: true, base: "s3:https://s3.eu-central-003.backblazeb2.com/bv" }),
    createPlace: (body: CreatePlaceBody) => {
      creates.push(body);
      return Promise.resolve({ ok: true, place: { id: "p1", name: body.name } });
    },
  };
});

const { AddPlaceDialog } = await import("./AddPlaceDialog");

beforeEach(() => {
  creates.length = 0;
});
afterEach(cleanup);

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

  it("opens the form of the picked tile in the same window, and Back returns to that tile", async () => {
    await open();
    await act(async () => {
      fireEvent.click(tile("Backblaze B2"));
    });
    expect(within(dialog()).queryByRole("listbox")).toBeNull();
    expect(within(dialog()).getByText("Backblaze B2")).toBeTruthy();
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["places.form.back"] }));
    });
    expect(tile("Backblaze B2").getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(tile("Backblaze B2"));
  });

  it("discards everything on Cancel, Escape and the backdrop", async () => {
    const { onClose } = await open();
    fireEvent.click(within(dialog()).getByRole("button", { name: en["common.cancel"] }));
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(dialog().parentElement!);
    expect(onClose).toHaveBeenCalledTimes(3);
    expect(creates).toEqual([]);
  });

  it("closes once the place is added, and says where to choose it", async () => {
    const changed = vi.fn();
    window.addEventListener(PLACES_CHANGED, changed);
    const { onClose, onAdded } = await open();
    await act(async () => {
      fireEvent.click(tile("Backblaze B2"));
    });
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["places.form.test"] }));
    });
    await act(async () => {
      fireEvent.click(within(dialog()).getByRole("button", { name: en["places.form.add"] }));
    });
    expect(creates).toHaveLength(1);
    expect(onAdded).toHaveBeenCalledWith({ id: "p1", name: "Backblaze B2" });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(screen.getByText(en["places.added"].replace("{name}", "Backblaze B2"))).toBeTruthy();
    window.removeEventListener(PLACES_CHANGED, changed);
  });
});
