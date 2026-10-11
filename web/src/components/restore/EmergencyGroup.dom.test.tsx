// @vitest-environment jsdom
/**
 * The way into an emergency through the group. A member keeps the sealed kit
 * of the install that is gone; this server asks for it, a person at the
 * member allows it, and the recovery code opens it. The request waits on the
 * member, so the window can be closed and opened again in the meantime. The
 * code is in its field and nowhere else.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import type { EmergencyState, OfferedDeposit } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";

const listOfferedDeposits = vi.fn();
const emergencyFromGroup = vi.fn();

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listOfferedDeposits: () => listOfferedDeposits(),
    emergencyFromGroup: (...a: unknown[]) => emergencyFromGroup(...a),
  };
});

const { EmergencyGroup } = await import("./EmergencyGroup");

const UPDATED = "2026-09-14T03:00:00Z";
const TOWER: OfferedDeposit = { memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower", updatedAt: UPDATED };
const CABIN: OfferedDeposit = { memberId: "m-nas", memberName: "nas", ownerId: "o-cabin", ownerName: "cabin", updatedAt: UPDATED };
const CODE = "ABCDE-FGHJK-MNPQR-STVWX";
const ASKED_KEY = "bv-emergency-asked";

const OPENED: EmergencyState = {
  backupKey: "own",
  undoable: false,
  session: { source: "group", encrypted: true, places: [], expiresAt: 1_800_000_000, cloudSignIn: false, rcloneConfig: false },
};

const onEmergency = vi.fn();
const onPairing = vi.fn();

async function open(active: boolean | null = true) {
  await act(async () => {
    render(
      <I18nProvider>
        <EmergencyGroup active={active} onEmergency={onEmergency} onPairing={onPairing} />
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const codeField = () => screen.getByLabelText(en["emergency.group.code"]) as HTMLInputElement;
const typeCode = (code: string) => fireEvent.change(codeField(), { target: { value: code } });
const lastCall = () => emergencyFromGroup.mock.calls.at(-1) as [string, string, string, boolean];
const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const named = (key: "emergency.group.waiting" | "emergency.group.resumeText", keeper = "nas") => en[key].replace("{keeper}", keeper);

/** Picks tower's deposit, types the code and asks for it. */
async function ask() {
  await open();
  await press(screen.getAllByRole("button", { name: en["emergency.group.choose"] })[1]);
  typeCode(CODE);
  await press(button(en["emergency.group.ask"]));
}

beforeEach(() => {
  localStorage.clear();
  listOfferedDeposits.mockResolvedValue({ ok: true, deposits: [CABIN, TOWER], silent: [] });
  emergencyFromGroup.mockResolvedValue({ ok: true, release: "asked" });
});

afterEach(() => {
  vi.useRealTimers();
  cleanup();
  vi.clearAllMocks();
});

describe("an install outside a group", () => {
  it("is told to join first and led to the pairing", async () => {
    await open(false);
    expect(screen.getByText(en["emergency.group.noGroup"])).toBeTruthy();
    expect(listOfferedDeposits).not.toHaveBeenCalled();
    await press(button(en["emergency.group.toPairing"]));
    expect(onPairing).toHaveBeenCalledTimes(1);
  });

  it("waits for the server to say whether it is in one", async () => {
    await open(null);
    expect(screen.getByText(en["dashboard.checking"])).toBeTruthy();
    expect(listOfferedDeposits).not.toHaveBeenCalled();
  });
});

