// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// CadenceBuilder — every cadence editor offers all five modes (#166).
//
// This file used to pin the opposite: an `allowEveryN` prop hid the "every N
// days" pill on the DR-drill, tamper-test and weekly-digest schedules, because
// the backend rejected that cadence for them ("this schedule does not support
// 'everyN'"). It rejected it for a real reason — none of those three jobs had a
// last-run record, so the scheduler could not enforce the interval and the
// cadence would have fired the job DAILY, meaning a nightly DR restore.
//
// All three record their own last-run pass now (internal/store
// schedule_job_runs.go, wired through SetJobRunStore), the API accepts the
// cadence, and the prop is gone with its last call site. What is left to pin is
// that no editor hides a mode any more, and that the row still tracks the value.
//
// Asserts the rendered pill row, not the props: what matters is which modes a
// user can actually click.
// ---------------------------------------------------------------------------
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { CadenceBuilder } from "./CadenceBuilder";
import { I18nProvider } from "../lib/i18n";

afterEach(() => {
  cleanup();
});

/**
 * The mode pills' visible labels, in rendered order. They are the shared
 * Selector's segments, so they carry role="tab" inside its tablist — not
 * role="button" (the only plain button in this fieldset is the time picker's
 * trigger).
 */
function modeLabels(): string[] {
  return screen.queryAllByRole("tab").map((el) => el.textContent ?? "");
}

/** The pill currently selected, via the Selector's aria-selected state. */
function selectedLabel(): string | undefined {
  return screen
    .queryAllByRole("tab")
    .find((el) => el.getAttribute("aria-selected") === "true")?.textContent ?? undefined;
}

function renderBuilder(value: string) {
  render(
    <I18nProvider>
      <CadenceBuilder label="Schedule" value={value} onChange={vi.fn()} />
    </I18nProvider>
  );
}

describe("everyN mode availability", () => {
  it("offers all five modes, in order", () => {
    renderBuilder("daily 02:00");
    expect(modeLabels()).toEqual(["Off", "Daily", "Weekly", "Every N days", "Cron"]);
  });

  it("offers everyN on a schedule that used to hide it", () => {
    // The drill/tamper/digest editors render exactly this component with no
    // extra props now — there is no per-call-site mode gate left to pass.
    renderBuilder("weekly Mon 08:00");
    expect(modeLabels()).toContain("Every N days");
  });

  it("selects everyN when the stored value already uses it", () => {
    renderBuilder("everyN 3 04:00");
    expect(modeLabels()).toContain("Every N days");
    expect(selectedLabel()).toBe("Every N days");
  });

  it("shows the interval field for a stored everyN value", () => {
    renderBuilder("everyN 14 03:00");
    expect(screen.getByDisplayValue("14")).toBeTruthy();
  });
});
