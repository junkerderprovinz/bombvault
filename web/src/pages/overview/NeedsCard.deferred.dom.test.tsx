// @vitest-environment jsdom
// A backup that waits for its app stands on the overview in amber, with its
// reason and its next try, and leads to its entry.
import { afterEach, expect, it } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { BackupItem, DeferredBackup } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";
import { formatClockTime } from "../../lib/reltime";
import { ToastProvider } from "../../lib/toast";
import { NeedsCard } from "./NeedsCard";

afterEach(cleanup);

it("shows a deferred backup in amber with its next try", async () => {
  const now = Math.floor(Date.now() / 1000);
  const deferral: DeferredBackup = {
    domain: "containers",
    name: "plex",
    reason: "Plex has 1 active session",
    since: now,
    nextTry: now + 600,
    deadline: now + 3600,
    atDeadline: "give-up",
    tries: 1,
  };
  const item = {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00" },
    lastBackup: now - 86400,
    lastRunStatus: "deferred",
    sourceBytes: 1,
    installed: true,
    runs: [],
    deferral,
  } as BackupItem;
  await act(async () => {
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <NeedsCard needs={[{ is: "deferred", item, deferral }]} onErrors={() => undefined} onItemsChanged={() => undefined} />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  const row = screen.getByTestId("need-deferred");
  expect(row.textContent).toContain(en["run.statusDeferred"]);
  expect(row.textContent).toContain("Plex has 1 active session");
  expect(row.textContent).toContain(formatClockTime(now + 600, false));
  expect(row.querySelector(".bg-statusWarnSolid")).toBeTruthy();
  expect(screen.getByRole("link", { name: "plex" }).getAttribute("href")).toContain("plex");
});
