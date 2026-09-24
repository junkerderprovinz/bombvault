// @vitest-environment jsdom
// The error panel has to know the Backup Everything pseudo-domain like the
// Activity Log does: as a filter option and as a translated label, and a group
// has to say which kind of run failed, or a failed dump reads as a failed
// container backup. listRuns and ackRuns are mocked.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, fireEvent } from "@testing-library/react";
import { countText, I18nProvider, en } from "../lib/i18n";
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
      en["activityLog.domainZFS"],
      en["activityLog.domainEverything"],
    ]);
  });
});

describe("a failed database dump in the panel", () => {
  it("names the kind and carries the remedy", async () => {
    renderPanel([
      run({
        id: "d1",
        kind: "dbdump",
        target: "immich_postgres",
        domain: "container",
        error: "database dump failed: the database refused the login: FATAL: password authentication failed",
      }),
    ]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    const head = await screen.findByText(en["runReason.dbdumpAuth"], { exact: false });
    const line = head.closest("p");
    expect(line?.textContent).toContain(`${en["run.kindDbDump"]}: ${en["runReason.dbdumpAuth"]}`);
    expect(line?.textContent).toContain("FATAL: password authentication failed");
    expect(screen.getByLabelText(en["dbdump.fixAuth"])).toBeTruthy();
  });

  it("counts a group in the form the count needs", async () => {
    renderPanel([
      run({ id: "d1", kind: "dbdump", target: "pg", domain: "container", error: "boom" }),
      run({ id: "d2", kind: "dbdump", target: "maria", domain: "container", error: "bang" }),
      run({ id: "d3", kind: "dbdump", target: "pg2", domain: "container", error: "bang" }),
    ]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    const panel = screen.getByRole("dialog").textContent ?? "";
    expect(panel).toContain(countText(en["errorPanel.count"], "en", 1));
    expect(panel).toContain(countText(en["errorPanel.count"], "en", 2));
    expect(panel).not.toContain("1 occurrences");
  });

  it("keeps the container's failed backup as a group of its own", async () => {
    renderPanel([
      run({ id: "d1", kind: "dbdump", target: "immich_postgres", domain: "container", error: "boom" }),
      run({ id: "b1", kind: "backup", target: "immich_postgres", domain: "container", error: "boom" }),
    ]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    expect(screen.getAllByRole("button", { name: en["errorPanel.resolve"] })).toHaveLength(2);
    const panel = screen.getByRole("dialog").textContent ?? "";
    expect(panel).toContain(`${en["run.kindDbDump"]}: boom`);
    expect(panel).toContain(`${en["run.kindBackup"]}: boom`);
  });
});

describe("a failure an assistant caused", () => {
  it("names the MCP key under a failed run", async () => {
    renderPanel([run({ id: "m1", startedVia: "mcp", startedViaKey: "k1", startedViaLabel: "office laptop" })]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    const panel = screen.getByRole("dialog").textContent ?? "";
    expect(panel).toContain(en["activityLog.viaMcpLine"].replace("{key}", "office laptop"));
  });

  it("says only that MCP started it when the key is gone", async () => {
    renderPanel([run({ id: "m2", startedVia: "mcp", startedViaKey: "k1" })]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    expect(screen.getByRole("dialog").textContent).toContain(en["activityLog.viaMcpLineUnknownKey"]);
  });

  it("leaves a run the web interface started alone", async () => {
    renderPanel([run()]);

    await waitFor(() => expect(listRuns).toHaveBeenCalled());
    expect(screen.getByRole("dialog").textContent).not.toContain(en["activityLog.viaMcpLineUnknownKey"]);
  });
});
