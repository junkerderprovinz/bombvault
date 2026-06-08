/**
 * containers-view.test.ts — unit tests for app/containers/view.ts
 *
 * Pure function: no Docker socket, no DB, no fs — always runs in CI.
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import type { ContainerInfo } from "dockerode";
import { toContainerRows } from "../app/containers/view";

// ---------------------------------------------------------------------------
// Minimal ContainerInfo fixture builder
// ---------------------------------------------------------------------------

function makeInfo(overrides: Partial<ContainerInfo> & { Names: string[]; Id: string; Image: string; State: string }): ContainerInfo {
  return {
    Id: overrides.Id,
    Names: overrides.Names,
    Image: overrides.Image,
    ImageID: overrides.ImageID ?? "sha256:abc",
    Command: overrides.Command ?? "",
    Created: overrides.Created ?? 0,
    Ports: overrides.Ports ?? [],
    Labels: overrides.Labels ?? {},
    State: overrides.State,
    Status: overrides.Status ?? overrides.State,
    HostConfig: overrides.HostConfig ?? { NetworkMode: "bridge" },
    NetworkSettings: overrides.NetworkSettings ?? { Networks: {} },
    Mounts: overrides.Mounts ?? [],
  } as ContainerInfo;
}

const APPDATA = "/mnt/user/appdata";

// ---------------------------------------------------------------------------
// toContainerRows — merges list + last-backup map
// ---------------------------------------------------------------------------

test("toContainerRows: strips leading slash from container name", () => {
  const list = [makeInfo({ Id: "id1", Names: ["/plex"], Image: "plex:latest", State: "running" })];
  const rows = toContainerRows(list, APPDATA, new Map());
  assert.equal(rows[0]?.name, "plex");
});

test("toContainerRows: keeps name without leading slash unchanged", () => {
  const list = [makeInfo({ Id: "id2", Names: ["myapp"], Image: "myapp:v1", State: "exited" })];
  const rows = toContainerRows(list, APPDATA, new Map());
  assert.equal(rows[0]?.name, "myapp");
});

test("toContainerRows: maps id, image, state from ContainerInfo", () => {
  const list = [makeInfo({ Id: "abc123", Names: ["/sonarr"], Image: "linuxserver/sonarr:latest", State: "running" })];
  const rows = toContainerRows(list, APPDATA, new Map());
  const row = rows[0]!;
  assert.equal(row.id, "abc123");
  assert.equal(row.image, "linuxserver/sonarr:latest");
  assert.equal(row.state, "running");
});

test("toContainerRows: derives conventional appdata path from name", () => {
  const list = [makeInfo({ Id: "id3", Names: ["/plex"], Image: "plex:latest", State: "running" })];
  const rows = toContainerRows(list, APPDATA, new Map());
  assert.deepEqual(rows[0]?.appdataPaths, ["/mnt/user/appdata/plex"]);
});

test("toContainerRows: lastBackup is null when container absent from map (never backed up)", () => {
  const list = [makeInfo({ Id: "id4", Names: ["/whoami"], Image: "traefik/whoami", State: "running" })];
  const rows = toContainerRows(list, APPDATA, new Map());
  assert.equal(rows[0]?.lastBackup, null);
});

test("toContainerRows: lastBackup is null when explicitly null in map", () => {
  const list = [makeInfo({ Id: "id5", Names: ["/whoami"], Image: "traefik/whoami", State: "running" })];
  const map = new Map<string, string | null>([["whoami", null]]);
  const rows = toContainerRows(list, APPDATA, map);
  assert.equal(rows[0]?.lastBackup, null);
});

test("toContainerRows: lastBackup is set from map when present", () => {
  const ts = "2026-06-08T12:00:00.000Z";
  const list = [makeInfo({ Id: "id6", Names: ["/plex"], Image: "plex:latest", State: "running" })];
  const map = new Map<string, string | null>([["plex", ts]]);
  const rows = toContainerRows(list, APPDATA, map);
  assert.equal(rows[0]?.lastBackup, ts);
});

test("toContainerRows: merges multiple containers correctly", () => {
  const ts = "2026-06-01T00:00:00.000Z";
  const list = [
    makeInfo({ Id: "a", Names: ["/plex"], Image: "plex:latest", State: "running" }),
    makeInfo({ Id: "b", Names: ["/sonarr"], Image: "sonarr:latest", State: "exited" }),
    makeInfo({ Id: "c", Names: ["/radarr"], Image: "radarr:latest", State: "running" }),
  ];
  const map = new Map<string, string | null>([
    ["plex", ts],
    ["sonarr", null],
    // radarr absent — no backup
  ]);
  const rows = toContainerRows(list, APPDATA, map);

  assert.equal(rows.length, 3);
  assert.equal(rows[0]?.lastBackup, ts);
  assert.equal(rows[1]?.lastBackup, null);
  assert.equal(rows[2]?.lastBackup, null);
});

test("toContainerRows: empty list returns empty array", () => {
  const rows = toContainerRows([], APPDATA, new Map());
  assert.deepEqual(rows, []);
});
