// @vitest-environment jsdom
// A switch that cannot be flipped right now keeps its state and its focus,
// shakes when pressed and says why.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Toggle } from "./Toggle";

afterEach(cleanup);

const REASON = "Only with Dark or System. Light is chosen above.";

describe("a refused Toggle", () => {
  it("keeps its state and does not call onChange", () => {
    const onChange = vi.fn();
    render(<Toggle checked label="OLED" onChange={onChange} refused={REASON} />);
    const sw = screen.getByRole("switch", { name: "OLED" });

    fireEvent.click(sw);

    expect(onChange).not.toHaveBeenCalled();
    expect(sw.getAttribute("aria-checked")).toBe("true");
  });

  it("stays focusable and marks itself unavailable", () => {
    render(<Toggle checked={false} label="OLED" onChange={() => {}} refused={REASON} />);
    const sw = screen.getByRole("switch", { name: "OLED" }) as HTMLButtonElement;
    expect(sw.disabled).toBe(false);
    expect(sw.getAttribute("aria-disabled")).toBe("true");
  });

  it("names its reason as the switch's description", () => {
    render(<Toggle checked={false} label="OLED" onChange={() => {}} refused={REASON} />);
    const sw = screen.getByRole("switch", { name: "OLED" });
    const ids = (sw.getAttribute("aria-describedby") ?? "").split(" ");
    expect(ids.map((id) => document.getElementById(id)?.textContent)).toContain(REASON);
  });

  it("shakes on a press", async () => {
    const { container } = render(<Toggle checked={false} label="OLED" onChange={() => {}} refused={REASON} />);
    fireEvent.click(screen.getByRole("switch", { name: "OLED" }));
    await waitFor(() => expect(container.querySelector(".glim-shake")).not.toBeNull());
  });

  it("flips as usual once nothing refuses it", () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} label="OLED" onChange={onChange} />);
    const sw = screen.getByRole("switch", { name: "OLED" });
    expect(sw.hasAttribute("aria-disabled")).toBe(false);
    fireEvent.click(sw);
    expect(onChange).toHaveBeenCalledWith(true);
  });
});
