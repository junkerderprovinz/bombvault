import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { loadConfig } from "../lib/config";

const VALID_KEY = "a".repeat(64); // 32 bytes hex

test("loadConfig parses a valid environment", () => {
  const cfg = loadConfig({
    APP_KEY: VALID_KEY,
    DATA_DIR: "/data",
    CONFIG_DIR: "/config",
    PORT: "3000",
    HTTPS_PORT: "3443",
    APPDATA_DIR: "/mnt/user/appdata",
  });
  assert.equal(cfg.APP_KEY, VALID_KEY);
  assert.equal(cfg.DATA_DIR, "/data");
  assert.equal(cfg.CONFIG_DIR, "/config");
  assert.equal(cfg.PORT, 3000);
  assert.equal(cfg.HTTPS_PORT, 3443);
  assert.equal(cfg.DB_PATH, join("/config", "bombvault.sqlite"));
});

test("CONFIG_DIR defaults to DATA_DIR when unset", () => {
  const cfg = loadConfig({ APP_KEY: VALID_KEY, DATA_DIR: "/data" });
  assert.equal(cfg.CONFIG_DIR, "/data");
  assert.equal(cfg.DB_PATH, join("/data", "bombvault.sqlite"));
});

test("rejects an APP_KEY that is not 64 hex chars", () => {
  assert.throws(() => loadConfig({ APP_KEY: "tooshort", DATA_DIR: "/data" }), /APP_KEY/);
});

test("rejects a missing APP_KEY", () => {
  assert.throws(() => loadConfig({ DATA_DIR: "/data" }), /APP_KEY/);
});

test("FLASH_TEMPLATES_DIR defaults to the Unraid templates path when unset", () => {
  const cfg = loadConfig({ APP_KEY: VALID_KEY, DATA_DIR: "/data" });
  assert.equal(cfg.FLASH_TEMPLATES_DIR, "/boot/config/plugins/dockerMan/templates-user");
});

test("FLASH_TEMPLATES_DIR takes the override value when set", () => {
  const cfg = loadConfig({
    APP_KEY: VALID_KEY,
    DATA_DIR: "/data",
    FLASH_TEMPLATES_DIR: "/custom/templates",
  });
  assert.equal(cfg.FLASH_TEMPLATES_DIR, "/custom/templates");
});

test("ensureDataDirs creates the data and config directories", () => {
  const base = mkdtempSync(join(tmpdir(), "bv-cfg-"));
  const cfg = loadConfig({
    APP_KEY: VALID_KEY,
    DATA_DIR: join(base, "data"),
    CONFIG_DIR: join(base, "config"),
  });
  cfg.ensureDataDirs();
  assert.ok(statSync(join(base, "data")).isDirectory(), "data dir should exist");
  assert.ok(statSync(join(base, "config")).isDirectory(), "config dir should exist");
  assert.doesNotThrow(() => cfg.ensureDataDirs()); // idempotent
});
