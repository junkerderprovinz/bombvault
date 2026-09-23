// @vitest-environment jsdom
// The badge is a notch pulled up by half its height, so a hint line under it
// would be covered by its lower half. The hint belongs in the badge's (i).
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { NotInstalledHeading } from "./NotInstalledHeading";

const t = ((key: string) => key) as unknown as Parameters<typeof NotInstalledHeading>[0]["t"];

afterEach(() => {
  cleanup();
});

describe("NotInstalledHeading", () => {
  it("carries the hint in the badge's (i), not in a line under the notch", () => {
    render(<NotInstalledHeading tip="containers.notInstalledHint" hueIndex={0} t={t} />);
    const badge = screen.getByText("containers.notInstalledTitle");
    const bubble = screen.getByLabelText("containers.notInstalledHint");
    expect(badge.contains(bubble)).toBe(true);
    expect(screen.queryByText("containers.notInstalledHint")).toBeNull();
  });
});
