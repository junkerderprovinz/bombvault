// @vitest-environment jsdom
// A failed dump is a failure of its own: it must raise the error count without
// hiding the container's failed backup, and nothing that happens to the dump
// afterwards, neither saving it to a folder nor importing it, may clear it.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import type { Run } from "../lib/api";

const listRuns = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return { ...actual, listRuns: () => listRuns() };
});

const { RunsCard, unresolvedErrorCount } = await import("./Dashboard");

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof RunsCard
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

beforeEach(() => listRuns.mockReset());
afterEach(cleanup);

describe("the error count", () => {
  it("counts a failed dump beside the failed backup of the same container", () => {
    const runs = [
      run({ id: "a", kind: "dbdump", error: "database dump failed: the database refused the login" }),
      run({ id: "b", kind: "backup" }),
    ];
    expect(unresolvedErrorCount(runs)).toBe(2);
  });

  it("keeps the failed dump when the dump is later saved to a folder", () => {
    const runs = [
      run({ id: "a", kind: "dbdumpsave", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(unresolvedErrorCount(runs)).toBe(1);
  });

  it("keeps the failed dump when a dump is later imported", () => {
    const runs = [
      run({ id: "a", kind: "dbimport", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(unresolvedErrorCount(runs)).toBe(1);
  });

  it("drops the dump failure once a later dump succeeds", () => {
    const runs = [
      run({ id: "a", kind: "dbdump", status: "success", error: "" }),
      run({ id: "b", kind: "dbdump", error: "database dump failed: no progress" }),
    ];
    expect(unresolvedErrorCount(runs)).toBe(0);
  });
});

describe("the run history", () => {
  it("reads the note of a successful run and warns where the note warns", async () => {
    listRuns.mockResolvedValue({
      ok: true,
      runs: [
        run({ id: "a", kind: "dbdump", status: "success", error: "database dump covers one database only" }),
        run({
          id: "b",
          kind: "dbimport",
          status: "success",
          error: "database imported; the previous data folder was kept: /mnt/user/appdata/pg.old",
        }),
      ],
    });
    render(
      <I18nProvider>
        <RunsCard t={t} />
      </I18nProvider>
    );

    const warned = await screen.findByText(en["runReason.dbdumpOneDatabase"]);
    expect(warned.className).toContain("text-statusWarn");
    const kept = screen.getByText(/pg\.old/);
    expect(kept.textContent).toContain(en["runReason.dbimportKeptOld"]);
    expect(kept.className).toContain("text-carbon-textMuted");
  });

  it("gives the kind column room for the longest label", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run({ kind: "dbdump", status: "success", error: "" })] });
    render(
      <I18nProvider>
        <RunsCard t={t} />
      </I18nProvider>
    );

    const kind = await screen.findByText(en["run.kindDbDump"]);
    await waitFor(() => expect(kind.className).toContain("break-words"));
    expect(kind.className).not.toContain("truncate");
  });
});
