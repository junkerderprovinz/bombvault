// @vitest-environment jsdom
// The phone pairing switch under Settings, Security saves on the flip and
// answers the way every other switch does.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const answers: { ok: boolean; error?: string }[] = [];
const sent: boolean[] = [];

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getPhonePairing: () => Promise.resolve({ ok: true, on: true }),
  setPhonePairing: (on: boolean) => {
    sent.push(on);
    return Promise.resolve(answers.shift() ?? { ok: true });
  },
}));

const { PhonePairingCard } = await import("./PhonePairingCard");

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <PhonePairingCard hueIndex={0} />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

const pairing = () => screen.getByRole("switch", { name: en["auth.phonePairing"] });

beforeEach(() => {
  answers.length = 0;
  sent.length = 0;
});

afterEach(cleanup);

describe("the phone pairing switch", () => {
  it("confirms a saved flip with the pulse", async () => {
    await renderCard();
    await act(async () => {
      fireEvent.click(pairing());
    });
    expect(sent).toEqual([false]);
    expect(pairing().getAttribute("aria-checked")).toBe("false");
    expect(pairing().parentElement?.className).toContain("glim-confirm");
  });

  it("shakes and stays as it was when the save is refused", async () => {
    answers.push({ ok: false, error: "database is locked" });
    await renderCard();
    await act(async () => {
      fireEvent.click(pairing());
    });
    expect(pairing().getAttribute("aria-checked")).toBe("true");
    expect(pairing().parentElement?.className).toContain("glim-shake");
  });
});
