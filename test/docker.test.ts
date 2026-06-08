/**
 * docker.test.ts — unit tests for lib/docker.ts
 *
 * All tests inject a fake DockerLike — no real Docker socket is required.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import type { ContainerInfo } from "dockerode";
import {
  buildCreateOptions,
  stopContainer,
  startContainer,
  removeContainer,
  createAndStartFromDefinition,
  type ContainerInspect,
  type ContainerHandle,
  type DockerLike,
} from "../lib/docker";

// ---------------------------------------------------------------------------
// Minimal inspect fixture
// ---------------------------------------------------------------------------

const BASE_INSPECT: ContainerInspect = {
  Id: "abc123",
  Name: "/plex",
  Image: "lscr.io/linuxserver/plex:latest",
  Config: {
    Image: "lscr.io/linuxserver/plex:latest",
    Env: ["PUID=1000", "PGID=1000"],
    Cmd: null,
  },
  HostConfig: {
    Binds: ["/mnt/user/appdata/plex:/config"],
    PortBindings: { "32400/tcp": [{ HostIp: "0.0.0.0", HostPort: "32400" }] },
    RestartPolicy: { Name: "unless-stopped" },
  },
  Mounts: [
    { Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config" },
  ],
};

// ---------------------------------------------------------------------------
// buildCreateOptions — pure mapping
// ---------------------------------------------------------------------------

test("buildCreateOptions: maps Image from Config.Image", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.equal(opts.Image, "lscr.io/linuxserver/plex:latest");
});

test("buildCreateOptions: strips leading slash from Name", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.equal(opts.name, "plex");
});

test("buildCreateOptions: name without leading slash is kept as-is", () => {
  const opts = buildCreateOptions({ ...BASE_INSPECT, Name: "myapp" });
  assert.equal(opts.name, "myapp");
});

test("buildCreateOptions: maps Env array", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.deepEqual(opts.Env, ["PUID=1000", "PGID=1000"]);
});

test("buildCreateOptions: Env defaults to empty array when null", () => {
  const opts = buildCreateOptions({
    ...BASE_INSPECT,
    Config: { ...BASE_INSPECT.Config, Env: null },
  });
  assert.deepEqual(opts.Env, []);
});

test("buildCreateOptions: Cmd defaults to empty array when null", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.deepEqual(opts.Cmd, []);
});

test("buildCreateOptions: maps Binds from HostConfig", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.deepEqual(opts.HostConfig.Binds, ["/mnt/user/appdata/plex:/config"]);
});

test("buildCreateOptions: maps PortBindings from HostConfig", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.deepEqual(opts.HostConfig.PortBindings, {
    "32400/tcp": [{ HostIp: "0.0.0.0", HostPort: "32400" }],
  });
});

test("buildCreateOptions: maps RestartPolicy from HostConfig", () => {
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.deepEqual(opts.HostConfig.RestartPolicy, { Name: "unless-stopped" });
});

test("buildCreateOptions: RestartPolicy defaults to {Name:'no'} when null", () => {
  const opts = buildCreateOptions({
    ...BASE_INSPECT,
    HostConfig: { ...BASE_INSPECT.HostConfig, RestartPolicy: null },
  });
  assert.deepEqual(opts.HostConfig.RestartPolicy, { Name: "no" });
});

// ---------------------------------------------------------------------------
// buildCreateOptions — SEC-105 security-relevant fields survive a round-trip
// ---------------------------------------------------------------------------

const SECURE_INSPECT: ContainerInspect = {
  ...BASE_INSPECT,
  Config: { ...BASE_INSPECT.Config, User: "1000:1000" },
  HostConfig: {
    ...BASE_INSPECT.HostConfig,
    CapAdd: ["NET_ADMIN"],
    CapDrop: ["MKNOD"],
    Privileged: true,
    SecurityOpt: ["no-new-privileges:true"],
    ReadonlyRootfs: true,
    NetworkMode: "host",
    Devices: [
      { PathOnHost: "/dev/dri", PathInContainer: "/dev/dri", CgroupPermissions: "rwm" },
    ],
  },
};

test("buildCreateOptions: SEC-105 preserves User", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.equal(opts.User, "1000:1000");
});

test("buildCreateOptions: SEC-105 preserves CapAdd / CapDrop", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.deepEqual(opts.HostConfig.CapAdd, ["NET_ADMIN"]);
  assert.deepEqual(opts.HostConfig.CapDrop, ["MKNOD"]);
});

test("buildCreateOptions: SEC-105 preserves Privileged", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.equal(opts.HostConfig.Privileged, true);
});

test("buildCreateOptions: SEC-105 preserves SecurityOpt", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.deepEqual(opts.HostConfig.SecurityOpt, ["no-new-privileges:true"]);
});

test("buildCreateOptions: SEC-105 preserves ReadonlyRootfs", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.equal(opts.HostConfig.ReadonlyRootfs, true);
});

test("buildCreateOptions: SEC-105 preserves NetworkMode", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.equal(opts.HostConfig.NetworkMode, "host");
});

test("buildCreateOptions: SEC-105 preserves Devices", () => {
  const opts = buildCreateOptions(SECURE_INSPECT);
  assert.deepEqual(opts.HostConfig.Devices, [
    { PathOnHost: "/dev/dri", PathInContainer: "/dev/dri", CgroupPermissions: "rwm" },
  ]);
});

test("buildCreateOptions: SEC-105 omits security fields not present in inspect (keeps docker defaults)", () => {
  // BASE_INSPECT has none of the SEC-105 fields set.
  const opts = buildCreateOptions(BASE_INSPECT);
  assert.equal(opts.User, undefined, "User absent");
  assert.equal(opts.HostConfig.CapAdd, undefined, "CapAdd absent");
  assert.equal(opts.HostConfig.CapDrop, undefined, "CapDrop absent");
  assert.equal(opts.HostConfig.Privileged, undefined, "Privileged absent");
  assert.equal(opts.HostConfig.SecurityOpt, undefined, "SecurityOpt absent");
  assert.equal(opts.HostConfig.ReadonlyRootfs, undefined, "ReadonlyRootfs absent");
  assert.equal(opts.HostConfig.NetworkMode, undefined, "NetworkMode absent");
  assert.equal(opts.HostConfig.Devices, undefined, "Devices absent");
});

// ---------------------------------------------------------------------------
// stopContainer — passes {t: timeoutSec} to container.stop
// ---------------------------------------------------------------------------

test("stopContainer: calls getContainer then stop({t}) in order", async () => {
  const log: string[] = [];

  const fakeContainer: ContainerHandle = {
    id: "cid",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => { log.push("start"); },
    stop: async (opts) => { log.push(`stop:${JSON.stringify(opts)}`); },
    remove: async () => { log.push("remove"); },
  };

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: (id) => { log.push(`get:${id}`); return fakeContainer; },
    createContainer: async () => { throw new Error("not expected"); },
    pull: async () => { throw new Error("not expected"); },
    modem: { followProgress: () => { throw new Error("not expected"); } },
  };

  await stopContainer(fakeDocker, "cid", 30);

  assert.deepEqual(log, ["get:cid", `stop:${JSON.stringify({ t: 30 })}`]);
});

test("stopContainer: passes the correct timeout value", async () => {
  let capturedOpts: { t?: number } | undefined;

  const fakeContainer: ContainerHandle = {
    id: "c2",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => {},
    stop: async (opts) => { capturedOpts = opts; },
    remove: async () => {},
  };

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: () => fakeContainer,
    createContainer: async () => { throw new Error("not expected"); },
    pull: async () => { throw new Error("not expected"); },
    modem: { followProgress: () => { throw new Error("not expected"); } },
  };

  await stopContainer(fakeDocker, "c2", 60);
  assert.deepEqual(capturedOpts, { t: 60 });
});

// ---------------------------------------------------------------------------
// createAndStartFromDefinition — pull → create → start order
// ---------------------------------------------------------------------------

test("createAndStartFromDefinition: executes pull → create → start in order", async () => {
  const log: string[] = [];

  const fakeContainer: ContainerHandle = {
    id: "new-container",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => { log.push("start"); },
    stop: async () => {},
    remove: async () => {},
  };

  // Minimal fake stream that followProgress immediately calls onFinished
  const fakeStream = {
    on: () => fakeStream,
    pipe: () => fakeStream,
  } as unknown as NodeJS.ReadableStream;

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: () => { throw new Error("not expected"); },
    createContainer: async (opts) => {
      log.push(`create:${opts.name ?? ""}`);
      return fakeContainer;
    },
    pull: async (repoTag) => {
      log.push(`pull:${repoTag}`);
      return fakeStream;
    },
    modem: {
      followProgress: (_stream, onFinished) => {
        // Immediately signal completion
        onFinished(null, []);
      },
    },
  };

  const result = await createAndStartFromDefinition(fakeDocker, BASE_INSPECT);

  assert.equal(result, fakeContainer, "should return the created container handle");
  assert.equal(log[0], "pull:lscr.io/linuxserver/plex:latest", "pull must be first");
  assert.equal(log[1], "create:plex", "create must be second");
  assert.equal(log[2], "start", "start must be third");
  assert.equal(log.length, 3, "exactly three steps");
});

test("createAndStartFromDefinition: uses the image from Config.Image for pull", async () => {
  let pulledTag = "";
  const fakeStream = {
    on: () => fakeStream,
  } as unknown as NodeJS.ReadableStream;

  const fakeContainer: ContainerHandle = {
    id: "c3",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => {},
    stop: async () => {},
    remove: async () => {},
  };

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: () => { throw new Error("not expected"); },
    createContainer: async () => fakeContainer,
    pull: async (tag) => { pulledTag = tag; return fakeStream; },
    modem: {
      followProgress: (_stream, onFinished) => { onFinished(null, []); },
    },
  };

  const customInspect: ContainerInspect = {
    ...BASE_INSPECT,
    Config: { ...BASE_INSPECT.Config, Image: "ghcr.io/my/image:v2" },
  };
  await createAndStartFromDefinition(fakeDocker, customInspect);
  assert.equal(pulledTag, "ghcr.io/my/image:v2");
});

// ---------------------------------------------------------------------------
// startContainer / removeContainer — delegate to getContainer correctly
// ---------------------------------------------------------------------------

test("startContainer: calls getContainer(id).start()", async () => {
  const log: string[] = [];

  const fakeContainer: ContainerHandle = {
    id: "s1",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => { log.push("started"); },
    stop: async () => {},
    remove: async () => {},
  };

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: (id) => { log.push(`get:${id}`); return fakeContainer; },
    createContainer: async () => { throw new Error("not expected"); },
    pull: async () => { throw new Error("not expected"); },
    modem: { followProgress: () => { throw new Error("not expected"); } },
  };

  await startContainer(fakeDocker, "s1");
  assert.deepEqual(log, ["get:s1", "started"]);
});

test("removeContainer: calls getContainer(id).remove()", async () => {
  const log: string[] = [];

  const fakeContainer: ContainerHandle = {
    id: "r1",
    inspect: async () => { throw new Error("not expected"); },
    start: async () => {},
    stop: async () => {},
    remove: async () => { log.push("removed"); },
  };

  const fakeDocker: DockerLike = {
    listContainers: async () => [] as ContainerInfo[],
    getContainer: (id) => { log.push(`get:${id}`); return fakeContainer; },
    createContainer: async () => { throw new Error("not expected"); },
    pull: async () => { throw new Error("not expected"); },
    modem: { followProgress: () => { throw new Error("not expected"); } },
  };

  await removeContainer(fakeDocker, "r1");
  assert.deepEqual(log, ["get:r1", "removed"]);
});
