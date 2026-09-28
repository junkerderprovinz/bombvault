// @vitest-environment jsdom
// A received repo saved before the sender's first copy arrives shows as
// waiting, not as a mistake, and the reachable/unreachable badge stays quiet
// until there is something to report.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ReceivedRepoStatus } from "../lib/api";

const base: ReceivedRepoStatus = {
  id: "r1",
  name: "attic (containers)",
  repo: "rest:http://192.168.1.50:8000/bombvault-containers/containers",
  deadManHours: 26,
  checkCadence: "daily 04:00",
  readDataPercent: 0,
  lastCheckAt: 0,
  lastCheckOk: null,
  lastCheckError: "",
  lastCheckReadData: false,
  enabled: true,
  createdAt: 0,
  sortOrder: 0,
  memberId: "member-1",
  needsPairing: false,
  waiting: true,
  lastReceived: "",
  snapshotCount: 0,
  reachable: false,
};

let rows: ReceivedRepoStatus[] = [base];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listReceivedRepos: () => Promise.resolve({ ok: true, repos: rows }),
  };
});

const { Receiver } = await import("./Receiver");

async function renderReceiver() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Receiver embedded />
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

afterEach(cleanup);

describe("a receiver waiting for its first copy", () => {
  it("shows the waiting badge with an explanation instead of unreachable", async () => {
    rows = [base];
    await renderReceiver();
    expect(await screen.findByText(en["receiver.waiting"])).not.toBeNull();
    expect(screen.queryByText(en["receiver.unreachable"])).toBeNull();
    expect(screen.queryByText(en["receiver.reachable"])).toBeNull();
    expect(screen.getByLabelText(en["receiver.waitingTip"])).not.toBeNull();
  });

  it("goes back to reporting reachability once it is active", async () => {
    rows = [{ ...base, waiting: false, reachable: true, lastReceived: "2026-01-01T00:00:00Z", snapshotCount: 3 }];
    await renderReceiver();
    expect(await screen.findByText(en["receiver.reachable"])).not.toBeNull();
    expect(screen.queryByText(en["receiver.waiting"])).toBeNull();
  });
});
