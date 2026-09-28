// @vitest-environment jsdom
// A pull source left with no explicit credential set picks up a matching one
// on the server by host, so the dialog explains that instead of forcing the
// admin to hunt down a login BombVault already holds.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { PullSourceView } from "../lib/api";

const base: PullSourceView = {
  id: "p1",
  name: "Tower next door",
  repo: "rest:http://192.168.1.9:8000/their-containers",
  credsRef: "cred-attic",
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

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listPullSources: () => Promise.resolve({ ok: true, sources: [base] }),
    getCloudCredSets: () =>
      Promise.resolve({ ok: true, sets: [{ id: "cred-attic", name: "mesh: attic" }] }),
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
  localStorage.clear();
});

afterEach(cleanup);

describe("pull source add dialog", () => {
  it("explains the automatic match while no credentials are chosen", async () => {
    await renderPull();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pull.addSource"] }));
    });
    expect(await screen.findByText(en["pull.credsAutoHint"])).not.toBeNull();
  });

  it("stays quiet once a source already carries a credential set", async () => {
    await renderPull();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["common.edit"] }));
    });
    await screen.findByRole("dialog", { name: en["common.edit"] });
    expect(screen.queryByText(en["pull.credsAutoHint"])).toBeNull();
  });
});
