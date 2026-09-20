// The remedy table is the only place that names the dbdump.fix* keys, so it is
// also the only place a dump failure can lose its advice. The prefix list is
// imported rather than copied: a reason added to runReason.ts without a remedy
// fails here instead of showing an empty bubble.
import { describe, expect, it } from "vitest";
import { coverageKey, dbDumpNameOf, ENGINE_NAMES, isDbDumpIdentity, pairsWith, remedyKey } from "./dbdump";
import { RUN_REASON_PREFIXES } from "./runReason";

describe("remedyKey", () => {
  it("every dump failure has a remedy", () => {
    const missing = Object.keys(RUN_REASON_PREFIXES)
      .filter((reason) => reason.startsWith("database dump failed:"))
      .filter((reason) => remedyKey(reason) === null);
    expect(missing).toEqual([]);
  });

  it("reads a reason that carries the tool's own message", () => {
    expect(remedyKey("database dump failed: the database refused the login: FATAL x")).toBe("dbdump.fixAuth");
  });

  it("offers no remedy for a cancellation, a note or an import", () => {
    expect(remedyKey("cancelled by the user")).toBeNull();
    expect(remedyKey("database dump covers one database only")).toBeNull();
    expect(remedyKey("database dump skipped: its run could not be recorded")).toBeNull();
    expect(remedyKey("database import failed: the import tool reported an error")).toBeNull();
    expect(remedyKey("")).toBeNull();
    expect(remedyKey(undefined)).toBeNull();
  });

  it("sends the failures that share advice to the same key", () => {
    expect(remedyKey("database dump failed: the dump was empty")).toBe("dbdump.fixDumpData");
    expect(remedyKey("database dump failed: the dump ended before its completion marker")).toBe("dbdump.fixDumpData");
    expect(remedyKey("database dump failed: the dump tool reported an error")).toBe("dbdump.fixDumpData");
    expect(remedyKey("database dump failed: the repository did not accept it")).toBe("dbdump.fixRepository");
    expect(remedyKey("database dump failed: the stored size does not match what was dumped")).toBe("dbdump.fixRepository");
  });

  it("leaves an error from restic or Docker without advice of ours", () => {
    expect(remedyKey("exit status 1: repository is already locked")).toBeNull();
  });
});

describe("coverageKey", () => {
  it("has a sentence for every coverage the backend can report", () => {
    expect(coverageKey("stopped")).toBe("dbdump.coverageStopped");
    expect(coverageKey("live")).toBe("dbdump.coverageLive");
    expect(coverageKey("none")).toBe("dbdump.coverageNone");
    expect(coverageKey("unknown")).toBe("dbdump.coverageUnknown");
    expect(coverageKey("")).toBeNull();
  });
});

describe("pairsWith", () => {
  const dump = { pairedSnapshotId: "abc123" };

  it("matches the snapshot the dump was taken with", () => {
    expect(pairsWith(dump, { id: "abc123" })).toBe(true);
  });

  it("matches an off-site copy through its original id", () => {
    expect(pairsWith(dump, { id: "copy999", original: "abc123" })).toBe(true);
  });

  it("matches nothing when the dump names no snapshot", () => {
    expect(pairsWith({}, { id: "abc123" })).toBe(false);
    expect(pairsWith({ pairedSnapshotId: "" }, { id: "" })).toBe(false);
  });
});

describe("dump identities", () => {
  it("reads the container name out of a dump tag", () => {
    expect(isDbDumpIdentity("dbdump:immich_postgres")).toBe(true);
    expect(dbDumpNameOf("dbdump:immich_postgres")).toBe("immich_postgres");
  });

  it("leaves a container identity alone", () => {
    expect(isDbDumpIdentity("container:immich_postgres")).toBe(false);
    expect(dbDumpNameOf("container:immich_postgres")).toBe("");
  });
});

describe("ENGINE_NAMES", () => {
  it("names the three engines as their makers write them", () => {
    expect(ENGINE_NAMES.postgres).toBe("PostgreSQL");
    expect(ENGINE_NAMES.mysql).toBe("MySQL");
    expect(ENGINE_NAMES.mariadb).toBe("MariaDB");
  });
});
