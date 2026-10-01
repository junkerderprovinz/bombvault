// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { setLabelMode } from "../lib/controls";
import { CompressionSelector } from "./CompressionSelector";

afterEach(() => {
  cleanup();
  setLabelMode("buttons", "textGlyph");
});

describe("CompressionSelector", () => {
  it("gives each of the three modes its own glyph", () => {
    render(<CompressionSelector value="auto" onChange={vi.fn()} />);
    const tabs = within(screen.getByRole("tablist", { name: "Compression" })).getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Off", "Automatic", "Maximum"]);
    const glyphs = tabs.map((t) => t.querySelector("svg")?.innerHTML);
    expect(glyphs.every(Boolean)).toBe(true);
    expect(new Set(glyphs).size).toBe(3);
  });

  it("keeps the mode names for screen readers when only glyphs show", () => {
    setLabelMode("buttons", "glyph");
    render(<CompressionSelector value="max" onChange={vi.fn()} />);
    const list = screen.getByRole("tablist", { name: "Compression" });
    expect(within(list).getByRole("tab", { name: "Maximum" }).getAttribute("aria-selected")).toBe("true");
    expect(within(list).getByRole("tab", { name: "Off" })).toBeTruthy();
    expect(within(list).getByRole("tab", { name: "Automatic" })).toBeTruthy();
  });
});
