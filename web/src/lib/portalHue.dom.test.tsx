// @vitest-environment jsdom
// A portalled panel stands in its trigger's palette position. It copies the
// root references, not a resolved colour, so it keeps following the palette.
import { useRef } from "react";
import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { hueVars } from "./appearance";
import { usePortalHue } from "./portalHue";

function Probe({ open }: { open: boolean }) {
  const trigger = useRef<HTMLButtonElement>(null);
  const hue = usePortalHue(open, trigger);
  return (
    <>
      <button ref={trigger}>trigger</button>
      <div data-testid="panel" className={hue.className} style={hue.style} />
    </>
  );
}

afterEach(cleanup);

it("copies the hue of the card around its trigger as root references", () => {
  render(
    <div className="glim-hue" style={hueVars(3)}>
      <Probe open />
    </div>,
  );
  const panel = screen.getByTestId("panel");
  expect(panel.className).toBe("glim-hue");
  expect(panel.style.getPropertyValue("--item-hue")).toBe("var(--rb-3)");
  expect(panel.style.getPropertyValue("--item-hue-ink")).toBe("var(--rb-ink-3)");
});

it("leaves a panel without a hued trigger on the global accent", () => {
  render(<Probe open />);
  const panel = screen.getByTestId("panel");
  expect(panel.className).toBe("");
  expect(panel.getAttribute("style")).toBeNull();
});
