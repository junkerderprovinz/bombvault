// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import {
  homeOption,
  olderCopies,
  placementObserved,
  placementOptions,
  placementPlan,
  placementView,
  renderWithProviders,
  stackNote,
} from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () =>
  (await import("../../lib/placement.testsupport")).createPlacementApi()
);

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
}));

const { PlacementStatus } = await import("./PlacementStatus");

const item = { domain: "containers", key: "nginx" } as const;

describe("PlacementStatus", () => {
  beforeEach(() => fake.reset());
  afterEach(cleanup);

  it("shows the plan, the state and the 3-2-1 mark", async () => {
    const view = placementView({ plan: placementPlan({ home: "NAS Keller" }), observed: placementObserved() });
    renderWithProviders(<PlacementStatus item={item} name="nginx" view={view} onChanged={vi.fn()} />);
    expect(await screen.findByText("On NAS Keller.")).toBeTruthy();
    expect(screen.getByText("Copied to B2.")).toBeTruthy();
    expect(screen.getByText("At 2 sites")).toBeTruthy();
    expect(screen.getByText("3-2-1 met")).toBeTruthy();
  });

  it("names the domain path after the place it lies at", async () => {
    fake.reply("getPlacementOptions", {
      ok: true,
      options: placementOptions({ domain: "files", homes: [homeOption({ name: "Pool", placeId: "p-pool", provider: "unraid-folder" })] }),
    });
    const view = placementView({ plan: placementPlan({ home: "" }) });
    renderWithProviders(<PlacementStatus item={{ domain: "files", key: "docs" }} name="Documents" view={view} onChanged={vi.fn()} />);
    expect(await screen.findByText("On Pool.")).toBeTruthy();
  });

  it("offers to delete older copies at a target no longer ticked", async () => {
    const view = placementView({ observed: placementObserved({ older: [olderCopies()] }) });
    renderWithProviders(<PlacementStatus item={item} name="nginx" view={view} onChanged={vi.fn()} />);
    const date = new Date(1_758_000_000 * 1000).toLocaleDateString();
    expect(await screen.findByText(`Older copies at Hetzner: 14, last seen ${date}`)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Delete in Hetzner" }));
    await waitFor(() => expect(fake.callsTo("getOffsiteRemoval")).toEqual([[item, "t-hz"]]));
  });

  it("locks the button at an append-only target and says why where a keyboard reaches it", async () => {
    const view = placementView({ observed: placementObserved({ older: [olderCopies({ appendOnly: true })] }) });
    renderWithProviders(<PlacementStatus item={item} name="nginx" view={view} onChanged={vi.fn()} />);
    const button = (await screen.findByRole("button", { name: "Delete in Hetzner" })) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    const why = screen.getByLabelText("Hetzner is append-only. Nothing here may delete from it.");
    expect(why.getAttribute("tabindex")).toBe("0");
  });

  it("tells the card once the copies are gone", async () => {
    const onChanged = vi.fn();
    const view = placementView({ observed: placementObserved({ older: [olderCopies()] }) });
    renderWithProviders(<PlacementStatus item={item} name="nginx" view={view} onChanged={onChanged} />);
    fireEvent.click(await screen.findByRole("button", { name: "Delete in Hetzner" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete in Hetzner" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("names the project folder when its copies differ", async () => {
    renderWithProviders(
      <PlacementStatus item={item} name="nginx" view={placementView({ stackNote: stackNote() })} onChanged={vi.fn()} />
    );
    expect(
      await screen.findByText("Project folder immich: on Unraid, copied to B2 (follows the containers default)")
    ).toBeTruthy();
  });
});
