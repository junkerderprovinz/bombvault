// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import type { Destination, PlacementOptions, PlacementView } from "../../lib/api";
import {
  destination,
  homeOption,
  placementOptions,
  placementView,
  renderWithProviders,
  sendToOption,
  targetOption,
} from "../../lib/placement.testsupport";
import { PlacementBar } from "./PlacementBar";

const two = placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false })] });
const onB2 = placementView({ segment: "offsite-only", repo: "repo-b2-direct", repoKind: "direct", repoTarget: "t-b2", skip: ["*"] });

function renderBar(view: PlacementView, options: PlacementOptions = two, destinations: Destination[] = []) {
  const handlers = { onLocal: vi.fn(), onTarget: vi.fn(), onDestination: vi.fn(), onHome: vi.fn() };
  renderWithProviders(
    <PlacementBar
      domain="containers"
      context="item"
      view={view}
      options={options}
      host="Unraid"
      destinations={destinations}
      {...handlers}
    />
  );
  return handlers;
}

function pressed(name: string): string | null {
  return screen.getByRole("button", { name }).getAttribute("aria-pressed");
}

// A disabled button takes no pointer events, so its bubble answers on the wrapper.
function bubbleOf(el: HTMLButtonElement): string | null | undefined {
  fireEvent.mouseEnter(el.disabled ? (el.parentElement as HTMLElement) : el);
  return document.querySelector(".glim-bubble")?.textContent;
}

describe("PlacementBar", () => {
  afterEach(cleanup);

  it("announces one group of buttons with the lit ones pressed", () => {
    renderBar(placementView({ skip: ["t-hz"] }));
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.getByRole("group", { name: "Placement" })).toBeTruthy();
    expect(pressed("Local")).toBe("true");
    expect(pressed("B2")).toBe("true");
    expect(pressed("Hetzner")).toBe("false");
  });

  it("lights a direct home's own target and leaves Local dark", () => {
    renderBar(onB2, placementOptions({ targets: two.targets, sendTo: [sendToOption({ repoId: "repo-b2-direct" })] }));
    expect(pressed("Local")).toBe("false");
    expect(pressed("B2")).toBe("true");
    expect(pressed("Hetzner")).toBe("false");
  });

  it("hands each click back with the state the button is asked to take", () => {
    const { onLocal, onTarget } = renderBar(placementView({ skip: ["t-hz"] }));
    fireEvent.click(screen.getByRole("button", { name: "Local" }));
    fireEvent.click(screen.getByRole("button", { name: "B2" }));
    fireEvent.click(screen.getByRole("button", { name: "Hetzner" }));
    expect(onLocal).toHaveBeenCalledWith(false);
    expect(onTarget.mock.calls).toEqual([
      ["t-b2", false],
      ["t-hz", true],
    ]);
  });

  it("offers a button for each destination the domain has no target under yet", () => {
    const { onDestination, onTarget } = renderBar(placementView(), two, [destination()]);
    expect(pressed("Wasabi")).toBe("false");
    fireEvent.click(screen.getByRole("button", { name: "Wasabi" }));
    expect(onDestination).toHaveBeenCalledWith("dest-wasabi");
    expect(onTarget).not.toHaveBeenCalled();
  });

  it("shows a home on a remote repository as a lit button that cannot be pressed", () => {
    const box = sendToOption({ kind: "remote", repoId: "repo-box", targetId: "", name: "Storagebox" });
    const { onTarget } = renderBar(
      placementView({ segment: "offsite-only", repo: "repo-box", repoLabel: "Storagebox", repoKind: "remote", skip: ["*"] }),
      placementOptions({ targets: two.targets, sendTo: [box] })
    );
    const home = screen.getByRole("button", { name: "Storagebox · remote" }) as HTMLButtonElement;
    expect(home.getAttribute("aria-pressed")).toBe("true");
    expect(home.disabled).toBe(true);
    expect(bubbleOf(home)).toBe("Has its own credentials and is already off the premises");
    expect(pressed("Local")).toBe("false");
    fireEvent.click(home);
    expect(onTarget).not.toHaveBeenCalled();
  });

  it("marks a switched-off target and lets it go dark but not light up", () => {
    const off = placementOptions({ targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false, enabled: false })] });
    renderBar(placementView({ skip: [] }), off);
    expect((screen.getByRole("button", { name: "Hetzner (off)" }) as HTMLButtonElement).disabled).toBe(false);
    cleanup();
    renderBar(placementView({ skip: ["t-hz"] }), off);
    expect((screen.getByRole("button", { name: "Hetzner (off)" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("says on the home's button why it cannot move", () => {
    renderBar(placementView({ repo: "repo-nas", repoKind: "local", locked: true, lockReason: "first-backup" }));
    expect(bubbleOf(screen.getByRole("button", { name: "Local" }) as HTMLButtonElement)).toBe(
      "Fixed since the first backup: NAS Keller · mounted"
    );
  });

  it("offers Stored on only while Local is lit and the domain has more than one home on the server", () => {
    renderBar(placementView());
    expect(screen.getByRole("combobox", { name: "Stored on" })).toBeTruthy();
    cleanup();
    renderBar(onB2);
    expect(screen.queryByRole("combobox", { name: "Stored on" })).toBeNull();
    cleanup();
    renderBar(placementView(), placementOptions({ targets: two.targets, homes: [homeOption()] }));
    expect(screen.queryByRole("combobox", { name: "Stored on" })).toBeNull();
  });

  it("keeps a stored home that is off in the list, marked and not selectable", () => {
    renderBar(placementView({ repo: "repo-cold", repoLabel: "Cold", repoKind: "local", repoOff: true }));
    fireEvent.click(screen.getByRole("combobox", { name: "Stored on" }));
    expect((screen.getByRole("option", { name: "Cold (off)" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("chooses nothing while the arrow keys move along it", () => {
    const { onLocal, onTarget } = renderBar(placementView());
    screen.getByRole("button", { name: "Local" }).focus();
    fireEvent.keyDown(screen.getByRole("group", { name: "Placement" }), { key: "ArrowRight" });
    fireEvent.keyDown(screen.getByRole("group", { name: "Placement" }), { key: "End" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Hetzner" }));
    expect(onLocal).not.toHaveBeenCalled();
    expect(onTarget).not.toHaveBeenCalled();
  });
});
