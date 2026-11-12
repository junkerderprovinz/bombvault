// @vitest-environment jsdom
// The welcome scene is drawn as the frame it rests on, so what a test finds in
// the markup is what someone sees where motion is off: the vault closed and
// sealed, with the three things to keep already inside.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";

import { WelcomeScene } from "./WelcomeScene";

afterEach(cleanup);

describe("the welcome scene", () => {
  it("stays out of the way of a screen reader, since the steps say what it shows", () => {
    const { container } = render(<WelcomeScene />);
    expect(container.firstElementChild?.getAttribute("aria-hidden")).toBe("true");
  });

  it("rests on the vault closed and sealed, with a door, a dial and the spark", () => {
    const { container } = render(<WelcomeScene />);
    for (const part of [".glim-welcome-door", ".glim-welcome-dial", ".glim-welcome-seal", ".glim-welcome-spark"]) {
      expect(container.querySelector(part), part).not.toBeNull();
    }
  });

  it("sends three things into the vault one after another, each from its own place and in its own hue", () => {
    const { container } = render(<WelcomeScene />);
    const items = [...container.querySelectorAll<SVGGElement>(".glim-welcome-item")];
    expect(items.map((g) => g.style.getPropertyValue("--turn"))).toEqual(["0", "1", "2"]);
    expect(new Set(items.map((g) => g.style.getPropertyValue("--from-y"))).size).toBe(3);
    expect(items.every((g) => g.classList.contains("glim-hue") && g.classList.contains("glim-pic"))).toBe(true);
  });
});
