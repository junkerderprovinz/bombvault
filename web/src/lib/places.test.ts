// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  PLACES_CHANGED,
  adoptRow,
  createPlace,
  deletePlace,
  ensurePlaceRepo,
  getPlacesCatalog,
  getStorageDomains,
  listPlaces,
  patchPlace,
  placesChanged,
  previewDomainCopies,
  previewDomainHome,
  probePlace,
  restServerRecipe,
  setDomainCopies,
  setDomainHome,
  subscribePlaces,
  tamperTestPlace,
  testPlace,
  type CopiesPreview,
  type HomePreview,
} from "./places";

type Call = { url: string; method: string; body: unknown };

function recordFetch(): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal("fetch", (url: string, init?: RequestInit) => {
    calls.push({ url, method: init?.method ?? "GET", body: init?.body ? JSON.parse(String(init.body)) : undefined });
    return Promise.resolve(new Response(JSON.stringify({ ok: true }), { status: 200 }));
  });
  return calls;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("the places client", () => {
  it("reads the catalog, the places and the domain rows", async () => {
    const calls = recordFetch();
    await getPlacesCatalog();
    await listPlaces();
    await getStorageDomains();
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      "GET /api/places/catalog",
      "GET /api/places",
      "GET /api/storage/domains",
    ]);
  });

  it("sends each place write to its route with its body", async () => {
    const calls = recordFetch();
    await probePlace({ provider: "b2", fields: { keyId: "k" } });
    await createPlace({ provider: "b2", fields: { keyId: "k" }, name: "B2" });
    await patchPlace("p/1", { retentionKeepLast: 3 });
    await deletePlace("p/1");
    await testPlace("p/1");
    await tamperTestPlace("p/1");
    await adoptRow("p/1", "", "flash");
    await ensurePlaceRepo("p/1", "vms");
    expect(calls).toEqual([
      { url: "/api/places/probe", method: "POST", body: { provider: "b2", fields: { keyId: "k" } } },
      { url: "/api/places", method: "POST", body: { provider: "b2", fields: { keyId: "k" }, name: "B2" } },
      { url: "/api/places/p%2F1", method: "PATCH", body: { retentionKeepLast: 3 } },
      { url: "/api/places/p%2F1", method: "DELETE", body: undefined },
      { url: "/api/places/p%2F1/test", method: "POST", body: undefined },
      { url: "/api/places/p%2F1/tamper-test", method: "POST", body: undefined },
      { url: "/api/places/p%2F1/adopt", method: "POST", body: { rowId: "", domain: "flash" } },
      { url: "/api/places/p%2F1/repo", method: "POST", body: { domain: "vms" } },
    ]);
  });

  it("previews before it writes a domain row", async () => {
    const calls = recordFetch();
    const home = { mode: "default", placeId: "p1", homePlace: "p0", homeHasBackups: true, backups: 0 } as HomePreview;
    const copies = { placeId: "p1", on: true, skip: [], enabled: false } as CopiesPreview;
    await previewDomainHome("vms", "p1");
    await setDomainHome("vms", { placeId: "p1", expect: home, applyToOpen: true });
    await previewDomainCopies("vms", "p1", true);
    await setDomainCopies("vms", { placeId: "p1", on: true, expect: copies });
    expect(calls).toEqual([
      { url: "/api/storage/domains/vms/home/preview", method: "POST", body: { placeId: "p1" } },
      { url: "/api/storage/domains/vms/home", method: "PUT", body: { placeId: "p1", expect: home, applyToOpen: true } },
      { url: "/api/storage/domains/vms/copies/preview", method: "POST", body: { placeId: "p1", on: true } },
      { url: "/api/storage/domains/vms/copies", method: "PUT", body: { placeId: "p1", on: true, expect: copies } },
    ]);
  });

  it("asks for a rest-server recipe", async () => {
    const calls = recordFetch();
    await restServerRecipe();
    expect(calls).toEqual([{ url: "/api/places/rest-server-recipe", method: "GET", body: undefined }]);
  });
});

describe("the places event", () => {
  it("reaches every subscriber until it unsubscribes", () => {
    const seen = vi.fn();
    const stop = subscribePlaces(seen);
    placesChanged();
    window.dispatchEvent(new Event(PLACES_CHANGED));
    stop();
    placesChanged();
    expect(PLACES_CHANGED).toBe("bv:places-changed");
    expect(seen).toHaveBeenCalledTimes(2);
  });
});
