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
  /** First non-empty IP from the container's network settings. Empty string when none. */
  ip: string;
  /** False for "orphan" rows: not installed on the host, but has backups. */
  installed: boolean;
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
  /** The resolved host mount root (e.g. "/host/user"), sourced from cfg.HostMountRoot. */
  hostMountRoot: string;
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

/** A single subdirectory entry from GET /api/browse */
export interface BrowseDirEntry {
  name: string;
  /** Relative path from HostMountRoot, e.g. "appdata/plex" */
  path: string;
}

/** Response from GET /api/browse?path=<subpath> */
export interface BrowseResponse {
  ok: boolean;
  /** The server's HostMountRoot (absolute path inside the container). */
  root?: string;
  /** The subpath that was listed (mirrors the ?path= query parameter). */
  path?: string;
  dirs?: BrowseDirEntry[];
  error?: string;
}

/** Response from GET /api/auth */
export interface AuthStatusResponse {
  ok: boolean;
  /** Whether authentication is currently enabled (a password has been set). */
  enabled: boolean;
  /** Whether the current request carries a valid session cookie. */
  authed: boolean;
}

/** Response from POST /api/auth/password */
export interface SetPasswordResponse extends OkEnvelope {
  /** Whether auth is now enabled after the change. */
  enabled?: boolean;
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

/** Rebuild the target list from the backup storage (disaster recovery after a fresh install). */
export function discover(): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON("/api/discover", { method: "POST" });
}

/** Delete ALL backups of a container and forget it from the store. */
export function deleteBackups(name: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/backups`, {
    method: "DELETE",
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

/** Re-run the host-integration probes fresh (the "Host Integration Check" button). */
export function runSpike(): Promise<SpikeResponse> {
  return fetchJSON("/api/spike", { method: "POST" });
}

/** Return the cached host-integration result (warmed at container startup) for an instant view. */
export function getSpike(): Promise<SpikeResponse> {
  return fetchJSON("/api/spike");
}

export function listRuns(): Promise<ListRunsResponse> {
  return fetchJSON("/api/runs");
}

/**
 * Lists the immediate subdirectories of <HostMountRoot>/<path>.
 * Pass an empty string (or omit) to list the mount root itself.
 */
export function browse(path: string = ""): Promise<BrowseResponse> {
  const qs = path ? `?path=${encodeURIComponent(path)}` : "";
  return fetchJSON(`/api/browse${qs}`);
}

// ---------------------------------------------------------------------------
// Auth API
// ---------------------------------------------------------------------------

/** GET /api/auth — returns current auth state (enabled, authed). */
export function getAuth(): Promise<AuthStatusResponse> {
  return fetchJSON("/api/auth");
}

/** POST /api/login — attempt password login; sets bv_session cookie on success. */
export function login(password: string): Promise<OkEnvelope> {
  return fetchJSON("/api/login", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

/** POST /api/logout — clears the bv_session cookie. */
export function logout(): Promise<OkEnvelope> {
  return fetchJSON("/api/logout", { method: "POST" });
}

/**
 * POST /api/auth/password — set or change the auth password.
 * Passing an empty string disables authentication.
 */
export function setAuthPassword(password: string): Promise<SetPasswordResponse> {
  return fetchJSON("/api/auth/password", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}