describe("the deposits of the group", () => {
  it("are listed by the server they belong to, with who keeps each and how old it is", async () => {
    await open();
    const rows = screen.getAllByTestId("emergency-deposit");
    expect(rows.map((row) => row.textContent)).toEqual([expect.stringContaining("cabin"), expect.stringContaining("tower")]);
    const when = formatRecent(Math.floor(Date.parse(UPDATED) / 1000), "en");
    expect(rows[1].textContent).toContain(en["emergency.group.deposit"].replace("{keeper}", "nas").replace("{when}", when));
  });

  it("name the members that did not answer", async () => {
    listOfferedDeposits.mockResolvedValue({ ok: true, deposits: [TOWER], silent: ["attic", "shed"] });
    await open();
    expect(screen.getByText(en["emergency.group.silent"].replace("{list}", "attic and shed"))).toBeTruthy();
  });

  it("say so when no member keeps one, and can be asked for again", async () => {
    listOfferedDeposits.mockResolvedValue({ ok: true, deposits: [], silent: [] });
    await open();
    expect(screen.getByText(en["emergency.group.none"])).toBeTruthy();
    await press(button(en["emergency.group.again"]));
    expect(listOfferedDeposits).toHaveBeenCalledTimes(2);
  });

  it("say what the server answered when the group could not be asked", async () => {
    listOfferedDeposits.mockResolvedValue({ ok: false, error: "this instance is in no group" });
    await open();
    expect(screen.getByText("this instance is in no group")).toBeTruthy();
  });
});

describe("the recovery code", () => {
  it("is put into its four groups whatever spaces, dashes and case it comes with", async () => {
    await open();
    await press(screen.getAllByRole("button", { name: en["emergency.group.choose"] })[1]);
    expect(button(en["emergency.group.ask"]).disabled).toBe(true);
    typeCode("abcde fghjk");
    expect(codeField().value).toBe("ABCDE-FGHJK");
    expect(button(en["emergency.group.ask"]).disabled).toBe(true);
    typeCode(" abcde-fghjk mnpqrstvwx ");
    expect(codeField().value).toBe(CODE);
    expect(button(en["emergency.group.ask"]).disabled).toBe(false);
  });

  it("goes with the request and is written down nowhere", async () => {
    await ask();
    expect(lastCall()).toEqual(["m-nas", "o-tower", CODE, true]);
    expect(localStorage.getItem(ASKED_KEY)).toBe(JSON.stringify({ memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower" }));
    expect(JSON.stringify({ ...localStorage })).not.toContain("ABCDE");
  });

  it("is refused in a sentence when it cannot be one", async () => {
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "deposit-code-shape", error: "the recovery code has twenty letters and digits", release: "" });
    await ask();
    expect(screen.getByText(en["emergency.code.deposit-code-shape"])).toBeTruthy();
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
  });
});

describe("a request that waits", () => {
  it("names the instance where someone has to allow it and looks for the answer every few seconds", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    expect(screen.getByRole("status").textContent).toBe(named("emergency.group.waiting"));
    expect(codeField().disabled).toBe(true);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(1);

    await tick(4000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(1);
    await tick(1500);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(2);
    expect(lastCall()).toEqual(["m-nas", "o-tower", CODE, false]);
    await tick(5000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(3);
  });

  it("is left alone while the page is hidden", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    await tick(16_000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(1);
    visibility.mockReturnValue("visible");
    await tick(5000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(2);
    visibility.mockRestore();
  });

  it("stays on the member when the person stops waiting", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    await press(button(en["emergency.group.stop"]));
    await tick(16_000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(1);
    expect(screen.getByText(named("emergency.group.resumeText"))).toBeTruthy();
    expect(localStorage.getItem(ASKED_KEY)).not.toBeNull();
    expect(button(en["emergency.group.resume"]).disabled).toBe(false);
  });

  it("opens the emergency once it is allowed and forgets the request and the code", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    emergencyFromGroup.mockResolvedValue({ ok: true, release: "allowed", emergency: OPENED });
    await tick(5500);
    expect(onEmergency).toHaveBeenCalledWith(OPENED);
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
    expect(codeField().value).toBe("");
    await tick(16_000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(2);
  });

  it("ends when it is declined, and can be asked again", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "release-declined", error: "the request was declined", release: "declined" });
    await tick(5500);
    expect(screen.getByText(en["emergency.code.release-declined"])).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
    await tick(16_000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(2);

    emergencyFromGroup.mockResolvedValue({ ok: true, release: "asked" });
    await press(button(en["emergency.group.ask"]));
    expect(lastCall()[3]).toBe(true);
    expect(screen.getByRole("status")).toBeTruthy();
  });

  it.each([
    ["expired", "emergency.release.expired"],
    ["released", "emergency.release.released"],
    ["unasked", "emergency.release.unasked"],
  ] as const)("says that it has to be asked again once it is %s", async (release, key) => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    emergencyFromGroup.mockResolvedValue({ ok: true, release });
    await tick(5500);
    expect(screen.getByText(en[key])).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
    expect(button(en["emergency.group.ask"]).disabled).toBe(false);
  });

  it("keeps waiting while the member is out of reach, and says so", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await ask();
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "member-silent", error: "that instance did not answer", release: "" });
    await tick(5500);
    expect(screen.getByText(en["emergency.code.member-silent"])).toBeTruthy();
    expect(screen.getByRole("status")).toBeTruthy();
    await tick(5000);
    expect(emergencyFromGroup).toHaveBeenCalledTimes(3);
  });
});

