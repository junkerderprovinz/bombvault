// @vitest-environment jsdom
// The login screen has no rail, so it shows the mark itself. Both theme marks
// switch on the `dark:` variant as in the rail, and both are decorative because
// the heading beside them already names the product.
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
  it("shows both theme marks, one per colour scheme", () => {
    const { container } = render(<LoginPage onLogin={vi.fn()} />);
    const marks = Array.from(container.querySelectorAll("img"));
    const srcs = marks.map((m) => m.getAttribute("src"));
    expect(srcs).toContain("/logo.svg");
    expect(srcs).toContain("/logo-light.svg");

    const dark = marks.find((m) => m.getAttribute("src") === "/logo.svg")!;
    const light = marks.find((m) => m.getAttribute("src") === "/logo-light.svg")!;
    // The dark mark belongs on the light surface, so it hides in dark mode.
    expect(dark.className).toContain("block");
    expect(dark.className).toContain("dark:hidden");
    expect(light.className).toContain("hidden");
    expect(light.className).toContain("dark:block");
  });

  it("keeps the marks out of the accessibility tree", () => {
    const { container } = render(<LoginPage onLogin={vi.fn()} />);
    for (const img of Array.from(container.querySelectorAll("img"))) {
      expect(img.getAttribute("alt")).toBe("");
      expect(img.getAttribute("aria-hidden")).toBe("true");
    }
  });
});
