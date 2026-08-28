// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// EverythingSection — the "Backup Everything" card on Settings > Schedules.
//
// This card shipped from main written against the app as it looked BEFORE
// this branch's UI conventions existed, and was brought up to them after the
// merge (see EverythingSection's own header in Settings.tsx for the full
// list). Two of those conversions are behavioural, not cosmetic, and this
// file pins both so a later round cannot quietly undo them:
//
//   1. The manual trigger became a square icon Badge. Its visible words did
//      not vanish, they MOVED into the tooltip (convention 3) — Badge routes
//      `tip` through IconTipButton, which puts it on `aria-label`, so the
//      control's accessible name is still the label it used to show. A test
//      that queries it BY that name is therefore also the regression guard
//      for the tooltip: drop the `tip` and this file stops finding the
//      button at all.
//
//   2. The started / already-running / error line that used to sit inline
//      beside the text button became a toast, because a 32px badge has no
//      room for it.
//
// And one that is pure state: the overlap warning is no longer permanent. It
// asserts "if BOTH are on, each domain runs twice", so it now renders only
// when that is actually true — which is what keeps it a live conditional
// warning (which convention 6 keeps VISIBLE) rather than a permanent
// explanation (which convention 6 moves into a bubble).
//
// SAFETY — why this is the right place to exercise the trigger at all:
// backupEverythingNow() starts a real, sequential, cross-domain backup pass
// on the server (containers → VMs → flash → folders → self-backup) that can
// run for hours and writes to real repositories. The api module is mocked
// here, so clicking the badge proves the wiring end-to-end (the click reaches
// the client call, the response reaches the toast) without a single byte
// being backed up. This is deliberately the ONLY place that click is
// exercised.
//
// jsdom opted in explicitly (real click + portal-rendered toast) — see
// Selector.dom.test.tsx's header for this repo's naming convention for the
// jsdom-opted-in exception.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, useT, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

// Only backupEverythingNow is exercised; ApiError is the real class so the
// component's `err instanceof ApiError` 409 branch is genuinely taken.
const backupEverythingNow = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, backupEverythingNow: (...a: unknown[]) => backupEverythingNow(...a) };
});

// Imported AFTER vi.mock so the component picks up the mocked client.
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

describe("EverythingSection — the manual trigger", () => {
  it("keeps its LABEL as its accessible name, in whichever mode it renders", () => {
    renderCard();
    const badge = runNowBadge();
    // This used to assert the control had NO text at all, which was right
    // while it was a square icon-only badge. Since #178 how much of a control
    // is shown is the viewer's choice, so the invariant worth pinning is the
    // one that has to survive every mode: it has a glyph, and it has an
    // accessible name. runNowBadge() finds it BY that name, so reaching this
    // line already proves the second half.
    expect(badge.querySelector("svg")).not.toBeNull();
    expect(badge).toHaveProperty("disabled", false);
  });

  it("starts exactly one pass per click and reports it as a toast", async () => {
    backupEverythingNow.mockResolvedValue({ ok: true, started: true });
    renderCard();

    fireEvent.click(runNowBadge());

    await waitFor(() => expect(backupEverythingNow).toHaveBeenCalledTimes(1));
    // The started line is a TOAST now, not an inline sibling of the button.
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

describe("EverythingSection — the overlap warning is conditional", () => {
  const warning = en["settings.everythingDuplicateWarning"];

  it("is hidden while this cadence is off, however many domains are scheduled", () => {
    renderCard(settingsWith({ everythingSchedule: "", containersSchedule: "daily 02:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  it("is hidden while this cadence is the only one on — nothing runs twice", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  it("appears once BOTH this cadence and a domain cadence are on", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", containersSchedule: "daily 02:00" }));
    expect(screen.getByText(warning)).toBeTruthy();
  });

  it("counts the self-backup cadence too — the pass ends with it", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", configSchedule: "daily 04:00" }));
    expect(screen.getByText(warning)).toBeTruthy();
  });

  it('treats the literal "off" cadence as off, not as a scheduled value', () => {
    renderCard(settingsWith({ everythingSchedule: "off", containersSchedule: "daily 02:00" }));
    expect(screen.queryByText(warning)).toBeNull();
  });

  // Issue #177 (manilx): turning the self-backup schedule off made "a help
  // text disappear". The box was hiding correctly — his self-backup was the
  // last scheduled domain, so nothing ran twice any more — but it opened with
  // "This runs independently of the per-domain schedules above", a permanent
  // fact that also sits in this Card's own info bubble. A conditional box that
  // starts with an explanation reads as an explanation, so its disappearance
  // reads as a bug. The box now carries only the conditional half.
  it("says nothing about running independently, that fact belongs to the bubble", () => {
    renderCard(settingsWith({ everythingSchedule: "daily 03:00", containersSchedule: "daily 02:00" }));
    expect(screen.getByText(warning).textContent).not.toMatch(/independently/i);
    // …and the permanent fact is still there, as the Card's hint.
    expect(en["settings.everythingHint"]).toMatch(/independent/i);
  });
});

describe("EverythingSection — explanations live in bubbles, not on the page", () => {
  it("renders neither explanation as permanent page text", () => {
    renderCard();
    // Both were permanent <p> elements before the convention pass. They are
    // now an InfoBubble `tip` each (the Card's own hint, and the hooks group
    // label's), which is not rendered until the bubble is opened.
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
