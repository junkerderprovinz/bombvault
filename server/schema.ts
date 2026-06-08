import type Database from "better-sqlite3";

// Forward-only migration runner. Each migration has a unique ascending integer
// version and idempotent-as-a-set SQL (it runs once, tracked in schema_migrations).
// P0 ships only the setting table; later phases append new migrations —
// never edit an existing one.
export interface Migration {
  version: number;
  name: string;
  sql: string;
}

export const MIGRATIONS: Migration[] = [
  {
    version: 1,
    name: "init_setting",
    sql: `
      CREATE TABLE IF NOT EXISTS setting (
        key   TEXT PRIMARY KEY,
        value TEXT NOT NULL
      );
    `,
  },
  {
    version: 2,
    name: "init_p1_backup",
    sql: `
      CREATE TABLE IF NOT EXISTS destination (
        id           TEXT PRIMARY KEY,
        name         TEXT NOT NULL,
        repo_path    TEXT NOT NULL,
        password_ref TEXT NOT NULL,
        created_at   INTEGER NOT NULL
      );

      CREATE TABLE IF NOT EXISTS backup_target (
        id             TEXT PRIMARY KEY,
        destination_id TEXT NOT NULL REFERENCES destination(id),
        container_name TEXT NOT NULL,
        appdata_paths  TEXT NOT NULL,
        options        TEXT NOT NULL DEFAULT '{}',
        created_at     INTEGER NOT NULL
      );

      CREATE TABLE IF NOT EXISTS run (
        id          TEXT PRIMARY KEY,
        target_id   TEXT NOT NULL REFERENCES backup_target(id),
        kind        TEXT NOT NULL,
        status      TEXT NOT NULL,
        started_at  INTEGER NOT NULL,
        finished_at INTEGER,
        snapshot_id TEXT,
        bytes       INTEGER,
        error       TEXT,
        log_ref     TEXT
      );

      CREATE INDEX IF NOT EXISTS idx_run_target ON run (target_id);
    `,
  },
  {
    version: 3,
    name: "unique_target_container",
    // SEC-106: a container maps to at most one backup_target. A UNIQUE index on
    // container_name prevents duplicate/ambiguous targets that could send a
    // restore to the wrong container (wrong-target hazard). Forward-only — never
    // edit v1/v2.
    sql: `
      CREATE UNIQUE INDEX IF NOT EXISTS idx_target_container_name
        ON backup_target (container_name);
    `,
  },
];

/** Apply every not-yet-applied migration, in order, inside transactions. Idempotent. */
export function runMigrations(db: Database.Database): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS schema_migrations (
      version    INTEGER PRIMARY KEY,
      name       TEXT NOT NULL,
      applied_at INTEGER NOT NULL
    );
  `);
  const applied = new Set(
    (db.prepare("SELECT version FROM schema_migrations").all() as { version: number }[]).map(
      (r) => r.version,
    ),
  );
  const record = db.prepare(
    "INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
  );
  for (const m of [...MIGRATIONS].sort((a, b) => a.version - b.version)) {
    if (applied.has(m.version)) continue;
    const tx = db.transaction(() => {
      db.exec(m.sql);
      record.run(m.version, m.name, Date.now());
    });
    tx();
  }
}
