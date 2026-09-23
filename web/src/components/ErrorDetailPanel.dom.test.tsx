// @vitest-environment jsdom
// The error panel has to know the Backup Everything pseudo-domain like the
// Activity Log does: as a filter option and as a translated label. listRuns and
// ackRuns are mocked.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { Run } from "../lib/api";

const listRuns = vi.fn();
const ackRuns = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, listRuns: () => listRuns(), ackRuns: (...a: unknown[]) => ackRuns(...a) };
});

const { ErrorDetailPanel } = await import("./ErrorDetailPanel");

function run(over: Partial<Run> = {}): Run {
  return {
    id: "r1",
    targetId: "everything",
    kind: "backup",
    status: "failed",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_500,
    snapshotId: "",
    bytes: 0,
    error: "the pre-command exited 1",
    acknowledged: false,
    target: "Backup Everything",
    domain: "everything",
    ...over,
  };
}

function renderPanel(runs: Run[]) {
  listRuns.mockResolvedValue({ ok: true, runs });
  return render(
    <I18nProvider>
      <ErrorDetailPanel onClose={() => {}} />
    </I18nProvider>
  );
}

beforeEach(() => {
  listRuns.mockReset();
  ackRuns.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("ErrorDetailPanel with the Backup Everything domain", () => {
  it("offers Backup Everything in the domain filter", async () => {
    renderPanel([run()]);
    await waitFor(() => expect(listRuns).toHaveBeenCalled());

    // The filter is a listbox, so its options exist only while it is open.
    fireEvent.click(screen.getAllByRole("combobox")[0]!);
    const labels = screen.getAllByRole("option").map((o) => o.textContent);
    expect(labels).toContain(en["activityLog.domainEverything"]);
  });

  it("labels a failed pass with its translated domain, not the raw tag", async () => {
    // A target name unlike the domain label, so the assertion cannot match the
    // target by accident. The line reads "<targets> · <domain label>".
    renderPanel([run({ target: "nightly-pass" })]);

    const affected = await screen.findByText(/nightly-pass/);
    expect(affected.textContent).toContain(`nightly-pass · ${en["activityLog.domainEverything"]}`);
    expect(affected.textContent).not.toContain("· everything");
  });

  it("lists every domain in the shared order", async () => {
    renderPanel([run()]);
    await waitFor(() => expect(listRuns).toHaveBeenCalled());

    fireEvent.click(screen.getAllByRole("combobox")[0]!);
    const labels = screen.getAllByRole("option").map((o) => o.textContent);
    expect(labels).toEqual([
      en["activityLog.filterAllDomains"],
      en["activityLog.domainContainers"],
      en["activityLog.domainVMs"],
      en["activityLog.domainFlash"],
      en["activityLog.domainConfig"],
      en["activityLog.domainFiles"],
      en["activityLog.domainEverything"],
    ]);
  });
});
