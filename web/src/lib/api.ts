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
  lastBackupStarted: number | null;
  preHook: string;
  postHook: string;
  /** Other container names to stop during this container's backup. */
  stopContainers: string[];
  /** restic --exclude patterns for this container's backup, one per line. */
  excludes: string[];
  /** Opt-in (#52): after a successful backup, pull the image and recreate the
   *  container if a newer image is available. Off by default. */
  updateAfterBackup?: boolean;
  /** Compose project (com.docker.compose.project label) this container belongs to,
   *  "" if none. Drives the "restore whole stack" panel. */
  stack: string;
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
  configEnabled: boolean;
  filesEnabled: boolean;
  containersPath: string;
  vmsPath: string;
  flashPath: string;
  configPath: string;
  filesPath: string;
  restoreFolder: string;
  // Flash zip export (#28): after each flash backup, also write the snapshot out
  // as a plain .zip to this folder for off-server sync. Keep=0 → a single
  // flash-latest.zip that's overwritten; Keep=N → keep the newest N timestamped
  // flash-<date>.zip files.
  flashZipExportEnabled: boolean;
  flashZipExportPath: string;
  flashZipExportKeep: number;
  containersOffsite: string;
  vmsOffsite: string;
  flashOffsite: string;
  configOffsite: string;
  filesOffsite: string;
  containersOffsiteSchedule: string;
  vmsOffsiteSchedule: string;
  flashOffsiteSchedule: string;
  configOffsiteSchedule: string;
  filesOffsiteSchedule: string;
  containersSchedule: string;
  vmsSchedule: string;
  flashSchedule: string;
  configSchedule: string;
  filesSchedule: string;
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
  /** Scheduled off-site DR drill; default on. When off, only the manual
   *  off-site DR button runs (the free local integrity check still runs). */
  offsiteDrillsEnabled: boolean;
  drillsSchedule: string;
  drillsSubsetPct: number;
  /** True once the user has downloaded + safely stored the encryption recovery
   *  kit, which dismisses the dashboard nag. */
  recoveryKitAck: boolean;
  // Ransomware protection (v4): per-domain "off-site repo is append-only
  // (immutable)" flags — BombVault then skips its own off-site prune and refuses
  // off-site deletes; the far side (rest-server --append-only) enforces it.
  containersOffsiteImmutable: boolean;
  vmsOffsiteImmutable: boolean;
  flashOffsiteImmutable: boolean;
  configOffsiteImmutable: boolean;
  filesOffsiteImmutable: boolean;
  /** Off-site growth budget in GB (0 = alarm off). */
  offsiteGrowthBudgetGB: number;
  /** Cadence for the scheduled off-site tamper test (default "weekly Sun 04:30"). */
  tamperTestSchedule: string;
  /** DR-drill target container ("" = auto: the most recently backed-up container). */
  drDrillTarget: string;
  /** #56: after a post-backup container update, remove the superseded old image.
   *  Opt-in (default off) — keeping the old image makes a snapshot rollback cheap. */
  pruneImageAfterUpdate: boolean;
}

