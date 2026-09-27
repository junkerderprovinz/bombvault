// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import type { PlacementOptions, PlacementView } from "../../lib/api";
import { en } from "../../lib/i18n";
import { stepForHome, stepForSendTo } from "../../lib/placement";
import { homeOption, placementOptions, placementView, renderWithProviders, sendToOption } from "../../lib/placement.testsupport";

vi.mock("../placeMarks", () => ({
  PlaceMark: ({ provider, onFill }: { provider: string; onFill?: boolean }) => (
    <span data-testid="mark" data-provider={provider} data-on-fill={String(!!onFill)} />
  ),
}));

const { PlacementBar } = await import("./PlacementBar");

const t = ((key: string) => en[key as keyof typeof en] ?? key) as never;

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

describe("PlacementBar with places that hold no repository yet", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  const unraid = homeOption({ name: "Unraid", placeId: "p-unraid", provider: "unraid-folder" });
  const nas = homeOption({ name: "NAS Keller", kind: "local", location: "nas/containers", placeId: "p-nas", provider: "synology" });
  const cold = homeOption({ id: "repo-cold", name: "Cold", kind: "local", location: "/mnt/remotes/cold", placeId: "p-cold" });
  const b2 = sendToOption({ kind: "remote", repoId: "", targetId: "", name: "B2", placeId: "p-b2", provider: "b2" });
  const hetzner = sendToOption({ repoId: "", targetId: "t-hz", name: "Hetzner", placeId: "p-hz", provider: "hetzner" });
  const box = sendToOption({ repoId: "", targetId: "t-box", name: "Storagebox" });
  const options = placementOptions({ homes: [unraid, nas, cold], sendTo: [b2, hetzner, box] });

  function choose(field: string, name: string) {
    fireEvent.click(screen.getByRole("combobox", { name: field }));
    fireEvent.click(screen.getByRole("option", { name }));
    fireEvent.click(screen.getByRole("button", { name: "Set" }));
  }

  it("keeps a local place apart from the domain path and hands each choice to its own step", () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const view = placementView({ repo: "repo-cold", repoKind: "local", repoLabel: "Cold" });
    const { onHome } = renderBar(view, options);
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["Unraid", "NAS Keller", "Cold"]);
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect(errors.mock.calls.flat().join(" ")).not.toMatch(/same key/);

    choose("Stored on", "NAS Keller");
    choose("Stored on", "Unraid");
    expect(onHome.mock.calls).toEqual([["place:p-nas"], [""]]);

    const [toPlace, toPath] = onHome.mock.calls.map(([key]) => stepForHome(key, view, options, t, "Unraid"));
    expect(toPlace).toMatchObject({ kind: "place", placeId: "p-nas", repoKind: "local", offsiteOnly: false });
    expect(toPath).toMatchObject({ kind: "save", change: { home: { repo: "" } } });
  });

  it("sends a remote place and a target at a place through the place, and only a target without one to the direct dialog", () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const view = placementView({ segment: "offsite-only", repo: "repo-old", repoKind: "remote", repoLabel: "Old", skip: ["*"] });
    const { onSendTo } = renderBar(view, options);
    fireEvent.click(screen.getByRole("combobox", { name: "Send to" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent).slice(0, 3)).toEqual([
      "B2",
      "Hetzner",
      "Storagebox · direct · created when first chosen",
    ]);
    fireEvent.click(screen.getByRole("combobox", { name: "Send to" }));
    expect(errors.mock.calls.flat().join(" ")).not.toMatch(/same key/);

    choose("Send to", "B2");
    choose("Send to", "Hetzner");
    choose("Send to", "Storagebox · direct · created when first chosen");
    expect(onSendTo.mock.calls).toEqual([[b2], [hetzner], [box]]);

    const steps = onSendTo.mock.calls.map(([opt]) => stepForSendTo(opt, view, t));
    expect(steps[0]).toMatchObject({ kind: "place", placeId: "p-b2", repoKind: "remote" });
    expect(steps[1]).toMatchObject({ kind: "place", placeId: "p-hz", repoKind: "direct" });
    expect(steps[2]).toEqual({ kind: "direct", target: box });
  });
});
