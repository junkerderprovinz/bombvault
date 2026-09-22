// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen } from "@testing-library/react";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());
const listed = vi.hoisted(() => ({ targets: [] as unknown[] }));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: listed.targets }),
}));

const { PlacementFlow } = await import("./PlacementFlow");

describe("PlacementFlow", () => {
  afterEach(cleanup);

  it("names this server and every enabled target it copies to", async () => {
    listed.targets = [
      { id: "t1", domain: "flash", name: "B2", repo: "b2:bucket:flash", enabled: true, sortOrder: 0 },
      { id: "t2", domain: "flash", name: "", repo: "sftp:u1@box:/flash", enabled: true, sortOrder: 1 },
      { id: "t3", domain: "flash", name: "Old", repo: "s3:old", enabled: false, sortOrder: 2 },
    ];
    renderWithProviders(<PlacementFlow domain="flash" />);
    expect(await screen.findByText("Unraid → B2 and sftp:u1@box:/flash")).toBeTruthy();
  });

  it("says nothing without an enabled target", async () => {
    listed.targets = [];
    const { container } = renderWithProviders(<PlacementFlow domain="config" />);
    await act(async () => {});
    expect(container.textContent).toBe("");
  });
});
