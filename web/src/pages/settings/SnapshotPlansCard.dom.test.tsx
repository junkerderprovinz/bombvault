// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { HostSnapPlan } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const PLAN: HostSnapPlan = {
  id: "p1",
  kind: "zfs",
  fs: "",
  source: "bvtest/docs",
  cadence: "daily 03:00",
  keep: 7,
  recursive: false,
  skipAsleep: true,
  enabled: true,
  lastRunAt: 0,
  lastCode: "",
  lastDetail: "",
};

let plans: HostSnapPlan[] = [PLAN];

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  listHostSnapshotPlans: () => Promise.resolve({ ok: true, plans }),
  listHostSnapshots: () =>
    Promise.resolve({
      ok: true,
      sources: [{ kind: "zfs", source: "tank/new", group: "tank", usedBytes: 0, newSinceLatestBytes: 0, canRecursive: true, canCreate: true }],
    }),
  addHostSnapshotPlan: () => Promise.resolve({ ok: true, plan: { ...PLAN, id: "p2", source: "tank/new" } }),
  openedAsPairedPhone: () => false,
}));

const { SnapshotPlansCard } = await import("./SnapshotPlansCard");

async function show() {
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SnapshotPlansCard hueIndex={0} />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  return screen.getAllByRole("listitem")[0];
}

afterEach(() => {
  cleanup();
  plans = [PLAN];
});

describe("a snapshot schedule", () => {
  it("leads with its target and a line on its plan", async () => {
    const plan = await show();
    const lines = [...plan.querySelectorAll("[data-plan-head] > *")].map((e) => e.textContent);
    expect(lines).toEqual([`${en["hostSnap.kind.zfs"]}: bvtest/docs`, `Daily at 03:00 · ${en["hostSnap.plans.keep"]}: 7`]);
  });

  it("says when it is switched off instead of when it runs", async () => {
    plans = [{ ...PLAN, enabled: false }];
    const plan = await show();
    expect(plan.querySelector("[data-plan-head]")?.textContent).toContain(en["jobs.notScheduled"]);
  });

  it("ends with its buttons, after every field", async () => {
    const plan = await show();
    const controls = within(plan).getAllByRole("button").concat(within(plan).getAllByRole("switch"));
    const order = [...plan.querySelectorAll("button, [role=switch]")].filter((e) => controls.includes(e as HTMLElement));
    const names = order.map((e) => e.getAttribute("aria-label") ?? e.textContent);
    expect(names.slice(-2)).toEqual([en["hostSnap.plans.runNow"], en["hostSnap.plans.remove"]]);
  });

  it("shows its target, its plan and its switch, and keeps the fields closed until Edit", async () => {
    const plan = await show();
    expect(within(plan).getByRole("switch", { name: en["hostSnap.plans.enabled"] })).toBeTruthy();
    expect(within(plan).queryByLabelText(en["hostSnap.plans.keep"])).toBeNull();
    const edit = within(plan).getByRole("button", { name: en["common.edit"] });
    expect(edit.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(edit);
    expect(within(plan).getByLabelText(en["hostSnap.plans.keep"])).toBeTruthy();
    fireEvent.click(within(plan).getByRole("button", { name: en["common.done"] }));
    expect(within(plan).queryByLabelText(en["hostSnap.plans.keep"])).toBeNull();
  });

  it("opens a new schedule with its fields shown", async () => {
    await show();
    fireEvent.click(screen.getByRole("button", { name: en["hostSnap.plans.add"] }));
    const window = await screen.findByRole("dialog");
    await act(async () => {
      fireEvent.click(await within(window).findByRole("button", { name: en["hostSnap.plans.add"] }));
    });
    const added = screen.getAllByRole("listitem")[1];
    expect(within(added).getByLabelText(en["hostSnap.plans.keep"])).toBeTruthy();
    expect(within(screen.getAllByRole("listitem")[0]).queryByLabelText(en["hostSnap.plans.keep"])).toBeNull();
  });
});
