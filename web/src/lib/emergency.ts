// What the emergency window says about the key of another install: the state
// of each place the key was tried on, why a step was refused, and what of the
// places goes into this server's settings.

import type {
  DepositRefusal,
  EmergencyPlace,
  EmergencyRefusal,
  EmergencySession,
  EmergencyState,
  ReleaseState,
  SealedLoss,
  Settings,
} from "./api";
import type { TranslationKey } from "./i18n";
import { DOMAIN_LABEL } from "./storageLocations";

type T = (key: TranslationKey, n?: number) => string;

/** A refusal of the emergency, of the request for a deposit in the group, or
 *  of the settings restore that follows. "release-decided" is left out: only
 *  the instance that keeps a deposit is told so. The two refusals for the
 *  session are said by useOwnLogin's sentences. */
export type EmergencyCode = EmergencyRefusal | Exclude<DepositRefusal, "release-decided"> | "settings-other-key";

const REFUSAL: Record<EmergencyCode, TranslationKey> = {
  "kit-sealed": "emergency.code.kit-sealed",
  "kit-identity": "emergency.code.kit-identity",
  "kit-not-for-identity": "emergency.code.kit-not-for-identity",
  "not-a-kit": "emergency.code.not-a-kit",
  "kit-key-broken": "emergency.code.kit-key-broken",
  "kit-no-key": "emergency.code.kit-no-key",
  "emergency-key": "emergency.code.emergency-key",
  "emergency-ended": "emergency.code.emergency-ended",
  "adopt-own-backups": "emergency.code.adopt-own-backups",
  "adopt-same-key": "emergency.code.adopt-same-key",
  "adopt-nothing-readable": "emergency.code.adopt-nothing-readable",
  "adopt-in-use": "emergency.code.adopt-in-use",
  "backup-key-locked": "emergency.code.backup-key-locked",
  "settings-other-key": "emergency.code.settings-other-key",
  "deposit-none": "emergency.code.deposit-none",
  "release-declined": "emergency.code.release-declined",
  "release-limit": "emergency.code.release-limit",
  "deposit-version": "emergency.code.deposit-version",
  "deposit-code-shape": "emergency.code.deposit-code-shape",
  "deposit-code": "emergency.code.deposit-code",
  "member-gone": "emergency.code.member-gone",
  "member-silent": "emergency.code.member-silent",
  "keeper-too-old": "emergency.code.keeper-too-old",
  "adopt-busy": "emergency.code.adopt-busy",
  "sign-ins-exist": "emergency.code.sign-ins-exist",
  "kit-ambiguous": "emergency.code.kit-ambiguous",
  "keeper-full": "deposit.code.keeper-full",
  "deposit-owner": "deposit.code.deposit-owner",
  "deposit-unreadable": "deposit.code.deposit-unreadable",
  "deposit-busy": "deposit.code.deposit-busy",
  "password-not-set": "secrets.passwordNotSet",
  "own-login-needed": "secrets.ownLoginNeeded",
};

/** refusalKey is the sentence of a refusal code this client knows. */
export function refusalKey(code: string): TranslationKey | undefined {
  return REFUSAL[code as EmergencyCode];
}

/** refusalText is the sentence of a refusal code. A code this client does not
 *  know shows what the server said. */
export function refusalText(t: T, code: string | undefined, said: string | undefined): string {
  const key = code ? REFUSAL[code as EmergencyCode] : undefined;
  return key ? t(key) : (said ?? t("settings.error"));
}

/** The states of a request for a deposit in which the person has to ask
 *  again, each with the sentence that says so. */
export const RELEASE_OVER: Partial<Record<ReleaseState | "unasked", TranslationKey>> = {
  expired: "emergency.release.expired",
  released: "emergency.release.released",
  unasked: "emergency.release.unasked",
};

const CODE_LENGTH = 20;
const CODE_GROUP = 5;

/** The characters of a typed recovery code that count: letters and digits,
 *  in upper case. The server sets the look-alikes right. */
function codeCharacters(typed: string): string {
  return typed.toUpperCase().replace(/[^0-9A-Z]/g, "");
}

/** recoveryCodeShown is a typed recovery code in its four groups of five,
 *  whatever spaces, dashes or case it was typed or pasted with. */
export function recoveryCodeShown(typed: string): string {
  const groups = codeCharacters(typed).slice(0, CODE_LENGTH).match(new RegExp(`.{1,${CODE_GROUP}}`, "g")) ?? [];
  return groups.join("-");
}

export function recoveryCodeComplete(typed: string): boolean {
  return codeCharacters(typed).length === CODE_LENGTH;
}

type PlaceState = EmergencyPlace["state"];

export const PLACE_LABEL: Record<PlaceState, TranslationKey> = {
  readable: "emergency.place.readable",
  missing: "emergency.place.missing",
  "wrong-key": "emergency.place.wrong-key",
  unreachable: "emergency.place.unreachable",
  "sign-in": "emergency.place.sign-in",
};

