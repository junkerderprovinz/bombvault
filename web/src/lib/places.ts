import {
  fetchJSON,
  type DefaultImpact,
  type DeploySnippetData,
  type KeptItem,
  type OkEnvelope,
  type SaveWarning,
  type TargetPreview,
} from "./api";

// Storage places and the domain rows built on them. Every write here answers
// with the usual envelope; a caller that saw ok announces the change with
// placesChanged(), so every card that lists places reads them again.

export type PlaceKind = "local" | "s3" | "rest" | "sftp" | "webdav" | "azure" | "rclone";
export type PlaceGroup = "cloud" | "self" | "here";
export type PlaceDomain = "containers" | "vms" | "flash" | "config" | "files";

/** The fixed order a place lists its folders in, as the server's places.Domains. */
export const PLACE_DOMAINS: PlaceDomain[] = ["containers", "vms", "flash", "config", "files"];

export interface CatalogField {
  key: string;
  secret?: boolean;
  /** An example value, never text to translate. */
  placeholder?: string;
  optional?: boolean;
}

export interface CatalogProvider {
  id: string;
  group: PlaceGroup;
  kind: PlaceKind;
  fields: CatalogField[];
  /** Folder picker roots relative to the host mount root; "" is the root itself. */
  pickRoots?: string[];
  /** The fixed answer to "Where is the device?"; absent when the form asks. */
  offPremises?: boolean;
  defaultPort?: number;
}

export interface PlaceUsage {
  homeDomains: string[];
  defaults: string[];
  copyDomains: string[];
  items: number;
  /** Snapshots its targets held at their last listing; they stay when the place goes. */
  copies: number;
  /** Domain paths and repositories the place's switches reach. */
  repositories: number;
}

export interface PlaceTestStatus {
  at: number;
  ok: boolean;
  error?: string;
  /** "test" for a test in this process, "run" for the copy runs to the place. */
  source: "test" | "run";
}

export interface PlaceCredsView {
  /** The place runs on the shared credentials rather than a set of its own. */
  shared: boolean;
  /** Non-secret values by catalog field key. */
  fields: Record<string, string>;
  /** Secret field keys that hold a value. */
  set: string[];
}

export interface Place {
  id: string;
  name: string;
  provider: string;
  kind: PlaceKind;
  base: string;
  folders: Record<string, string>;
  offPremises: boolean;
  storageClass: string;
  immutable: boolean;
  retentionKeepLast: number;
  retentionKeepDaily: number;
  retentionKeepWeekly: number;
  retentionKeepMonthly: number;
  limitUpload: number;
  limitDownload: number;
  growthBudgetGb: number;
  enabled: boolean;
  sortOrder: number;
  /** The credential set the place keeps its secrets in, "" for the shared credentials. */
  credsRef: string;
  usage: PlaceUsage;
  /** Domain to whether an address there already holds backups. */
  locked: Record<string, boolean>;
  /** The place is itself a repository, so it has no folders below it. */
  repository: boolean;
  lastTest?: PlaceTestStatus;
  creds: PlaceCredsView;
}

/** A domain path, target or named repository whose address fits no place. */
export interface UnplacedRow {
  /** "" for a domain's own path. */
  rowId: string;
  /** "" for a named repository, which no domain owns. */
  domain: string;
  role: "path" | "target" | "repository" | "direct";
  name: string;
  repo: string;
  /** The row's append-only flag; a domain path keeps it on its domain's primary row. */
  immutable: boolean;
  /** The row takes an append-only switch of its own. */
  protectable: boolean;
  /** Items whose backups lie there, for the question before append-only goes off. */
  items: number;
}

export type FolderState = "empty" | "repository" | "absent" | "error";

/** One thing a probe found out, as a translation key and its parameters. */
export interface ProbeFact {
  key: string;
  params?: Record<string, string>;
}

export interface ProbeError {
  code?: string;
  error: string;
}

