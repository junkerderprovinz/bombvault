// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Settings } from "../../lib/api";
import { en } from "../../lib/i18n";
import { OwnRetentionCard } from "./OwnRetentionCard";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof OwnRetentionCard
>[0]["t"];

function baseSettings(own: Settings["ownRetention"] = {}): Settings {
  return {
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 0,
    ownRetention: own,
  } as unknown as Settings;
}

const save = vi.fn<(patch: Partial<Settings>) => Promise<boolean>>();
const cancelDebounce = vi.fn();

function Harness({ initial }: { initial: Settings }) {
  const [settings, setSettings] = useState<Settings | null>(initial);
  if (!settings) return null;
  return (
    <OwnRetentionCard
      settings={settings}
      setSettings={setSettings}
      save={(patch) => save(patch)}
      debouncedSave={(_key, run) => run()}
      cancelDebounce={cancelDebounce}
      t={t}
    />
  );
}

function toggleFor(source: string): HTMLElement {
  return screen.getByRole("switch", { name: en["settings.ownRetentionFor"].replace("{source}", source) });
}

afterEach(() => {
  cleanup();
  save.mockReset();
  cancelDebounce.mockReset();
});

describe("OwnRetentionCard", () => {
  it("lists every source on the shared rules until one is switched on", () => {
    render(<Harness initial={baseSettings()} />);
    for (const key of ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const) {
      expect(toggleFor(en[key]).getAttribute("aria-checked")).toBe("false");
    }
    expect(screen.queryAllByRole("spinbutton")).toHaveLength(0);
  });

  it("starts a source's own rules from the shared values and saves them", async () => {
    save.mockResolvedValue(true);
    render(<Harness initial={baseSettings()} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.vms"]));
    });
    expect(save).toHaveBeenCalledWith({
      ownRetention: { vms: { keepLast: 5, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 0 } },
    });
    expect(toggleFor(en["nav.vms"]).getAttribute("aria-checked")).toBe("true");
    expect(screen.getAllByRole("spinbutton").map((f) => (f as HTMLInputElement).value)).toEqual(["5", "7", "4", "6", "0"]);
  });

  it("saves an edited rule with the rest of the map", async () => {
    save.mockResolvedValue(true);
    const vms = { keepLast: 0, keepDaily: 0, keepWeekly: 4, keepMonthly: 6, keepYearly: 0 };
    const files = { keepLast: 0, keepDaily: 7, keepWeekly: 0, keepMonthly: 0, keepYearly: 0 };
    render(<Harness initial={baseSettings({ vms, files })} />);
    const vmsRow = toggleFor(en["nav.vms"]).closest(".flex-col") as HTMLElement;
    await act(async () => {
      fireEvent.change(within(vmsRow).getAllByRole("spinbutton")[1], { target: { value: "3" } });
    });
    expect(save).toHaveBeenLastCalledWith({ ownRetention: { vms: { ...vms, keepDaily: 3 }, files } });
  });

  it("puts a source back on the shared rules when switched off", async () => {
    save.mockResolvedValue(true);
    const vms = { keepLast: 0, keepDaily: 0, keepWeekly: 4, keepMonthly: 6, keepYearly: 0 };
    render(<Harness initial={baseSettings({ vms })} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.vms"]));
    });
    expect(cancelDebounce).toHaveBeenCalledWith("ownRetention");
    expect(save).toHaveBeenCalledWith({ ownRetention: {} });
    expect(screen.queryAllByRole("spinbutton")).toHaveLength(0);
  });

  it("flips the switch back when the save is refused", async () => {
    save.mockResolvedValue(false);
    render(<Harness initial={baseSettings()} />);
    await act(async () => {
      fireEvent.click(toggleFor(en["nav.zfs"]));
    });
    expect(toggleFor(en["nav.zfs"]).getAttribute("aria-checked")).toBe("false");
    expect(screen.queryAllByRole("spinbutton")).toHaveLength(0);
  });
});
