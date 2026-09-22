// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());
const listed = vi.hoisted(() => ({
  targets: [
    { id: "t1", enabled: true },
    { id: "t2", enabled: true },
  ] as unknown[],
}));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: listed.targets }),
}));

const { DiscoverFindings } = await import("./DiscoverFindings");
const { subscribeRepos } = await import("../../lib/useNamedRepos");

const oldB2 = {
  repoId: "r1",
  name: "B2 old",
  targets: [
    { id: "t1", name: "B2" },
    { id: "t2", name: "B2 VMs" },
  ],
};

describe("DiscoverFindings", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("names the paused domains and where their default is confirmed", () => {
    renderWithProviders(<DiscoverFindings paused={["containers", "files"]} leftOpen={[]} directRepos={[]} />);
    expect(
      screen.getByText(
        "Off-site copies for Containers and Folders are paused until the default is confirmed under Settings > Paths & Storage > Placement defaults."
      )
    ).toBeTruthy();
  });

  it("names the items whose location was left open", () => {
    renderWithProviders(<DiscoverFindings paused={[]} leftOpen={["nginx", "Photos"]} directRepos={[]} />);
    expect(screen.getByText("Location left open because a backup was running: nginx and Photos")).toBeTruthy();
  });

  it("connects a found repository with the chosen target", async () => {
    const seen = vi.fn();
    const off = subscribeRepos(seen);
    renderWithProviders(<DiscoverFindings paused={[]} leftOpen={[]} directRepos={[oldB2]} />);
    expect(screen.getByText("B2 old holds backups written directly to a target. Its retention is on hold.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Connect with B2 VMs" }));
    expect(await screen.findByText("B2 old is now the direct repository of B2 VMs.")).toBeTruthy();
    expect(fake.callsTo("connectRepo")).toEqual([["r1", "t2"]]);
    await waitFor(() => expect(screen.queryByRole("button", { name: "Connect with B2" })).toBeNull());
    expect(seen).toHaveBeenCalledTimes(1);
    off();
  });

  it("says why a connection was refused and keeps the offer", async () => {
    fake.reply("connectRepo", { ok: false, error: "taken", code: "companion-taken" });
    renderWithProviders(<DiscoverFindings paused={[]} leftOpen={[]} directRepos={[oldB2]} />);
    fireEvent.click(screen.getByRole("button", { name: "Connect with B2" }));
    expect(await screen.findByText("This target already has a direct repository.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Connect with B2" })).toBeTruthy();
  });

  it("only offers a target that is switched on", async () => {
    listed.targets = [
      { id: "t1", enabled: true },
      { id: "t2", enabled: false },
    ];
    renderWithProviders(<DiscoverFindings paused={[]} leftOpen={[]} directRepos={[oldB2]} />);
    expect(screen.getByRole("button", { name: "Connect with B2" })).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Connect with B2 VMs" })).toBeNull());
    expect(screen.getByRole("button", { name: "Connect with B2" })).toBeTruthy();
  });
});
