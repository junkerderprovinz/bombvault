// @vitest-environment jsdom
/**
 * The pictures of a storage location: the retention timeline and the two
 * scenes. Each is drawn as the frame it rests on, so what a test finds in the
 * markup is what someone sees where motion is off.
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { I18nProvider } from "../../lib/i18n";
import { KEEP_PRESETS, sampleBackups, type KeepCounts, type KeepInput } from "../../lib/keepPlan";
import { KeepTimeline } from "./KeepTimeline";
import { CopyScene, LockScene } from "./scenes";

const HERE = dirname(fileURLToPath(import.meta.url));

afterEach(cleanup);

function timeline(counts: KeepCounts, backups: readonly KeepInput[]) {
  return render(
    <I18nProvider>
      <KeepTimeline counts={counts} backups={backups} />
    </I18nProvider>
  );
}

describe("the retention timeline", () => {
  const backups = sampleBackups(new Date());

  it("draws one dot per backup and counts what stays and what goes", () => {
    const { container } = timeline([...KEEP_PRESETS.balanced], backups);

    const dots = container.querySelectorAll(".glim-keep-dot");
    const gone = container.querySelectorAll(".glim-keep-gone");
    expect(dots).toHaveLength(backups.length);
    expect(gone.length).toBeGreaterThan(0);
    const [kept, removed] = [...container.querySelectorAll("p b")].map((b) => Number(b.textContent));
    expect(removed).toBe(gone.length);
    expect(kept + removed).toBe(backups.length);
  });

  it("runs from the oldest backup to today and gives the newest the first rule that keeps it", () => {
    const { container } = timeline([...KEEP_PRESETS.balanced], backups);

    const dots = [...container.querySelectorAll(".glim-keep-dot")];
    expect(dots[dots.length - 1].className).toContain("glim-keep-band-daily");
    expect(screen.getByText("6 years ago")).toBeTruthy();
    expect(screen.getByText("today")).toBeTruthy();
  });

  it("names each count with its unit and leaves out a count of newest backups that is not set", () => {
    timeline([0, 14, 8, 12, 3], backups);

    expect(screen.getByText("14 days")).toBeTruthy();
    expect(screen.getByText("8 weeks")).toBeTruthy();
    expect(screen.getByText("12 months")).toBeTruthy();
    expect(screen.getByText("3 years")).toBeTruthy();
    expect(screen.queryByText("0 latest")).toBeNull();
  });

  it("shows a count of newest backups once it is set", () => {
    timeline([5, 0, 0, 0, 0], backups);

    expect(screen.getByText("5 latest")).toBeTruthy();
  });

  it("keeps every backup when no count is set", () => {
    const { container } = timeline([0, 0, 0, 0, 0], backups);

    expect(screen.getByText("Keep all")).toBeTruthy();
    expect(container.querySelectorAll(".glim-keep-gone")).toHaveLength(0);
    expect(container.querySelectorAll(".glim-keep-band-all.glim-keep-dot")).toHaveLength(backups.length);
  });

  it("sorts a backup into the day its own offset names", () => {
    // One calendar day in restic's stamps, two in UTC.
    const sameDay = [{ time: "2026-03-02T00:30:00+02:00" }, { time: "2026-03-02T23:30:00+02:00" }, { time: "2026-02-20T10:00:00+02:00" }];
    const { container } = timeline([0, 2, 0, 0, 0], sameDay);

    expect(container.querySelectorAll(".glim-keep-gone")).toHaveLength(1);
    expect(container.querySelectorAll(".glim-keep-band-daily.glim-keep-dot")).toHaveLength(2);
  });
});

describe("the scenes", () => {
  const place = { name: "NAS", kind: "offsite" as const };

  it("says in words what the lock does, for each state of the switch", () => {
    const { rerender } = render(
      <I18nProvider>
        <LockScene place={place} locked />
      </I18nProvider>
    );
    expect(screen.getByRole("img", { name: "Someone tries to delete a backup. The lock refuses, and the backup stays." })).toBeTruthy();
    expect(screen.getByText("Delete refused")).toBeTruthy();

    rerender(
      <I18nProvider>
        <LockScene place={place} locked={false} />
      </I18nProvider>
    );
    expect(screen.getByRole("img", { name: "Someone deletes a backup. Without delete protection it is gone." })).toBeTruthy();
    expect(screen.getByText("The backup is gone.")).toBeTruthy();
  });

  it("rests on the outcome: the pack sealed where the lock holds, missing where it does not", () => {
    const locked = render(
      <I18nProvider>
        <LockScene place={place} locked />
      </I18nProvider>
    );
    expect(locked.container.querySelector(".glim-scene-seal")).not.toBeNull();
    expect(locked.container.querySelector(".glim-lock-gone")).toBeNull();
    locked.unmount();

    const open = render(
      <I18nProvider>
        <LockScene place={place} locked={false} />
      </I18nProvider>
    );
    expect(open.container.querySelector(".glim-scene-seal")).toBeNull();
    expect(open.container.querySelector(".glim-lock-gone")).not.toBeNull();
  });

  it("shows fifteen packs on each side of a copy, three of them new", () => {
    const { container } = render(
      <I18nProvider>
        <CopyScene target={place} />
      </I18nProvider>
    );
    expect(screen.getByRole("img", { name: /Twelve of fifteen data packs/ })).toBeTruthy();
    expect(container.querySelectorAll(".glim-scene-pack-old")).toHaveLength(24);
    expect(container.querySelectorAll(".glim-scene-pack-new")).toHaveLength(6);
    expect(container.querySelectorAll(".glim-parcel-fly")).toHaveLength(3);
  });
});

describe("how the pictures are drawn", () => {
  const css = readFileSync(join(HERE, "..", "..", "index.css"), "utf8").replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, " "));
  const PICTURE = /\.glim-(vessel|keep|scene|parcel|lock)-/;

  /** Every rule that names a picture class and sets `property`, with where it
   *  stands in the stylesheet. */
  function rulesSetting(property: string): { selector: string; at: number }[] {
    const out: { selector: string; at: number }[] = [];
    for (const m of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      if (PICTURE.test(m[1]) && new RegExp(`(^|[\\s;])${property}\\s*:`).test(m[2])) {
        out.push({ selector: m[1].trim(), at: m.index });
      }
    }
    return out;
  }

  /** The ranges of the blocks that only apply where the OS allows motion. */
  function motionBlocks(): [number, number][] {
    const out: [number, number][] = [];
    const needle = "@media (prefers-reduced-motion: no-preference)";
    for (let i = css.indexOf(needle); i !== -1; i = css.indexOf(needle, i + 1)) {
      let depth = 0;
      for (let j = css.indexOf("{", i); j < css.length; j++) {
        if (css[j] === "{") depth++;
        else if (css[j] === "}" && --depth === 0) {
          out.push([i, j]);
          break;
        }
      }
    }
    return out;
  }

  it("moves only where the OS allows motion and the level is not off", () => {
    const moving = [...rulesSetting("animation"), ...rulesSetting("transition")];
    expect(moving.length).toBeGreaterThan(10);
    const blocks = motionBlocks();
    for (const rule of moving) {
      expect(rule.selector, "a picture that moves at the level off").toMatch(/^:root:not\(\[data-motion="off"\]\) /);
      expect(
        blocks.some(([from, to]) => rule.at > from && rule.at < to),
        `${rule.selector} moves under reduced motion`
      ).toBe(true);
    }
  });

  it("takes its corners from the shape engine", () => {
    const rounded = [...rulesSetting("rx"), ...rulesSetting("border-radius")];
    expect(rounded.length).toBeGreaterThan(5);
    for (const rule of rounded) {
      const body = css.slice(rule.at, css.indexOf("}", rule.at));
      expect(body, rule.selector).toMatch(/(rx|border-radius):\s*(inherit|[^;]*var\(--radius-(control|pill|card)\))/);
    }
  });

  it("never draws inside a filled shape with the accent's contrast colour", () => {
    const pictures = css.slice(css.indexOf(".glim-pic {"));
    expect(pictures.length).toBeGreaterThan(1000);
    expect(pictures).not.toMatch(/accent-contrast/);
    for (const file of ["scenes.tsx", "Vessel.tsx", "KeepTimeline.tsx"]) {
      expect(readFileSync(join(HERE, file), "utf8"), file).not.toMatch(/accent-contrast|accentContrast/);
    }
  });
});
