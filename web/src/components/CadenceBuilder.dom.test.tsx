// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// CadenceBuilder — the mode allow-list (#166).
//
// BaukeZwart picked "Every N days" on a Settings schedule card and the save
// came back "this schedule does not support 'everyN'". The picker was the
// problem, not the save: its mode list was hard-coded to all five modes at
// every call site, including the drills / tamper-test / digest / per-item
// schedules whose backend REFUSES everyN (they have no last-run gate, so an
// everyN cadence there would silently fire daily —
// `rejectEveryNSchedules` in internal/api/handlers.go). And because the whole
// Schedules tab shares one Save button that PUTs the full settings object,
// one everyN in the drills card rejected the entire tab's save, which is why
// the report reads "the schedule can't be saved".
//
// These tests pin the contract from the UI side: the restricted picker never
// OFFERS a cadence the backend would reject, the default picker is unchanged,
// and a legacy everyN value already in the DB is still displayed rather than
// silently rewritten.
//
// jsdom opt-in per this repo's `.dom.test.tsx` convention (vitest.config.ts
// stays "node" by default). No I18nProvider is mounted: I18nContext's default
// value already resolves keys against the inline `en` table, which is exactly
// the English labelling asserted below.
// ---------------------------------------------------------------------------
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CadenceBuilder, EXACT_CADENCE_MODES, type CadenceMode } from "./CadenceBuilder";
import { en } from "../lib/i18n";

afterEach(() => {
  cleanup();
});

const EVERY_N_PILL = en["cadence.everyN"]; // "Every N days"

/**
 * A controlled host mirroring how Settings.tsx owns the cadence string itself:
 * the builder is fully controlled, so the round-trip a real save would PUT is
 * whatever lands in `last`.
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

function pillNames(): string[] {
  return screen.getAllByRole("tab").map((el) => el.textContent ?? "");
}

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
    // DEFAULT_CADENCE: 3 days at 02:00 — the containers/VMs/flash/files/
    // Backup Everything schedules all accept this shape.
    expect(spy).toHaveBeenCalledWith("everyN 3 02:00");
  });

  it("shows no every-N-unavailable hint", () => {
    render(<Card initial="off" />);
    expect(screen.queryByText(en["cadence.everyNUnavailable"])).toBeNull();
  });
});

describe("CadenceBuilder — EXACT_CADENCE_MODES (drills / tamper / digest / per-item)", () => {
  it("does not offer Every N days", () => {
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} />);
    expect(pillNames()).toEqual([en["cadence.off"], en["cadence.daily"], en["cadence.weekly"], en["cadence.cron"]]);
    expect(screen.queryByRole("tab", { name: EVERY_N_PILL })).toBeNull();
  });

  it("explains why the mode is missing instead of leaving a silent gap", () => {
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} />);
    expect(screen.getByText(en["cadence.everyNUnavailable"])).toBeTruthy();
  });

  it("BaukeZwart's scenario: no reachable pill can produce a cadence the save rejects", () => {
    const emitted: string[] = [];
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} onChangeSpy={(v) => emitted.push(v)} />);
    for (const name of pillNames()) {
      fireEvent.click(screen.getByRole("tab", { name }));
    }
    expect(emitted.length).toBeGreaterThan(0);
    // The exact grammar the backend refuses for these schedules (#166).
    expect(emitted.filter((v) => v.startsWith("everyN"))).toEqual([]);
  });
});

describe("CadenceBuilder — legacy everyN value already stored", () => {
  it("keeps the pill so a value from an older build or an import is displayed, not eaten", () => {
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} />);
    const pill = screen.getByRole("tab", { name: EVERY_N_PILL });
    expect(pill.getAttribute("aria-selected")).toBe("true");
    // Rendered as its real cadence, not silently reset to off/daily.
    expect(screen.getByText("every 3 days at 4:00")).toBeTruthy();
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
