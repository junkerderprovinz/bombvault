// @vitest-environment jsdom
// A pull source's card reports the last pull instead of probing, because a pull
// is something that happened: "it worked at 04:00" is the answer. Never pulled,
// succeeded, failed and switched off each have their own wording.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, countText, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { PullSourceView } from "../lib/api";

const base: PullSourceView = {
  id: "p1",
  name: "Tower next door",
  repo: "rest:http://192.168.1.9:8000/their-containers",
  credsRef: "",
  domain: "containers",
  cadence: "daily 04:00",
  limitDownload: 0,
  limitUpload: 0,
  lastPullAt: 1_700_000_000,
  lastPullOk: true,
  lastPullError: "",
  snapshotsPulled: 7,
  enabled: true,
  createdAt: 1_600_000_000,
  sortOrder: 0,
  hasAppKey: true,
};

let rows: PullSourceView[] = [base];
const deleted: string[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listPullSources: () => Promise.resolve({ ok: true, sources: rows }),
    deletePullSource: (id: string) => {
      deleted.push(id);
      return Promise.resolve({ ok: true });
    },
  };
});

const { Pull } = await import("./Pull");

async function renderPull() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Pull />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

beforeEach(() => {
  rows = [base];
  deleted.length = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("pull source card", () => {
  it("reports the last result, not a fresh probe", async () => {
    await renderPull();
    expect(screen.queryByText(en["pull.pullOk"])).not.toBeNull();
    expect(screen.queryByText(countText(en["pull.snapshotsPulled"], "en", 7))).not.toBeNull();
  });

  it("says never pulled rather than guessing at a verdict", async () => {
    rows = [{ ...base, lastPullOk: null, lastPullAt: 0, snapshotsPulled: 0 }];
    await renderPull();
    expect(screen.queryByText(en["pull.neverPulled"])).not.toBeNull();
    expect(screen.queryByText(en["pull.pullOk"])).toBeNull();
  });

  it("shows the failure and its reason", async () => {
    rows = [{ ...base, lastPullOk: false, lastPullError: "could not open the pull source" }];
    await renderPull();
    expect(screen.queryByText(en["pull.pullFailed"])).not.toBeNull();
    expect(screen.queryByText("could not open the pull source")).not.toBeNull();
  });

  it("a disabled source says so instead of reporting an old success", async () => {
    // lastPullOk is still true from before the source was switched off.
    rows = [{ ...base, enabled: false }];
    await renderPull();
    expect(screen.queryByText(en["pull.pullingOff"])).not.toBeNull();
    expect(screen.queryByText(en["pull.pullOk"])).toBeNull();
  });

  it("cannot be pulled from while it is switched off", async () => {
    rows = [{ ...base, enabled: false }];
    await renderPull();
    const btn = screen.getByRole("button", { name: en["pull.pullNow"] }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  it("removes only on the second click", async () => {
    await renderPull();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.remove"] }));
    });
    expect(deleted).toEqual([]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["offsite.targets.confirmRemove"] }));
    });
    expect(deleted).toEqual(["p1"]);
  });
});
