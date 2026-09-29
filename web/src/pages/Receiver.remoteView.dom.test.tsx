// @vitest-environment jsdom
// Receiver keeps its check trigger in remote view (POST /api/receiver/repos/
// {id}/check is on the allowlist), and the repo list is metadata, but editing
// or removing a monitoring entry and adding a new one have no route on it, so
// those controls must disappear rather than merely disable while a peer's
// instance is open.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listReceivedRepos: () =>
      Promise.resolve({
        ok: true,
        repos: [
          {
            id: "r1",
            name: "tower off-site",
            repo: "rest:http://192.168.1.9:8000/tower",
            deadManHours: 26,
            checkCadence: "",
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
            lastReceived: "",
            snapshotCount: 12,
            reachable: true,
          },
        ],
      }),
  };
});

const { Receiver } = await import("./Receiver");

afterEach(cleanup);

async function renderReceiver(initialEntries: string[]) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={initialEntries}>
        <InstanceProvider>
          <I18nProvider>
            <ToastProvider>
              <Receiver />
            </ToastProvider>
          </I18nProvider>
        </InstanceProvider>
      </MemoryRouter>,
    );
  });
}

it("offers editing, removing and adding a monitoring entry locally", async () => {
  await renderReceiver(["/instances"]);
  expect(await screen.findByText("tower off-site")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.checkNow"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.edit"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.remove"] })).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.addRepo"] })).toBeTruthy();
});

it("hides editing, removing and adding a monitoring entry while a peer's instance is open, and keeps the check trigger and the list", async () => {
  await renderReceiver(["/instances?instance=member-1&instanceName=attic"]);
  expect(await screen.findByText("tower off-site")).toBeTruthy();
  expect(screen.getByRole("button", { name: en["receiver.checkNow"] })).toBeTruthy();
  expect(screen.queryByRole("button", { name: en["receiver.edit"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["receiver.remove"] })).toBeNull();
  expect(screen.queryByRole("button", { name: en["receiver.addRepo"] })).toBeNull();
});
