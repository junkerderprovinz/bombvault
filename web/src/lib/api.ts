// ---------------------------------------------------------------------------
// API types — match the Go JSON shapes exactly
// ---------------------------------------------------------------------------

/** The `{ok,error}` envelope returned by every mutating endpoint. */
export interface OkEnvelope {
  ok: boolean;
  error?: string;
}

/** A container row from GET /api/containers */
export interface Container {
  name: string;
  image: string;
  state: string;
  status: string;
  includeInSchedule: boolean;
  lastBackup: number | null;
}

export interface ListContainersResponse {
  ok: boolean;
  containers: Container[];
}

/** Response from POST /api/containers/{name}/backup */
export interface BackupResponse extends OkEnvelope {
  snapshotId?: string;
  bytes?: number;
}

/** A restic snapshot from GET /api/containers/{name}/snapshots */
export interface Snapshot {
  id: string;
  time: string;
  paths: string[];
  tags: string[];
  hostname: string;
}

export interface ListSnapshotsResponse {
  ok: boolean;
  snapshots: Snapshot[];
}

/** Settings from GET /api/settings (nested under "settings") */
export interface Settings {
  encryptionEnabled: boolean;
  containersEnabled: boolean;
  vmsEnabled: boolean;
  flashEnabled: boolean;
  containersPath: string;
  vmsPath: string;
  flashPath: string;
  containersSchedule: string;
  vmsSchedule: string;
  flashSchedule: string;
  defaultLanguage: string;
}

export interface GetSettingsResponse {
  ok: boolean;
  settings: Settings;
}

/** A run record from GET /api/runs — camelCase matches store.Run JSON tags */
export interface Run {
  id: string;
  targetId: string;
  kind: string;
  status: string;
  startedAt: number;
  finishedAt: number | null;
  snapshotId: string;
  bytes: number;
  error: string;
}

export interface ListRunsResponse {
  ok: boolean;
  runs: Run[];
}

/** A spike check from POST /api/spike */
export interface SpikeCheck {
  Name: string;
  OK: boolean;
  Detail: string;
  BestEffort: boolean;
}

export interface SpikeResponse {
  ok: boolean;
  allOk: boolean;
  checks: SpikeCheck[];
}

// ---------------------------------------------------------------------------
// fetchJSON — base wrapper
// ---------------------------------------------------------------------------

class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function fetchJSON<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const res = await fetch(path, {
    headers: {
      "Content-Type": "application/json",
      ...(options?.headers ?? {}),
    },
    ...options,
  });
  if (!res.ok) {
    throw new ApiError(res.status, `HTTP ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}

// ---------------------------------------------------------------------------
// API functions
// ---------------------------------------------------------------------------

export function getHealth(): Promise<{ ok: boolean }> {
  return fetchJSON("/api/health");
}

export function listContainers(): Promise<ListContainersResponse> {
  return fetchJSON("/api/containers");
}

export function backupNow(name: string): Promise<BackupResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/backup`, {
    method: "POST",
  });
}

export function listSnapshots(name: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/snapshots`);
}

export function restore(
  name: string,
  snapshotId: string,
  confirm: boolean
): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, confirm }),
  });
}

export function setInclude(
  name: string,
  includeInSchedule: boolean
): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ includeInSchedule }),
  });
}

export function getSettings(): Promise<GetSettingsResponse> {
  return fetchJSON("/api/settings");
}

export function putSettings(settings: Settings): Promise<OkEnvelope> {
  return fetchJSON("/api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function runSpike(): Promise<SpikeResponse> {
  return fetchJSON("/api/spike", { method: "POST" });
}

export function listRuns(): Promise<ListRunsResponse> {
  return fetchJSON("/api/runs");
}
