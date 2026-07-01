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

/**
 * Response from the single-backup start endpoints (container/VM/flash). The
 * backup is now ASYNC: the server fires it in the background and answers
 * immediately, so the response only acknowledges acceptance — it carries NO
 * synchronous snapshot id. `started` is true once the job is running; on a
 * conflict it is `{ok:false, error:"a backup is already running"}`. The button
 * watches SSE progress + the recorded run for the real outcome.
 */
export interface BackupResponse extends OkEnvelope {
  started?: boolean;
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
  restoreFolder: string;
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
  offsiteRetentionKeepLast: number;
  offsiteRetentionKeepDaily: number;
  offsiteRetentionKeepWeekly: number;
  offsiteRetentionKeepMonthly: number;
  offsiteLimitUpload: number;
  offsiteLimitDownload: number;
  metricsEnabled: boolean;
  metricsToken: string;
  drillsEnabled: boolean;
  drillsSchedule: string;
  drillsSubsetPct: number;
  /** True once the user has downloaded + safely stored the encryption recovery
   *  kit, which dismisses the dashboard nag. */
  recoveryKitAck: boolean;
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

/** Per-domain RPO (protection) status from GET /api/status. */
export interface DomainStatus {
  domain: string; // "containers" | "vms" | "flash"
  enabled: boolean;
  schedule: string;
  lastSuccess: number; // unix seconds; 0 = never
  periodSeconds: number; // expected RPO window; 0 = no expectation
  status: string; // "off" | "never" | "overdue" | "warn" | "ok"
  lastVerified: number; // unix seconds of the last restore-verification drill; 0 = never
  lastVerifiedOK: boolean; // whether that last drill passed
}

/** A recorded restore-verification "drill" from POST/GET /api/verify. */
export interface RestoreDrill {
  domain: string;
  source: string;
  at: number; // unix seconds the drill ran
  ok: boolean; // true when the checked data was intact
  detail: string; // short scrubbed reason on failure; empty on success
}

export interface StatusResponse {
  ok: boolean;
  domains?: DomainStatus[];
  error?: string;
}

/** Per-domain backup outcome counts for a single calendar day. */
export interface DayStat {
  ok: number;
  failed: number;
}

/** One calendar day's backup outcomes split by domain, from GET /api/history. */
export interface HistoryDay {
  date: string; // local YYYY-MM-DD
  containers: DayStat;
  vms: DayStat;
  flash: DayStat;
}

export interface HistoryResponse {
  ok: boolean;
  days?: HistoryDay[];
  error?: string;
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

export function getHealth(): Promise<{ ok: boolean; version?: string }> {
  return fetchJSON("/api/health");
}

export function listContainers(): Promise<ListContainersResponse> {
  return fetchJSON("/api/containers");
}

/**
 * Start a single container backup. ASYNC: returns as soon as the server has
 * fired the job ({ok:true, started:true}); the backup runs detached on the
 * server (surviving this connection dying), so the caller must WATCH SSE
 * progress + the recorded run for the outcome instead of awaiting completion.
 */
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

/** Response from POST /api/containers/{name}/restore-files */
export interface RestoreFilesResponse extends OkEnvelope {
  /** Alternate-folder restore: the resolved target folder; "" for an in-place restore. */
  target?: string;
}

/**
 * POST /api/containers/{name}/restore-files — restore selected files/dirs from a
 * snapshot. An empty targetPath restores them in place (original locations); a
 * relative subpath extracts the selection into that folder under the host mount
 * (non-destructive: the running container is never touched).
 */
export function restoreContainerFiles(
  name: string,
  snapshotId: string,
  paths: string[],
  targetPath: string,
  confirm: boolean,
  source?: string
): Promise<RestoreFilesResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore-files${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, paths, targetPath, confirm }),
  });
}

/** Response from POST /api/containers/{name}/restore-to */
export interface RestoreToResponse extends OkEnvelope {
  /** The resolved target folder the snapshot was extracted into. */
  target?: string;
}

/**
 * POST /api/containers/{name}/restore-to — extract a whole snapshot into an
 * ALTERNATE folder under the host mount (non-destructive: the running container
 * is never touched). targetPath is a relative subpath under the host mount.
 */
export function restoreContainerToPath(
  name: string,
  snapshotId: string,
  targetPath: string,
  source?: string
): Promise<RestoreToResponse> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore-to${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, targetPath }),
  });
}

/** Summary of what changed between two snapshots, from GET /api/containers/{name}/diff */
export interface SnapshotDiff {
  addedFiles: number;
  removedFiles: number;
  changedFiles: number;
  addedBytes: number;
  removedBytes: number;
}

/**
 * GET /api/containers/{name}/diff?from=&to= — compare two snapshots and return a
 * summary of what changed between them (restic diff).
 */
