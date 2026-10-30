import type { ReactNode } from "react";
import {
  IconAdd,
  IconBackupNow,
  IconCheck,
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
import { IconFolderAdd } from "./navGlyphs";
import {
  IconApply,
  IconBack,
  IconClearSelection,
  IconCompare,
  IconConfirm,
  IconExport,
  IconEye,
  IconForgotPassword,
  IconForward,
  IconImport,
  IconInfo,
  IconInstall,
  IconKey,
  IconKeyRevoke,
  IconLatest,
  IconLink,
  IconLog,
  IconMail,
  IconPlay,
  IconPrune,
  IconRefresh,
  IconReject,
  IconRetry,
  IconSave,
  IconSearch,
  IconSelectAll,
  IconSendTest,
  IconShieldOff,
  IconShieldOn,
  IconSignIn,
  IconSignOut,
  IconStar,
  IconStop,
  IconUndo,
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
  [/backupNow|backupAll|backupSelected|runNow/i, () => <IconBackupNow />],
  // Undo takes back the step just made, which is not a restore of a backup.
  [/undo/i, () => <IconUndo />],
  // Checking again is a retry. Above the restore rule, which every
  // restoreCheck key would otherwise match, and the pairing rule.
  [/\.recheck$|checkAgain/i, () => <IconRetry />],
  [/showFiles/i, () => <IconEye />],
  // Stopping a restore is a cancel like any other, not a restore.
  [/restore\.cancel$/i, () => <IconClose />],
  // "restor" also catches the "restoring" keys shown while a run is in flight.
  [/restor|recreate|rebuild/i, () => <IconRestore />],
  [/replicate|sync|refreshStatus|pollNow/i, () => <IconSync />],

  // Destructive and corrective actions.
  [/\.(delete|remove)|confirmRemove|removeExclusion|assistRemove|forget/i, () => <IconTrash />],
  [/prune|reclaim/i, () => <IconPrune />],
  [/unlock/i, () => <IconUnlock />],

  // Above the sign-in rule, which every signIns key would otherwise match.
  [/makeDefault/i, () => <IconStar />],

  // Agreeing and refusing: taking over or using the one shown applies it, and
  // Done, Got it, accepting and allowing confirm. Above the creation rules,
  // which zfs.add.done would otherwise match.
  [/apply|\.takeOver$|takeover\.accept|adopt\.take|primary\.use|folder\.use|useThis|useLocal|settingsIO\.confirm/i, () => <IconApply />],
  [/decline|deny|reject|refuse/i, () => <IconReject />],
  [/\bdone\b|gotIt|dismissKey|confirm(?!Password)|accept|approve|\.allow$|cameraAllow|acknowledge|resolveAll|markAll|\.stored$|recoverySaved/i, () => <IconConfirm />],

  // Creation and editing.
  [/folder\.create|makeFolder/i, () => <IconFolderAdd />],
  [/\.add|addSet|addPreset|addTarget|addTag|credSets\.add|registryAdd|passkeyAdd|passkeyCreate|certAddAddress/i, () => <IconAdd />],
  // A list that fetches its next page grows by the same plus as one that gains
  // an entry.
  [/loadMore/i, () => <IconAdd />],
  [/edit|rename|editSet|retype/i, () => <IconPencil />],
  [/save/i, () => <IconSave />],

  // Selection. "Exclude all" and "Include all" are clear and select all under
  // other names. A reset goes back to the default.
  [/clearSelection|clearDayFilter|resetSelection|excludeAll|assistExclude/i, () => <IconClearSelection />],
  [/selectAll|selectEvery|includeAll/i, () => <IconSelectAll />],
  [/reset|useDefault/i, () => <IconRefresh />],

  // Navigation and dialogs.
  [/cancel|skip|close|dismiss/i, () => <IconClose />],
  [/back|previous|prev\b/i, () => <IconBack />],
  [/jumpToLatest/i, () => <IconLatest />],
  [/next|continue|forward/i, () => <IconForward />],

  // Connections, above the probing rules so recovery.connectPreview, which
  // connects first and previews after, gets the link.
  [/connect|pair|link|reconnect/i, () => <IconLink />],

  // Probing and inspection. A test message is sent, not checked.
  [/notify\.test|sendTest/i, () => <IconSendTest />],
  [/test|probe|verify|check|drill|appendOnly|tamper/i, () => <IconCheck />],
  // Marking an anomaly as expected records a new normal level.
  [/expected/i, () => <IconSave />],

  [/scan|discover|browse|search/i, () => <IconSearch />],
  [/show|reveal|preview|view/i, () => <IconEye />],
  [/hint|info|explain|examples/i, () => <IconInfo />],

  // Transfer. Import and export read a file in and write one out, while upload
  // and download move one between here and elsewhere. Pulling another
  // instance's snapshots into this one is a download.
  [/import/i, () => <IconImport />],
  [/export/i, () => <IconExport />],
  [/download|pullNow|pullFrom/i, () => <IconDownload />],
  [/upload|send|offer|push|proposeButton/i, () => <IconUpload />],
  [/copy/i, () => <IconCopy />],

  // Signing in and two-factor sit above the credentials rule (the key marks a
  // secret, signing in is a door) and above the power rule, which is for
  // shutting a machine down.
  [/signIn|logIn\b/i, () => <IconSignIn />],
  [/logout|signOut/i, () => <IconSignOut />],
  [/twoFactorEnable|totpEnable/i, () => <IconShieldOn />],
  [/twoFactorDisable|totpDisable|loginDisable/i, () => <IconShieldOff />],
  // Trusting a certificate is a security decision, like arming a factor.
  [/\.trust(Changed)?$/i, () => <IconShieldOn />],
  // Revoking takes one credential away, which is neither the row being
  // deleted nor the second factor going off.
  [/revoke|passkeyRemove$/i, () => <IconKeyRevoke />],
  [/compare|diff\b/i, () => <IconCompare />],
  [/forgotPassword/i, () => <IconForgotPassword />],
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

  // An envelope for any address. Nothing matches coffee or a donation: the only
  // mark for that is Buy Me a Coffee's own, and a brand is passed at its call
  // site, never reached by pattern.
  [/\.mail|contact|writeToUs/i, () => <IconMail />],

  // Above the places rule, which settings.dashTileInstall would otherwise match.
  [/install/i, () => <IconInstall />],
  [/\.log$/i, () => <IconLog />],

  // Places and configuration, last because they are the vaguest.
  [/folder|path|directory/i, () => <IconFolder />],
  [/settings|config|setup|wizard|options/i, () => <IconGear />],
  [/refresh|reload|retry|tryAgain|again\b|rotate/i, () => <IconRetry />],
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
