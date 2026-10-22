// @vitest-environment jsdom
// The group as a network: this server in the middle, a node for everything
// paired or connected, an arrow per role in the role's glyph, and a legend.
// The picture holds a wide and a narrow arrangement of the same nodes.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import { I18nProvider, en } from "../../lib/i18n";
import { NetPicture, useNetHover } from "./NetPicture";
import type { NetNode } from "./netModel";

const NODES: NetNode[] = [
  {
    key: "m1",
    kind: "peer",
    name: "attic",
    sub: "Direct",
    dim: false,
    flows: [
      { dir: "out", role: "receiver", waiting: false },
      { dir: "out", role: "fetcher", waiting: true },
      { dir: "in", role: "zfs", waiting: false },
    ],
  },
  { key: "zfs:nas", kind: "zfs", name: "Backup-NAS", sub: "over SSH", dim: true, flows: [] },
  { key: "m3", kind: "phone", name: "Pixel 8", sub: "Android app", dim: false, flows: [] },
];

function Harness({ nodes }: { nodes: NetNode[] }) {
  const { hot, handlers } = useNetHover();
  return (
    <div {...handlers}>
      <NetPicture selfName="cellar" nodes={nodes} hot={hot} />
      <div data-net="m1" data-testid="entry">
        attic
      </div>
    </div>
  );
}

function draw(nodes: NetNode[] = NODES) {
  return render(
    <I18nProvider>
      <Harness nodes={nodes} />
    </I18nProvider>,
  );
}

const wide = () => document.querySelector(".glim-net-wide") as SVGSVGElement;
const narrow = () => document.querySelector(".glim-net-narrow") as SVGSVGElement;
const spoke = (svg: SVGSVGElement, key: string) => svg.querySelector(`[data-net="${key}"]`) as SVGGElement;

afterEach(cleanup);

