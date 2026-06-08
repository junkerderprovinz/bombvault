import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { appdataPathForName, resolveAppdataPaths } from "../lib/appdata";
import type { ContainerInspectSubset } from "../lib/appdata";

const FIXTURES = join(import.meta.dirname, "fixtures");
const APPDATA = "/mnt/user/appdata";

function load(name: string): ContainerInspectSubset {
  return JSON.parse(readFileSync(join(FIXTURES, name), "utf8")) as ContainerInspectSubset;
}

// --- appdataPathForName ---

test("appdataPathForName strips a leading slash from name and joins with appdataDir", () => {
  assert.equal(appdataPathForName("/plex", APPDATA), "/mnt/user/appdata/plex");
});

test("appdataPathForName works when name has no leading slash", () => {
  assert.equal(appdataPathForName("whoami", APPDATA), "/mnt/user/appdata/whoami");
});

// --- resolveAppdataPaths with bind mounts under appdataDir ---

test("resolveAppdataPaths returns only bind Sources under appdataDir", () => {
  const inspect = load("inspect-bind-appdata.json");
  const paths = resolveAppdataPaths(inspect, APPDATA);
  // Only /mnt/user/appdata/plex qualifies; /mnt/user/Media is outside; volume is Type!=bind
  assert.deepEqual(paths, ["/mnt/user/appdata/plex"]);
});

test("resolveAppdataPaths excludes bind mounts outside appdataDir", () => {
  const inspect = load("inspect-bind-appdata.json");
  const paths = resolveAppdataPaths(inspect, APPDATA);
  assert.ok(!paths.includes("/mnt/user/Media"), "media mount must not be included");
});

test("resolveAppdataPaths excludes volume-type mounts", () => {
  const inspect = load("inspect-bind-appdata.json");
  const paths = resolveAppdataPaths(inspect, APPDATA);
  assert.ok(
    !paths.some((p) => p.includes("docker/volumes")),
    "volume-type mounts must not be included",
  );
});

// --- resolveAppdataPaths fallback when no qualifying mounts ---

test("resolveAppdataPaths falls back to name convention when no bind mounts match appdataDir", () => {
  const inspect = load("inspect-no-appdata.json");
  const paths = resolveAppdataPaths(inspect, APPDATA);
  assert.deepEqual(paths, ["/mnt/user/appdata/whoami"]);
});

// --- de-duplication ---

test("resolveAppdataPaths de-duplicates repeated sources", () => {
  const inspect: ContainerInspectSubset = {
    Name: "/dedupe",
    Mounts: [
      { Type: "bind", Source: "/mnt/user/appdata/dedupe", Destination: "/config" },
      { Type: "bind", Source: "/mnt/user/appdata/dedupe", Destination: "/cfg2" },
    ],
  };
  const paths = resolveAppdataPaths(inspect, APPDATA);
  assert.deepEqual(paths, ["/mnt/user/appdata/dedupe"]);
});
