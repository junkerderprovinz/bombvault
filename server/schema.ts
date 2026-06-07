import type Database from "better-sqlite3";

// Forward-only migration runner. Each migration has a unique ascending integer
// version and idempotent-as-a-set SQL (it runs once, tracked in schema_migrations).
// P0 ships only the setting + user tables; later phases append new migrations —
// never edit an existing one.
export interface Migration {
  version: number;
  name: string;
  sql: string;
}

export const MIGRATIONS: Migration[] = [
  {
    version: 1,
    name: "init_setting_user",
    sql: `
      CREATE TABLE IF NOT EXISTS setting (
        key   TEXT PRIMARY KEY,
        value TEXT NOT NULL
      );
      CREATE TABLE IF NOT EXISTS user (
        id            INTEGER PRIMARY KEY AUTOINCREMENT,
        username      TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        created_at    INTEGER NOT NULL
      );
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
