import { randomUUID } from "node:crypto";
import type Database from "better-sqlite3";
import { encryptSecret, decryptSecret } from "./secrets";

// ── Row interfaces ────────────────────────────────────────────────────────────

export interface DestinationRow {
  id: string;
  name: string;
  repo_path: string;
  /** Encrypted token — never the raw password. Use getDestinationPassword() to decrypt. */
  password_ref: string;
  created_at: number;
}

export interface BackupTargetRow {
  id: string;
  destination_id: string;
  container_name: string;
  /** JSON-encoded string[]. Use parseTarget() for the decoded form. */
  appdata_paths: string;
  /** JSON-encoded Record<string,unknown>. Use parseTarget() for the decoded form. */
  options: string;
  created_at: number;
}

export interface ParsedTarget {
  id: string;
  destination_id: string;
  container_name: string;
  appdata_paths: string[];
  options: Record<string, unknown>;
  created_at: number;
}

export interface RunRow {
  id: string;
  target_id: string;
  kind: string;
  status: string;
  started_at: number;
  finished_at: number | null;
  snapshot_id: string | null;
  bytes: number | null;
  error: string | null;
  log_ref: string | null;
}

// ── Input types ───────────────────────────────────────────────────────────────

export interface CreateDestinationInput {
  name: string;
  repoPath: string;
  password: string;
}

export interface CreateTargetInput {
  containerRef: string;
  displayName?: string;
  appdataPaths: string[];
  destinationId: string;
  options?: Record<string, unknown>;
}

export interface CreateRunInput {
  targetId: string;
  kind: "backup" | "restore";
}

export interface FinishRunInput {
  status: "success" | "failed";
  snapshotId?: string;
  bytes?: number;
  error?: string;
}

// ── Factory ───────────────────────────────────────────────────────────────────

/**
 * createRepo returns a typed CRUD object over the v2 backup tables.
 *
 * @param db      - A better-sqlite3 Database instance (must have run runMigrations).
 * @param appKey  - The APP_KEY hex string used by encryptSecret/decryptSecret.
 * @param now     - Injectable clock (default: Date.now). Override in tests for determinism.
 */
export function createRepo(
  db: Database.Database,
  appKey: string,
  now: () => number = Date.now,
) {
  // ── Destinations ────────────────────────────────────────────────────────────

  const insertDestination = db.prepare<[string, string, string, string, number]>(
    "INSERT INTO destination (id, name, repo_path, password_ref, created_at) VALUES (?, ?, ?, ?, ?)",
  );

  function createDestination(input: CreateDestinationInput): DestinationRow {
    const id = randomUUID();
    // IMPORTANT: password is NEVER stored as plaintext — always encrypted.
    const passwordRef = encryptSecret(input.password, appKey);
    const createdAt = now();
    insertDestination.run(id, input.name, input.repoPath, passwordRef, createdAt);
    return {
      id,
      name: input.name,
      repo_path: input.repoPath,
      password_ref: passwordRef,
      created_at: createdAt,
    };
  }

  function getDestinationRow(id: string): DestinationRow | undefined {
    return db
      .prepare<[string], DestinationRow>("SELECT * FROM destination WHERE id = ?")
      .get(id);
  }

  function getDestinationPassword(id: string): string {
    const row = getDestinationRow(id);
    if (!row) throw new Error(`destination not found: ${id}`);
    return decryptSecret(row.password_ref, appKey);
  }

  function listDestinations(): DestinationRow[] {
    return db.prepare<[], DestinationRow>("SELECT * FROM destination ORDER BY created_at ASC").all();
  }

  // ── Backup targets ──────────────────────────────────────────────────────────

  const insertTarget = db.prepare<[string, string, string, string, string, number]>(
    "INSERT INTO backup_target (id, destination_id, container_name, appdata_paths, options, created_at) VALUES (?, ?, ?, ?, ?, ?)",
  );

  function createTarget(input: CreateTargetInput): BackupTargetRow {
    const id = randomUUID();
    const appdataPathsJson = JSON.stringify(input.appdataPaths);
    const optionsJson = JSON.stringify(input.options ?? {});
    const createdAt = now();
    insertTarget.run(
      id,
      input.destinationId,
      input.containerRef,
      appdataPathsJson,
      optionsJson,
      createdAt,
    );
    return {
      id,
      destination_id: input.destinationId,
      container_name: input.containerRef,
      appdata_paths: appdataPathsJson,
      options: optionsJson,
      created_at: createdAt,
    };
  }

  function getTarget(id: string): BackupTargetRow | undefined {
    return db
      .prepare<[string], BackupTargetRow>("SELECT * FROM backup_target WHERE id = ?")
      .get(id);
  }

  function parseTarget(row: BackupTargetRow): ParsedTarget {
    return {
      id: row.id,
      destination_id: row.destination_id,
      container_name: row.container_name,
      appdata_paths: JSON.parse(row.appdata_paths) as string[],
      options: JSON.parse(row.options) as Record<string, unknown>,
      created_at: row.created_at,
    };
  }

  // ── Runs ────────────────────────────────────────────────────────────────────

  const insertRun = db.prepare<[string, string, string, string, number]>(
    "INSERT INTO run (id, target_id, kind, status, started_at) VALUES (?, ?, ?, ?, ?)",
  );

  const updateRun = db.prepare<[string, number | null, string | null, number | null, string | null, string]>(
    "UPDATE run SET status = ?, finished_at = ?, snapshot_id = ?, bytes = ?, error = ? WHERE id = ?",
  );

  function createRun(input: CreateRunInput): RunRow {
    const id = randomUUID();
    const startedAt = now();
    insertRun.run(id, input.targetId, input.kind, "running", startedAt);
    return {
      id,
      target_id: input.targetId,
      kind: input.kind,
      status: "running",
      started_at: startedAt,
      finished_at: null,
      snapshot_id: null,
      bytes: null,
      error: null,
      log_ref: null,
    };
  }

  function finishRun(id: string, finish: FinishRunInput): RunRow {
    const finishedAt = now();
    updateRun.run(
      finish.status,
      finishedAt,
      finish.snapshotId ?? null,
      finish.bytes ?? null,
      finish.error ?? null,
      id,
    );
    const row = getRun(id);
    if (!row) throw new Error(`run not found after update: ${id}`);
    return row;
  }

  function getRun(id: string): RunRow | undefined {
    return db.prepare<[string], RunRow>("SELECT * FROM run WHERE id = ?").get(id);
  }

  /** Returns the most recent successful backup run for a target, or undefined. */
  function lastBackupRun(targetId: string): RunRow | undefined {
    return db
      .prepare<[string], RunRow>(
        "SELECT * FROM run WHERE target_id = ? AND kind = 'backup' AND status = 'success' ORDER BY started_at DESC LIMIT 1",
      )
      .get(targetId);
  }

  return {
    // destinations
    createDestination,
    getDestinationRow,
    getDestinationPassword,
    listDestinations,
    // targets
    createTarget,
    getTarget,
    parseTarget,
    // runs
    createRun,
    finishRun,
    getRun,
    lastBackupRun,
  };
}

export type BackupRepo = ReturnType<typeof createRepo>;
