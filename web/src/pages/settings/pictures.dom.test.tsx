// @vitest-environment jsdom
// The week of planned runs and the pictures on the Verify and Security pages.
// A picture's markup is its last frame, so what it says has to be in the
// document without any animation having run.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import type { ScheduleNext, Settings } from "../../lib/api";

const api = vi.hoisted(() => ({ getScheduleNext: vi.fn(), getSettings: vi.fn() }));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...api,
}));

const { CryptScene, PackGrid, SandboxPicture } = await import("./pictures");
const { WeekCard, plansOf } = await import("./WeekCard");

function settings(over: Partial<Settings> = {}): Settings {
  const schedules = Object.fromEntries(
    ["containers", "vms", "flash", "files", "zfs", "config"].flatMap((d) => [
      [`${d}Schedule`, "off"],
      [`${d}OffsiteSchedule`, ""],
    ])
  );
  return {
    containersEnabled: true,
    vmsEnabled: true,
    flashEnabled: false,
    filesEnabled: false,
    zfsEnabled: false,
    configEnabled: false,
    everythingSchedule: "off",
    drillsEnabled: false,
    drillsSchedule: "weekly Sun 05:00",
    tamperTestSchedule: "off",
    ...schedules,
    ...over,
  } as unknown as Settings;
}

async function renderWeek(s: Settings, next: ScheduleNext[] = []) {
  api.getScheduleNext.mockResolvedValue(next);
  await act(async () => {
    render(
      <I18nProvider>
        <WeekCard settings={s} hueIndex={0} />
      </I18nProvider>
    );
  });
}

const strokes = () => [...document.querySelectorAll<HTMLElement>(".glim-week-stroke")];

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  // The server's clock is the browser's here, so no zone is named.
  api.getSettings.mockResolvedValue({ ok: true });
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  for (const fn of Object.values(api)) fn.mockReset();
});

describe("plansOf", () => {
  const t = (key: string) => key;

  it("lists the full backup, each domain that is on with its off-site copy, and the checks", () => {
    const keys = plansOf(settings({ drillsEnabled: true }), [], t as never).map((p) => p.key);
    expect(keys).toEqual(["everything", "containers", "containers-offsite", "vms", "vms-offsite", "drills", "tamper"]);
  });

  it("leaves the trial restore out while it is switched off", () => {
    expect(plansOf(settings(), [], t as never).map((p) => p.key)).not.toContain("drills");
  });

  it("hands each plan the next fire the server reports for it", () => {
    const next = [
      { job: "backup", domain: "vms", next: "2026-10-14T02:00:00Z" },
      { job: "offsite", domain: "vms", next: "2026-10-15T02:00:00Z" },
    ];
    const plans = plansOf(settings(), next, t as never);
    expect(plans.find((p) => p.key === "vms")?.next?.toISOString()).toBe("2026-10-14T02:00:00.000Z");
    expect(plans.find((p) => p.key === "vms-offsite")?.next?.toISOString()).toBe("2026-10-15T02:00:00.000Z");
    expect(plans.find((p) => p.key === "containers")?.next).toBeUndefined();
  });
});

