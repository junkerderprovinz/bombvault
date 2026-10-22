// @vitest-environment jsdom
// The sections and the coverage follow the end of a run instead of keeping
// what they said when the overview opened.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import { ATTENTION_CHANGED_EVENT } from "../lib/attention";
import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import { DAY, FakeEventSource, NOW, domain, installMatchMedia, item, resetServer, run, server } from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

beforeEach(() => {
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("Dashboard after a run", () => {
  it("reads the sections again when a run ends", async () => {
    server.domains = [domain({ status: "overdue", lastSuccess: NOW - 3 * DAY })];
    server.items = [item()];
    server.runs = [run({ id: "old", finishedAt: NOW - 3 * DAY })];
    render(inApp(<Dashboard />));
    await act(async () => {});
    expect(screen.getAllByRole("img", { name: en["overview.check.localOverdue"] }).length).toBeGreaterThan(0);

    server.domains = [domain()];
    server.runs = [run({ id: "new", finishedAt: NOW - 10 }), run({ id: "old", finishedAt: NOW - 3 * DAY })];
    await act(async () => {
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    });
    await act(async () => {});

    expect(screen.queryAllByRole("img", { name: en["overview.check.localOverdue"] })).toHaveLength(0);
    expect(screen.getAllByRole("img", { name: en["overview.check.localCurrent"] }).length).toBeGreaterThan(0);
  });
});
