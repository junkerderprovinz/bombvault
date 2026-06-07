import Database from "better-sqlite3";
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
  runMigrations(db);
  return db;
}

export function getDb(): Database.Database {
  return db ?? initDb();
}
