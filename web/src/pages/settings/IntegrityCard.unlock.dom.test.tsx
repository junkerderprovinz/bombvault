// @vitest-environment jsdom
// A repository shared with another domain gets `restic unlock` without
// --remove-all, so another domain's running backup keeps its lock. That clears
// only locks restic considers stale, and a lock left by a previous container
// is not stale until it is old enough, so the button can succeed while
// changing nothing. The server names those repositories on success and
// failure, and the card has to show them.
import { render, screen, cleanup, waitFor, fireEvent, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const unlockDomain = vi.fn();
const pushed: { message: string; severity?: string }[] = [];

vi.mock("../../lib/api", () => ({
  unlockDomain: (...a: unknown[]) => unlockDomain(...a),
  checkDomain: vi.fn().mockResolvedValue({ ok: true }),
  pruneDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: vi.fn().mockResolvedValue({ ok: true }),
  tamperTest: vi.fn().mockResolvedValue({ ok: true }),
  getDrills: vi.fn().mockResolvedValue({ ok: true, drills: [], latest: null }),
  getStatus: vi.fn().mockResolvedValue({ ok: true }),
  listContainers: vi.fn().mockResolvedValue({ containers: [] }),
  listVMs: vi.fn().mockResolvedValue({ vms: [] }),
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { en } from "../../lib/i18n";

// The real English table rather than a key-echoing stub: the assertions are
// about the sentence an operator reads, with {list} filled in.
const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];

const settings = { drDrillTarget: "", drDrillTargetVm: "" } as never;

function renderCard() {
  return render(
    <IntegrityCard
      t={t}
      settings={settings}
      setSettings={() => {}}
      save={async () => true}
    />
  );
}

beforeEach(() => {
  unlockDomain.mockReset();
  pushed.length = 0;
});
afterEach(cleanup);

describe("the unlock button", () => {
  it("names the repository it could only clear stale locks on, even when it succeeds", async () => {
    unlockDomain.mockResolvedValue({
      ok: true,
      skipped: ["Cold (another domain writes to it too, so only stale locks were cleared there)"],
    });
    renderCard();

    const buttons = await screen.findAllByRole("button", { name: /unlock/i });
    await act(async () => {
      fireEvent.click(buttons[0]);
    });

    await waitFor(() => expect(unlockDomain).toHaveBeenCalled());
    await waitFor(() => {
      const said = pushed.map((p) => p.message).join("\n");
      expect(
        said,
        "the button came back green and said nothing.\n" +
          "A shared repository only gets the stale-lock clear, so this success can mean nothing\n" +
          "changed on the very repository the operator pressed it for."
      ).toMatch(/Cold/);
    });
  });

  it("says it as a warning, not as a success", async () => {
    unlockDomain.mockResolvedValue({ ok: true, skipped: ["Cold (shared)"] });
    renderCard();
    const buttons = await screen.findAllByRole("button", { name: /unlock/i });
    await act(async () => {
      fireEvent.click(buttons[0]);
    });

    await waitFor(() => {
      const line = pushed.find((p) => /Cold/.test(p.message));
      expect(line?.severity).toBe("warn");
    });
  });

  it("says it on the failure path too, alongside the error", async () => {
    unlockDomain.mockResolvedValue({
      ok: false,
      error: "unlocking the containers repository: boom",
      skipped: ["Cold (another domain writes to it too, so only stale locks were cleared there)"],
    });
    renderCard();
    const buttons = await screen.findAllByRole("button", { name: /unlock/i });
    await act(async () => {
      fireEvent.click(buttons[0]);
    });

    await waitFor(() => {
      const said = pushed.map((p) => p.message).join("\n");
      expect(said).toMatch(/boom/);
      expect(
        said,
        "on the failure path the operator needs BOTH halves: what went wrong, and which\n" +
          "other repository got the weaker treatment."
      ).toMatch(/Cold/);
    });
  });

  it("stays quiet when there is nothing to report", async () => {
    unlockDomain.mockResolvedValue({ ok: true, skipped: [] });
    renderCard();
    const buttons = await screen.findAllByRole("button", { name: /unlock/i });
    await act(async () => {
      fireEvent.click(buttons[0]);
    });

    await waitFor(() => expect(unlockDomain).toHaveBeenCalled());
    expect(pushed, "an ordinary unlock must not produce a toast").toHaveLength(0);
  });
});
