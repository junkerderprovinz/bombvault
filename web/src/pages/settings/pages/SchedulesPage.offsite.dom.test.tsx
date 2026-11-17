// @vitest-environment jsdom
// The Schedule page offers plans for off-site copies only while there can be
// some: an instance that keeps every backup on the premises gets none.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../../lib/i18n";
import { ToastProvider } from "../../../lib/toast";
import type { OffsiteTarget, Settings } from "../../../lib/api";
import { SettingsStoreContext, type SettingsStore } from "../settingsStore";

vi.mock("../../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../../lib/api")>()),
  listItems: () => Promise.resolve({ ok: true, items: [] }),
}));
vi.mock("../WeekCard", () => ({ WeekCard: () => null }));

const { SchedulesPage } = await import("./SchedulesPage");

function settings(over: Partial<Settings>): Settings {
  return {
    containersEnabled: false,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: true,
    zfsEnabled: false,
    configEnabled: false,
    filesSchedule: "daily 02:00",
    filesOffsiteSchedule: "",
    filesOffsite: "",
    containersOffsite: "",
    vmsOffsite: "",
    flashOffsite: "",
    zfsOffsite: "",
    configOffsite: "",
    everythingSchedule: "off",
    everythingPreHook: "",
    everythingPostHook: "",
    noOffsiteByChoice: false,
    ...over,
  } as unknown as Settings;
}

async function show(over: Partial<Settings> = {}, allTargets: OffsiteTarget[] = []) {
  const store = {
    settings: settings(over),
    allTargets,
    containers: [],
    vms: [],
    fieldPulse: {},
    schedFieldBusy: {},
    schedFieldShake: {},
    mergedFieldBusy: {},
    mergedFieldShake: {},
    scheduleField: vi.fn(),
    scheduleUpdate: vi.fn(),
    autoSaveScheduleField: vi.fn(),
    autoSaveField: vi.fn(),
  } as unknown as SettingsStore;
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SettingsStoreContext.Provider value={store}>
              <SchedulesPage />
            </SettingsStoreContext.Provider>
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
}

const copyPlans = () => screen.queryAllByText(en["schedules.offsite"]);

describe("the plans for off-site copies", () => {
  beforeEach(() => localStorage.setItem("bv-lang", "en"));
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("are there while off-site copies can be set up", async () => {
    await show();
    expect(screen.getByText(en["schedules.plans"])).toBeTruthy();
    expect(copyPlans().length).toBeGreaterThan(0);
  });

  it("step aside while no section copies off-site by choice", async () => {
    await show({ noOffsiteByChoice: true });
    expect(screen.getByText(en["schedules.plans"])).toBeTruthy();
    expect(copyPlans()).toHaveLength(0);
  });

  it("stay while a section copies off-site, where the choice does not count", async () => {
    await show({ noOffsiteByChoice: true, filesOffsite: "rest:http://box:8000/files" });
    expect(copyPlans().length).toBeGreaterThan(0);
  });
});
