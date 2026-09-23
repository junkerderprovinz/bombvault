// @vitest-environment jsdom
/**
 * The outcomes that matter most are the two warnings: a set nothing backs up,
 * and a set two schedules back up. The server decides the outcome
 * (internal/schedule/effective_internal_test.go pins the rule), so these tests
 * pin only how it is formatted.
 */
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { EffectiveScheduleLine } from "./EffectiveScheduleLine";
import type { EffectiveSchedule } from "../lib/api";

function draw(effective?: EffectiveSchedule) {
  return render(<EffectiveScheduleLine effective={effective} />);
}

afterEach(cleanup);

describe("EffectiveScheduleLine", () => {
  it("renders nothing when the server sent no verdict", () => {
    const { container } = draw(undefined);
    expect(container.textContent).toBe("");
  });

  it("shows a set that is not backed up in the fail colour", () => {
    draw({ kind: "none", spec: "", alsoSpec: "" });
    const line = screen.getByText(/not backed up automatically/i);
    expect(line.closest("p")?.className).toContain("text-statusFail");
  });

  it("names both cadences when a set runs twice, in the warning colour", () => {
    draw({ kind: "both", spec: "weekly mon 02:00", alsoSpec: "daily 05:00" });
    const line = screen.getByText(/runs twice/i);
    const text = line.closest("p")?.textContent ?? "";
    // Naming only one would leave the reader looking for the other schedule.
    expect(text).toMatch(/2:00/); // formatCadence drops the leading zero
    expect(text).toMatch(/5:00/);
    expect(line.closest("p")?.className).toContain("text-statusWarn");
  });

  it("names the Folders card for a domain run", () => {
    draw({ kind: "domain", spec: "weekly mon 02:00", alsoSpec: "" });
    // The name comes from jobs.filesSection, the card's own title key.
    expect(screen.getByText(/folders/i)).toBeTruthy();
  });

  it("names Backup Everything for an everything-only run", () => {
    draw({ kind: "everything", spec: "daily 05:00", alsoSpec: "" });
    expect(screen.getByText(/backup everything/i)).toBeTruthy();
  });

  it("calls an override its own schedule and shows no second cadence", () => {
    draw({ kind: "own", spec: "daily 03:00", alsoSpec: "" });
    const text = screen.getByText(/its own schedule/i).closest("p")?.textContent ?? "";
    expect(text).toMatch(/3:00/);
    expect(text).not.toMatch(/5:00/);
  });

  it("renders nothing for a kind it does not know", () => {
    const { container } = draw({ kind: "quarterly" as EffectiveSchedule["kind"], spec: "daily 05:00", alsoSpec: "" });
    expect(container.textContent).toBe("");
  });
});
