// @vitest-environment jsdom
// The welcome page's pictures are drawn as the frames they rest on, so what a
// test finds in the markup is what someone sees where motion is off: every
// area switched on, the vault standing open, then closed and sealed.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";

import type { CSSProperties } from "react";
import { AreasPicture, BackupPicture, Parcel, PlacePicture } from "./welcomeArt";

afterEach(cleanup);

describe("the welcome page's pictures", () => {
  it("stay out of the way of a screen reader, since the steps say what they show", () => {
    for (const picture of [<AreasPicture key="a" />, <PlacePicture key="p" />, <BackupPicture key="b" done="Backed up" />]) {
      const { container, unmount } = render(picture);
      expect(container.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
      unmount();
    }
  });

  it("rest with every area switched on, each in its own hue", () => {
    const { container } = render(<AreasPicture />);
    const knobs = [...container.querySelectorAll<SVGCircleElement>(".glim-welcome-switch")];
    const tracks = [...container.querySelectorAll<SVGRectElement>(".glim-welcome-track")];
    expect(knobs).toHaveLength(3);
    knobs.forEach((knob, i) => {
      const end = Number(tracks[i].getAttribute("x")) + Number(tracks[i].getAttribute("width"));
      expect(Number(knob.getAttribute("cx")), "the knob rests at the far end").toBeGreaterThan(end - 10);
    });
    const tiles = [...container.querySelectorAll<SVGGElement>("g.glim-hue")];
    expect(tiles.map((g) => g.style.getPropertyValue("--turn"))).toEqual(["0", "1", "2"]);
  });

  it("rest with the vault standing open in the second card", () => {
    const { container } = render(<PlacePicture />);
    expect(container.querySelector(".glim-welcome-drop .glim-welcome-door-open")).not.toBeNull();
    expect(container.querySelector(".glim-welcome-seal")).toBeNull();
  });

  it("rest with the vault closed and sealed in the third, its badge saying so", () => {
    const { container } = render(<BackupPicture done="Backed up" />);
    for (const part of [".glim-welcome-door", ".glim-welcome-dial", ".glim-welcome-seal", ".glim-welcome-ring"]) {
      expect(container.querySelector(part), part).not.toBeNull();
    }
    expect(container.querySelector(".glim-welcome-seal")?.textContent).toBe("Backed up");
    expect(container.querySelectorAll(".glim-welcome-confetti").length).toBeGreaterThan(8);
  });

  it("send each parcel in its own hue, from where the page measured to where it lands", () => {
    const { container } = render(<Parcel turn={2} style={{ "--fly-to-x": "40px" } as CSSProperties} />);
    const parcel = container.querySelector<HTMLElement>(".glim-welcome-parcel")!;
    expect(parcel.style.getPropertyValue("--turn")).toBe("2");
    expect(parcel.style.getPropertyValue("--fly-to-x")).toBe("40px");
    expect(parcel.classList.contains("glim-hue")).toBe(true);
  });
});
