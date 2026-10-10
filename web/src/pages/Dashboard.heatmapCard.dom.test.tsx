// @vitest-environment jsdom
// The backup history card's domain strip is the page-level selector: the
// grooved well spanning the card, with its segments pinned to one width. A
// row of loose chips docked in the card's corner is what it replaced.
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "../lib/i18n";
import { HealthHeatmapCard } from "./overview/HealthHeatmapCard";

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  getHistory: () => Promise.resolve({ ok: true, days: [] }),
}));

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof HealthHeatmapCard
>[0]["t"];

afterEach(cleanup);

describe("HealthHeatmapCard domain strip", () => {
  it("is the grooved well at the page-level scale, spanning the card", () => {
    render(<HealthHeatmapCard t={t} selectedDay={null} onSelectDay={() => {}} />);
    const strip = screen.getByRole("tablist", { name: en["dashboard.healthTitle"] });
    const classes = strip.className.split(/\s+/);

    expect(classes).toContain("bg-carbon-surface3");
    expect(classes).toContain("w-full");
    expect(classes).not.toContain("w-fit");
    for (const tab of screen.getAllByRole("tab")) {
      expect(tab.className).toContain("h-[var(--badge-md)]");
      expect(tab.className).not.toContain("bg-carbon-surface2");
    }
    expect(screen.getAllByRole("tab")).toHaveLength(6);
  });

  it("sits in the card's body rather than in a corner row of its own", () => {
    render(<HealthHeatmapCard t={t} selectedDay={null} onSelectDay={() => {}} />);
    const strip = screen.getByRole("tablist", { name: en["dashboard.healthTitle"] });
    expect(strip.parentElement!.className).toContain("rounded-card");
  });
});
