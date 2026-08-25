// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// CadenceBuilder — the `allowEveryN` mode gate (#166).
//
// The backend rejects an everyN cadence for the DR-drill, tamper-test and
// weekly-digest schedules ("this schedule does not support 'everyN'"), because
// none of those jobs supplies the last-run query the scheduler's interval gate
// needs. Offering the pill anyway let the user pick a mode whose save always
// failed and silently snapped back — the reporter's "can't be saved".
//
// Asserts the rendered pill row, not the prop: what matters is which modes a
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

function renderBuilder(value: string, allowEveryN?: boolean) {
  render(
    <I18nProvider>
      <CadenceBuilder
        label="Schedule"
        value={value}
        onChange={vi.fn()}
        allowEveryN={allowEveryN}
      />
    </I18nProvider>
  );
}

describe("everyN mode availability", () => {
  it("offers every mode by default (backup schedules are gated properly)", () => {
    renderBuilder("daily 02:00");
    expect(modeLabels()).toContain("Every N days");
  });

  it("hides everyN where the backend rejects it, keeping the other four", () => {
    renderBuilder("daily 02:00", false);
    expect(modeLabels()).toEqual(["Off", "Daily", "Weekly", "Cron"]);
  });

  it("still shows everyN when the STORED value already uses it", () => {
    // A value written before the restriction existed (or restored from an
    // imported settings file) must stay visible and selected — hiding it would
    // leave the pill row with nothing active and no way to see what is being
    // changed away from.
    renderBuilder("everyN 3 04:00", false);
    expect(modeLabels()).toContain("Every N days");
  });
});
