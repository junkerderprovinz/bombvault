// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, waitFor } from "@testing-library/react";
import type { Place } from "../../lib/places";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());
const listed = vi.hoisted(() => ({ targets: [] as unknown[], places: [] as unknown[] }));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: listed.targets }),
}));

vi.mock("../../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places: listed.places, unplaced: [] }),
}));

vi.mock("../placeMarks", () => ({
  PlaceMark: ({ provider }: { provider: string }) => <span data-testid="mark" data-provider={provider} />,
}));

const { PlacementFlow } = await import("./PlacementFlow");

function place(id: string, name: string, provider: string, homeDomains: string[] = []): Place {
  return { id, name, provider, usage: { homeDomains, defaults: [], copyDomains: [], items: 0, copies: 0, repositories: 0 } } as unknown as Place;
}

describe("PlacementFlow", () => {
  afterEach(cleanup);

  it("names this server and every enabled target it copies to", async () => {
    listed.places = [];
    listed.targets = [
      { id: "t1", domain: "flash", name: "B2", repo: "b2:bucket:flash", enabled: true, sortOrder: 0 },
      { id: "t2", domain: "flash", name: "", repo: "sftp:u1@box:/flash", enabled: true, sortOrder: 1 },
      { id: "t3", domain: "flash", name: "Old", repo: "s3:old", enabled: false, sortOrder: 2 },
    ];
    const { container } = renderWithProviders(<PlacementFlow domain="flash" />);
    await waitFor(() => expect(container.textContent).toBe("Unraid → B2 and sftp:u1@box:/flash"));
    expect(container.querySelector('[data-testid="mark"]')).toBeNull();
  });

  it("names the place the domain is stored at and marks each place", async () => {
    listed.places = [place("p-unraid", "Unraid array", "unraid-folder", ["flash"]), place("p-b2", "B2 Backups", "b2")];
    listed.targets = [
      { id: "t1", domain: "flash", name: "B2", repo: "s3:https://s3.example.com/bv/flash", enabled: true, sortOrder: 0, placeId: "p-b2" },
      { id: "t2", domain: "flash", name: "Box", repo: "sftp:u1@box:/flash", enabled: true, sortOrder: 1 },
    ];
    const { container } = renderWithProviders(<PlacementFlow domain="flash" />);
    await waitFor(() => expect(container.textContent).toBe("Unraid array → B2 and Box"));
    const marks = [...container.querySelectorAll('[data-testid="mark"]')].map((m) => m.getAttribute("data-provider"));
    expect(marks).toEqual(["unraid-folder", "b2"]);
  });

  it("says nothing without an enabled target", async () => {
    listed.targets = [];
    const { container } = renderWithProviders(<PlacementFlow domain="config" />);
    await act(async () => {});
    expect(container.textContent).toBe("");
  });
});
