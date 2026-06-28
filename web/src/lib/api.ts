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
  preHook: string;
  postHook: string;
  /** Other container names to stop during this container's backup. */
  stopContainers: string[];
  /** True for BombVault's own container: it can't be backed up (it would stop
   *  itself), so the UI hides its backup action and excludes it from "select all". */
  self?: boolean;
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
  error?: string;
}

/** A file/dir node inside a snapshot, from GET /api/containers/{name}/files */
export interface FileEntry {
  path: string;
  type: string;
  size: number;
}

export interface ListFilesResponse {
  ok: boolean;
  files: FileEntry[];
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
  containersOffsite: string;
  vmsOffsite: string;
  flashOffsite: string;
  containersOffsiteSchedule: string;
  vmsOffsiteSchedule: string;
  flashOffsiteSchedule: string;
  containersSchedule: string;
  vmsSchedule: string;
  flashSchedule: string;
  defaultLanguage: string;
  retentionKeepLast: number;
  retentionKeepDaily: number;
  retentionKeepWeekly: number;
  retentionKeepMonthly: number;
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
  target: string; // human target name (container/VM name, or "Unraid flash")
  domain: string; // "container" | "vm" | "flash" | ""
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

export class ApiError extends Error {
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

/**
 * Start a SERVER-SIDE batch backup of the given containers. The work runs on the
 * server independently of this request, so closing the browser (even stopping
 * the container the UI runs in) won't interrupt it; watch progress over SSE
 * ("batch:containers" + per-container keys). Throws ApiError(409) if a batch is
 * already running.
 */
export function backupAll(names: string[]): Promise<OkEnvelope & { started?: number }> {
  return fetchJSON("/api/containers/backup-all", {
    method: "POST",
    body: JSON.stringify({ names }),
  });
}

/** Query suffix selecting the off-site repo when source==="offsite" (else local). */
export function srcParam(source?: string, sep: "?" | "&" = "?"): string {
  return source === "offsite" ? `${sep}source=offsite` : "";
}

export function listSnapshots(name: string, source?: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/snapshots${srcParam(source)}`);
}

export function restore(
  name: string,
  snapshotId: string,
  confirm: boolean,
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, confirm }),
  });
}

/** GET /api/containers/{name}/files?snapshot=<id> — list files in a snapshot. */
export function listSnapshotFiles(
  name: string,
  snapshot: string,
  source?: string
): Promise<ListFilesResponse> {
  return fetchJSON(
    `/api/containers/${encodeURIComponent(name)}/files?snapshot=${encodeURIComponent(snapshot)}${srcParam(source, "&")}`
  );
}

/** POST /api/containers/{name}/restore-file — restore one file to its origin. */
export function restoreContainerFile(
  name: string,
  snapshotId: string,
  path: string,
  confirm: boolean,
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore-file${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, path, confirm }),
  });
}

/** PATCH /api/containers/{name} — set pre/post-backup hook commands. */
export function setContainerHooks(
  name: string,
  preHook: string,
  postHook: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ preHook, postHook }),
  });
}

/** One bind mount of a container, annotated for the backup-folder selector. */
export interface MountInfo {
  source: string; // host path (shown to the user)
  dest: string; // in-container mount point
  selected: boolean; // currently included in the backup
  isAppdata: boolean; // auto-detected appdata default
  reachable: boolean; // reachable under the host mount (backable)
}

export interface ContainerMountsResponse extends OkEnvelope {
  mounts?: MountInfo[];
  custom?: string[];
}

/** GET /api/containers/{name}/mounts — list bind mounts + the current selection. */
export function getContainerMounts(name: string): Promise<ContainerMountsResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/mounts`);
}

/** PATCH /api/containers/{name} — set the explicit backup-folder selection (host paths). */
export function setBackupPaths(name: string, backupPaths: string[]): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ backupPaths }),
  });
}