export interface GetSettingsResponse {
  ok: boolean;
  settings: Settings;
  /** The resolved host mount root (e.g. "/host/user"), sourced from cfg.HostMountRoot. */
  hostMountRoot: string;
  /** Present only on the graceful failure envelope ({ok:false} at HTTP 200). */
  error?: string;
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
  target: string; // human target name (container/VM/file-set name, or "Unraid flash")
  domain: string; // "container" | "vm" | "flash" | "files" | ""
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
  verifiedDetail: string; // scrubbed reason of the last LOCAL subset drill; "" on success
  drillDetail: string; // scrubbed reason of the last OFF-SITE DR drill; "" on success
  // Ransomware-protection scorecard facts (v4). Protection is the red/amber/green
  // aggregate; "" for a disabled domain (the dashboard card renders nothing for it).
  offsiteConfigured: boolean; // an off-site repo is configured for this domain
  offsiteImmutable: boolean; // the off-site repo is flagged append-only (immutable)
  lastTamperAt: number; // unix seconds of the last off-site tamper test; 0 = never
  lastTamperOK: boolean; // whether that test proved append-only protection
  lastReplicationAt: number; // unix seconds of the last off-site replication; 0 = never
  lastReplicationOK: boolean; // whether that replication succeeded
  lastDrDrillAt: number; // unix seconds of the last off-site DR drill; 0 = never
  lastDrDrillOK: boolean; // whether that DR drill passed
  // Latest OFF-SITE SUBSET drill (integrity check against the off-site repo) —
  // the only off-site drill VMs can run. Drives the "off-site verified" badge (#63).
  lastOffsiteSubsetAt: number; // unix seconds; 0 = never
  lastOffsiteSubsetOK: boolean; // whether that check passed
  // Whether the scheduled off-site DR drill is active (DrillsEnabled &&
  // OffsiteDrillsEnabled && offsiteConfigured). When false but offsiteConfigured,
  // the dashboard shows a neutral "manual only" pill instead of a red failure.
  offsiteDrillScheduled: boolean;
  protection: string; // "" (disabled) | "red" | "amber" | "green"
  // Per-check states derived server-side from the SAME inputs as `protection`, so
  // the dashboard card renders each row as a pure function of the backend and can
  // never contradict the chip. encryptionOn/pruneStrategySet are the two config
  // facts the card also renders (no separate /api/settings round-trip needed).
  tamperState: string; // "" | "never" | "failed" | "stale" | "ok"
  replicationState: string; // "" | "never" | "overdue" | "ok"
  drillState: string; // "" | "never" | "failed" | "overdue" | "ok"
  encryptionOn: boolean; // repo encryption is enabled
  pruneStrategySet: boolean; // an off-site retention strategy is configured
}

/** A recorded restore-verification "drill" from POST/GET /api/verify. */
export interface RestoreDrill {
  domain: string;
  source: string;
  at: number; // unix seconds the drill ran
  ok: boolean; // true when the checked data was intact
  detail: string; // short scrubbed reason on failure; empty on success
  // Drill flavour: "subset" (restic check --read-data-subset) or "dr" (a real
  // off-site sandbox restore). Distinguishes an off-site DR check from a local
  // subset integrity check (which can also run against the off-site repo).
  kind: string;
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
  config: DayStat;
  files: DayStat;
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

/**
 * Poll the lightest endpoint (GET /api/health) until BombVault is reachable
 * again after it restarts itself to apply a config restore (Recovery tab). It
 * resolves `true` once health answers 200 — by default only AFTER it has first
 * been seen unreachable, so a poll that starts before the old process dies
 * doesn't return prematurely — and `false` once `timeoutMs` elapses. Pure and
 * testable: all timing knobs are parameters, and it uses raw `fetch` (not
 * getHealth) so a failing request is caught rather than thrown.
 */
export async function waitForAppBack(opts?: {
  timeoutMs?: number;
  intervalMs?: number;
  requireDownFirst?: boolean;
}): Promise<boolean> {
  const timeoutMs = opts?.timeoutMs ?? 180000;
  const intervalMs = opts?.intervalMs ?? 2000;
  const requireDownFirst = opts?.requireDownFirst ?? true;
  const deadline = Date.now() + timeoutMs;
  // When requireDownFirst is false, treat the app as already "seen down" so the
  // first successful probe resolves immediately.
  let sawDown = !requireDownFirst;
  while (Date.now() < deadline) {
    try {
      const res = await fetch("/api/health", { cache: "no-store" });
      if (res.ok) {
        if (sawDown) return true;
        // Reachable but we never saw it go down yet — still the pre-restart
        // process; keep polling until it drops, then comes back.
      } else {
        sawDown = true; // a non-2xx (e.g. 502 from the proxy) means it's cycling
      }
    } catch {
      sawDown = true; // network error → the container is down/restarting
    }
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  return false;
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

/**
 * Start an in-place container restore. ASYNC (like backupNow): validation runs
 * synchronously (a bad request still answers ok:false right away), then the
 * server returns {ok:true, started:true} and the restore runs detached — it
 * survives this connection dying, so a multi-hour restore can't be killed by
 * the browser/proxy dropping the request. Watch the "container:<name>" SSE
 * progress key + the recorded run (kind "restore") for the outcome.
 */
export function restore(
  name: string,
  snapshotId: string,
  confirm: boolean,
  source?: string,
  leaveStopped?: boolean
): Promise<OkEnvelope & { started?: boolean }> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, confirm, leaveStopped }),
  });
}

