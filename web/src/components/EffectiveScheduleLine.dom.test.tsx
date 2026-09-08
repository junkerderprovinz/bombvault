// @vitest-environment jsdom
/**
 * The sentence on the Folders card (#199).
 *
 * The two outcomes worth a test are the two that are warnings: a set nothing
 * backs up, and a set two schedules back up. Those are the states manilx was in
 * while believing the opposite, and a line that reports them in the same muted
 * grey as the harmless ones would be no better than the hint text it replaces.
 *
 * The line never computes the outcome itself: the server sends `kind`, and this
 * only formats it. So what these tests pin is the formatting contract, and the
 * Go side pins the rule (internal/schedule/effective_internal_test.go).
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

  it("says plainly that a set is not backed up, and says it in the fail colour", () => {
    draw({ kind: "none", spec: "", alsoSpec: "" });
    const line = screen.getByText(/not backed up automatically/i);
    // Muted grey would bury the one outcome the reader must not miss.
    expect(line.closest("p")?.className).toContain("text-statusFail");
  });

  it("names BOTH cadences when a set runs twice, and warns", () => {
    draw({ kind: "both", spec: "weekly mon 02:00", alsoSpec: "daily 05:00" });
    const line = screen.getByText(/runs twice/i);
    const text = line.closest("p")?.textContent ?? "";
    // A "runs twice" that named only one of them would leave the reader
    // hunting for the second schedule, which is the whole confusion.
    expect(text).toMatch(/2:00/); // formatCadence drops the leading zero
    expect(text).toMatch(/5:00/);
    expect(line.closest("p")?.className).toContain("text-statusWarn");
  });

  it("attributes a domain run to the Folders card by its own name", () => {
    draw({ kind: "domain", spec: "weekly mon 02:00", alsoSpec: "" });
    // "Folders" is not a literal here: it comes from jobs.filesSection, the key
    // the card itself uses, so the sentence follows a renamed card.
    expect(screen.getByText(/folders/i)).toBeTruthy();
  });

  it("attributes an everything-only run to Backup Everything", () => {
    draw({ kind: "everything", spec: "daily 05:00", alsoSpec: "" });
    expect(screen.getByText(/backup everything/i)).toBeTruthy();
  });

  it("calls an override its own schedule and shows no second cadence", () => {
    draw({ kind: "own", spec: "daily 03:00", alsoSpec: "" });
    const text = screen.getByText(/its own schedule/i).closest("p")?.textContent ?? "";
    expect(text).toMatch(/3:00/);
    expect(text).not.toMatch(/5:00/);
  });

  it("stays silent on a kind it does not know, rather than guessing", () => {
    // A newer backend could add a sixth outcome; showing a half-built sentence
    // for it would be worse than showing none.
    const { container } = draw({ kind: "quarterly" as EffectiveSchedule["kind"], spec: "daily 05:00", alsoSpec: "" });
    expect(container.textContent).toBe("");
  });
});