describe("the net picture", () => {
  it("draws this server and every node once in each arrangement", () => {
    draw();
    for (const svg of [wide(), narrow()]) {
      expect(svg.getAttribute("aria-label")).toBe(en["instances.net.alt"]);
      const names = within(svg as unknown as HTMLElement)
        .getAllByRole("button")
        .map((b) => b.getAttribute("aria-label"));
      expect(names).toEqual(["attic", "Backup-NAS", "Pixel 8", "cellar"]);
      expect(spoke(svg, "self").textContent).toContain(en["instances.thisInstance"]);
    }
  });

  it("gives the wide arrangement more room than the narrow one", () => {
    draw();
    const width = (svg: SVGSVGElement) => Number(svg.getAttribute("viewBox")!.split(" ")[2]);
    expect(width(wide())).toBeGreaterThan(width(narrow()));
  });

  it("stands every BombVault, this server and a phone too, as the logo with what it is in a badge", () => {
    draw();
    for (const svg of [wide(), narrow()]) {
      for (const key of ["self", "m1", "m3"]) {
        const node = spoke(svg, key).querySelector(".glim-net-node")!;
        expect(node.querySelector('image[href="/logo.svg"]'), key).toBeTruthy();
        expect(node.querySelector(".glim-net-kind"), key).toBeTruthy();
        expect(node.querySelector(".glim-net-kind-glyph svg"), key).toBeTruthy();
        expect(node.querySelector(".glim-net-tile"), key).toBeNull();
      }
    }
    const phone = spoke(wide(), "m3").querySelector(".glim-net-kind-glyph path")!.getAttribute("d");
    const instance = spoke(wide(), "m1").querySelector(".glim-net-kind-glyph path")!.getAttribute("d");
    expect(phone).not.toBe(instance);
    expect(spoke(wide(), "zfs:nas").querySelector(".glim-net-tile-zfs")).toBeTruthy();
    expect(spoke(wide(), "zfs:nas").querySelector("image")).toBeNull();
  });

  it("lets a long name wrap in both arrangements rather than cutting it short", () => {
    const long = "the-backup-server-in-the-cellar-behind-the-old-boiler";
    draw([{ ...NODES[0], name: long }]);
    for (const svg of [wide(), narrow()]) {
      const label = spoke(svg, "m1").querySelector("foreignObject")!;
      expect(label.textContent).toContain(long);
      expect(Number(label.getAttribute("height"))).toBeGreaterThanOrEqual(74);
    }
  });

  it("draws one arrow per role, dashed and without a travelling dot while it waits", () => {
    draw();
    const lanes = [...spoke(wide(), "m1").querySelectorAll("[data-lane]")];
    expect(lanes.map((l) => l.getAttribute("data-lane"))).toEqual(["out:receiver:on", "out:fetcher:waiting", "in:zfs:on"]);
    expect(lanes.map((l) => l.classList.contains("glim-net-wait"))).toEqual([false, true, false]);
    expect(lanes.map((l) => l.querySelectorAll(".glim-net-dot").length)).toEqual([1, 0, 1]);
    expect(spoke(wide(), "zfs:nas").querySelectorAll("[data-lane]")).toHaveLength(0);
  });

  it("says in words what each arrow means", () => {
    draw();
    const titles = [...spoke(wide(), "m1").querySelectorAll("[data-lane] > title")].map((node) => node.textContent);
    expect(titles).toEqual([
      "Receiver: from this server to attic",
      `Fetcher: from this server to attic · ${en["instances.net.waiting"]}`,
      "ZFS server: from attic to this server",
    ]);
  });

  it("marks an instance that is not polled or switched off", () => {
    draw();
    expect(spoke(wide(), "zfs:nas").classList.contains("glim-net-dim")).toBe(true);
    expect(spoke(wide(), "m1").classList.contains("glim-net-dim")).toBe(false);
  });

  it("explains the arrows in a legend with every role and the waiting line", () => {
    draw();
    const key = document.querySelector(".glim-net-key") as HTMLElement;
    expect(key.textContent).toBe(
      en["receiver.title"] + en["instances.role.fetcher"] + en["zfs.replica.servers.title"] + en["instances.net.waiting"],
    );
    expect(within(key).getByLabelText(en["instances.net.tip"])).toBeTruthy();
  });

  it("has no legend while nothing in it can hold a role", () => {
    draw(NODES.slice(1));
    expect(document.querySelector(".glim-net-key")).toBeNull();
  });

  it("is nothing at all while this server stands alone", () => {
    draw([]);
    expect(screen.queryByTestId("net-picture")).toBeNull();
  });
});

describe("pointing at an instance", () => {
  it("lights up its node from the entry beside the picture, and dims the others", () => {
    draw();
    fireEvent.pointerOver(screen.getByTestId("entry"));
    expect(spoke(wide(), "m1").classList.contains("glim-net-hot")).toBe(true);
    expect(spoke(wide(), "m3").classList.contains("glim-net-hot")).toBe(false);
    expect(wide().classList.contains("glim-net-has")).toBe(true);
    fireEvent.pointerOver(spoke(wide(), "m3").querySelector('[role="button"]')!);
    expect(spoke(wide(), "m3").classList.contains("glim-net-hot")).toBe(true);
    expect(spoke(narrow(), "m3").classList.contains("glim-net-hot")).toBe(true);
  });

  it("dims nothing while the pointer is on this server", () => {
    draw();
    fireEvent.pointerOver(spoke(wide(), "self").querySelector('[role="button"]')!);
    expect(spoke(wide(), "self").classList.contains("glim-net-hot")).toBe(true);
    expect(wide().classList.contains("glim-net-has")).toBe(false);
  });

  it("brings the entry into view on a click or a key press on its node", () => {
    const shown = vi.fn();
    draw();
    screen.getByTestId("entry").scrollIntoView = shown;
    const node = spoke(wide(), "m1").querySelector('[role="button"]')!;
    fireEvent.click(node);
    expect(shown).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(node, { key: "Enter" });
    expect(shown).toHaveBeenCalledTimes(2);
    fireEvent.keyDown(node, { key: "a" });
    expect(shown).toHaveBeenCalledTimes(2);
  });
});
