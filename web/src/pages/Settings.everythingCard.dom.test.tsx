// @vitest-environment jsdom
// The "Backup Everything" card on Settings > Schedules. Its manual trigger
// keeps its label as the accessible name and reports through toasts, and the
// overlap warning shows only while this cadence and a domain cadence are both
// on. The api module is mocked, so clicking the trigger here checks the wiring
// without starting a real backup pass that can run for hours.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, useT, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

// Only backupEverythingNow is replaced; ApiError stays the real class, so the
// component's `err instanceof ApiError` branch for 409 is taken.
const backupEverythingNow = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, backupEverythingNow: (...a: unknown[]) => backupEverythingNow(...a) };
});

// Imported after vi.mock so the component picks up the mocked client.
const { EverythingSection } = await import("./Settings");
const { ApiError } = await import("../lib/api");

/** A Settings object with only the fields this card reads. */
function settingsWith(over: Partial<Settings> = {}): Settings {
  return {
    everythingSchedule: "",
    everythingPreHook: "",
    everythingPostHook: "",
    containersSchedule: "",
    vmsSchedule: "",
    flashSchedule: "",
    filesSchedule: "",
    configSchedule: "",
    ...over,
  } as Settings;
}

function Harness({ settings }: { settings: Settings }) {
  const { t } = useT();
  return <EverythingSection settings={settings} update={() => {}} t={t} hueIndex={7} />;
}

function renderCard(settings: Settings = settingsWith()) {
  return render(
    <I18nProvider>
      <ToastProvider>
        <Harness settings={settings} />
      </ToastProvider>
    </I18nProvider>
  );
}

/** The manual trigger, found by the accessible name `tip` gives it. */
function runNowBadge() {
  return screen.getByRole("button", { name: en["settings.everythingRunNow"] });
}

beforeEach(() => {
  backupEverythingNow.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("EverythingSection manual trigger", () => {
  it("keeps its label as its accessible name in every display mode", () => {
    renderCard();
    const badge = runNowBadge();
    // How much of a control shows is the viewer's choice; every mode keeps a
    // glyph and an accessible name, and runNowBadge() found it by that name.
    expect(badge.querySelector("svg")).not.toBeNull();
    expect(badge).toHaveProperty("disabled", false);
  });

  it("starts exactly one pass per click and reports it as a toast", async () => {
    backupEverythingNow.mockResolvedValue({ ok: true, started: true });
    renderCard();

    fireEvent.click(runNowBadge());

    await waitFor(() => expect(backupEverythingNow).toHaveBeenCalledTimes(1));
    // The started line arrives as a toast.
    expect(await screen.findByText(en["settings.everythingStarted"])).toBeTruthy();
  });

  it("reports an already-running pass (409) with its own message", async () => {
    backupEverythingNow.mockRejectedValue(new ApiError(409, "conflict"));
    renderCard();

    fireEvent.click(runNowBadge());

    expect(await screen.findByText(en["settings.everythingAlreadyRunning"])).toBeTruthy();
    // The raw transport message never reaches the user on this branch.
    expect(screen.queryByText("conflict")).toBeNull();
  });

  it("surfaces a non-409 failure's own message", async () => {
    backupEverythingNow.mockRejectedValue(new Error("host unreachable"));
    renderCard();

    fireEvent.click(runNowBadge());

    expect(await screen.findByText("host unreachable")).toBeTruthy();
  });

  it("surfaces a rejected envelope's error", async () => {
    backupEverythingNow.mockResolvedValue({ ok: false, error: "no repository configured" });
    renderCard();

    fireEvent.click(runNowBadge());

    expect(await screen.findByText("no repository configured")).toBeTruthy();
  });
});

describe("EverythingSection overlap warning", () => {
  const warning = en["settings.everythingDuplicateWarning"];

  it("is hidden while this cadence is off, however many domains are scheduled", () => {
    renderCard(settingsWith({ everythingSchedule: "", containersSchedule: "daily 02:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  it("is hidden while this cadence is the only one on, since nothing runs twice", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  it("appears once both this cadence and a domain cadence are on", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", containersSchedule: "daily 02:00" }));
    expect(screen.getByText(warning)).toBeTruthy();
  });

  it("counts the self-backup cadence too, since the pass ends with it", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", configSchedule: "daily 04:00" }));
    expect(screen.getByText(warning)).toBeTruthy();
  });

  it('treats the literal "off" cadence as off, not as a scheduled value', () => {
    renderCard(settingsWith({ everythingSchedule: "off", containersSchedule: "daily 02:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  // A conditional box that opens with a permanent fact reads as an
  // explanation, and then its disappearance looks like a bug.
  it("says nothing about running independently, that fact belongs to the bubble", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", containersSchedule: "daily 02:00" }));
    expect(screen.getByText(warning).textContent).not.toMatch(/independently/i);
    // …and the permanent fact is still there, as the Card's hint.
    expect(en["settings.everythingHint"]).toMatch(/independent/i);
  });
});

describe("EverythingSection explanations live in bubbles, not on the page", () => {
  it("renders neither explanation as permanent page text", () => {
    renderCard();
    // The Card's hint and the hooks label's hint are InfoBubble tips, which
    // render only once a bubble opens.
    expect(screen.queryByText(en["settings.everythingHint"])).toBeNull();
    expect(screen.queryByText(en["settings.everythingHooksHint"])).toBeNull();
  });

  it("still shows the hook fields' own visible labels and a group heading", () => {
    renderCard();
    expect(screen.getByText(en["hooks.title"])).toBeTruthy();
    expect(screen.getByText(en["hooks.pre"])).toBeTruthy();
    expect(screen.getByText(en["hooks.post"])).toBeTruthy();
  });
});
