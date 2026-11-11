// @vitest-environment jsdom
// The fortress on the Pairing page: outside one, the twelve words join it;
// inside, the phrase shows behind the password with its QR code, the server's
// name saves itself after a pause, and the phrase changes and BombVault leaves
// from the Fortress card.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../../../lib/i18n";
import type { FortressState } from "../../../lib/api";
import { FortressSection } from "./FortressSection";
import { hueCounter } from "../shared";

const PHRASE = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
const NEW_PHRASE = "zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong";

let view: FortressState;
let joinAnswer: Record<string, unknown> | null;
const calls: string[] = [];
const toasts: string[] = [];

function state(over: Partial<FortressState> = {}): FortressState {
  return { ok: true, joined: false, phrase: false, device: "Tower", defaultDevice: "Tower", passwordSet: true, members: [], permissions: [], pairedWith: [], ...over };
}
const joined = (over: Partial<FortressState> = {}) => state({ joined: true, phrase: true, ...over });

vi.mock("../../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../../lib/api")>();
  return {
    ...actual,
    getFortress: () => Promise.resolve(view),
    joinFortress: (phrase: string) => {
      calls.push(`join ${phrase}`);
      if (joinAnswer) return Promise.resolve(joinAnswer);
      view = joined();
      return Promise.resolve(view);
    },
    showFortressPhrase: (password: string) =>
      Promise.resolve(password === "hunter2" ? { ok: true, phrase: PHRASE } : { ok: false, code: "passwordWrong", error: "wrong" }),
    changeFortressPhrase: (password: string) => {
      calls.push(`change ${password}`);
      if (view.passwordSet && password !== "hunter2") return Promise.resolve({ ok: false, code: "passwordWrong", error: "wrong" });
      return Promise.resolve({ ok: true, phrase: NEW_PHRASE, left: ["Cellar"] });
    },
    setFortressDevice: (device: string) => {
      calls.push(`device ${device}`);
      view = { ...view, device: device || view.defaultDevice };
      return Promise.resolve(view);
    },
    leaveFortress: () => {
      calls.push("leave");
      view = state();
      return Promise.resolve(view);
    },
  };
});

vi.mock("../../../lib/toast", () => ({ useToast: () => ({ push: (m: string) => toasts.push(m) }) }));

function renderSection() {
  return render(
    <I18nProvider>
      <FortressSection nextHue={hueCounter()} />
    </I18nProvider>,
  );
}

