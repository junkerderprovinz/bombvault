import type { TranslationKey, useT } from "./i18n";
import { placementErrorText } from "./placementCodes";
import type { FolderState, PlaceHolders, PlaceKind, PlaceRefusal, ProbeFact } from "./places";

type T = ReturnType<typeof useT>["t"];

/** keyOf reads a table of keys without reaching Object.prototype, so an id
 *  such as "constructor" counts as one this version does not know. */
function keyOf(table: Record<string, TranslationKey>, id: string): TranslationKey | undefined {
  return Object.hasOwn(table, id) ? table[id] : undefined;
}

const DOMAIN_KEYS: Record<string, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  flash: "nav.flash",
  config: "nav.config",
  files: "nav.files",
};

/** domainNames joins domain names the way the language lists things. */
export function domainNames(t: T, lang: string, domains: string[]): string {
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(domains.map((d) => domainName(t, d)));
}

export function domainName(t: T, domain: string): string {
  const key = keyOf(DOMAIN_KEYS, domain);
  return key ? t(key) : domain;
}

const CODE_KEYS: Record<string, TranslationKey> = {
  "place-name-taken": "places.error.nameTaken",
  "place-home-domain": "places.error.homeDomain",
  "place-is-repository": "places.error.isRepository",
  "place-domain-unavailable": "places.error.domainUnavailable",
  "place-address-taken": "places.error.addressTaken",
  "place-off": "places.error.off",
  "place-no-append-only": "places.error.noAppendOnly",
};

function holdersText(t: T, lang: string, h: PlaceHolders): string {
  const parts: string[] = [];
  if (h.homeDomains.length > 0) parts.push(t("places.holder.home").replace("{domains}", domainNames(t, lang, h.homeDomains)));
  if (h.defaults.length > 0) parts.push(t("places.holder.default").replace("{domains}", domainNames(t, lang, h.defaults)));
  if (h.items.length > 0) parts.push(t("places.holder.items", h.items.length));
  if (h.directInUse.length > 0) parts.push(t("places.holder.direct", h.directInUse.length));
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(parts);
}

/** probeErrorText is why a probe or one of its folders failed, in words: the
 *  translated code where the placement table knows it (direct-access-denied),
 *  the server's message otherwise. place-probe-failed itself only says that it
 *  failed. */
export function probeErrorText(t: T, lang: string, res: { code?: string; error?: string }): string {
  const code = res.code === "place-probe-failed" ? undefined : res.code;
  return placementErrorText(t, lang, { ok: false, code, error: res.error }, "common.actionFailed");
}

/** placeErrorText is the sentence a refused place write shows. Codes this
 *  table does not know go to the placement table, then to the server's text. */
export function placeErrorText(t: T, lang: string, res: PlaceRefusal, fallback: TranslationKey): string {
  switch (res.code) {
    // Only a removal names what holds the place. Any other write refused this
    // way would have left a domain without its folder there.
    case "place-in-use":
      if (res.holders) return t("places.error.inUse").replace("{holders}", holdersText(t, lang, res.holders));
      return t("places.error.folderInUse");
    case "place-location-established":
      return t("places.error.locationEstablished", res.snapshots ?? 0).replace(
        "{domains}",
        domainNames(t, lang, res.domains ?? [])
      );
    case "place-probe-failed":
      return probeFailureText(t, lang, res.probe ?? res);
  }
  const key = res.code ? keyOf(CODE_KEYS, res.code) : undefined;
  if (key) return t(key);
  return placementErrorText(t, lang, res, fallback);
}

/** probeFailureText is the sentence for a probe or test answer with ok false. */
export function probeFailureText(t: T, lang: string, res: { code?: string; error?: string }): string {
  return t("places.error.probeFailed").replace("{reason}", probeErrorText(t, lang, res));
}

const FACT_KEYS: Record<string, TranslationKey> = {
  "places.probe.baseIsRepository": "places.probe.baseIsRepository",
  "places.probe.bucketsHidden": "places.probe.bucketsHidden",
  "places.probe.bucketNew": "places.probe.bucketNew",
  "places.probe.b2Bucket": "places.probe.b2Bucket",
  "places.probe.b2Prefix": "places.probe.b2Prefix",
};

/** probeFactText is one finding of a probe as a sentence, or null for a key
 *  this version does not know, which the caller leaves out. */
export function probeFactText(t: T, fact: ProbeFact): string | null {
  const key = keyOf(FACT_KEYS, fact.key);
  if (!key) return null;
  let text = t(key);
  for (const [name, value] of Object.entries(fact.params ?? {})) text = text.replace(`{${name}}`, value);
  return text;
}

const FOLDER_STATE_KEYS: Record<FolderState, TranslationKey> = {
  empty: "places.folderState.empty",
  repository: "places.folderState.repository",
  absent: "places.folderState.absent",
  error: "places.folderState.error",
};

export function folderStateText(t: T, state: FolderState): string {
  return t(FOLDER_STATE_KEYS[state]);
}

const PROVIDER_KEYS: Record<string, TranslationKey> = {
  b2: "places.provider.b2",
  s3: "places.provider.s3",
  r2: "places.provider.r2",
  wasabi: "places.provider.wasabi",
  "hetzner-os": "places.provider.hetzner-os",
  storj: "places.provider.storj",
  idrive: "places.provider.idrive",
  scaleway: "places.provider.scaleway",
  ovh: "places.provider.ovh",
  digitalocean: "places.provider.digitalocean",
  ionos: "places.provider.ionos",
  contabo: "places.provider.contabo",
  exoscale: "places.provider.exoscale",
  vultr: "places.provider.vultr",
  gcs: "places.provider.gcs",
  azure: "places.provider.azure",
  storagebox: "places.provider.storagebox",
  minio: "places.provider.minio",
  seaweedfs: "places.provider.seaweedfs",
  garage: "places.provider.garage",
  ceph: "places.provider.ceph",
  juicefs: "places.provider.juicefs",
  rustfs: "places.provider.rustfs",
  versitygw: "places.provider.versitygw",
  "s3-other": "places.provider.s3-other",
  nextcloud: "places.provider.nextcloud",
  owncloud: "places.provider.owncloud",
  opencloud: "places.provider.opencloud",
  "rest-server": "places.provider.rest-server",
  sftp: "places.provider.sftp",
  bombvault: "places.provider.bombvault",
  rclone: "places.provider.rclone",
  synology: "places.provider.synology",
  qnap: "places.provider.qnap",
  truenas: "places.provider.truenas",
  "unraid-other": "places.provider.unraid-other",
  share: "places.provider.share",
  "unraid-folder": "places.provider.unraid-folder",
};

/** providerName is a provider's name on its tile, its own id for one this
 *  version does not know. */
export function providerName(t: T, id: string): string {
  const key = keyOf(PROVIDER_KEYS, id);
  return key ? t(key) : id;
}

const KIND_KEYS: Record<PlaceKind, TranslationKey> = {
  local: "places.kind.local",
  s3: "places.kind.s3",
  rest: "places.kind.rest",
  sftp: "places.kind.sftp",
  webdav: "places.kind.webdav",
  azure: "places.kind.azure",
  rclone: "places.kind.rclone",
};

export function kindName(t: T, kind: PlaceKind): string {
  return t(KIND_KEYS[kind]);
}
