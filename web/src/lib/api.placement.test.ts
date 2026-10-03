import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockImplementation(() =>
    Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ ok: true }) })
  );
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

function sent(i: number): { url: string; method: string; body: unknown } {
  const [url, init] = fetchMock.mock.calls[i];
  return { url, method: init?.method ?? "GET", body: init?.body ? JSON.parse(init.body) : undefined };
}

describe("placement calls", () => {
  it("patches each domain's own item route with exactly the change", async () => {
    const { setItemPlacement } = await import("./api");
    await setItemPlacement({ domain: "containers", key: "nginx" }, { copies: { skip: ["*"] } });
    await setItemPlacement({ domain: "vms", key: "win 11" }, { home: { follow: true } });
    await setItemPlacement({ domain: "files", key: "s1" }, { home: { repo: "" } });
    expect([sent(0), sent(1), sent(2)]).toEqual([
      { url: "/api/containers/nginx", method: "PATCH", body: { copies: { skip: ["*"] } } },
      { url: "/api/vms/win%2011", method: "PATCH", body: { home: { follow: true } } },
      { url: "/api/files/sets/s1", method: "PATCH", body: { home: { repo: "" } } },
    ]);
  });

  it("previews an item's change on the items route", async () => {
    const { previewItemPlacement } = await import("./api");
    await previewItemPlacement({ domain: "vms", key: "win 11" }, { copies: { follow: true } });
    expect(sent(0)).toEqual({ url: "/api/items/vms/win%2011/placement/preview", method: "POST", body: { copies: { follow: true } } });
  });

  it("names the target of a new-target preview only when there is one", async () => {
    const { getNewTargetPreview } = await import("./api");
    await getNewTargetPreview("vms", "b2:bucket:vms");
    await getNewTargetPreview("vms", "b2:bucket:vms", "t1");
    expect([sent(0).url, sent(1).url]).toEqual([
      "/api/placement/new-target-preview?domain=vms&repo=b2%3Abucket%3Avms",
      "/api/placement/new-target-preview?domain=vms&repo=b2%3Abucket%3Avms&target=t1",
    ]);
  });

  it("puts a default together with the numbers it was shown", async () => {
    const { putPlacementDefault } = await import("./api");
    const expect_ = { dropped: [], added: [], openTakeHome: 2, home: "", skip: [] };
    await putPlacementDefault("files", { skip: ["*"] }, expect_);
    expect(sent(0)).toEqual({ url: "/api/placement/default/files", method: "PUT", body: { skip: ["*"], expect: expect_ } });
  });

  it("creates a direct repository beside its target", async () => {
    const { createDirectRepo } = await import("./api");
    await createDirectRepo("t1", "", "b2:bucket:vms-direct");
    expect(sent(0)).toEqual({ url: "/api/repos", method: "POST", body: { name: "", repo: "b2:bucket:vms-direct", companionOf: "t1" } });
  });

  it("sends the answer to the new-target question only when there is one", async () => {
    const { acceptMeshOffer, createOffsiteTarget } = await import("./api");
    const target = { domain: "vms", name: "B2", repo: "b2:bucket:vms" } as Parameters<typeof createOffsiteTarget>[0];
    const answer = { identities: ["vm:win11"], default: false };
    await createOffsiteTarget(target);
    await createOffsiteTarget(target, answer);
    await acceptMeshOffer("o1", "vms", answer);
    expect(sent(0).body).not.toHaveProperty("alsoExclude");
    expect(sent(1).body).toMatchObject({ alsoExclude: answer });
    expect(sent(2)).toEqual({ url: "/api/fleet/mesh-offers/o1/accept", method: "POST", body: { domain: "vms", alsoExclude: answer } });
  });

  it("leaves items out of the field's target through the exclude route", async () => {
    const { excludeFromTarget } = await import("./api");
    const body = { domain: "containers", field: true, identities: ["container:plex"], default: false } as const;
    await excludeFromTarget(body);
    expect(sent(0)).toEqual({ url: "/api/placement/exclude", method: "POST", body });
  });
});
