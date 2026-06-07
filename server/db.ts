import Database from "better-sqlite3";
import { chmodSync, existsSync } from "node:fs";
import { getConfig } from "../lib/config";
import { runMigrations } from "./schema";

// Single shared connection. initDb() opens the file (creating dirs), enables WAL,
// and runs migrations. Idempotent — safe to call from the server boot and tests.
let db: Database.Database | null = null;

export function initDb(): Database.Database {
  if (db) return db;
  const cfg = getConfig();
  cfg.ensureDataDirs();
  db = new Database(cfg.DB_PATH);
  db.pragma("journal_mode = WAL");
  // SEC-003: the DB holds the argon2 password hash and (later) encrypted repo
  // secrets — restrict it to the owner. chmod is meaningless on Windows and can
  // behave oddly, so skip it there; wrap in try/catch so a chmod failure on an
  // exotic FS never crashes startup.
  // The WAL (-wal) and SHM (-shm) sidecars hold the same sensitive pages as
  // the main DB file, so they get the same 0o600 treatment when they exist.
  if (process.platform !== "win32") {
    for (const path of [cfg.DB_PATH, `${cfg.DB_PATH}-wal`, `${cfg.DB_PATH}-shm`]) {
      if (!existsSync(path)) continue;
      try {
        chmodSync(path, 0o600);
      } catch {
        // best-effort; the parent dir is already 0o700
      }
    }
  }
  runMigrations(db);
  return db;
}

export function getDb(): Database.Database {
  return db ?? initDb();
}