/** placeAdvice says what was tried on a place that did not open and what to
 *  do about it, undefined for one that is readable. */
export function placeAdvice(t: T, place: EmergencyPlace): string | undefined {
  switch (place.state) {
    case "readable":
      return undefined;
    case "missing":
      return place.folder ? t("emergency.place.missingDo").replace("{folder}", () => place.folder ?? "") : t("emergency.place.missingOutside");
    case "wrong-key":
      return t("emergency.place.wrong-keyDo");
    case "unreachable":
      return t("emergency.place.unreachableDo");
    case "sign-in":
      return t("emergency.place.sign-inDo");
  }
}

/** placeName is what a place is called: its section and whether it is the
 *  local or the off-site one, or the name the old install gave it. A typed
 *  place has neither and goes by its address. */
export function placeName(t: T, place: EmergencyPlace): string {
  const label = place.domain ? (DOMAIN_LABEL as Record<string, TranslationKey | undefined>)[place.domain] : undefined;
  const section = label ? t(label) : "";
  if (place.kind === "named" || place.kind === "destination") return [place.name, section].filter(Boolean).join(" · ");
  if (!section) return place.location;
  return `${section} · ${t(place.kind === "offsite" ? "source.offsite" : "source.local")}`;
}

const PATH_KEY = {
  containers: ["containersPath", "containersOffsite"],
  vms: ["vmsPath", "vmsOffsite"],
  flash: ["flashPath", "flashOffsite"],
  files: ["filesPath", "filesOffsite"],
  zfs: ["zfsPath", "zfsOffsite"],
  config: ["configPath", "configOffsite"],
} as const;

export type PlaceSettings = Partial<Pick<Settings, (typeof PATH_KEY)[keyof typeof PATH_KEY][number]>>;

/**
 * settingsOfPlaces turns the places a recovery kit names per section into the
 * settings that point this server at them. A named repository and a
 * destination are no setting of a section and come back with the settings
 * backup.
 */
export function settingsOfPlaces(places: EmergencyPlace[]): PlaceSettings {
  const patch: PlaceSettings = {};
  for (const place of places) {
    const keys = place.domain ? PATH_KEY[place.domain as keyof typeof PATH_KEY] : undefined;
    if (!keys) continue;
    if (place.kind === "local" && place.folder) patch[keys[0]] = place.folder;
    else if (place.kind === "offsite") patch[keys[1]] = place.location;
  }
  return patch;
}

/** minutesLeft is how long the key of an emergency stays in the server's
 *  memory, 0 once its time is up. */
export function minutesLeft(expiresAt: number, nowMs: number): number {
  return Math.max(0, Math.ceil((expiresAt * 1000 - nowMs) / 60_000));
}

/** The key of the emergency in progress is the one this server backs up
 *  under: it was taken over, or APP_KEY was set to it by hand. */
export function sessionKeyInUse(state: EmergencyState | null): boolean {
  return state?.session?.refusal === "adopt-same-key";
}

/** The emergency in progress took its key over. */
export function takenOver(state: EmergencyState | null): boolean {
  return sessionKeyInUse(state) && state?.backupKey === "adopted";
}

const REPLACED_LABEL: Record<keyof EmergencySession["replaces"], TranslationKey> = {
  cloudSignIn: "emergency.replaces.cloud",
  rcloneConfig: "emergency.replaces.rclone",
};

/** replacedText names this server's own sign-ins that the kit's would
 *  replace, in a list. */
export function replacedText(t: T, lang: string, replaces: EmergencySession["replaces"]): string {
  const kinds = (Object.keys(REPLACED_LABEL) as (keyof EmergencySession["replaces"])[]).filter((kind) => replaces[kind]);
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(kinds.map((kind) => t(REPLACED_LABEL[kind])));
}

const LOSS_LABEL: Record<keyof SealedLoss["kinds"], TranslationKey> = {
  cloud: "emergency.lost.cloud",
  rclone: "emergency.lost.rclone",
  registry: "emergency.lost.registry",
  notifications: "emergency.lost.notifications",
  "home-assistant": "emergency.lost.home-assistant",
  group: "emergency.lost.group",
  peers: "emergency.lost.peers",
  other: "emergency.lost.other",
};

/** lossText names what a settings restore could not bring over, or undefined
 *  when it brought everything. */
export function lossText(t: T, lang: string, lost: SealedLoss | undefined): string | undefined {
  if (!lost || lost.count === 0) return undefined;
  const kinds = (Object.keys(LOSS_LABEL) as (keyof SealedLoss["kinds"])[]).filter((kind) => (lost.kinds[kind] ?? 0) > 0);
  const sentence = t("emergency.lost", lost.count);
  if (kinds.length === 0) return sentence;
  const list = new Intl.ListFormat(lang, { type: "conjunction" }).format(kinds.map((kind) => t(LOSS_LABEL[kind])));
  return `${sentence} ${t("emergency.lostKinds").replace("{list}", () => list)}`;
}