const button = (hook: string) => document.querySelector<HTMLButtonElement>(`[data-fortress="${hook}"]`)!;
const findButtonOut = async (hook: string) => {
  await screen.findByText("Not in a fortress yet");
  return button(hook);
};
const findButton = async (hook: string) => {
  await screen.findByText(en["fortress.title"]);
  return button(hook);
};

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  view = state();
  joinAnswer = null;
  calls.length = 0;
  toasts.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the switch for giving backups back to the apps", () => {
  it("stands beside the server's name in the fortress and saves when pressed", async () => {
    view = joined();
    const flipped: boolean[] = [];
    render(
      <I18nProvider>
        <FortressSection nextHue={hueCounter()} giveBack={{ on: false, onChange: (v) => flipped.push(v) }} />
      </I18nProvider>,
    );
    const toggle = await screen.findByRole("switch", { name: en["appImport.toggle"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(toggle);
    expect(flipped).toEqual([true]);
  });

  it("waits until BombVault is in a fortress", async () => {
    render(
      <I18nProvider>
        <FortressSection nextHue={hueCounter()} giveBack={{ on: false, onChange: () => {} }} />
      </I18nProvider>,
    );
    await screen.findByText("Not in a fortress yet");
    expect(screen.queryByRole("switch", { name: en["appImport.toggle"] })).toBeNull();
  });
});

describe("FortressSection", () => {
  it("joins with the twelve words pasted into the first field", async () => {
    renderSection();
    expect(await screen.findByText("Not in a fortress yet")).toBeTruthy();
    fireEvent.click(button("open-join"));
    const first = document.querySelector<HTMLInputElement>('[data-word="0"]')!;
    fireEvent.change(first, { target: { value: PHRASE } });
    expect(document.querySelectorAll<HTMLInputElement>("[data-word]")[11].value).toBe("about");
    await act(async () => fireEvent.click(button("join")));
    expect(calls).toEqual([`join ${PHRASE}`]);
    expect(await screen.findByText("BombVault is in the fortress now")).toBeTruthy();
  });

  it("names a mistyped word and keeps the window open", async () => {
    joinAnswer = { ok: false, reason: "unknown_word", word: "recieve", position: 7 };
    renderSection();
    fireEvent.click(await screen.findByRole("button", { name: "Join a fortress" }));
    // The window draws its word fields a moment after it opens.
    const first = await waitFor(() => {
      const field = document.querySelector('[data-word="0"]');
      expect(field).toBeTruthy();
      return field!;
    });
    fireEvent.change(first, { target: { value: PHRASE } });
    await act(async () => fireEvent.click(button("join")));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe(en["pairing.errUnknownWord"].replace("{position}", "7").replace("{word}", "recieve"));
  });

  it("shows the phrase with its QR code only after the password", async () => {
    view = joined();
    renderSection();
    const show = await screen.findByRole("button", { name: "Show phrase" });
    await act(async () => fireEvent.click(show));
    const field = screen.getByLabelText("Password of BombVault");
    fireEvent.change(field, { target: { value: "wrong" } });
    await act(async () => fireEvent.click(button("unlock")));
    expect((await screen.findByRole("alert")).textContent).toBe("That password is not right.");
    fireEvent.change(field, { target: { value: "hunter2" } });
    await act(async () => fireEvent.click(button("unlock")));
    expect(button("words").querySelectorAll("li")).toHaveLength(12);
    expect(document.querySelector('[data-fortress="qr"]')).toBeTruthy();
  });

  it("saves the server's name after a pause in typing", async () => {
    view = joined();
    renderSection();
    const field = (await screen.findByLabelText(en["fortress.deviceName"])) as HTMLInputElement;
    vi.useFakeTimers();
    fireEvent.change(field, { target: { value: "cella" } });
    fireEvent.change(field, { target: { value: "cellar" } });
    await act(async () => {
      vi.advanceTimersByTime(800);
    });
    expect(calls).toEqual(["device cellar"]);
    expect(toasts).toEqual([en["settings.saved"]]);
    expect(field.value).toBe("cellar");
  });

  it("says that the pairing with the other instances comes along into the fortress", async () => {
    view = state({ pairedWith: ["attic"] });
    renderSection();
    fireEvent.click(await findButtonOut("open-join"));
    expect(screen.getByText("BombVault is already paired with attic. Joining moves that pairing into the fortress, so there is nothing to pair again.")).toBeTruthy();
  });

  it("says where the phrase comes from in words that fit this page", async () => {
    renderSection();
    expect(await screen.findByText(en["fortress.outHint"].replace("{app}", "BombVault"))).toBeTruthy();
    fireEvent.click(button("open-join"));
    expect(screen.getByText(en["fortress.phraseWhere"])).toBeTruthy();
    expect(document.body.textContent).not.toMatch(/under Instances|instances of BombVault show up here/);
  });

  it("asks for the password before the phrase changes, and a wrong one changes nothing", async () => {
    view = joined();
    renderSection();
    fireEvent.click(await findButton("open-change"));
    await act(async () => fireEvent.click(button("change")));
    expect(calls).toEqual([]);
    const field = screen.getByLabelText("Password of BombVault");
    fireEvent.change(field, { target: { value: "wrong" } });
    await act(async () => fireEvent.click(button("change-unlock")));
    expect(calls).toEqual(["change wrong"]);
    expect((await screen.findByRole("alert")).textContent).toBe("That password is not right.");
    expect(document.querySelector('[data-fortress="words"]')).toBeNull();
  });

  it("keeps the phrase when the password window closes", async () => {
    view = joined();
    renderSection();
    fireEvent.click(await findButton("open-change"));
    await act(async () => fireEvent.click(button("change")));
    await act(async () => fireEvent.keyDown(document.activeElement ?? document.body, { key: "Escape" }));
    expect(calls).toEqual([]);
    expect((await screen.findByRole("alert")).textContent).toBe(en["fortress.changeKept"]);
  });

  it("changes the phrase without asking where BombVault has no password", async () => {
    view = joined({ passwordSet: false });
    renderSection();
    fireEvent.click(await findButton("open-change"));
    await act(async () => fireEvent.click(button("change")));
    expect(calls).toEqual(["change "]);
    expect(button("words").textContent).toContain("wrong");
  });

  it("changes the phrase for the password and leaves from the Fortress card", async () => {
    view = joined();
    renderSection();
    expect(await screen.findByText(en["fortress.title"])).toBeTruthy();
    fireEvent.click(button("open-change"));
    await act(async () => fireEvent.click(button("change")));
    fireEvent.change(screen.getByLabelText("Password of BombVault"), { target: { value: "hunter2" } });
    await act(async () => fireEvent.click(button("change-unlock")));
    expect(calls).toEqual(["change hunter2"]);
    expect(button("words").textContent).toContain("wrong");
    expect(screen.getByText("Still on the old phrase: Cellar. Enter the new one there once.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    fireEvent.click(button("open-leave"));
    await act(async () => fireEvent.click(button("leave")));
    expect(calls).toEqual(["change hunter2", "leave"]);
    expect(await screen.findByText("Not in a fortress yet")).toBeTruthy();
  });

  it("offers neither showing nor changing a phrase BombVault does not keep", async () => {
    view = joined({ phrase: false });
    renderSection();
    expect(await screen.findByText(en["fortress.noPhraseKept"])).toBeTruthy();
    expect(document.querySelector('[data-fortress="show"]')).toBeNull();
    expect(document.querySelector('[data-fortress="open-change"]')).toBeNull();
    expect(button("open-leave")).toBeTruthy();
  });
});
