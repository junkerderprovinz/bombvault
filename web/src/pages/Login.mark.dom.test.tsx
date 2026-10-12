// @vitest-environment jsdom
// The login screen has no rail, so it shows the mark itself. The logo reads on
// both themes, and it is decorative because the heading beside it already
// names the product.
import { render, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../lib/api", () => ({
  login: vi.fn(),
  loginWithPasskey: vi.fn(),
  passkeyStatus: vi.fn(),
  passkeysAvailableInBrowser: () => false,
}));

import { LoginPage } from "./Login";

afterEach(cleanup);

describe("LoginPage mark", () => {
  it("shows the one logo in either colour scheme", () => {
    const { container } = render(<LoginPage onLogin={vi.fn()} />);
    const marks = Array.from(container.querySelectorAll("img"));
    expect(marks.map((m) => m.getAttribute("src"))).toEqual(["/logo.svg"]);
    expect(marks[0].className).not.toMatch(/\bdark:|\bhidden\b/);
  });

  it("keeps the mark out of the accessibility tree", () => {
    const { container } = render(<LoginPage onLogin={vi.fn()} />);
    for (const img of Array.from(container.querySelectorAll("img"))) {
      expect(img.getAttribute("alt")).toBe("");
      expect(img.getAttribute("aria-hidden")).toBe("true");
    }
  });
});
