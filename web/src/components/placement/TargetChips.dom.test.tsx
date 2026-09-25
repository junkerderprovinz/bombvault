// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { en } from "../../lib/i18n";
import { placementOptions, placementView, renderWithProviders, targetOption } from "../../lib/placement.testsupport";

vi.mock("../placeMarks", () => ({
  PlaceMark: ({ provider, onFill }: { provider: string; onFill?: boolean }) => (
    <span data-testid="mark" data-provider={provider} data-on-fill={String(!!onFill)} />
  ),
}));

const { TargetChips } = await import("./TargetChips");

// A disabled button takes no pointer events, so its bubble answers on the wrapper.
function bubbleOf(el: HTMLElement): string | null | undefined {
  fireEvent.mouseEnter((el as HTMLButtonElement).disabled ? (el.parentElement as HTMLElement) : el);
  return document.querySelector(".glim-bubble")?.textContent;
}

describe("TargetChips", () => {
  afterEach(cleanup);

  const two = placementOptions({
    targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false, enabled: false })],
  });

  it("puts every target in one row, a switched-off one dimmed with its stored tick", () => {
    renderWithProviders(
      <TargetChips label="Copy to" targets={two.targets} view={placementView()} options={two} onToggle={vi.fn()} />
    );
    const off = screen.getByRole("button", { name: "Hetzner (off)" }) as HTMLButtonElement;
    expect(off.disabled).toBe(true);
    expect(off.getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("group", { name: "Copy to" })).toBeTruthy();
  });

  it("locks the last ticked enabled target with the way out", () => {
    renderWithProviders(
      <TargetChips label="Copy to" targets={two.targets} view={placementView()} options={two} onToggle={vi.fn()} />
    );
    const b2 = screen.getByRole("button", { name: "B2" }) as HTMLButtonElement;
    expect(b2.disabled).toBe(true);
    expect(bubbleOf(b2)).toBe("Choose Local for no copy");
  });

  it("shows the id of a deleted target until the next save", () => {
    renderWithProviders(
      <TargetChips label="Copy to" targets={two.targets} view={placementView({ skip: ["t-gone"] })} options={two} onToggle={vi.fn()} />
    );
    expect((screen.getByRole("button", { name: "(deleted)" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("reports the new state of a clicked chip and warns about differing credentials", () => {
    const onToggle = vi.fn();
    const both = placementOptions({
      targets: [targetOption(), targetOption({ id: "t-hz", name: "Hetzner", primary: false, hint: "creds-differ" })],
    });
    renderWithProviders(
      <TargetChips label="Copy to" targets={both.targets} view={placementView({ skip: ["t-hz"] })} options={both} onToggle={onToggle} />
    );
    const hz = screen.getByRole("button", { name: "Hetzner" });
    fireEvent.click(hz);
    expect(onToggle).toHaveBeenCalledWith("t-hz", true);
    expect(bubbleOf(hz)).toBe(
      "The domain repository is remote and uses other credentials than this target. Copying to it fails until they match."
    );
  });

  it("carries each target's place mark, drawn in the chip's ink once ticked", () => {
    const placed = placementOptions({
      targets: [
        targetOption({ placeId: "p-b2", provider: "b2" }),
        targetOption({ id: "t-nas", name: "NAS Keller", primary: false, placeId: "p-nas", provider: "synology" }),
      ],
    });
    renderWithProviders(
      <TargetChips label="Copy to" targets={placed.targets} view={placementView({ skip: ["t-nas"] })} options={placed} onToggle={vi.fn()} />
    );
    const mark = (name: string) => screen.getByRole("button", { name }).querySelector('[data-testid="mark"]');
    expect(mark("B2")?.getAttribute("data-provider")).toBe("b2");
    expect(mark("B2")?.getAttribute("data-on-fill")).toBe("true");
    expect(mark("NAS Keller")?.getAttribute("data-on-fill")).toBe("false");
  });

  it("says beside the chips where a place becomes a copy target", () => {
    renderWithProviders(
      <TargetChips label="Copy to" targets={two.targets} view={placementView()} options={two} onToggle={vi.fn()} />
    );
    expect(screen.getByLabelText(en["placement.copyToHint"])).toBeTruthy();
  });
});