/** Notification config (webhook / Matrix / Healthchecks). */
export interface NotifyConfig {
  on: string; // "never" | "failure" | "always"
  webhookUrl: string;
  webhookFormat: string; // generic | discord | slack | gotify | ntfy
  matrixHomeserver: string;
  matrixToken: string;
  matrixRoom: string;
  healthchecksUrl: string;
  unraid: boolean;
}

export interface GetNotifyResponse extends OkEnvelope {
  notify?: NotifyConfig;
}

/** GET /api/notify — the current (decrypted) notification config. */
export function getNotify(): Promise<GetNotifyResponse> {
  return fetchJSON("/api/notify");
}

/** POST /api/notify — store the notification config (encrypted at rest). */
export function setNotify(cfg: NotifyConfig): Promise<OkEnvelope> {
  return fetchJSON("/api/notify", { method: "POST", body: JSON.stringify(cfg) });
}

/** GET /api/cloud — cloud-backend credential summary (no secrets returned). */
export interface CloudInfo extends OkEnvelope {
  s3KeyId?: string;
  s3Region?: string;
  restUser?: string;
  s3SecretSet?: boolean;
  restPasswordSet?: boolean;
}
export function getCloud(): Promise<CloudInfo> {
  return fetchJSON("/api/cloud");
}

/** POST /api/cloud — store cloud-backend credentials (encrypted). Blank secret = keep. */
export interface CloudCreds {
  s3KeyId: string;
  s3Secret: string;
  s3Region: string;
  restUser: string;
  restPassword: string;
}
export function setCloud(c: CloudCreds): Promise<OkEnvelope> {
  return fetchJSON("/api/cloud", { method: "POST", body: JSON.stringify(c) });
}

/** POST /api/notify/test — send a test notification using the given config. */
export function testNotify(cfg: NotifyConfig): Promise<OkEnvelope> {
  return fetchJSON("/api/notify/test", { method: "POST", body: JSON.stringify(cfg) });
}

/** POST /api/containers/{name}/export — write a plain tar+xml export, returns the folder. */
export interface ExportResponse extends OkEnvelope {
  path?: string;
}
export function exportContainer(name: string): Promise<ExportResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/export`, { method: "POST" });
}

/** POST /api/vms/{name}/export — write a plain tar+xml export of a VM, returns the folder. */
export function exportVM(name: string): Promise<ExportResponse> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/export`, { method: "POST" });
}

/** PATCH /api/containers/{name} — set the other containers to stop during backup. */
export function setStopContainers(name: string, stopContainers: string[]): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ stopContainers }),
  });
}

/** Rebuild the target list from the backup storage (disaster recovery after a fresh install). */
export function discover(): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON("/api/discover", { method: "POST" });
}

/** Rebuild the VM target list from backup storage (restore a VM deleted from the host). */
export function discoverVMs(): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON("/api/vms/discover", { method: "POST" });
}

/** Delete ALL backups of a container and forget it from the store. */
export function deleteBackups(name: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/backups`, {
    method: "DELETE",
  });
}

/**
 * Delete ALL backups of a VM from the selected source (local or off-site) in one
 * go and prune the freed space. On the local source the VM is also forgotten from
 * the store; on off-site the target is kept (still restorable from local).
 */
export function deleteBackupsVM(name: string, source?: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/backups${srcParam(source)}`, {
    method: "DELETE",
  });
}

/** Clear a stale VM entry (its target row) without touching any repo — for a
 *  no-longer-installed VM that has no backups left to delete. */
export function forgetVM(name: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}`, {
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

/** POST /api/check/{domain} — verify a domain's restic repo integrity. */
export function checkDomain(
  domain: "containers" | "vms" | "flash",
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/check/${domain}${srcParam(source)}`, { method: "POST" });
}

/** POST /api/unlock/{domain} — clear stale repository locks (restic unlock). */
export function unlockDomain(
  domain: "containers" | "vms" | "flash",
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/unlock/${domain}${srcParam(source)}`, { method: "POST" });
}

/** POST /api/prune/{domain} — reclaim space from forgotten snapshots (restic prune). */
export function pruneDomain(
  domain: "containers" | "vms" | "flash",
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/prune/${domain}${srcParam(source)}`, { method: "POST" });
}

