// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// CadenceBuilder — which cadence modes a picker offers (#166).
//
// BaukeZwart picked "Every N days" on a Settings schedule card and the save came
// back "this schedule does not support 'everyN'". Two rounds of work met here,
// pulling in opposite directions, and this file pins where they landed:
//
//   - The first round treated the PICKER as the drifting side and taught it a
//     `modes` allow-list, so the drills / tamper-test / digest / per-item cards
//     stopped offering a mode their backend refused.
//   - The round that superseded it made the mode genuinely WORK for the first
//     three: each pass now stamps its own last-run row (internal/store/
//     schedule_job_runs.go, migration v89) and the scheduler gates on it, so the
//     interval is really enforced instead of the daily trigger firing the job
//     nightly — which for the drill schedule meant a nightly DR restore. The API
//     accepts everyN for them now, and their cards pass no `modes` at all.
//
// So the allow-list mechanism SURVIVES and is still sharp — it just has one call
// site left instead of four: the per-item overrides, whose backend still refuses
// everyN (SetScheduleCadence / SetVMScheduleCadence in internal/api/service.go)
// because a per-item entry has no last-run fact of its own. The five off-site
// cadences are refused server-side too (`rejectEveryNSchedules`), but they are
// edited as raw text inputs rather than through this component, so no picker
// there has a mode list to restrict.
//
// Both directions are asserted below, deliberately: that an UNRESTRICTED picker
// offers and emits everyN (the schedules that gained it), and that a RESTRICTED
// one never offers a cadence the save would reject (the per-item box). Asserts
// the rendered pill row, not the props — what matters is which modes a user can
// actually click.
//
// jsdom opt-in per this repo's `.dom.test.tsx` convention (vitest.config.ts stays
// "node" by default).
// ---------------------------------------------------------------------------
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CadenceBuilder, EXACT_CADENCE_MODES, type CadenceMode } from "./CadenceBuilder";
import { I18nProvider, en } from "../lib/i18n";

const EVERY_N_PILL = en["cadence.everyN"]; // "Every N days"

/**
 * A controlled host mirroring how Settings.tsx owns the cadence string itself:
 * the builder is fully controlled, so the round-trip a real save would PUT is
 * whatever lands in the spy.
 */
function Card({
  initial,
  modes,
  onChangeSpy,
}: {
  initial: string;
  modes?: CadenceMode[];
  onChangeSpy?: (v: string) => void;
}) {
  const [value, setValue] = useState(initial);
  return (
    <CadenceBuilder
      label="Schedule"
      value={value}
      modes={modes}
      onChange={(v) => {
        setValue(v);
        onChangeSpy?.(v);
      }}
    />
  );
}

/**
 * The mode pills' visible labels, in rendered order. They are the shared
 * Selector's segments, so they carry role="tab" inside its tablist — not
 * role="button" (the only plain button in this fieldset is the time picker's
 * trigger).
 */
function pillNames(): string[] {
  return screen.queryAllByRole("tab").map((el) => el.textContent ?? "");
}

/** The pill currently selected, via the Selector's aria-selected state. */
function selectedLabel(): string | undefined {
  return (
    screen.queryAllByRole("tab").find((el) => el.getAttribute("aria-selected") === "true")?.textContent ?? undefined
  );
}

/** Uncontrolled render through the real provider, for display-only assertions. */
function renderBuilder(value: string) {
  render(
    <I18nProvider>
      <CadenceBuilder label="Schedule" value={value} onChange={vi.fn()} />
    </I18nProvider>
  );
}

afterEach(() => {
  cleanup();
});

