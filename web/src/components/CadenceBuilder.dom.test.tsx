// @vitest-environment jsdom
// Which cadence modes the picker offers. A picker without `modes` offers all of
// them, Every N days included. The per-item overrides pass EXACT_CADENCE_MODES,
// because SetScheduleCadence and SetVMScheduleCadence in internal/api/service.go
// refuse everyN: a per-item entry has no last-run row to measure the interval
// from. The tests assert the rendered pills, since those are what a user can
// click.
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { CadenceBuilder, EXACT_CADENCE_MODES, type CadenceMode } from "./CadenceBuilder";
import { I18nProvider, en } from "../lib/i18n";

const EVERY_N_PILL = en["cadence.everyN"]; // "Every N days"

/** A controlled host like Settings.tsx, so the spy receives what a save would send. */
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

/** The mode pills' labels in rendered order. They are Selector segments, so their role is "tab". */
function pillNames(): string[] {
  return screen.queryAllByRole("tab").map((el) => el.textContent ?? "");
}

/** The pill currently selected, via the Selector's aria-selected state. */
function selectedLabel(): string | undefined {
  return (
    screen.queryAllByRole("tab").find((el) => el.getAttribute("aria-selected") === "true")?.textContent ?? undefined
  );
}

/** Renders a fixed value through the real provider, for display-only assertions. */
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

describe("CadenceBuilder with the default modes", () => {
  it("offers all five modes, Every N days included", () => {
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
    expect(spy).toHaveBeenCalledWith("everyN 3 02:00");
  });

  it("shows no every-N-unavailable hint", () => {
    render(<Card initial="off" />);
    expect(screen.queryByText(en["cadence.everyNUnavailable"])).toBeNull();
  });

  it("offers everyN while a weekly cadence is stored", () => {
    renderBuilder("weekly Mon 08:00");
    expect(pillNames()).toContain(EVERY_N_PILL);
  });

  it("emits everyN when switching from a weekly cadence", () => {
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

describe("CadenceBuilder with EXACT_CADENCE_MODES", () => {
  it("does not offer Every N days", () => {
    render(<Card initial="off" modes={EXACT_CADENCE_MODES} />);
    expect(pillNames()).toEqual([en["cadence.off"], en["cadence.daily"], en["cadence.weekly"], en["cadence.cron"]]);
    expect(screen.queryByRole("tab", { name: EVERY_N_PILL })).toBeNull();
  });

  it("explains why Every N days is missing", () => {
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
    expect(emitted.filter((v) => v.startsWith("everyN"))).toEqual([]);
  });
});

describe("CadenceBuilder with EXACT_CADENCE_MODES and a stored everyN value", () => {
  it("keeps the everyN pill so the stored value is shown rather than lost", () => {
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} />);
    const pill = screen.getByRole("tab", { name: EVERY_N_PILL });
    expect(pill.getAttribute("aria-selected")).toBe("true");
    expect(screen.getByDisplayValue("3")).toBeTruthy();
    expect(screen.getByText("04:00")).toBeTruthy();
  });

  it("never rewrites that value on mount", () => {
    const spy = vi.fn();
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} onChangeSpy={spy} />);
    expect(spy).not.toHaveBeenCalled();
  });

  it("drops the everyN pill once the user picks another mode", () => {
    render(<Card initial="everyN 3 04:00" modes={EXACT_CADENCE_MODES} />);
    fireEvent.click(screen.getByRole("tab", { name: en["cadence.daily"] }));
    expect(screen.queryByRole("tab", { name: EVERY_N_PILL })).toBeNull();
  });
});
