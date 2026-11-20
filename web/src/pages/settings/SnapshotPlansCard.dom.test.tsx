// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
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
  return screen.getByRole("listitem");
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
});