describe("CadenceBuilder — default modes (schedules whose backend accepts everyN)", () => {
  it("still offers all five modes, Every N days included", () => {
    render(<Card initial="off" />);
    expect(pillNames()).toEqual([
      en["cadence.off"],
      en["cadence.daily"],
      en["cadence.weekly"],
      EVERY_N_PILL,
      en["cadence.cron"],
    ]);
  });

  it("emits an everyN cadence when that pill is picked", () => {
    const spy = vi.fn();
    render(<Card initial="off" onChangeSpy={spy} />);
    fireEvent.click(screen.getByRole("tab", { name: EVERY_N_PILL }));
    // DEFAULT_CADENCE: 3 days at 02:00 — the containers/VMs/flash/files/config
    // and Backup Everything schedules all accept this shape.
    expect(spy).toHaveBeenCalledWith("everyN 3 02:00");
  });

  it("shows no every-N-unavailable hint", () => {
    render(<Card initial="off" />);
    expect(screen.queryByText(en["cadence.everyNUnavailable"])).toBeNull();
  });

  it("offers everyN on a schedule that used to hide it", () => {
    // The drill / tamper-test / digest editors render exactly this component
    // with no `modes` prop now — there is no per-call-site gate left to pass,
    // because schedule_job_runs makes their interval enforceable.
    renderBuilder("weekly Mon 08:00");
    expect(pillNames()).toContain(EVERY_N_PILL);
  });

  it("emits everyN from an unrestricted picker — the exact cadence #166 could not save", () => {
    const emitted: string[] = [];
    render(<Card initial="weekly Mon 08:00" onChangeSpy={(v) => emitted.push(v)} />);
    fireEvent.click(screen.getByRole("tab", { name: EVERY_N_PILL }));
    expect(emitted.filter((v) => v.startsWith("everyN"))).not.toEqual([]);
  });

  it("selects everyN when the stored value already uses it", () => {
    renderBuilder("everyN 3 04:00");
    expect(pillNames()).toContain(EVERY_N_PILL);
    expect(selectedLabel()).toBe(EVERY_N_PILL);
  });

  it("shows the interval field for a stored everyN value", () => {
    renderBuilder("everyN 14 03:00");
    expect(screen.getByDisplayValue("14")).toBeTruthy();
  });
});

describe("CadenceBuilder — EXACT_CADENCE_MODES (the per-item overrides)", () => {
  it("does not offer Every N days", () => {
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} />);
    expect(pillNames()).toEqual([en["cadence.off"], en["cadence.daily"], en["cadence.weekly"], en["cadence.cron"]]);
    expect(screen.queryByRole("tab", { name: EVERY_N_PILL })).toBeNull();
  });

  it("explains why the mode is missing instead of leaving a silent gap", () => {
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} />);
    expect(screen.getByText(en["cadence.everyNUnavailable"])).toBeTruthy();
  });

  it("no reachable pill can produce a cadence the per-item save rejects", () => {
    const emitted: string[] = [];
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} onChangeSpy={(v) => emitted.push(v)} />);
    for (const name of pillNames()) {
      fireEvent.click(screen.getByRole("tab", { name }));
    }
    expect(emitted.length).toBeGreaterThan(0);
    // The exact grammar service.go still refuses for a per-item entry (#166/#121).
    expect(emitted.filter((v) => v.startsWith("everyN"))).toEqual([]);
  });
});

describe("CadenceBuilder — legacy everyN value already stored", () => {
  it("keeps the pill so a value from an older build or an import is displayed, not eaten", () => {
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} />);
    const pill = screen.getByRole("tab", { name: EVERY_N_PILL });
    expect(pill.getAttribute("aria-selected")).toBe("true");
    // Shown as its real stored cadence, not silently reset to off/daily: the
    // interval and time fields both carry the parsed value. (This used to assert
    // the prose line "every 3 days at 4:00"; CadenceBuilder's own inline preview
    // paragraph is gone — every call site renders a ScheduleRow/ScheduleBadge
    // above the editor instead, which is where that prose lives now, so asserting
    // it here would only pin a component this one no longer contains.)
    expect(screen.getByDisplayValue("3")).toBeTruthy();
    expect(screen.getByText("04:00")).toBeTruthy();
  });

  it("never rewrites that value on mount", () => {
    const spy = vi.fn();
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} onChangeSpy={spy} />);
    expect(spy).not.toHaveBeenCalled();
  });

  it("drops the legacy pill once the user moves off it — there is nothing left to preserve", () => {
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} />);
    fireEvent.click(screen.getByRole("tab", { name: en["cadence.daily"] }));
    expect(screen.queryByRole("tab", { name: EVERY_N_PILL })).toBeNull();
  });
});
