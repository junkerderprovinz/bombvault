// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import type { PlacementOptions, PlacementView } from "../../lib/api";
import { homeOption, placementOptions, placementView, renderWithProviders, sendToOption } from "../../lib/placement.testsupport";

vi.mock("../placeMarks", () => ({
  PlaceMark: ({ provider, onFill }: { provider: string; onFill?: boolean }) => (
    <span data-testid="mark" data-provider={provider} data-on-fill={String(!!onFill)} />
  ),
}));

const { PlacementBar } = await import("./PlacementBar");

function renderBar(view: PlacementView, options: PlacementOptions = placementOptions()) {
  const handlers = { onSegment: vi.fn(), onHome: vi.fn(), onSendTo: vi.fn(), onChip: vi.fn() };
  renderWithProviders(<PlacementBar view={view} options={options} host="Unraid" {...handlers} />);
  return handlers;
}

describe("PlacementBar", () => {
  afterEach(cleanup);

  it("shows all three segments as a toolbar, the chosen one pressed", () => {
    renderBar(placementView());
    expect(screen.getByRole("toolbar", { name: "Placement" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Local + off-site" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: "Local" }).getAttribute("aria-pressed")).toBe("false");
    expect(screen.getByRole("button", { name: "Off-site only" }).getAttribute("aria-pressed")).toBe("false");
  });

  it("locks Off-site only while the options offer nothing to send to", () => {
    renderBar(placementView(), placementOptions({ sendTo: [] }));
    expect((screen.getByRole("button", { name: "Off-site only" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("puts Stored on and the chips under Local + off-site", () => {
    renderBar(placementView());
    expect(screen.getByRole("combobox", { name: "Stored on" })).toBeTruthy();
    expect(screen.getByRole("group", { name: "Copy to" })).toBeTruthy();
    expect(screen.queryByText("Send to")).toBeNull();
  });

  it("puts only Send to under Off-site only, with a direct repository still to be made", () => {
    renderBar(placementView({ segment: "offsite-only", repoKind: "direct", repo: "direct:t-b2", skip: ["*"] }));
    expect(screen.queryByRole("combobox", { name: "Stored on" })).toBeNull();
    expect(screen.getByText("Send to")).toBeTruthy();
    expect(screen.getByText("B2 · direct · created when first chosen")).toBeTruthy();
  });

  it("keeps a stored home that is off in the list, marked and not selectable", () => {
    renderBar(placementView({ repo: "repo-cold", repoLabel: "Cold", repoKind: "local", repoOff: true }));
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect((screen.getByRole("option", { name: "Cold (off)" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("hands a send-to choice back as the option it came from", () => {
    const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox" });
    const opts = placementOptions({ sendTo: [sendToOption({ repoId: "repo-direct" }), box] });
    const { onSendTo } = renderBar(placementView({ segment: "offsite-only", repo: "repo-direct", repoKind: "direct", skip: ["*"] }), opts);
    fireEvent.click(screen.getByRole("combobox", { name: "Send to" }));
    fireEvent.click(screen.getByRole("option", { name: "Storagebox · remote" }));
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
    expect(onSendTo).toHaveBeenCalledWith(box);
  });

  it("chooses nothing while the arrow keys move along it", () => {
    const { onSegment } = renderBar(placementView());
    screen.getByRole("button", { name: "Local" }).focus();
    fireEvent.keyDown(screen.getByRole("toolbar"), { key: "ArrowRight" });
    fireEvent.keyDown(screen.getByRole("toolbar"), { key: "End" });
    expect(onSegment).not.toHaveBeenCalled();
  });

  it("names each place by its name alone, with its mark beside it", () => {
    const opts = placementOptions({
      homes: [
        homeOption({ name: "Unraid", placeId: "p-unraid", provider: "unraid-folder" }),
        homeOption({ id: "repo-nas", name: "NAS Keller", location: "nas/containers", kind: "local", placeId: "p-nas", provider: "synology" }),
      ],
    });
    renderBar(placementView(), opts);
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    const options = screen.getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["Unraid", "NAS Keller"]);
    expect(options.map((o) => o.querySelector('[data-testid="mark"]')?.getAttribute("data-provider"))).toEqual([
      "unraid-folder",
      "synology",
    ]);
  });

  it("keeps the wording of a repository without a place, and gives it no mark", () => {
    renderBar(placementView());
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect(screen.getByRole("option", { name: "NAS Keller · mounted" }).querySelector('[data-testid="mark"]')).toBeNull();
  });

  it("sends to a place by its name and mark", () => {
    const b2 = sendToOption({ kind: "remote", repoId: "repo-b2", targetId: "", name: "B2", placeId: "p-b2", provider: "b2" });
    const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox" });
    renderBar(
      placementView({ segment: "offsite-only", repo: "repo-b2", repoKind: "remote", skip: ["*"] }),
      placementOptions({ sendTo: [b2, box] })
    );
    fireEvent.click(screen.getByRole("combobox", { name: "Send to" }));
    const option = screen.getByRole("option", { name: "B2" });
    expect(option.querySelector('[data-testid="mark"]')?.getAttribute("data-provider")).toBe("b2");
  });
});
