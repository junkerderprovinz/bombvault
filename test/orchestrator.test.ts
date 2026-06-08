/**
 * orchestrator.test.ts — unit tests for server/orchestrator.ts
 *
 * All deps are injected mocks — no real Docker socket, restic binary, or DB.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import type { ContainerInspect } from "../lib/docker";
import type { BackupSummary } from "../lib/restic";
import type { RunRow, FinishRunInput } from "../lib/backup-repo";
import {
  backupContainer,
  restoreContainer,
  type BackupDeps,
  type RestoreDeps,
  type DockerDeps,
  type ResticDeps,
  type TemplateDeps,
  type RunRecorder,
} from "../server/orchestrator";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const INSPECT: ContainerInspect = {
  Id: "abc123",
  Name: "/plex",
  Image: "lscr.io/linuxserver/plex:latest",
  Config: {
    Image: "lscr.io/linuxserver/plex:latest",
    Env: ["PUID=1000"],
    Cmd: null,
  },
  HostConfig: {
    Binds: ["/mnt/user/appdata/plex:/config"],
    PortBindings: {},
    RestartPolicy: { Name: "unless-stopped" },
  },
  Mounts: [
    { Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config" },
  ],
};

const SUMMARY: BackupSummary = {
  snapshotId: "deadbeef12345678",
  bytesAdded: 1024,
  totalBytesProcessed: 4096,
};

let runIdSeq = 0;
function makeRunRow(targetId: string, kind: "backup" | "restore"): RunRow {
  return {
    id: `run-${++runIdSeq}`,
    target_id: targetId,
    kind,
    status: "running",
    started_at: 1000,
    finished_at: null,
    snapshot_id: null,
    bytes: null,
    error: null,
    log_ref: null,
  };
}

// ---------------------------------------------------------------------------
// Mock factories
// ---------------------------------------------------------------------------

function makeDockerMock(log: string[], opts?: { stopThrows?: boolean; startThrows?: boolean }): DockerDeps {
  return {
    async stopContainer(id, timeoutSec) {
      log.push(`stop:${id}:${timeoutSec}`);
      if (opts?.stopThrows) throw new Error("stop failed");
    },
    async startContainer(id) {
      log.push(`start:${id}`);
      if (opts?.startThrows) throw new Error("start failed");
    },
    async removeContainer(id) {
      log.push(`remove:${id}`);
    },
    async createAndStartFromDefinition(inspect) {
      log.push(`createAndStart:${inspect.Name}`);
    },
    async pullImage(image) {
      log.push(`pull:${image}`);
    },
  };
}

function makeResticMock(
  log: string[],
  opts?: { backupThrows?: boolean },
): ResticDeps {
  return {
    async backup(repo, paths, tags, _password) {
      log.push(`backup:${repo}:${paths.join(",")}:${tags.join(",")}`);
      if (opts?.backupThrows) throw new Error("restic backup failed");
      return SUMMARY;
    },
    async restore(repo, snapshotId, targetDir, _password) {
      log.push(`restore:${repo}:${snapshotId}:${targetDir}`);
    },
  };
}

function makeTemplateMock(log: string[], xml: string | null = "<xml/>"): TemplateDeps {
  return {
    readTemplate(dir, name) {
      log.push(`readTemplate:${dir}:${name}`);
      return xml;
    },
    writeTemplate(dir, name, content) {
      log.push(`writeTemplate:${dir}:${name}:${content.slice(0, 10)}`);
    },
  };
}

function makeRunsMock(log: string[]): RunRecorder {
  return {
    recordRunStart(targetId, kind) {
      log.push(`runStart:${targetId}:${kind}`);
      return makeRunRow(targetId, kind);
    },
    recordRunFinish(runId, finish: FinishRunInput) {
      log.push(`runFinish:${runId}:${finish.status}${finish.error ? `:${finish.error}` : ""}`);
      return { ...makeRunRow("x", "backup"), id: runId, status: finish.status };
    },
  };
}

function makeBackupDeps(
  log: string[],
  overrides?: {
    docker?: DockerDeps;
    restic?: ResticDeps;
    templates?: TemplateDeps;
    runs?: RunRecorder;
  },
): BackupDeps {
  return {
    containerRef: "plex",
    containerName: "Plex",
    repoPath: "/repo",
    repoPassword: "secret",
    appdataPaths: ["/mnt/user/appdata/plex"],
    stopTimeoutSec: 30,
    targetId: "target-1",
    snapshotTemplatesDir: "/data/templates",
    flashTemplatesDir: "/boot/templates",
    docker: overrides?.docker ?? makeDockerMock(log),
    restic: overrides?.restic ?? makeResticMock(log),
    templates: overrides?.templates ?? makeTemplateMock(log),
    runs: overrides?.runs ?? makeRunsMock(log),
  };
}

function makeRestoreDeps(
  log: string[],
  overrides?: {
    confirmed?: boolean;
    docker?: DockerDeps;
    restic?: ResticDeps;
    templates?: TemplateDeps;
    runs?: RunRecorder;
  },
): RestoreDeps {
  return {
    confirmed: overrides?.confirmed ?? true,
    containerRef: "plex",
    containerName: "Plex",
    repoPath: "/repo",
    repoPassword: "secret",
    snapshotId: "deadbeef12345678",
    restoreTargetDir: "/mnt/user/appdata",
    templateXml: "<xml>restored</xml>",
    flashTemplatesDir: "/boot/templates",
    inspect: INSPECT,
    targetId: "target-1",
    docker: overrides?.docker ?? makeDockerMock(log),
    restic: overrides?.restic ?? makeResticMock(log),
    templates: overrides?.templates ?? makeTemplateMock(log),
    runs: overrides?.runs ?? makeRunsMock(log),
  };
}

// ---------------------------------------------------------------------------
// backupContainer — happy path call order
// ---------------------------------------------------------------------------

test("backupContainer: happy path calls runStart → stop → backup → readTemplate → start → runFinish:success in order", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log);

  const result = await backupContainer(deps);

  assert.equal(result.snapshotId, SUMMARY.snapshotId, "returns the backup summary");

  // runStart must be first
  assert.equal(log[0], "runStart:target-1:backup", "first: runStart");
  // stop must precede backup
  const stopIdx = log.findIndex((e) => e.startsWith("stop:plex"));
  const backupIdx = log.findIndex((e) => e.startsWith("backup:"));
  assert.ok(stopIdx > 0, "stop must appear after runStart");
  assert.ok(backupIdx > stopIdx, "backup must come after stop");
  // readTemplate after backup
  const readIdx = log.findIndex((e) => e.startsWith("readTemplate:"));
  assert.ok(readIdx > backupIdx, "readTemplate must come after backup");
  // start after readTemplate (in finally block)
  const startIdx = log.findIndex((e) => e.startsWith("start:plex"));
  assert.ok(startIdx > readIdx, "start must come after readTemplate");
  // runFinish:success must be last
  const finishEntries = log.filter((e) => e.startsWith("runFinish:"));
  const finishEntry = finishEntries[finishEntries.length - 1];
  assert.ok(finishEntry?.includes(":success"), "runFinish must be success");
  assert.equal(log.lastIndexOf(finishEntry), log.length - 1, "runFinish must be last");
});

test("backupContainer: passes container:ref and p1 tags to restic", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log);

  await backupContainer(deps);

  const backupEntry = log.find((e) => e.startsWith("backup:"));
  assert.ok(backupEntry, "backup must have been called");
  assert.ok(backupEntry.includes("container:plex"), "tag container:ref present");
  assert.ok(backupEntry.includes("p1"), "tag p1 present");
});

test("backupContainer: returns the BackupSummary from restic.backup", async () => {
  const log: string[] = [];
  const summary = await backupContainer(makeBackupDeps(log));
  assert.deepEqual(summary, SUMMARY);
});

// ---------------------------------------------------------------------------
// backupContainer — always-restart on failure
// ---------------------------------------------------------------------------

test("backupContainer: container is started AND run recorded as failed when restic throws", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log, {
    restic: makeResticMock(log, { backupThrows: true }),
  });

  await assert.rejects(
    () => backupContainer(deps),
    /restic backup failed/,
    "must re-throw the original error",
  );

  // start must still have been called
  const startIdx = log.findIndex((e) => e.startsWith("start:plex"));
  assert.ok(startIdx >= 0, "container must be started even after backup failure");

  // runFinish must record failed
  const finishEntry = log.find((e) => e.startsWith("runFinish:"));
  assert.ok(finishEntry, "runFinish must be recorded");
  assert.ok(finishEntry.includes(":failed"), "status must be failed");
});

test("backupContainer: runFinish is called before the error propagates", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log, {
    restic: makeResticMock(log, { backupThrows: true }),
  });

  await assert.rejects(() => backupContainer(deps));

  const finishEntries2 = log.filter((e) => e.startsWith("runFinish:"));
  const finishIdx = finishEntries2.length > 0 ? log.lastIndexOf(finishEntries2[finishEntries2.length - 1]) : -1;
  assert.ok(finishIdx >= 0, "runFinish must appear in log");
});

test("backupContainer: start is called even when backup throws (finally guarantee)", async () => {
  let startCalled = false;
  const log: string[] = [];

  const docker = makeDockerMock(log);
  const originalStart = docker.startContainer.bind(docker);
  docker.startContainer = async (id) => {
    startCalled = true;
    return originalStart(id);
  };

  const deps = makeBackupDeps(log, {
    docker,
    restic: makeResticMock(log, { backupThrows: true }),
  });

  await assert.rejects(() => backupContainer(deps));
  assert.ok(startCalled, "startContainer must be called unconditionally");
});

// ---------------------------------------------------------------------------
// backupContainer — template persistence
// ---------------------------------------------------------------------------

test("backupContainer: persists template copy with snapshotId prefix when template exists", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log);

  await backupContainer(deps);

  const writeEntry = log.find((e) => e.startsWith("writeTemplate:"));
  assert.ok(writeEntry, "writeTemplate must be called");
  assert.ok(
    writeEntry.includes(SUMMARY.snapshotId),
    "writeTemplate path must include snapshotId",
  );
});

test("backupContainer: does not call writeTemplate when readTemplate returns null", async () => {
  const log: string[] = [];
  const deps = makeBackupDeps(log, {
    templates: makeTemplateMock(log, null),
  });

  await backupContainer(deps);

  const writeEntry = log.find((e) => e.startsWith("writeTemplate:"));
  assert.equal(writeEntry, undefined, "writeTemplate must not be called when template is null");
});

// ---------------------------------------------------------------------------
// restoreContainer — confirmed guard
// ---------------------------------------------------------------------------

test("restoreContainer: throws immediately if confirmed is false — no run recorded", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log, { confirmed: false });

  await assert.rejects(
    () => restoreContainer(deps),
    /confirmed/,
    "must throw with a message mentioning confirmed",
  );

  assert.equal(
    log.find((e) => e.startsWith("runStart:")),
    undefined,
    "runStart must NOT be called when confirmed is false",
  );
});

test("restoreContainer: throws immediately if confirmed is not set (undefined cast to false)", async () => {
  const log: string[] = [];
  // Deliberately test a falsy value
  const deps = makeRestoreDeps(log, { confirmed: false });
  await assert.rejects(() => restoreContainer(deps));
});

// ---------------------------------------------------------------------------
// restoreContainer — happy path call order
// ---------------------------------------------------------------------------

test("restoreContainer: happy path orders pull → stop → remove → restore → writeTemplate → createAndStart → runFinish:success", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log);

  await restoreContainer(deps);

  // runStart must be first
  assert.equal(log[0], "runStart:target-1:restore", "first: runStart");

  const pullIdx = log.findIndex((e) => e.startsWith("pull:"));
  const stopIdx = log.findIndex((e) => e.startsWith("stop:plex"));
  const removeIdx = log.findIndex((e) => e.startsWith("remove:plex"));
  const restoreIdx = log.findIndex((e) => e.startsWith("restore:"));
  const writeIdx = log.findIndex((e) => e.startsWith("writeTemplate:"));
  const createIdx = log.findIndex((e) => e.startsWith("createAndStart:"));
  const finishIdx = log.findIndex((e) => e.startsWith("runFinish:"));

  assert.ok(pullIdx > 0,         "pull must appear");
  assert.ok(stopIdx > pullIdx,    "stop must come after pull");
  assert.ok(removeIdx > stopIdx,  "remove must come after stop");
  assert.ok(restoreIdx > removeIdx, "restore must come after remove");
  assert.ok(writeIdx > restoreIdx, "writeTemplate must come after restore");
  assert.ok(createIdx > writeIdx,  "createAndStart must come after writeTemplate");
  assert.ok(finishIdx > createIdx, "runFinish must come after createAndStart");

  const finishEntry = log[finishIdx];
  assert.ok(finishEntry.includes(":success"), "runFinish must be success");
});

test("restoreContainer: passes correct snapshotId and targetDir to restic.restore", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log);

  await restoreContainer(deps);

  const restoreEntry = log.find((e) => e.startsWith("restore:"));
  assert.ok(restoreEntry, "restore must be called");
  assert.ok(restoreEntry.includes("deadbeef12345678"), "snapshotId must be passed");
  assert.ok(restoreEntry.includes("/mnt/user/appdata"), "restoreTargetDir must be passed");
});

test("restoreContainer: writes template to flashTemplatesDir with containerName", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log);

  await restoreContainer(deps);

  const writeEntry = log.find((e) => e.startsWith("writeTemplate:"));
  assert.ok(writeEntry, "writeTemplate must be called");
  assert.ok(writeEntry.includes("/boot/templates"), "must write to flashTemplatesDir");
  assert.ok(writeEntry.includes("Plex"), "must use containerName");
});

test("restoreContainer: calls createAndStartFromDefinition with the stored inspect", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log);

  await restoreContainer(deps);

  const entry = log.find((e) => e.startsWith("createAndStart:"));
  assert.ok(entry, "createAndStartFromDefinition must be called");
  assert.ok(entry.includes("/plex"), "must pass inspect with correct Name");
});

// ---------------------------------------------------------------------------
// restoreContainer — ignores "no such container" errors from stop/remove
// ---------------------------------------------------------------------------

test("restoreContainer: proceeds even when stopContainer throws (container absent)", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log, {
    docker: {
      ...makeDockerMock(log),
      async stopContainer(id, _t) {
        log.push(`stop:${id}:throws`);
        throw new Error("no such container");
      },
    },
  });

  // Should NOT throw
  await assert.doesNotReject(() => restoreContainer(deps));

  const restoreEntry = log.find((e) => e.startsWith("restore:"));
  assert.ok(restoreEntry, "restore must still proceed after stop failure");
});

test("restoreContainer: proceeds even when removeContainer throws (container absent)", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log, {
    docker: {
      ...makeDockerMock(log),
      async removeContainer(id) {
        log.push(`remove:${id}:throws`);
        throw new Error("no such container");
      },
    },
  });

  await assert.doesNotReject(() => restoreContainer(deps));

  const restoreEntry = log.find((e) => e.startsWith("restore:"));
  assert.ok(restoreEntry, "restore must still proceed after remove failure");
});

// ---------------------------------------------------------------------------
// restoreContainer — failure recording
// ---------------------------------------------------------------------------

test("restoreContainer: records runFinish:failed and re-throws when restic.restore throws", async () => {
  const log: string[] = [];
  const deps = makeRestoreDeps(log, {
    restic: {
      async backup() { throw new Error("not used"); },
      async restore() { throw new Error("restic restore failed"); },
    },
  });

  await assert.rejects(() => restoreContainer(deps), /restic restore failed/);

  const finishEntry = log.find((e) => e.startsWith("runFinish:"));
  assert.ok(finishEntry, "runFinish must be recorded");
  assert.ok(finishEntry.includes(":failed"), "status must be failed");
});