/** POST /api/offsite/{domain} — replicate a domain's local repo to its off-site repo now. */
export function replicateOffsite(
  domain: "containers" | "vms" | "flash"
): Promise<OkEnvelope> {
  return fetchJSON(`/api/offsite/${domain}`, { method: "POST" });
}

/** DELETE /api/snapshots/{domain}/{id} — forget a single snapshot. */
export function deleteSnapshot(
  domain: "containers" | "vms" | "flash",
  id: string,
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/snapshots/${domain}/${encodeURIComponent(id)}${srcParam(source)}`, { method: "DELETE" });
}

/** GET /api/rclone — configured rclone remote names (never secrets). */
export function getRclone(): Promise<OkEnvelope & { remotes?: string[] }> {
  return fetchJSON("/api/rclone");
}

/** POST /api/rclone — store the rclone config (encrypted). Empty conf clears it. */
export function setRclone(conf: string): Promise<OkEnvelope> {
  return fetchJSON("/api/rclone", {
    method: "POST",
    body: JSON.stringify({ conf }),
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
// VM API types — match VMView in internal/api/service.go exactly
// ---------------------------------------------------------------------------

/** A VM row from GET /api/vms */
export interface VM {
  name: string;
  state: string;
  /** Backup method — currently always "graceful". */
  method: string;
  includeInSchedule: boolean;
  lastBackup: number | null;
}

export interface ListVMsResponse {
  ok: boolean;
  vms: VM[];
}

// ---------------------------------------------------------------------------
// VM API functions
// ---------------------------------------------------------------------------

export function listVMs(): Promise<ListVMsResponse> {
  return fetchJSON("/api/vms");
}

export function backupVMNow(name: string): Promise<BackupResponse> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/backup`, {
    method: "POST",
  });
}

export function listVMSnapshots(name: string, source?: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/snapshots${srcParam(source)}`);
}

export function restoreVM(
  name: string,
  snapshotId: string,
  confirm: boolean,
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, confirm }),
  });
}

export function setVMInclude(
  name: string,
  includeInSchedule: boolean
): Promise<OkEnvelope> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ includeInSchedule }),
  });
}

/** PATCH /api/vms/{name} — set the backup method ("graceful" | "live"). */
export function setVMMethod(name: string, method: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ method }),
  });
}

/** GET /api/vm/ssh — the libvirt SSH host + BombVault's public key to authorize. */
export function getVMSSH(): Promise<
  OkEnvelope & { host?: string; publicKey?: string }
> {
  return fetchJSON("/api/vm/ssh");
}

/** POST /api/vm/ssh/test — check libvirt is reachable over SSH. */
export function testVMSSH(): Promise<OkEnvelope> {
  return fetchJSON("/api/vm/ssh/test", { method: "POST" });
}

// ---------------------------------------------------------------------------
// Flash API (singleton domain — the Unraid USB)
// ---------------------------------------------------------------------------

/** POST /api/flash/backup — back up the whole Unraid flash (/boot). */
export function backupFlashNow(): Promise<BackupResponse> {
  return fetchJSON("/api/flash/backup", { method: "POST" });
}

/** GET /api/flash/snapshots — list flash snapshots. */
export function listFlashSnapshots(source?: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/flash/snapshots${srcParam(source)}`);
}

/**
 * GET /api/flash/download — URL that streams a flash snapshot as a zip download
 * (restic dump). Used as a plain <a> link: the GET carries the session cookie,
 * so the browser downloads flash-<id>.zip. Non-destructive; the live /boot is
 * never touched, and a zip drops straight into the Unraid USB creator.
 */
export function flashDownloadURL(snapshotId: string, source?: string): string {
  return `/api/flash/download?snapshot=${encodeURIComponent(snapshotId)}${srcParam(source, "&")}`;
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
