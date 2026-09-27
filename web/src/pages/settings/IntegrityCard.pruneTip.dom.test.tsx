// @vitest-environment jsdom
/**
 * Prune's bubble around the confirm dialog (#243). The dialog hands focus back
 * to Prune when it closes, and a confirmed prune disables the button, which
 * swaps the element under an open bubble. Neither may leave a bubble behind.
 */
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const pruneDomain = vi.fn();

vi.mock("../../lib/api", () => ({
  pruneDomain: (...a: unknown[]) => pruneDomain(...a),
  unlockDomain: vi.fn().mockResolvedValue({ ok: true }),
  checkDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: vi.fn().mockResolvedValue({ ok: true }),
  tamperTest: vi.fn().mockResolvedValue({ ok: true }),
  getDrills: vi.fn().mockResolvedValue({ ok: true, drills: [], latest: null }),
  getStatus: vi.fn().mockResolvedValue({ ok: true }),
  listContainers: vi.fn().mockResolvedValue({ containers: [] }),
  listVMs: vi.fn().mockResolvedValue({ vms: [] }),
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push: () => {}, quiet: false, setQuiet: () => {} }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { AdvancedProvider } from "../../lib/advanced";
import { en } from "../../lib/i18n";

// The card follows live progress, and jsdom has no EventSource.
class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {
    /* no-op */
  }
}
vi.stubGlobal("EventSource", FakeEventSource);

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];
const settings = { drDrillTarget: "", drDrillTargetVm: "" } as never;
const pruneTip = (en as Record<string, string>)["integrity.pruneHint"];

function pruneBubbles(): Element[] {
  return [...document.querySelectorAll(".glim-bubble")].filter((b) => b.textContent === pruneTip);
}

function renderCard() {
  return render(
    <AdvancedProvider>
      <IntegrityCard t={t} settings={settings} setSettings={() => {}} save={async () => true} />
    </AdvancedProvider>
  );
}

// Presses a button the way a mouse does: the pointer first, which is what the
// tooltip reads to tell a click from keyboard focus, then focus and click.
async function pointerPress(el: HTMLElement) {
  fireEvent.pointerDown(el);
  act(() => el.focus());
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  // Prune is advanced-only, so the card shows it only with the mode on.
  localStorage.setItem("bombvault.advanced", "1");
  pruneDomain.mockReset();
});
afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("the Prune button's bubble (#243)", () => {
  // The keyboard path, where opening the bubble on the returned focus is right,
  // so only the swap to the disabled button can close it.
  it("does not stay on the page after a confirmed prune, neither while it runs nor after", async () => {
    let finish: (v: { ok: boolean }) => void = () => {};
    pruneDomain.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    renderCard();

    const prune = (await screen.findAllByRole("button", { name: /^prune$/i }))[0];
    fireEvent.keyDown(document.body, { key: "Tab" });
    act(() => prune.focus());
    await act(async () => {
      fireEvent.click(prune);
    });

    const dialog = await screen.findByRole("dialog");
    fireEvent.keyDown(document.body, { key: "Enter" });
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: /^confirm$/i }));
    });

    await waitFor(() => expect(pruneDomain).toHaveBeenCalled());
    expect(pruneBubbles(), "the bubble stayed while the prune ran").toHaveLength(0);

    await act(async () => {
      finish({ ok: true });
    });
    expect(pruneBubbles(), "the bubble came back when the prune ended").toHaveLength(0);
  });

  // The mouse path through Cancel: the button stays enabled, so nothing swaps
  // it, and the focus handed back must not open the bubble in the first place.
  it("does not open when Cancel hands the focus back after a click", async () => {
    renderCard();

    const prune = (await screen.findAllByRole("button", { name: /^prune$/i }))[0];
    await pointerPress(prune);
    const dialog = await screen.findByRole("dialog");
    await pointerPress(within(dialog).getByRole("button", { name: /^cancel$/i }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.activeElement, "the dialog no longer hands focus back to Prune").toBe(prune);
    expect(pruneBubbles(), "the bubble opened where the pointer no longer was").toHaveLength(0);
    expect(pruneDomain).not.toHaveBeenCalled();
  });
});