/**
 * POST /api/restore/cancel {key} — cancel an in-flight restore by its progress
 * key ("container:<name>" / "vm:<name>" / "stack:<project>"). Cancelling an
 * unknown or already-finished key is a no-op success ({ok:true,cancelled:false}).
 * A user-cancelled restore records the run as "cancelled" (distinct from
 * "failed") and fires no failure alert.
 */
export function cancelRestore(key: string): Promise<{ ok: boolean; cancelled: boolean }> {
  return fetchJSON("/api/restore/cancel", {
    method: "POST",
    body: JSON.stringify({ key }),
  });
}

/**
 * POST /api/stacks/{project}/restore — restore every backed-up container in a
 * compose stack from its latest backup, left stopped; when startAfter is true they
 * are then started in dependency order.
 *
 * ASYNC (see restore): validation + member enumeration run synchronously (a bad
 * request — including an empty stack — still answers ok:false right away), then
 * the server returns {ok:true, started:true} and the per-member loops run
 * detached. The ack carries NO member results; each member's restore records a
 * kind "restore" run, so outcomes live in the run history.
 */
export function restoreStack(
  project: string,
  startAfter: boolean,
  confirm: boolean,
  source?: string
): Promise<OkEnvelope & { started?: boolean }> {
  return fetchJSON(`/api/stacks/${encodeURIComponent(project)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ startAfter, confirm }),
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
  /** True once the server accepted the job and is running it detached. */
  started?: boolean;
  /** Alternate-folder restore: the resolved target folder; "" for an in-place restore. */
  target?: string;
}

/**
 * POST /api/containers/{name}/restore-files — restore selected files/dirs from a
 * snapshot. An empty targetPath restores them in place (original locations); a
 * relative subpath extracts the selection into that folder under the host mount
 * (non-destructive: the running container is never touched).
 *
 * ASYNC (see restore): the ack carries the resolved target; the restic work runs
 * detached — watch SSE progress + the recorded run for the outcome.
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
  /** True once the server accepted the job and is running it detached. */
  started?: boolean;
  /** The resolved target folder the snapshot is extracted into. */
  target?: string;
}

/**
 * POST /api/containers/{name}/restore-to — extract a whole snapshot into an
 * ALTERNATE folder under the host mount (non-destructive: the running container
 * is never touched). targetPath is a relative subpath under the host mount.
 *
 * ASYNC (see restore — this is THE flow that died on multi-hour extractions):
 * the ack carries the resolved target; the restic work runs detached — watch
 * SSE progress + the recorded run for the outcome.
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
  // Optional per-domain Healthchecks ping URLs, keyed by domain ("container",
  // "VM", "flash", "config"). A set URL overrides healthchecksUrl for that
  // domain; a blank/absent entry falls back to the global healthchecksUrl.
  healthchecksByDomain?: Record<string, string>;
  unraid: boolean;
  smtpEnabled: boolean;
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  smtpPassword: string;
  smtpFrom: string;
  smtpTo: string;
  smtpTls: string; // "starttls" | "tls" | "none"
  // #56: collapse a scheduled per-domain run's per-item messages into one summary.
  scheduledSummary: boolean;
  // #56: notify per container updated by the post-backup image update.
  notifyOnUpdate: boolean;
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

/** PATCH /api/containers/{name} — toggle the post-backup image update (#52). */
export function setUpdateAfterBackup(name: string, updateAfterBackup: boolean): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ updateAfterBackup }),
  });
}

/** PATCH /api/containers/{name} — set this container's restic --exclude patterns. */
export function setContainerExcludes(name: string, excludes: string[]): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ excludes }),
  });
}

/**
 * POST /api/containers/{name}/excludes/preview — resolve candidate exclude lines
 * against the container's live mounts WITHOUT saving them. Each preview entry
 * reports the resolved `--exclude` pattern, how it was derived (`status`:
 * "basename" | "translated" | "passthrough"), and whether it would exclude
 * anything in this container's backup (`matches`) so the UI can warn on a line
 * that matches nothing.
 */
export function previewContainerExcludes(
  name: string,
  patterns: string[]
): Promise<{ ok: boolean; preview: { raw: string; resolved: string; status: string; matches: boolean }[] }> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}/excludes/preview`, {
    method: "POST",
    body: JSON.stringify({ patterns }),
  });
}

