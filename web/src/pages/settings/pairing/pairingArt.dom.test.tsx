// @vitest-environment jsdom
// The relay pictures. Each is drawn as the frame it rests on, so what a test
// finds in the markup is what someone sees where motion is off, and the
// stylesheet says how fast a letter travels where it is on.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { I18nProvider, en, useT } from "../../../lib/i18n";
import { RouteDiagram } from "./pairingArt";
import type { RelayForm } from "./relayForm";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "..", "..", "index.css"), "utf8");

function Picture({ form }: { form: RelayForm }) {
  const { t } = useT();
  return <RouteDiagram form={form} t={t} />;
}

function draw(form: RelayForm): HTMLElement {
  const { container } = render(
    <I18nProvider>
      <Picture form={form} />
    </I18nProvider>,
  );
  return container;
}

const logos = (root: HTMLElement) => root.querySelectorAll('image[href="/logo.svg"]').length;
const letters = (root: HTMLElement, leg: string) => root.querySelectorAll(`.glim-letter-${leg}`);

afterEach(cleanup);

describe("the relay in four forms", () => {
  it("draws the project relay between two instances, each as the app's logo", () => {
    const root = draw("project");
    expect(screen.getByRole("img", { name: en["relay.pic.relay"] })).toBeTruthy();
    expect(logos(root)).toBe(2);
    expect(root.querySelector('image[href="/parleyport.svg"]')).toBeTruthy();
    expect(root.textContent).toContain(en["relay.containerName"]);
    expect(root.textContent).not.toContain(en["relay.pic.selfHosted"]);
  });

  it("marks an own relay as self-hosted", () => {
    const root = draw("own");
    expect(root.textContent).toContain(en["relay.pic.selfHosted"]);
    expect(root.querySelector('image[href="/parleyport.svg"]')).toBeTruthy();
  });

  it("makes one of the two instances the relay when a BombVault serves it", () => {
    const root = draw("instance");
    expect(screen.getByRole("img", { name: en["relay.pic.instance"] })).toBeTruthy();
    expect(logos(root)).toBe(2);
    expect(root.querySelector('image[href="/parleyport.svg"]')).toBeNull();
    expect(root.textContent).toContain(en["relay.diagramRelay"]);
    expect(root.querySelector(".glim-relay-hop.glim-relay-home image")).toBeTruthy();
  });

  it("shows two neighbours talking directly and a third out of reach without a relay", () => {
    const root = draw("off");
    expect(screen.getByRole("img", { name: en["relay.pic.direct"] })).toBeTruthy();
    expect(logos(root)).toBe(3);
    expect(root.querySelector(".glim-route-cross")).toBeTruthy();
    expect(root.querySelector(".glim-letter-refused")).toBeTruthy();
  });

  it("names both networks in every form", () => {
    for (const form of ["project", "own", "instance", "off"] as const) {
      const root = draw(form);
      expect(root.textContent).toContain(en["relay.diagramYourNetwork"]);
      expect(root.textContent).toContain(en["relay.diagramOtherNetwork"]);
      cleanup();
    }
  });

  it("sends a letter into the relay and another out of it, there and then back", () => {
    const root = draw("project");
    const into = [...letters(root, "in")];
    const out = [...letters(root, "out")];
    expect(into).toHaveLength(2);
    expect(out).toHaveLength(2);
    expect(into.filter((l) => l.classList.contains("glim-letter-back"))).toHaveLength(1);
    expect(out.filter((l) => l.classList.contains("glim-letter-back"))).toHaveLength(1);
    // Both legs of one direction meet at the relay.
    const path = (el: Element) => (el as SVGElement).style.getPropertyValue("offset-path");
    const there = into.find((l) => !l.classList.contains("glim-letter-back"))!;
    const onward = out.find((l) => !l.classList.contains("glim-letter-back"))!;
    expect(path(there)).toMatch(/L180 70"\)$/);
    expect(path(onward)).toMatch(/^path\("M180 70 /);
    expect(root.querySelector(".glim-relay-ring")).toBeTruthy();
    expect(root.querySelector(".glim-relay-hop")).toBeTruthy();
  });

  it("rests a letter where it can be seen while nothing moves", () => {
    expect(draw("project").querySelectorAll(".glim-letter-parked")).toHaveLength(2);
    cleanup();
    expect(draw("instance").querySelectorAll(".glim-letter-parked")).toHaveLength(1);
    cleanup();
    expect(draw("off").querySelectorAll(".glim-letter-parked")).toHaveLength(1);
  });
});

/** The share of its loop a keyframe set spends moving a letter from one
 *  distance along its path to another. */
function travel(name: string): { from: number; to: number; share: number } {
  const body = CSS.match(new RegExp(`@keyframes ${name} \\{([\\s\\S]*?)\\n\\}`))![1];
  const stops = [...body.matchAll(/([\d.,% ]+)\{[^}]*offset-distance: (\d+)%/g)].flatMap((m) =>
    m[1].split(",").map((at) => ({ at: parseFloat(at), distance: Number(m[2]) })),
  );
  const leaves = Math.max(...stops.filter((s) => s.distance === stops[0].distance).map((s) => s.at));
  const arrives = Math.min(...stops.filter((s) => s.distance === 100).map((s) => s.at));
  return { from: stops[0].distance, to: 100, share: (arrives - leaves) / 100 };
}

describe("how fast a letter travels", () => {
  // Units of the picture per pass of the scene: the length of the way, over
  // the share of the loop spent on it and the loop's share of a pass.
  const speed = (name: string, length: number, loop: number) => length / (travel(name).share * loop);

  it("is the same on every way, whatever its length", () => {
    const direct = CSS.match(/\.glim-letter-direct \{\s*--letter-loop: calc\(var\(--motion-scene-dur, 6s\) \* ([\d.]+)\)/)![1];
    const speeds = [
      speed("glim-letter-arc", 183, 1),
      speed("glim-letter-in", 91.5, 1),
      speed("glim-letter-out", 91.5, 1),
      speed("glim-letter-direct", 60, Number(direct)),
    ];
    for (const s of speeds) expect(Math.abs(s - speeds[0]) / speeds[0]).toBeLessThan(0.03);
  });

  it("lets the way back start when the way there is over", () => {
    for (const name of ["glim-letter-arc", "glim-letter-direct", "glim-letter-out"]) {
      const body = CSS.match(new RegExp(`@keyframes ${name} \\{([\\s\\S]*?)\\n\\}`))![1];
      const gone = Math.min(...[...body.matchAll(/([\d.]+)%, 100%\s*\{[^}]*opacity: 0/g)].map((m) => parseFloat(m[1])));
      expect(gone).toBeLessThanOrEqual(50);
    }
    expect(CSS).toMatch(/\.glim-letter-back,[\s\S]{0,120}animation-delay: calc\(var\(--letter-loop\) \/ 2\)/);
  });

  it("moves nothing where motion is off or reduced", () => {
    const gated = CSS.slice(CSS.indexOf("@media (prefers-reduced-motion: no-preference)"), CSS.lastIndexOf("@media (prefers-reduced-motion: reduce)"));
    const outside = CSS.replace(gated, "");
    for (const cls of ["glim-letter", "glim-relay-ring", "glim-relay-hop", "glim-step-pointer", "glim-step-confetti", "glim-net-dot", "glim-net-link"]) {
      const rule = new RegExp(`\\.${cls}[ ,{][^}]*\\{[^}]*animation:`);
      expect(outside, `${cls} animates outside the motion gate`).not.toMatch(rule);
      expect(gated).toMatch(new RegExp(`:root:not\\(\\[data-motion="off"\\]\\) [^{]*\\.${cls}\\b[^{]*\\{[^}]*animation`));
    }
  });
});
