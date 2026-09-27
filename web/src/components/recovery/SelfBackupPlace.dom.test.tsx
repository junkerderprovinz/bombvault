// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { PlaceLine } from "./SelfBackupPlace";

describe("PlaceLine", () => {
  afterEach(cleanup);

  it("shows the address alone for a location on no place", () => {
    const { container } = render(<PlaceLine label="Stored in" name="" address="backups/config" />);
    expect(screen.getByText("backups/config")).toBeTruthy();
    const empty = [...container.querySelectorAll("span")].filter((s) => !s.textContent);
    expect(empty).toEqual([]);
  });
});
