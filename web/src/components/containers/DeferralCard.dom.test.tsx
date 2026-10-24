// @vitest-environment jsdom
// The card for a busy app saves its whole policy, shows only the fields the
// chosen check needs, never shows a stored key, and says when a waiting
// backup is tried again.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { DeferralPatch, DeferredBackup } from "../../lib/api";

const saved: DeferralPatch[] = [];

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    setDeferral: (_name: string, patch: DeferralPatch) => {
      saved.push(patch);
      return Promise.resolve({ ok: true });
    },
  };
});

const { DEFAULT_DEFERRAL, DeferralCard } = await import("./DeferralCard");

async function show(node: React.ReactNode) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>{node}</ToastProvider>
      </I18nProvider>
    );
  });
}

// Leaving the page saves what is still waiting for its debounce.
async function leave() {
  await act(async () => {
    cleanup();
  });
}

afterEach(() => {
  cleanup();
  saved.length = 0;
});

it("offers the retry policy and the traffic check with no activity check chosen", async () => {
  await show(<DeferralCard name="plex" initial={DEFAULT_DEFERRAL} hueIndex={0} />);
  expect(screen.getByText(en["deferral.retryMin"])).toBeTruthy();
  expect(screen.getByText(en["deferral.atDeadline"])).toBeTruthy();
  expect(screen.getByRole("switch", { name: new RegExp(en["deferral.net"]) })).toBeTruthy();
  expect(screen.queryByText(en["deferral.apiKey"])).toBeNull();
  expect(screen.queryByText(en["deferral.paused"])).toBeNull();
});

it("asks Jellyfin for a key, keeps a stored one hidden and saves a new one once", async () => {
  await show(<DeferralCard name="jf" initial={{ ...DEFAULT_DEFERRAL, preset: "jellyfin", apiKeySet: true }} hueIndex={0} />);
  const key = screen.getByLabelText(en["deferral.apiKey"]) as HTMLInputElement;
  expect(key.value).toBe("");
  expect(key.placeholder).toBe(en["cloud.secretSet"]);
  expect(screen.getByText(en["deferral.paused"])).toBeTruthy();
  expect(screen.getByRole("switch", { name: new RegExp(en["deferral.failOpen"]) })).toBeTruthy();
  await act(async () => {
    fireEvent.change(key, { target: { value: " abc123 " } });
  });
  await leave();
  expect(saved).toHaveLength(1);
  expect(saved[0]).toMatchObject({ preset: "jellyfin", apiKey: "abc123", retryMin: 30, maxHours: 12 });
});

it("keeps the numbers and the end of the wait beside their label on a phone", async () => {
  await show(<DeferralCard name="jf" initial={{ ...DEFAULT_DEFERRAL, preset: "jellyfin", netKiB: 100 }} hueIndex={0} />);
  for (const key of ["deferral.paused", "deferral.netKiB", "deferral.netMin", "deferral.retryMin", "idle.maxHours", "deferral.atDeadline"] as const) {
    const row = document.querySelector(`[data-search-row="${en[key]}"]`);
    expect(row?.firstElementChild?.className, key).toContain("basis-0");
  }
});

it("tells Plex users there is no key to enter", async () => {
  await show(<DeferralCard name="plex" initial={{ ...DEFAULT_DEFERRAL, preset: "plex" }} hueIndex={0} />);
  expect(screen.getByText(en["deferral.plexToken"])).toBeTruthy();
  expect(screen.queryByText(en["deferral.apiKey"])).toBeNull();
});

it("switching the traffic check and the window on saves them with their defaults", async () => {
  await show(<DeferralCard name="game" initial={DEFAULT_DEFERRAL} hueIndex={0} />);
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: new RegExp(en["deferral.net"]) }));
  });
  expect(screen.getByText(en["deferral.netKiB"])).toBeTruthy();
  await act(async () => {
    fireEvent.click(screen.getByRole("switch", { name: new RegExp(en["deferral.window"]) }));
  });
  expect(screen.getByText(en["deferral.windowTimes"])).toBeTruthy();
  await leave();
  expect(saved.at(-1)).toMatchObject({ netKiB: 100, netMin: 5, windowFrom: "00:00", windowTo: "08:00" });
});

it("keeps a number typed out of range inside the server's limits", async () => {
  await show(<DeferralCard name="plex" initial={DEFAULT_DEFERRAL} hueIndex={0} />);
  await act(async () => {
    fireEvent.change(screen.getByLabelText(en["deferral.retryMin"]), { target: { value: "1" } });
  });
  await leave();
  expect(saved.at(-1)?.retryMin).toBe(5);
});

it("says when a waiting backup is tried again and what happens at its deadline", async () => {
  const now = Math.floor(Date.now() / 1000);
  const waiting: DeferredBackup = {
    domain: "containers",
    name: "plex",
    reason: "Plex has 2 active sessions",
    since: now - 600,
    nextTry: now + 1200,
    deadline: now + 3600,
    atDeadline: "back-up",
    tries: 2,
  };
  await show(<DeferralCard name="plex" initial={DEFAULT_DEFERRAL} waiting={waiting} hueIndex={0} />);
  const line = screen.getByRole("status");
  expect(line.textContent).toContain("Plex has 2 active sessions");
  expect(line.textContent).toContain(en["deferral.deadlineBackUp"].split(" {time}")[0]);
  expect(line.textContent).toMatch(/^[^{}]+$/);
});
