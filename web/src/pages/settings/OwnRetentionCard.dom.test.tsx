// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Settings } from "../../lib/api";
import { en } from "../../lib/i18n";
import { RetentionRulesCard } from "./OwnRetentionCard";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof RetentionRulesCard
>[0]["t"];

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 0,
    ownRetention: {},
    offsiteRetentionKeepLast: 0,
    offsiteRetentionKeepDaily: 14,
    offsiteRetentionKeepWeekly: 0,
    offsiteRetentionKeepMonthly: 12,
    offsiteRetentionKeepYearly: 2,
    ownOffsiteRetention: {},
    ...over,
  } as unknown as Settings;
}

const save = vi.fn<(patch: Partial<Settings>) => Promise<boolean>>();
const cancelDebounce = vi.fn();

function Harness({ initial }: { initial: Settings }) {
  const [settings, setSettings] = useState<Settings | null>(initial);
  if (!settings) return null;
  return (
    <MemoryRouter>
      <RetentionRulesCard
        settings={settings}
        setSettings={setSettings}
        save={(patch) => save(patch)}
        debouncedSave={(_key, run) => run()}
        cancelDebounce={cancelDebounce}
        t={t}
      />
    </MemoryRouter>
  );
}

const LABEL = en["settings.ownOffsiteRetentionFor"];
const FIELD = "ownOffsiteRetention";
const SHARED = { keepLast: 0, keepDaily: 14, keepWeekly: 0, keepMonthly: 12, keepYearly: 2 };

function toggleFor(source: string): HTMLElement {
  return screen.getByRole("switch", { name: LABEL.replace("{source}", source) });
}

/** The number fields of one source, under its switch. */
function fieldsOf(source: string): HTMLInputElement[] {
  const row = toggleFor(source).closest(".flex-col") as HTMLElement;
  return within(row).queryAllByRole("spinbutton") as HTMLInputElement[];
}

afterEach(() => {
  cleanup();
  save.mockReset();
  cancelDebounce.mockReset();
});

describe("the off-site rules card", () => {
  it("shows the shared rules and every source on them until one is switched on", () => {
    render(<Harness initial={baseSettings()} />);
    for (const key of ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const) {
      expect(toggleFor(en[key]).getAttribute("aria-checked")).toBe("false");
    }
    expect(screen.getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(
      Object.values(SHARED).map(String)
    );
  });

  it("starts a source's own rules from the shared values and saves them", async () => {
    save.mockResolvedValue(true);
    render(<Harness initial={baseSettings()} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.vms"]));
    });
    expect(save).toHaveBeenCalledWith({ [FIELD]: { vms: SHARED } });
    expect(toggleFor(en["nav.vms"]).getAttribute("aria-checked")).toBe("true");
    expect(fieldsOf(en["nav.vms"]).map((f) => f.value)).toEqual(Object.values(SHARED).map(String));
  });

  it("saves an edited rule with the rest of the map", async () => {
    save.mockResolvedValue(true);
    const vms = { keepLast: 0, keepDaily: 0, keepWeekly: 4, keepMonthly: 6, keepYearly: 0 };
    const files = { keepLast: 0, keepDaily: 7, keepWeekly: 0, keepMonthly: 0, keepYearly: 0 };
    render(<Harness initial={baseSettings({ [FIELD]: { vms, files } })} />);
    await act(async () => {
      fireEvent.change(fieldsOf(en["nav.vms"])[1], { target: { value: "3" } });
    });
    expect(save).toHaveBeenLastCalledWith({ [FIELD]: { vms: { ...vms, keepDaily: 3 }, files } });
  });

  it("puts a source back on the shared rules when switched off", async () => {
    save.mockResolvedValue(true);
    const vms = { keepLast: 0, keepDaily: 0, keepWeekly: 4, keepMonthly: 6, keepYearly: 0 };
    render(<Harness initial={baseSettings({ [FIELD]: { vms } })} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.vms"]));
    });
    expect(cancelDebounce).toHaveBeenCalledWith(FIELD);
    expect(save).toHaveBeenCalledWith({ [FIELD]: {} });
    expect(fieldsOf(en["nav.vms"])).toHaveLength(0);
  });

  it("flips the switch back when the save is refused", async () => {
    save.mockResolvedValue(false);
    render(<Harness initial={baseSettings()} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.zfs"]));
    });
    expect(toggleFor(en["nav.zfs"]).getAttribute("aria-checked")).toBe("false");
    expect(fieldsOf(en["nav.zfs"])).toHaveLength(0);
  });

  it("shows each source by its name alone, with the whole sentence as the switch's name", () => {
    render(<Harness initial={baseSettings()} />);
    for (const key of ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const) {
      const row = toggleFor(en[key]).closest("[data-search-row]") as HTMLElement;
      expect(row.getAttribute("data-search-row")).toBe(en[key]);
      expect(within(row).getByText(en[key])).toBeTruthy();
      expect(row.textContent).not.toContain(LABEL.replace("{source}", ""));
      expect(within(row).getAllByLabelText(en["settings.ownOffsiteRetentionToggleHint"])).toHaveLength(1);
    }
  });

  it("leaves the local rules alone", async () => {
    save.mockResolvedValue(true);
    render(<Harness initial={baseSettings()} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.flash"]));
    });
    expect(Object.keys(save.mock.calls[0][0])).toEqual([FIELD]);
  });
});

describe("the shared rules of a card", () => {
  it("save the edited field alone", async () => {
    save.mockResolvedValue(true);
    render(<Harness initial={baseSettings()} />);
    const monthly = screen.getAllByRole("spinbutton")[3];
    await act(async () => {
      fireEvent.change(monthly, { target: { value: "24" } });
    });
    expect(save).toHaveBeenCalledWith({ offsiteRetentionKeepMonthly: 24 });
    expect((monthly as HTMLInputElement).value).toBe("24");
  });
});

describe("the off-site card", () => {
  it("sends additional targets to the Off-site page", () => {
    render(<Harness initial={baseSettings()} />);
    const link = screen.getByRole("link", { name: en["settings.retentionExtraTargets"] });
    expect(link.getAttribute("href")).toBe("/settings/storage#offsite");
  });

  it("carries the append-only note in its (i), not as loose text", () => {
    render(<Harness initial={baseSettings()} />);
    expect(screen.queryByText(en["settings.retentionImmutableNotPruned"])).toBeNull();
    const heading = screen.getByRole("heading", { name: new RegExp(`^${en["settings.retentionOffsiteTitle"]}`) });
    const tip = within(heading).getByLabelText(new RegExp(en["settings.retentionOffsiteHint"].slice(0, 30)));
    expect(tip.getAttribute("aria-label")).toContain(en["settings.retentionImmutableNotPruned"]);
    expect(tip.getAttribute("aria-label")).toContain(en["settings.retentionCombineInfo"]);
  });
});