export function diffSnapshots(
  name: string,
  from: string,
  to: string,
  source?: string
): Promise<{ ok: boolean; diff?: SnapshotDiff; error?: string }> {
  const qs = `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}${srcParam(source, "&")}`;
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/diff${qs}`);
}

/** POST /api/containers/{name}/tag — add tags to a snapshot (restic tag --add). */
export function tagSnapshot(
  name: string,
  snapshotId: string,
  tags: string[],
  source?: string
): Promise<{ ok: boolean; error?: string }> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/tag${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, tags }),
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

/** Notification config (webhook / Matrix / Healthchecks / Unraid / email). */
export interface NotifyConfig {
  on: string; // "never" | "failure" | "always"
  webhookUrl: string;
  webhookFormat: string; // generic | discord | slack | gotify | ntfy
  matrixHomeserver: string;
  matrixToken: string;
  matrixRoom: string;
  healthchecksUrl: string;
  unraid: boolean;
  smtpEnabled: boolean;
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  smtpPassword: string;
  smtpFrom: string;
  smtpTo: string;
  smtpTls: string; // "starttls" | "tls" | "none"
}

export interface GetNotifyResponse extends OkEnvelope {
  notify?: NotifyConfig;
  // The SMTP password / Matrix token are never sent to the browser; these flags
  // report whether one is stored so the form can show "configured" and treat a
  // blank submit as "keep the stored secret".
  smtpPasswordSet?: boolean;
  matrixTokenSet?: boolean;
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

/**
 * POST /api/containers/schedule-include — one-click set the "include in schedule"
 * flag for EVERY installed container (true = include all, false = exclude all).
 */
export function setIncludeAll(include: boolean): Promise<OkEnvelope> {
  return fetchJSON("/api/containers/schedule-include", {
    method: "POST",
    body: JSON.stringify({ include }),
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

/**
 * GET /api/recovery-kit — URL that streams the encryption-key recovery kit as a
 * download (bombvault-recovery-kit.md). Used as a plain <a href> with download:
 * the GET carries the session cookie, so the browser saves the file. The kit
 * contains the master APP_KEY + the derived restic password — store it offline.
 */
export function recoveryKitUrl(): string {
  return "/api/recovery-kit";
}

/**
 * POST /api/recovery-kit/ack — record that the kit has been stored, dismissing
 * the dashboard nag. Flips only that one flag server-side, so it can't clobber
 * unrelated settings changes (unlike resubmitting the whole settings object).
 */
export function ackRecoveryKit(): Promise<OkEnvelope> {
  return fetchJSON("/api/recovery-kit/ack", { method: "POST" });
}

/** POST /api/check/{domain} — verify a domain's restic repo integrity. */
export function checkDomain(
  domain: "containers" | "vms" | "flash",
  source?: string
): Promise<OkEnvelope> {
  return fetchJSON(`/api/check/${domain}${srcParam(source)}`, { method: "POST" });
}

/**
 * POST /api/verify/{domain}?source= — run a restore-verification drill
 * (restic check --read-data-subset) that reads back real pack data to prove the
 * backup is restorable, and record the result. The drill can take a while.
 */
export function runDrill(
  domain: string,
  source?: string
): Promise<{ ok: boolean; drill?: RestoreDrill; error?: string }> {
  return fetchJSON(`/api/verify/${encodeURIComponent(domain)}${srcParam(source)}`, {
    method: "POST",
  });
}

/**
 * GET /api/verify?domain=&source=&limit= — the recorded restore-verification
 * drills for a domain + source (newest first), plus the latest one for the badge.
 */
export function getDrills(
  domain: string,
  source?: string,
  limit?: number
): Promise<{ ok: boolean; drills?: RestoreDrill[]; latest?: RestoreDrill | null; error?: string }> {
  const lim = limit ? `&limit=${limit}` : "";
  return fetchJSON(
    `/api/verify?domain=${encodeURIComponent(domain)}${srcParam(source, "&")}${lim}`
  );
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

/** GET /api/status — per-domain RPO (protection) status for the dashboard. */
export function getStatus(): Promise<StatusResponse> {
  return fetchJSON("/api/status");
}

/** GET /api/history?days=N — per-day backup outcomes for the health heatmap. */
export function getHistory(days?: number): Promise<HistoryResponse> {
  const qs = days ? `?days=${days}` : "";
  return fetchJSON(`/api/history${qs}`);
}

/**
 * One repository-size sample for a domain at a point in time.
 * `rawSize` is the physical (deduplicated + compressed) repo size, `restoreSize`
 * the logical bytes those snapshots would restore to, `snapshots` the count.
 */
export interface RepoStat {
  domain: string;
  source: string;
  at: number; // unix seconds
  rawSize: number;
  restoreSize: number;
  snapshots: number;
}

export interface StatsResponse {
  ok: boolean;
  stats?: RepoStat[];
  latest?: RepoStat | null;
  error?: string;
}

/** GET /api/stats?domain=&source=&limit= — repo-size history for the storage card. */
export function getStats(
  domain: string,
  source = "local",
  limit = 90
): Promise<StatsResponse> {
  const qs = `?domain=${encodeURIComponent(domain)}&source=${encodeURIComponent(source)}&limit=${limit}`;
  return fetchJSON(`/api/stats${qs}`);
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

/** Start a single VM backup. ASYNC — see backupNow. */
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

/**
 * POST /api/vms/schedule-include — one-click set the "include in schedule" flag
 * for EVERY known VM (true = include all, false = exclude all).
 */
export function setVMIncludeAll(include: boolean): Promise<OkEnvelope> {
  return fetchJSON("/api/vms/schedule-include", {
    method: "POST",
    body: JSON.stringify({ include }),
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

/**
 * POST /api/flash/backup — start a backup of the whole Unraid flash (/boot).
 * ASYNC — see backupNow.
 */
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
