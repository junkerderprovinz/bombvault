// @vitest-environment jsdom
// The upkeep of a shared repository in the receiving server's window: what it
// saves, its last passes, a tidy-up started at once and a rule of its own. A
// repository this server only copies into shows nothing here.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { SharedUpkeep, SharedUpkeepRules } from "../../lib/api";

const owned: SharedUpkeep = {
  id: "sr1",
  location: "/mnt/user/shared",
  role: "owner",
  members: [
    { host: "bv-0123456789ab", name: "Tower" },
    { host: "bv-ba9876543210", name: "Cube" },
  ],
  rules: {
    keep: { keepLast: 0, keepDaily: 0, keepWeekly: 0, keepMonthly: 0, keepYearly: 0 },
    keepSet: false,
    pruneCadence: "weekly sun 03:00",
    pruneHours: 6,
    maxUnused: 5,
    checkSubset: 5,
  },
  forget: { at: 1_700_000_000, forgotten: 3, skipped: ["Cube"] },
  prune: { at: 1_700_000_000, seconds: 125, postponed: [], checked: true },
  space: {
    at: 1_700_000_000,
    total: 3 << 20,
    hosts: [
      { host: "bv-0123456789ab", name: "Tower", bytes: 2 << 20 },
      { host: "bv-ba9876543210", name: "Cube", bytes: 2 << 20 },
    ],
  },
  nextWindow: 1_700_500_000,
  running: false,
};

let repos: SharedUpkeep[] = [];
const saved: { id: string; rules: SharedUpkeepRules }[] = [];
const runs: string[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSharedUpkeep: () => Promise.resolve({ ok: true, repos }),
    setSharedUpkeepRules: (id: string, rules: SharedUpkeepRules) => {
      saved.push({ id, rules });
      return Promise.resolve({ ok: true });
    },
    runSharedUpkeep: (id: string) => {
      runs.push(id);
      return Promise.resolve({ ok: true });
    },
  };
});

const { SharedUpkeepSection } = await import("./SharedUpkeepSection");

function Shown() {
  const { t } = useT();
  return <SharedUpkeepSection t={t} />;
}

async function show(list: SharedUpkeep[]) {
  repos = list;
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Shown />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

afterEach(() => {
  cleanup();
  saved.length = 0;
  runs.length = 0;
});

describe("SharedUpkeepSection", () => {
  it("says what the repository saves and how its last passes went", async () => {
    await show([owned]);
    expect(screen.getByText("Holds 3.0 MB. Kept apart, the 2 servers would need 4.0 MB. Saved: 1.0 MB.")).toBeTruthy();
    expect(screen.getByText("Tower uses 2.0 MB")).toBeTruthy();
    expect(screen.getByText(/Kept every backup of Cube/)).toBeTruthy();
    expect(screen.getByText("The check after it found no errors.")).toBeTruthy();
    expect(document.querySelector('[data-new="shared-upkeep"]')).not.toBeNull();
  });

  it("shows nothing for a repository this server only copies into", async () => {
    await show([{ ...owned, role: "member", rules: undefined }]);
    expect(screen.queryByText("Shared repository")).toBeNull();
  });

  it("starts a tidy-up at once", async () => {
    await show([owned]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Tidy up now" }));
    });
    expect(runs).toEqual(["sr1"]);
  });

  it("gives the repository a rule of its own", async () => {
    await show([owned]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Set a rule of its own" }));
    });
    expect(saved).toEqual([{ id: "sr1", rules: { ...owned.rules, keepSet: true } }]);
  });

  it("names the members still copying when the tidy-up moved on", async () => {
    await show([{ ...owned, prune: { at: 1_700_000_000, seconds: 0, postponed: ["Cube"], checked: false } }]);
    expect(screen.getByText(/still copying: Cube/)).toBeTruthy();
  });
});