export interface ProbeRequest {
  provider?: string;
  fields: Record<string, string>;
  /** Probes a stored place with its stored secrets under whatever the form holds. */
  placeId?: string;
}

export interface ProbeResult {
  /** The address the server built; absent while a bucket is still to be
   *  chosen, and for a new WebDAV place, whose remote is named when it is added. */
  base?: string;
  /** The form as the probe completed it, without secrets. */
  fields?: Record<string, string>;
  buckets?: string[];
  facts?: ProbeFact[];
  folders?: Record<string, FolderState>;
  repoIds?: Record<string, string>;
  errors?: Record<string, ProbeError>;
}

export interface CreatePlaceBody {
  provider: string;
  fields: Record<string, string>;
  name: string;
  /** Only for a provider that asks "Where is the device?". */
  offPremises?: boolean;
  folders?: Record<string, string>;
}

export interface PatchPlaceBody {
  name: string;
  offPremises: boolean;
  storageClass: string;
  immutable: boolean;
  retentionKeepLast: number;
  retentionKeepDaily: number;
  retentionKeepWeekly: number;
  retentionKeepMonthly: number;
  limitUpload: number;
  limitDownload: number;
  growthBudgetGb: number;
  enabled: boolean;
  /** The provider's form without its secrets, from which the server builds a new base. */
  address: Record<string, string>;
  /** The credential form; a secret left blank keeps the stored one. */
  fields: Record<string, string>;
  folders: Record<string, string>;
}

/** What keeps a place from being removed, as a place-in-use refusal names it. */
export interface PlaceHolders {
  homeDomains: string[];
  defaults: string[];
  items: { domain: string; key: string }[];
  directInUse: string[];
}

/** A refused place write, with the fields some codes carry. */
export type PlaceRefusal = OkEnvelope & {
  holders?: PlaceHolders;
  snapshots?: number;
  domains?: string[];
  probe?: OkEnvelope & ProbeResult;
};

/** One append-only verdict for the domain paths and copies at a place. */
export interface TamperVerdict {
  /** False where the kind cannot be probed; only a rest-server can. */
  testable?: boolean;
  /** Every domain path and copy there refused the delete. */
  protected?: boolean;
  /** What the server accepted, when it did. */
  detail?: string;
}

export interface DomainChip {
  placeId: string;
  targetId?: string;
  on: boolean;
  disabled: boolean;
  reason?: "off" | "creds-differ";
}

export interface ExceptionItem {
  identity: string;
  name: string;
  link: string;
}

export interface DomainRow {
  domain: PlaceDomain;
  homePlace: string;
  storedIn: string;
  /** Only after an explicit check; the row itself never lists a repository. */
  homeHasBackups?: boolean;
  chips: DomainChip[];
  exceptions: ExceptionItem[];
  paused: boolean;
  schedule: string;
  unreadable: boolean;
}

export interface HomePreview {
  mode: "home-place" | "default" | "home-move";
  placeId: string;
  homePlace: string;
  homeHasBackups: boolean;
  repoId?: string;
  creates?: "direct" | "repository";
  impact?: DefaultImpact;
  backups: number;
}

export interface CopiesPreview {
  placeId: string;
  on: boolean;
  targetId?: string;
  suffix?: string;
  skip: string[];
  enabled: boolean;
  newTarget?: TargetPreview;
  impact?: DefaultImpact;
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return fetchJSON(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) });
}

function placePath(id: string, rest = ""): string {
  return `/api/places/${encodeURIComponent(id)}${rest}`;
}

function domainPath(d: string, rest: string): string {
  return `/api/storage/domains/${encodeURIComponent(d)}${rest}`;
}

export function getPlacesCatalog(): Promise<OkEnvelope & { providers?: CatalogProvider[] }> {
  return fetchJSON("/api/places/catalog");
}

export function listPlaces(): Promise<OkEnvelope & { places?: Place[]; unplaced?: UnplacedRow[] }> {
  return fetchJSON("/api/places");
}

