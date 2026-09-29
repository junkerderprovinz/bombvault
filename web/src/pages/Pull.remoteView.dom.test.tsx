// @vitest-environment jsdom
// Pull keeps its run trigger in remote view (POST /api/pull/sources/{id}/run
// is on the allowlist), and the source list is metadata, but testing the
// connection, editing or removing a source and adding a new one have no route
// on it, so those controls must disappear rather than merely disable while a
// peer's instance is open.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import type { PullSourceView } from "../lib/api";

const source: PullSourceView = {
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

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listPullSources: () => Promise.resolve({ ok: true, sources: [source] }),
  };
});

const { Pull } = await import("./Pull");

afterEach(cleanup);

async function renderPull(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <ToastProvider>
              <Pull />
            </ToastProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

it("offers testing, editing, removing and adding a pull source locally", async () => {
  await renderPull(["/instances"]);
  expect(await screen.findByText("Tower next door")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["pull.pullNow"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["offsite.test"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["common.edit"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.remove"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["pull.addSource"] })).toBeTruthy();
});

it("hides testing, editing, removing and adding a pull source while a peer's instance is open, and keeps the run trigger and the list", async () => {
  await renderPull(["/instances?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("Tower next door")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["pull.pullNow"] })).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["offsite.test"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["common.edit"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["receiver.remove"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["pull.addSource"] })).toBeNull();
});
