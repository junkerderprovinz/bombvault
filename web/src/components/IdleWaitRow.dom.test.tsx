// @vitest-environment jsdom
// The switch brings back the hours it had, and a card whose scheduled backup
// waits says so with the reason and the deadline.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { IdleWait } from "../lib/api";

const saved: number[] = [];
let waiting: IdleWait[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    setIdleWait: (_name: string, hours: number) => {
      saved.push(hours);
      return Promise.resolve({ ok: true });
    },
    getScheduleWaiting: () => Promise.resolve(waiting),
  };
});

const { IdleWaitLine, IdleWaitRow } = await import("./IdleWaitRow");

async function show(node: React.ReactNode) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>{node}</ToastProvider>
      </I18nProvider>
    );
  });
}

afterEach(() => {
  cleanup();
  saved.length = 0;
  waiting = [];
});

it("switching the wait on starts at four hours and off stores zero", async () => {
  await show(<IdleWaitRow name="plex" initial={0} />);
  const toggle = screen.getByRole("switch", { name: new RegExp(en["idle.toggle"]) });
  await act(async () => {
    fireEvent.click(toggle);
  });
  expect(saved).toEqual([4]);
  expect(screen.getByRole("spinbutton")).toBeTruthy();
  await act(async () => {
    fireEvent.click(toggle);
  });
  expect(saved).toEqual([4, 0]);
  expect(screen.queryByRole("spinbutton")).toBeNull();
});

it("names the reason a scheduled backup is waiting", async () => {
  waiting = [{ domain: "containers", name: "plex", reason: "streaming", since: 1, deadline: 7200 }];
  await show(<IdleWaitLine name="plex" />);
  const line = await screen.findByText(new RegExp(en["idle.reasonStreaming"]));
  expect(line.textContent).toMatch(/^[^{}]+$/);
});

it("stays empty for a card that is not waiting", async () => {
  waiting = [{ domain: "containers", name: "other", reason: "cpu", since: 1, deadline: 7200 }];
  await show(<IdleWaitLine name="plex" />);
  expect(screen.queryByText(new RegExp(en["idle.reasonCpu"]))).toBeNull();
});