/** Tests a place before it is added or changed. The result is the whole answer. */
export function probePlace(req: ProbeRequest): Promise<OkEnvelope & ProbeResult> {
  return post("/api/places/probe", req);
}

export function createPlace(body: CreatePlaceBody): Promise<PlaceRefusal & { place?: Place }> {
  return post("/api/places", body);
}

export function patchPlace(
  id: string,
  body: Partial<PatchPlaceBody>
): Promise<PlaceRefusal & { place?: Place; warnings?: SaveWarning[] }> {
  return fetchJSON(placePath(id), { method: "PATCH", body: JSON.stringify(body) });
}

export function deletePlace(id: string): Promise<PlaceRefusal & { removedTargets?: number }> {
  return fetchJSON(placePath(id), { method: "DELETE" });
}

/** Probes every address the place stands for. The result is the whole answer. */
export function testPlace(id: string): Promise<OkEnvelope & ProbeResult> {
  return post(placePath(id, "/test"));
}

/** Sends a harmless delete to every domain path and switched-on copy at the
 *  place. A folder on this server is refused, since nothing can keep it from
 *  deletion. */
export function tamperTestPlace(id: string): Promise<PlaceRefusal & TamperVerdict> {
  return post(placePath(id, "/tamper-test"));
}

/** Puts a row without a place at this place. An empty rowId is the domain's own path. */
export function adoptRow(placeId: string, rowId: string, domain: string): Promise<PlaceRefusal & { place?: Place }> {
  return post(placePath(placeId, "/adopt"), { rowId, domain });
}

/** Switches append-only on a row without a place, named as adoptRow names it. */
export function setUnplacedAppendOnly(rowId: string, domain: string, immutable: boolean): Promise<PlaceRefusal> {
  return fetchJSON("/api/places/unplaced", { method: "PATCH", body: JSON.stringify({ rowId, domain, immutable }) });
}

/** The repository the place stands for in a domain, made when it is missing; "" is the domain path. */
export function ensurePlaceRepo(placeId: string, domain: string): Promise<PlaceRefusal & { repoId?: string }> {
  return post(placePath(placeId, "/repo"), { domain });
}

/** A one-time recipe for an append-only rest-server with one user for this
 *  BombVault. The password lives only in this answer. */
export function restServerRecipe(): Promise<OkEnvelope & { snippet?: DeploySnippetData }> {
  return fetchJSON("/api/places/rest-server-recipe");
}

export function getStorageDomains(): Promise<OkEnvelope & { domains?: DomainRow[] }> {
  return fetchJSON("/api/storage/domains");
}

export function previewDomainHome(d: string, placeId: string): Promise<PlaceRefusal & HomePreview> {
  return post(domainPath(d, "/home/preview"), { placeId });
}

export function setDomainHome(
  d: string,
  body: { placeId: string; expect: HomePreview; applyToOpen: boolean }
): Promise<PlaceRefusal & { preview?: HomePreview; reset?: string[]; kept?: KeptItem[] }> {
  return fetchJSON(domainPath(d, "/home"), { method: "PUT", body: JSON.stringify(body) });
}

export function previewDomainCopies(d: string, placeId: string, on: boolean): Promise<PlaceRefusal & CopiesPreview> {
  return post(domainPath(d, "/copies/preview"), { placeId, on });
}

export function setDomainCopies(
  d: string,
  body: { placeId: string; on: boolean; expect: CopiesPreview }
): Promise<PlaceRefusal & { preview?: CopiesPreview }> {
  return fetchJSON(domainPath(d, "/copies"), { method: "PUT", body: JSON.stringify(body) });
}

/** The window event sent after every place or domain-row write. */
export const PLACES_CHANGED = "bv:places-changed";

export function placesChanged(): void {
  window.dispatchEvent(new Event(PLACES_CHANGED));
}

export function subscribePlaces(onChange: () => void): () => void {
  window.addEventListener(PLACES_CHANGED, onChange);
  return () => window.removeEventListener(PLACES_CHANGED, onChange);
}