/**
 * Rebuild the target list from the backup storage (disaster recovery after a fresh
 * install). `probe` makes it READ-ONLY: it opens + decrypts the repo to prove it is
 * readable with the current APP_KEY and returns the same count, but writes no
 * targets — used by the Recovery readiness check so merely testing readability
 * never resurrects orphan entries (#44). The default rebuilds the targets.
 */
export function discover(probe = false): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON(`/api/discover${probe ? "?probe=true" : ""}`, { method: "POST" });
}

/** Rebuild the VM target list from backup storage. `probe` = read-only readiness check (see discover, #44). */
export function discoverVMs(probe = false): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON(`/api/vms/discover${probe ? "?probe=true" : ""}`, { method: "POST" });
}

/**
 * Runs both domain discovers (rebuild containers + VMs from the encrypted backup
 * defs in storage) and returns the counts. The caller then re-fetches
 * listContainers()/listVMs() to show the reconstructed targets.
 *
 * handleDiscover answers HTTP 200 with an {ok:false, error} envelope on a real
 * failure (e.g. a wrong APP_KEY), so a naive `discovered ?? 0` would silently
 * report "0 found" and hide the actual error. To avoid that, if EITHER discover
 * fails, its (scrubbed) message is surfaced as `error` — the caller shows the
 * real failure instead of a misleading "nothing to recover".
 */
