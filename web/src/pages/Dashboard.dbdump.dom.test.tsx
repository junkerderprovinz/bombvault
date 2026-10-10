// @vitest-environment jsdom
// A failed dump is a failure of its own: it must count as one without hiding
// the container's failed backup, and nothing that happens to the dump
// afterwards, neither saving it to a folder nor importing it, may clear it.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { Run } from "../lib/api";
import { failedRunsNeedingAttention } from "../lib/runFailures";
import { HistoryCard } from "./overview/HistoryCard";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof HistoryCard
>[0]["t"];

function run(over: Partial<Run> = {}): Run {
  return {
    id: "r1",
    targetId: "t1",
    kind: "backup",
    status: "failed",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_500,
    snapshotId: "",
    bytes: 0,
    error: "restic exited 1",
    acknowledged: false,
    target: "immich_postgres",
    domain: "container",
    ...over,
  };
}

function renderHistory(runs: Run[]) {
  render(
    <I18nProvider>
      <HistoryCard
        t={t}
        lang="en"
        runs={runs}
        loading={false}
        failed={false}
        view="runs"
        onView={() => {}}
        logDay={null}
        onClearLogDay={() => {}}
        logRun={null}
        onClearLogRun={() => {}}
        onOpenRun={() => {}}
      />
    </I18nProvider>
  );
}
afterEach(cleanup);

describe("the failures that need attention", () => {
  it("count a failed dump beside the failed backup of the same container", () => {
    const runs = [
      run({ id: "a", kind: "dbdump", error: "database dump failed: the database refused the login" }),
      run({ id: "b", kind: "backup" }),
    ];
    expect(failedRunsNeedingAttention(runs).map((r) => r.id)).toEqual(["a", "b"]);
  });

  it("keep the failed dump when the dump is later saved to a folder", () => {
    const runs = [
      run({ id: "a", kind: "dbdumpsave", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(failedRunsNeedingAttention(runs)).toHaveLength(1);
  });

  it("keep the failed dump when a dump is later imported", () => {
    const runs = [
      run({ id: "a", kind: "dbimport", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(failedRunsNeedingAttention(runs)).toHaveLength(1);
  });

  it("drop the dump failure once a later dump succeeds", () => {
    const runs = [
      run({ id: "a", kind: "dbdump", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(failedRunsNeedingAttention(runs)).toEqual([]);
  });
});

describe("the run history", () => {
  it("reads the note of a successful run and warns where the note warns", async () => {
    renderHistory([
      run({ id: "a", kind: "dbdump", status: "success", error: "database dump covers one database only" }),
      run({
        id: "b",
        kind: "dbimport",
        status: "success",
        error: "database imported; the previous data folder was kept: /mnt/user/appdata/pg.old",
      }),
    ]);

    const warned = await screen.findByText(en["runReason.dbdumpOneDatabase"]);
    expect(warned.className).toContain("text-statusWarn");
    // The folder path sits in its own isolate, so the sentence is the parent.
    const kept = screen.getByText(/pg\.old/).closest("span[class]");
    expect(kept?.textContent).toContain(en["runReason.dbimportKeptOld"]);
    expect(kept?.className).toContain("text-carbon-textMuted");
  });

  it("gives the kind room for the longest label instead of cutting it short", async () => {
    renderHistory([run({ kind: "dbdump", status: "success", error: "" })]);

    const kind = await screen.findByText(en["run.kindDbDump"]);
    expect(kind.className).not.toContain("truncate");
    expect(kind.parentElement!.className).toContain("flex-wrap");
  });
});
