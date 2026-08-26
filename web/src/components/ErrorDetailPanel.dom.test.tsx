// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// ErrorDetailPanel — the "Backup Everything" pseudo-domain half.
//
// The Backup Everything feature taught ActivityLog.tsx about its new
// "everything" domain and stopped there. This panel is the app's SECOND
// domain-rendering surface (the modal behind the dashboard's error count) and
// was never told, so a failed pass rendered a raw lowercase "everything" chip
// where flash/config/files all get a real translation, and could not be
// filtered to at all.
//
// Covered here because the live box cannot reach it: the panel only opens
// when the dashboard's error count is > 0, and that count excludes the
// off-site failure the test container happens to carry — so its StatCard is
// not clickable there. A mounted component does not have that problem.
//
// listRuns/ackRuns are mocked; nothing here talks to a server.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
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

describe("ErrorDetailPanel — the Backup Everything domain", () => {
  it("offers the pseudo-domain in its filter, like the Activity Log does", async () => {
    renderPanel([run()]);
    await waitFor(() => expect(listRuns).toHaveBeenCalled());

    const domainSelect = screen.getAllByRole("combobox")[0] as HTMLSelectElement;
    const values = [...domainSelect.options].map((o) => o.value);
    expect(values).toContain("everything");

    // …and it is labelled with the shared, translated key, not a raw literal.
    const option = [...domainSelect.options].find((o) => o.value === "everything")!;
    expect(option.text).toBe(en["activityLog.domainEverything"]);
  });

  it("labels a failed pass with its translated domain, not the raw tag", async () => {
    // A target name deliberately UNLIKE the domain label, so matching the
    // label cannot accidentally be matching the target: the "affected" line
    // renders "<targets> · <domainLabel(domain)>", and with the domain case
    // missing it renders the bare lowercase tag from `default: return d`.
    renderPanel([run({ target: "nightly-pass" })]);

    const affected = await screen.findByText(/nightly-pass/);
    expect(affected.textContent).toContain(`nightly-pass · ${en["activityLog.domainEverything"]}`);
    expect(affected.textContent).not.toContain("· everything");
  });

  it("keeps the five original domains labelled too", async () => {
    renderPanel([run()]);
    await waitFor(() => expect(listRuns).toHaveBeenCalled());

    const domainSelect = screen.getAllByRole("combobox")[0] as HTMLSelectElement;
    const values = [...domainSelect.options].map((o) => o.value);
    // The new option is an addition, not a replacement.
    expect(values).toEqual(["all", "containers", "vms", "flash", "config", "files", "everything"]);
  });
});