describe("asking", () => {
  it("is turned away for an hour after too many requests", async () => {
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "release-limit", error: "this server asked for deposits too often", release: "limited" });
    await ask();
    expect(screen.getByText(en["emergency.code.release-limit"])).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
  });

  it("leads back to the list when the member keeps that deposit no more", async () => {
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "deposit-none", error: "that instance keeps no deposit", release: "none" });
    await ask();
    expect(screen.getByText(en["emergency.code.deposit-none"])).toBeTruthy();
    await press(button(en["emergency.group.other"]));
    await waitFor(() => expect(listOfferedDeposits).toHaveBeenCalledTimes(1));
    expect(screen.getAllByTestId("emergency-deposit")).toHaveLength(2);
  });
});

describe("a wrong code", () => {
  it("is typed again without a second request, and opens the emergency when it fits", async () => {
    emergencyFromGroup.mockResolvedValue({ ok: false, code: "deposit-code", error: "the recovery code does not open this deposit", release: "allowed" });
    await ask();
    expect(screen.getByText(en["emergency.code.deposit-code"])).toBeTruthy();
    expect(codeField().value).toBe("");
    expect(codeField().disabled).toBe(false);
    expect(button(en["emergency.group.tryCode"]).disabled).toBe(true);
    expect(screen.queryByRole("button", { name: en["emergency.group.ask"] })).toBeNull();

    emergencyFromGroup.mockResolvedValue({ ok: true, release: "allowed", emergency: OPENED });
    typeCode("zzzzz-yyyyy-xxxxx-wwwww");
    await press(button(en["emergency.group.tryCode"]));
    expect(lastCall()).toEqual(["m-nas", "o-tower", "ZZZZZ-YYYYY-XXXXX-WWWWW", false]);
    expect(onEmergency).toHaveBeenCalledWith(OPENED);
  });
});

describe("the window opened again on a waiting request", () => {
  beforeEach(() => {
    localStorage.setItem(ASKED_KEY, JSON.stringify({ memberId: "m-nas", memberName: "nas", ownerId: "o-tower", ownerName: "tower" }));
  });

  it("names where the request waits and asks for the code, not for the deposit", async () => {
    await open();
    expect(listOfferedDeposits).not.toHaveBeenCalled();
    expect(screen.getByText(named("emergency.group.resumeText"))).toBeTruthy();
    expect(codeField().value).toBe("");
    typeCode(CODE);
    await press(button(en["emergency.group.resume"]));
    expect(lastCall()).toEqual(["m-nas", "o-tower", CODE, false]);
    expect(screen.getByRole("status").textContent).toBe(named("emergency.group.waiting"));
  });

  it("forgets the request when another deposit is chosen", async () => {
    await open();
    await press(button(en["emergency.group.other"]));
    expect(localStorage.getItem(ASKED_KEY)).toBeNull();
    await waitFor(() => expect(screen.getAllByTestId("emergency-deposit")).toHaveLength(2));
  });

  it("ignores what is stored when it is no request", async () => {
    localStorage.setItem(ASKED_KEY, "{broken");
    await open();
    await waitFor(() => expect(screen.getAllByTestId("emergency-deposit")).toHaveLength(2));
  });
});
