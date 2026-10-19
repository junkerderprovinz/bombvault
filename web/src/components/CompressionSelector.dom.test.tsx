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
    const modes = within(screen.getByRole("radiogroup", { name: "Compression" })).getAllByRole("radio");
    expect(modes.map((t) => t.textContent)).toEqual(["Off", "Automatic", "Maximum"]);
    const glyphs = modes.map((t) => t.querySelector("svg")?.innerHTML);
    expect(glyphs.every(Boolean)).toBe(true);
    expect(new Set(glyphs).size).toBe(3);
  });

  it("keeps the mode names for screen readers when only glyphs show", () => {
    setLabelMode("buttons", "glyph");
    render(<CompressionSelector value="max" onChange={vi.fn()} />);
    const list = screen.getByRole("radiogroup", { name: "Compression" });
    expect(within(list).getByRole("radio", { name: "Maximum" }).getAttribute("aria-checked")).toBe("true");
    expect(within(list).getByRole("radio", { name: "Off" })).toBeTruthy();
    expect(within(list).getByRole("radio", { name: "Automatic" })).toBeTruthy();
  });
});
