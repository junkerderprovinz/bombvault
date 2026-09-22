// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import type { PlacementDomain, PlacementOptions, PlacementView } from "../../lib/api";
import {
  defaultRow,
  placementOptions,
  placementView,
  renderWithProviders,
  sendToOption,
} from "../../lib/placement.testsupport";
import { PlacementBar } from "./PlacementBar";

function renderBar(
  view: PlacementView,
  options: PlacementOptions = placementOptions(),
  context: "item" | "default" = "item",
  domain: PlacementDomain = "containers"
) {
  const handlers = { onSegment: vi.fn(), onHome: vi.fn(), onSendTo: vi.fn(), onChip: vi.fn() };
  renderWithProviders(
    <PlacementBar domain={domain} context={context} view={view} options={options} host="Unraid" {...handlers} />
  );
  return handlers;
}

// A default sitting on its target's own repository, which is where the copy
// line is shown.
function offsiteDefault(): { view: PlacementView; options: PlacementOptions } {
  const row = defaultRow({ home: "repo-direct", homeKind: "direct" });
  return {
    view: placementView({ segment: "offsite-only", repo: row.home, repoKind: "direct" }),
    options: placementOptions({ sendTo: [sendToOption({ repoId: "repo-direct" })] }),
  };
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

  it("keeps a copy line under Off-site only for a default, worded for containers", () => {
    const { view, options } = offsiteDefault();
    renderBar(view, options, "default");
    expect(screen.getByText("Project folders and items whose location is a copy source:")).toBeTruthy();
    expect(screen.getByRole("group", { name: "Project folders and items whose location is a copy source:" })).toBeTruthy();
  });

  it("leaves the project folders out of that line for a domain that has none", () => {
    const { view, options } = offsiteDefault();
    renderBar(view, { ...options, domain: "vms" }, "default", "vms");
    expect(screen.getByRole("group", { name: "Items whose location is a copy source:" })).toBeTruthy();
  });

  it("chooses nothing while the arrow keys move along it", () => {
    const { onSegment } = renderBar(placementView());
    screen.getByRole("button", { name: "Local" }).focus();
    fireEvent.keyDown(screen.getByRole("toolbar"), { key: "ArrowRight" });
    fireEvent.keyDown(screen.getByRole("toolbar"), { key: "End" });
    expect(onSegment).not.toHaveBeenCalled();
  });
});