export async function discoverAll(): Promise<{ containers: number; vms: number; error?: string }> {
  const [c, v] = await Promise.all([discover(), discoverVMs()]);
  const failed = [c, v].find((r) => !r.ok);
  return {
    containers: c.discovered ?? 0,
    vms: v.discovered ?? 0,
    ...(failed ? { error: failed.error ?? "discover failed" } : {}),
  };
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

/**
 * PUT /api/settings — persist the settings object. `warnings` is a
 * backward-compatible extension of the ok envelope: non-fatal advisories (e.g.
 * an off-site retention policy that is inert because the repo is append-only).
 */
export function putSettings(
  settings: Settings
): Promise<OkEnvelope & { warnings?: string[] }> {
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
 * POST /api/verify/{domain}?source=&kind= — run a restore-verification drill and
 * record the result. `kind` selects what runs:
 *   - "subset" (default): a `restic check --read-data-subset` integrity check on
 *     the selected `source` repo — reads back real pack data to prove the backup
 *     is intact. Works for all domains + both sources.
 *   - "dr": a REAL off-site sandbox restore (containers + flash only; VMs are
 *     refused server-side). It always runs against the off-site repo, so the
 *     `source` argument is ignored for this kind.
 * The drill can take a while (a DR restore downloads real data).
 */
export function runDrill(
  domain: string,
  source?: string,
  kind: "subset" | "dr" = "subset"
): Promise<{ ok: boolean; drill?: RestoreDrill; error?: string }> {
  const params = new URLSearchParams();
  if (source === "offsite") params.set("source", "offsite");
  if (kind === "dr") params.set("kind", "dr");
  const qs = params.toString();
  return fetchJSON(
    `/api/verify/${encodeURIComponent(domain)}${qs ? `?${qs}` : ""}`,
    { method: "POST" }
  );
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
  domain: "containers" | "vms" | "flash" | "config",
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

/**
 * POST /api/offsite/{domain}/test — probe the off-site repo without modifying it:
 * whether it is reachable and whether it is an initialised restic repository.
 */
export function testOffsite(
  domain: "containers" | "vms" | "flash"
): Promise<OkEnvelope & { reachable?: boolean; initialized?: boolean }> {
  return fetchJSON(`/api/offsite/${domain}/test`, { method: "POST" });
}

/**
 * A one-time rest-server deployment recipe for a domain's append-only off-site
 * repo. The plaintext `password` is shown ONCE and never persisted server-side;
 * `htpasswd` is its bcrypt line for the far-side .htpasswd file. `dockerRun` and
 * `compose` are ready-to-paste recipes (with the echo pre-step + repo-URL hint).
 */
export interface DeploySnippetData {
  user: string;
  password: string;
  htpasswd: string;
  dockerRun: string;
  compose: string;
}

/**
 * GET /api/offsite/{domain}/deploy-snippet — generate a fresh rest-server
 * deployment recipe (docker run + compose + generated htpasswd credentials).
 * Nothing is stored server-side; the plaintext password lives only in this
 * response, so it must be saved by the user right away.
 */
export function deploySnippet(
  domain: "containers" | "vms" | "flash"
): Promise<OkEnvelope & { snippet?: DeploySnippetData }> {
  return fetchJSON(`/api/offsite/${domain}/deploy-snippet`);
}

/**
 * POST /api/offsite/{domain}/tamper-test — actively PROVE the far side refuses
 * deletes: side-effect-free authenticated DELETEs against provably non-existent
 * object IDs. `testable` is false for non-REST repos (rclone/S3/local can't be
 * probed this way); `protected` is true only when every probe was refused;
 * `detail` carries the scrubbed reason when not protected.
 */
export function tamperTest(
  domain: "containers" | "vms" | "flash"
): Promise<OkEnvelope & { testable?: boolean; protected?: boolean; detail?: string }> {
  return fetchJSON(`/api/offsite/${domain}/tamper-test`, { method: "POST" });
}

/** DELETE /api/snapshots/{domain}/{id} — forget a single snapshot. */
export function deleteSnapshot(
  domain: "containers" | "vms" | "flash" | "config" | "files",
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
  lastBackupStarted: number | null;
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

/** Start a VM restore. ASYNC — see restore; watch "vm:<name>" over SSE. */
export function restoreVM(
  name: string,
  snapshotId: string,
  confirm: boolean,
  source?: string,
  leaveStopped?: boolean
): Promise<OkEnvelope & { started?: boolean }> {
  return fetchJSON(`/api/vms/${encodeURIComponent(name)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, confirm, leaveStopped }),
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
// Config API (singleton domain — BombVault's OWN settings self-backup)
// ---------------------------------------------------------------------------

/**
 * POST /api/config/backup — start a backup of BombVault's own /config (settings
 * DB + rclone.conf + ssh keypair) so a rebuilt server can restore itself.
 * ASYNC — mirrors backupFlashNow: the server fires the job detached and answers
 * immediately; watch the "config" progress key + the recorded run for the outcome.
 */
export function backupConfigNow(): Promise<BackupResponse> {
  return fetchJSON("/api/config/backup", { method: "POST" });
}

/** GET /api/config/snapshots — list BombVault's own config self-backups. */
export function listConfigSnapshots(source?: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/config/snapshots${srcParam(source)}`);
}

/**
 * POST /api/config/restore — STAGE a restore of BombVault's own settings and
 * trigger the self-restart that applies it (the live SQLite DB can't be swapped
 * in-process). `staged` confirms the snapshot was written to the staging area;
 * `autoRestart` is true when a container restart was scheduled, false when Docker
 * was unreachable and the user must restart the container manually. On a restore
 * error the 200 body carries {ok:false, error} (an APP_KEY / encryption mismatch
 * surfaces as a scrubbed message). Consumed by the Recovery tab (Task 13).
 */
export function restoreConfig(
  snapshot: string,
  source?: string
): Promise<OkEnvelope & { staged?: boolean; autoRestart?: boolean }> {
  return fetchJSON("/api/config/restore", {
    method: "POST",
    body: JSON.stringify({ snapshot, source }),
  });
}

// ---------------------------------------------------------------------------
// Files API (file-set backup domain, #62) — matches FileSetView in
// internal/api/service.go exactly
// ---------------------------------------------------------------------------

/** A file-set row from GET /api/files. */
export interface FileSetView {
  id: string;
  name: string;
  /** Relative subpath under the host mount root (FolderBrowser convention).
   *  "" for a set rebuilt by Discover from tags alone (no path known). */
  path: string;
  /** restic --exclude patterns for this set's backup, one per line. */
  excludes: string[];
  /** Included in scheduled backups + "back up all". */
  enabled: boolean;
  /** Unix seconds of the last successful backup; 0 = never. */
  lastBackup: number;
  /** Whether the resolved source path currently exists on disk. */
  pathExists: boolean;
}

export interface ListFileSetsResponse {
  ok: boolean;
  fileSets: FileSetView[];
  error?: string;
}

/** GET /api/files — list all configured file sets. */
export function listFileSets(): Promise<ListFileSetsResponse> {
  return fetchJSON("/api/files");
}

/** POST /api/files/sets — create a file set (path required; validated server-side). */
export function createFileSet(set: {
  name: string;
  path: string;
  excludes: string[];
  enabled?: boolean;
}): Promise<OkEnvelope & { id?: string }> {
  return fetchJSON("/api/files/sets", {
    method: "POST",
    body: JSON.stringify(set),
  });
}

/** PATCH /api/files/sets/{id} — partial update; omitted fields keep their value. */
export function patchFileSet(
  id: string,
  patch: { name?: string; path?: string; excludes?: string[]; enabled?: boolean }
): Promise<OkEnvelope> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

/** DELETE /api/files/sets/{id} — remove the set entry (+ run history); its
 *  backups stay in the repo and can be resurfaced via discoverFiles. */
export function deleteFileSet(id: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/** DELETE /api/files/sets/{id}/backups — delete ALL backups of a file set
 *  (every fileset-tagged snapshot, pruned) and forget the set. */
export function deleteFileSetBackups(id: string): Promise<OkEnvelope> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}/backups`, {
    method: "DELETE",
  });
}

/** POST /api/files/sets/{id}/backup — start a single file-set backup.
 *  ASYNC — see backupNow; watch "files:<name>" over SSE + the recorded run. */
export function backupFileSet(id: string): Promise<BackupResponse> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}/backup`, {
    method: "POST",
  });
}

/** POST /api/files/backup-all — SERVER-SIDE batch backup of the given file sets
 *  (see backupAll; watch "batch:files" + per-set keys over SSE). */
export function backupFilesAll(ids: string[]): Promise<OkEnvelope & { started?: number }> {
  return fetchJSON("/api/files/backup-all", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

/** GET /api/files/sets/{id}/snapshots — list one file set's snapshots (tag-filtered). */
export function fileSetSnapshots(id: string, source?: string): Promise<ListSnapshotsResponse> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}/snapshots${srcParam(source)}`);
}

/**
 * POST /api/files/sets/{id}/restore — restore a file-set snapshot. An EMPTY
 * targetPath restores IN PLACE over the set's source folder (requires confirm;
 * refused for path-less discovered sets); a non-empty targetPath extracts the
 * snapshot into that folder under the host mount (non-destructive, FolderBrowser
 * convention). ASYNC (see restore): the ack carries the resolved target; watch
 * the "files:<name>" SSE key + the recorded run (kind "restore") for the outcome.
 */
export function restoreFileSet(
  id: string,
  snapshotId: string,
  confirm: boolean,
  targetPath = "",
  source?: string
): Promise<OkEnvelope & { started?: boolean; target?: string }> {
  return fetchJSON(`/api/files/sets/${encodeURIComponent(id)}/restore${srcParam(source)}`, {
    method: "POST",
    body: JSON.stringify({ snapshotId, targetPath, confirm }),
  });
}

/** Rebuild the file-set list from the fileset: tags in backup storage. `probe` =
 *  read-only readiness check (see discover, #44). Discovered sets arrive DISABLED
 *  with an empty path (tags alone don't carry it) — set a folder before backing up. */
export function discoverFiles(probe = false): Promise<OkEnvelope & { discovered?: number }> {
  return fetchJSON(`/api/files/discover${probe ? "?probe=true" : ""}`, { method: "POST" });
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
