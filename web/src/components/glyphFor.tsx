import type { ReactNode } from "react";
import {
  IconAdd,
  IconBackupNow,
  IconCheckCircle,
  IconClose,
  IconCopy,
  IconDownload,
  IconFolder,
  IconGear,
  IconLocal,
  IconPencil,
  IconPower,
  IconRestore,
  IconSync,
  IconTrash,
} from "./Sidebar";
import {
  IconBack,
  IconCancel,
  IconClearSelection,
  IconCoffee,
  IconCompare,
  IconEye,
  IconForward,
  IconInfo,
  IconKey,
  IconLink,
  IconMail,
  IconPlay,
  IconPrune,
  IconRefresh,
  IconSave,
  IconSearch,
  IconSelectAll,
  IconShieldOff,
  IconShieldOn,
  IconSignIn,
  IconSignOut,
  IconStop,
  IconUnlock,
  IconUpload,
} from "./glyphs";

// glyphFor picks the symbol a button wears by meaning, so one verb has one shape
// wherever it appears. It is keyed off the translation key, which is the same
// in every language. The first matching rule wins, so specific patterns sit
// above general ones ("backupSelected" before "selected", "unlock" before
// "lock").

type Rule = [RegExp, () => ReactNode];

const RULES: Rule[] = [
  // Backups and restores, the app's own verbs, before anything generic.
  [/backupNow|backupAll|backupSelected|runNow|backupOrder\.save/i, () => <IconBackupNow />],
  // "restor" also catches the "restoring" keys shown while a run is in flight.
  [/restor|recreate|rebuild/i, () => <IconRestore />],
  [/replicate|sync|refreshStatus|pollNow/i, () => <IconSync />],

  // Destructive and corrective actions.
  [/\.(delete|remove)|confirmRemove|removeExclusion|assistRemove|forget/i, () => <IconTrash />],
  [/prune|reclaim/i, () => <IconPrune />],
  [/unlock/i, () => <IconUnlock />],

  // Creation and editing.
  [/\.add|addSet|addPreset|addTarget|addTag|credSets\.add|registryAdd|passkeyAdd|passkeyCreate/i, () => <IconAdd />],
  // A list that fetches its next page grows by the same plus as one that gains
  // an entry.
  [/loadMore/i, () => <IconAdd />],
  [/edit|rename|editSet/i, () => <IconPencil />],
  [/save|apply/i, () => <IconSave />],

  // Selection. "Exclude all" and "Include all" are clear and select all under
  // other names.
  [/clearSelection|clearDayFilter|clearOrder|reset|excludeAll|assistExclude/i, () => <IconClearSelection />],
  [/selectAll|selectEvery|includeAll/i, () => <IconSelectAll />],

  // Navigation and dialogs.
  [/cancel|skip|decline/i, () => <IconCancel />],
  [/close|dismiss/i, () => <IconClose />],
  [/back|previous|prev\b/i, () => <IconBack />],
  [/next|continue|forward|jumpToLatest/i, () => <IconForward />],

  // Connections, above the probing rules so recovery.connectPreview, which
  // connects first and previews after, gets the link.
  [/connect|pair|link|reconnect/i, () => <IconLink />],

  // Probing and inspection. "accept", "confirm" and "resolveAll" agree to what
  // is on screen, so they take the same check.
  [/test|probe|verify|check|drill|appendOnly|tamper|accept|approve|confirm(?!Password)|resolveAll|\.stored$/i, () => <IconCheckCircle />],
  // Settling an anomaly: seen and closed takes the same check as the other
  // agreements, while marking it as expected records a new normal level.
  [/acknowledge/i, () => <IconCheckCircle />],
  [/expected/i, () => <IconSave />],

  [/scan|discover|browse|search/i, () => <IconSearch />],
  [/show|reveal|preview|view/i, () => <IconEye />],
  [/hint|info|explain|examples/i, () => <IconInfo />],

  // Transfer. Pulling another instance's snapshots into this one is a
  // download.
  [/download|export|pullNow|pullFrom/i, () => <IconDownload />],
  [/upload|send|offer|push|proposeButton/i, () => <IconUpload />],
  [/copy/i, () => <IconCopy />],

  // Signing in and two-factor sit above the credentials rule (the key marks a
  // secret, signing in is a door) and above the power rule, which is for
  // shutting a machine down.
  [/signIn|logIn\b/i, () => <IconSignIn />],
  [/logout|signOut/i, () => <IconSignOut />],
  [/twoFactorEnable|totpEnable/i, () => <IconShieldOn />],
  [/twoFactorDisable|totpDisable/i, () => <IconShieldOff />],
  [/compare|diff\b/i, () => <IconCompare />],
  [/credential|password|secret|token|key\b/i, () => <IconKey />],

  // Lifecycle.
  [/start|run\b|play/i, () => <IconPlay />],
  [/stop|abort|halt/i, () => <IconStop />],
  [/power|shutdown|reboot/i, () => <IconPower />],

  // Local storage, as opposed to off-site. Below the probing rules so
  // drill.checkLocal keeps its check mark, above the places rule so
  // settings.pathMode.local does not take the folder, and anchored to the end
  // so only the switches match.
  [/\.local$|Local$/i, () => <IconLocal />],

  // Nouns rather than verbs, for the generic cases: a plain cup for a donation
  // and an envelope for any address. The About card's buttons carry their
  // companies' own marks at the call site, since a brand must not be reachable
  // by pattern.
  [/coffee|donate|sponsor/i, () => <IconCoffee />],
  [/\.mail|contact|writeToUs/i, () => <IconMail />],

  // Places and configuration, last because they are the vaguest.
  [/folder|path|directory/i, () => <IconFolder />],
  [/settings|config|setup|wizard|options/i, () => <IconGear />],
  [/refresh|reload|retry|tryAgain/i, () => <IconRefresh />],
];

/**
 * glyphFor returns the glyph for a translation key, or undefined when nothing
 * matches, in which case the button keeps its text.
 */
export function glyphFor(key: string): ReactNode | undefined {
  for (const [pattern, make] of RULES) {
    if (pattern.test(key)) return make();
  }
  return undefined;
}
