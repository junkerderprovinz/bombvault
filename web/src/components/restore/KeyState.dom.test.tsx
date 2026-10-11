// @vitest-environment jsdom
/**
 * The line on the Restore page that says which key this server writes its
 * backups with, once that is not its own.
 */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import type { EmergencyState } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const undoEmergency = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, undoEmergency: () => undoEmergency() };
});

const { KeyState } = await import("./KeyState");

const ADOPTED_AT = Date.UTC(2026, 8, 14, 12) / 1000;
const onChanged = vi.fn();

function adopted(over: Partial<EmergencyState> = {}): EmergencyState {
  return { backupKey: "adopted", source: "kit", adoptedAt: ADOPTED_AT, undoable: true, ...over };
}

function show(state: EmergencyState | null) {
  render(
    <I18nProvider>
      <ToastProvider>
        <KeyState state={state} onChanged={onChanged} />
      </ToastProvider>
    </I18nProvider>
  );
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("the key this server backs up under", () => {
  it("is not mentioned while it is the server's own", () => {
    show({ backupKey: "own", undoable: false });
    expect(document.body.textContent).toBe("");
  });

  it("is named with the day and the way it was taken over, and with what the kit prints", () => {
    show(adopted());
    const on = new Date(ADOPTED_AT * 1000).toLocaleDateString("en", { dateStyle: "medium" });
    const line = screen.getByTestId("key-adopted");
    expect(line.textContent).toContain(en["restorePage.keyAdopted.kit"].replace("{date}", on));
    expect(line.textContent).toContain(en["restorePage.keyAdoptedKit"]);
  });

  it.each(["typed", "group"] as const)("says so when it came %s", (source) => {
    show(adopted({ source }));
    expect(screen.getByTestId("key-adopted").textContent).toContain(en[`restorePage.keyAdopted.${source}`].split("{date}")[0]);
  });

  it("can be given back after a question, until the first backup", async () => {
    const own: EmergencyState = { backupKey: "own", undoable: false };
    undoEmergency.mockResolvedValue({ ok: true, emergency: own });
    show(adopted());
    await press(screen.getByRole("button", { name: en["emergency.undo"] }));
    expect(undoEmergency).not.toHaveBeenCalled();
    const question = screen.getByRole("dialog");
    expect(within(question).getByText(en["emergency.undoAsk"])).toBeTruthy();
    await press(within(question).getByRole("button", { name: en["emergency.undo"] }));
    expect(undoEmergency).toHaveBeenCalledTimes(1);
    expect(onChanged).toHaveBeenCalledWith(own);
  });

  it("stays once a backup was written with it, and says so", () => {
    show(adopted({ undoable: false }));
    expect(screen.getByText(en["emergency.code.adopt-in-use"])).toBeTruthy();
    expect(screen.queryByRole("button", { name: en["emergency.undo"] })).toBeNull();
  });

  it("says why an undo was refused after all", async () => {
    undoEmergency.mockResolvedValue({ ok: false, code: "adopt-in-use", error: "backups were written under the key" });
    show(adopted());
    await press(screen.getByRole("button", { name: en["emergency.undo"] }));
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["emergency.undo"] }));
    expect(await screen.findByText(en["emergency.code.adopt-in-use"])).toBeTruthy();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("names both remedies when it no longer opens", () => {
    show(adopted({ backupKey: "locked" }));
    expect(screen.getByText(en["emergency.code.backup-key-locked"])).toBeTruthy();
    expect(screen.queryByTestId("key-adopted")).toBeNull();
  });
});
