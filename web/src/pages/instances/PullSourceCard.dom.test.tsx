// @vitest-environment jsdom
// A pull source's card reports the last pull instead of probing, because a pull
// is something that happened: "it worked at 04:00" is the answer. Never pulled,
// succeeded, failed and switched off each have their own wording.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, countText, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import type { PullSourceView } from "../../lib/api";

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
  memberId: "member-1",
  needsPairing: false,
};

const deleted: string[] = [];
let refreshed = 0;

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    deletePullSource: (id: string) => {
      deleted.push(id);
      return Promise.resolve({ ok: true });
    },
  };
});

const { PullSourceCard } = await import("./PullSourceCard");

function Card({ source }: { source: PullSourceView }) {
  const { t } = useT();
  return <PullSourceCard source={source} t={t} index={0} onRefresh={() => refreshed++} onEdit={() => undefined} />;
}

async function renderCard(over: Partial<PullSourceView> = {}) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Card source={{ ...base, ...over }} />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

beforeEach(() => {
  deleted.length = 0;
  refreshed = 0;
  localStorage.clear();
});

afterEach(cleanup);

describe("pull source card", () => {
  it("reports the last result, not a fresh probe", async () => {
    await renderCard();
    expect(screen.queryByText(en["pull.pullOk"])).not.toBeNull();
    expect(screen.queryByText(countText(en["pull.snapshotsPulled"], "en", 7))).not.toBeNull();
  });

  it("says never pulled rather than guessing at a verdict", async () => {
    await renderCard({ lastPullOk: null, lastPullAt: 0, snapshotsPulled: 0 });
    expect(screen.queryByText(en["pull.neverPulled"])).not.toBeNull();
    expect(screen.queryByText(en["pull.pullOk"])).toBeNull();
  });

  it("shows the failure and its reason", async () => {
    await renderCard({ lastPullOk: false, lastPullError: "could not open the pull source" });
    expect(screen.queryByText(en["pull.pullFailed"])).not.toBeNull();
    expect(screen.queryByText("could not open the pull source")).not.toBeNull();
  });

  it("a disabled source says so instead of reporting an old success", async () => {
    // lastPullOk is still true from before the source was switched off.
    await renderCard({ enabled: false });
    expect(screen.queryByText(en["pull.pullingOff"])).not.toBeNull();
    expect(screen.queryByText(en["pull.pullOk"])).toBeNull();
  });

  it("cannot be pulled from while it is switched off", async () => {
    await renderCard({ enabled: false });
    const btn = screen.getByRole("button", { name: en["pull.pullNow"] }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  it("removes only on the second click", async () => {
    await renderCard();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["receiver.remove"] }));
    });
    expect(deleted).toEqual([]);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["offsite.targets.confirmRemove"] }));
    });
    expect(deleted).toEqual(["p1"]);
    expect(refreshed).toBe(1);
  });
});