describe("the week of planned runs", () => {
  it("draws one stroke for each start and counts them", async () => {
    await renderWeek(settings({ containersSchedule: "daily 03:00", vmsSchedule: "weekly Sun 04:00" }));
    expect(strokes()).toHaveLength(8);
    expect(document.querySelectorAll(".glim-week-backup.glim-week-stroke")).toHaveLength(8);
    expect(screen.getByText("8 runs a week")).toBeTruthy();
  });

  it("says so in the singular and draws an empty week for no plans", async () => {
    await renderWeek(settings({ vmsSchedule: "weekly Sun 04:00" }));
    expect(screen.getByText("1 run a week")).toBeTruthy();
    cleanup();
    await renderWeek(settings());
    expect(strokes()).toHaveLength(0);
    expect(document.querySelectorAll(".glim-week-lane")).toHaveLength(7);
  });

  it("tells a backup, a copy, a check and a trial restore apart by kind", async () => {
    await renderWeek(
      settings({
        containersSchedule: "weekly Mon 03:00",
        containersOffsiteSchedule: "weekly Tue 03:00",
        tamperTestSchedule: "weekly Wed 03:00",
        drillsEnabled: true,
      })
    );
    for (const kind of ["backup", "copy", "check", "probe"]) {
      expect(document.querySelectorAll(`.glim-week-${kind}.glim-week-stroke`), kind).toHaveLength(1);
    }
  });

  it("names a stroke and its time, and reads it out when it is pointed at", async () => {
    await renderWeek(settings({ containersSchedule: "weekly Mon 03:00" }));
    const [stroke] = strokes();
    expect(stroke.getAttribute("aria-label")).toBe(`${en["jobs.containersSection"]}, 03:00`);
    fireEvent.mouseEnter(stroke);
    expect(screen.getByText(new RegExp(`Mon 03:00 · ${en["jobs.containersSection"]}`))).toBeTruthy();
    fireEvent.mouseLeave(stroke);
    expect(screen.getByText("1 run a week")).toBeTruthy();
  });

  it("lights the strokes of one kind from the legend and dims the rest", async () => {
    await renderWeek(settings({ containersSchedule: "weekly Mon 03:00", tamperTestSchedule: "weekly Wed 03:00" }));
    fireEvent.mouseEnter(screen.getByText(en["tile.week.check"]));
    expect(document.querySelector(".glim-week-check.glim-week-stroke")?.className).toContain("glim-week-lit");
    expect(document.querySelector(".glim-week-backup.glim-week-stroke")?.className).toContain("glim-week-dim");
  });

  it("marks the stretch where three plans start within two hours and names it", async () => {
    await renderWeek(
      settings({
        everythingSchedule: "weekly Sun 03:00",
        containersSchedule: "weekly Sun 03:30",
        vmsSchedule: "weekly Sun 04:00",
      })
    );
    expect(document.querySelectorAll(".glim-week-crowd")).toHaveLength(1);
    expect(screen.getByText(/Sun 03:00 to 04:00: 3 plans within two hours/)).toBeTruthy();
  });

  it("places a plan that counts days once the server has said when it fires next", async () => {
    await renderWeek(settings({ containersSchedule: "everyN 3 02:00" }), [
      { job: "backup", domain: "containers", next: "2026-10-14T12:00:00Z" },
    ]);
    expect(strokes()).toHaveLength(2);
  });

  it("shows now on today's lane only", async () => {
    await renderWeek(settings());
    expect(document.querySelectorAll(".glim-week-now")).toHaveLength(1);
    expect(screen.getByText(en["tile.week.now"])).toBeTruthy();
  });
});

describe("the pictures", () => {
  const show = (node: React.ReactNode) => render(<I18nProvider>{node}</I18nProvider>);

  it("draws a backup as a hundred packs and marks as many as the sample says", () => {
    show(<PackGrid percent={5} />);
    expect(document.querySelectorAll("rect.glim-pack:not(.glim-scene-seal)")).toHaveLength(100);
    expect(document.querySelectorAll(".glim-pack-hit")).toHaveLength(5);
    expect(screen.getByText("5 of 100 data packs")).toBeTruthy();
    expect(screen.getByRole("img", { name: en["tile.verify.packsAlt"] })).toBeTruthy();
  });

  it("adds marks for a larger sample and keeps the ones it had", () => {
    const marked = () =>
      [...document.querySelectorAll(".glim-pack-hit rect")].map((r) => `${r.getAttribute("x")}:${r.getAttribute("y")}`);
    show(<PackGrid percent={5} />);
    const five = marked();
    cleanup();
    show(<PackGrid percent={20} />);
    const twenty = marked();
    expect(twenty).toHaveLength(20);
    expect(twenty).toEqual(expect.arrayContaining(five));
  });

  it("keeps the sample between none and all of the packs", () => {
    show(<PackGrid percent={250} />);
    expect(document.querySelectorAll(".glim-pack-hit")).toHaveLength(100);
    cleanup();
    show(<PackGrid percent={-4} />);
    expect(document.querySelectorAll(".glim-pack-hit")).toHaveLength(0);
  });

  it("shows the sandbox with the start test as it is set", () => {
    show(<SandboxPicture startTest={false} />);
    expect(screen.getByRole("img", { name: en["tile.verify.sandboxAlt"] })).toBeTruthy();
    expect(screen.getByText(en["tile.verify.startTestOff"])).toBeTruthy();
    expect(document.querySelector(".glim-sandbox-pulse")).toBeNull();
    cleanup();
    show(<SandboxPicture startTest />);
    expect(screen.getByText(en["settings.startTest"])).toBeTruthy();
    expect(document.querySelector(".glim-sandbox-pulse")).not.toBeNull();
  });

  it("leaves four sealed packs at the storage location and the key in the recovery kit", () => {
    show(<CryptScene />);
    const scene = screen.getByRole("img", { name: en["tile.crypt.sceneAlt"] });
    expect(scene.querySelectorAll(".glim-parcel-fly")).toHaveLength(4);
    expect(scene.textContent).toContain(en["tile.crypt.atHome"]);
    expect(scene.textContent).toContain(en["tile.crypt.location"]);
    expect(scene.textContent).toContain(en["recovery.title"]);
    expect(scene.querySelector(".glim-crypt-key")).not.toBeNull();
  });
});
