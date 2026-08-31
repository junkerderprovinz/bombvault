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
  IconEye,
  IconForward,
  IconInfo,
  IconKey,
  IconLink,
  IconPlay,
  IconPrune,
  IconRefresh,
  IconSave,
  IconSearch,
  IconSelectAll,
  IconStop,
  IconUnlock,
  IconUpload,
} from "./glyphs";

// ---------------------------------------------------------------------------
// glyphFor (#178, [202]) — which symbol a button wears, decided once.
//
// jdp asked for every button to get a glyph. Drawing 146 unique symbols would
// not have helped anyone: most of these buttons are the SAME VERB in different
// places, and a reader learns "this shape means delete" far faster from twelve
// repeated symbols than from a hundred and forty-six unique ones. So the
// mapping is by MEANING, keyed off the translation key, which is stable across
// all 42 languages in a way the visible text is not.
//
// Order matters: the first matching rule wins, so the specific patterns sit
// above the general ones ("backupSelected" before "selected", "unlock" before
// "lock"). A key that matches nothing returns undefined, and Button then falls
// back to showing that button's text rather than an empty square.
// ---------------------------------------------------------------------------

type Rule = [RegExp, () => ReactNode];

const RULES: Rule[] = [
  // Backups and restores, the app's own verbs, before anything generic.
  [/backupNow|backupAll|backupSelected|runNow|backupOrder\.save/i, () => <IconBackupNow />],
  [/restore/i, () => <IconRestore />],
  [/replicate|sync|refreshStatus/i, () => <IconSync />],

  // Destructive and corrective actions.
  [/\.(delete|remove)|removeExclusion|assistRemove|forget/i, () => <IconTrash />],
  [/prune|reclaim/i, () => <IconPrune />],
  [/unlock/i, () => <IconUnlock />],

  // Creation and editing.
  [/\.add|addSet|addPreset|addTarget|addTag|credSets\.add|registryAdd/i, () => <IconAdd />],
  [/edit|rename|editSet/i, () => <IconPencil />],
  [/save|apply|confirm(?!Password)/i, () => <IconSave />],

  // Selection.
  [/clearSelection|clearDayFilter|clearOrder|reset/i, () => <IconClearSelection />],
  [/selectAll|selectEvery/i, () => <IconSelectAll />],

  // Navigation and dialogs.
  [/cancel|skip|decline/i, () => <IconCancel />],
  [/close|dismiss/i, () => <IconClose />],
  [/back|previous|prev\b/i, () => <IconBack />],
  [/next|continue|forward|jumpToLatest/i, () => <IconForward />],

  // Connections, ahead of the probing block on purpose. "Connect" is the
  // stronger verb whenever a key carries both: `recovery.connectPreview` is a
  // button that CONNECTS to a foreign repository, and previewing it is what
  // follows. Below the probing rules it matched `preview` and wore an eye
  // (jdp: "der Verbinden und prüfen button soll eine kette als glyph
  // bekommen"). Exactly one key changes glyph by this move — the other two
  // connect keys never matched anything above it.
  [/connect|pair|link|reconnect/i, () => <IconLink />],

  // Probing and inspection.
  [/test|verify|check|drill/i, () => <IconCheckCircle />],
  [/scan|discover|browse|search/i, () => <IconSearch />],
  [/show|reveal|preview|view/i, () => <IconEye />],
  [/hint|info|explain|examples/i, () => <IconInfo />],

  // Transfer.
  [/download|export/i, () => <IconDownload />],
  [/upload|send|offer|push/i, () => <IconUpload />],
  [/copy/i, () => <IconCopy />],

  // Secrets.
  [/credential|password|secret|token|key\b/i, () => <IconKey />],

  // Lifecycle.
  [/start|run\b|play/i, () => <IconPlay />],
  [/stop|abort|halt/i, () => <IconStop />],
  [/power|shutdown|reboot/i, () => <IconPower />],

  // Local storage, as opposed to off-site ([310]).
  //
  // Two ordering constraints, both load-bearing. It sits BELOW the probing
  // block because `drill.checkLocal` is a button that CHECKS, and the thing it
  // checks is a detail of the verb — above that block it would wear a drive
  // and stop looking like its three siblings in the same row. It sits ABOVE
  // "places and configuration" because `recovery.configLocalPath` ends in
  // "Path" and would otherwise take the folder, which is the one glyph a local
  // control must not wear: a Browse button already has it, and two different
  // functions sharing a symbol is the collision jdp reported once already.
  //
  // Anchored to the end of the key rather than matching "local" anywhere, so
  // it takes `source.local` and `settings.pathMode.local` — the actual
  // switches — without swallowing every key that merely mentions locality.
  [/\.local$|Local$/i, () => <IconLocal />],

  // Places and configuration, last because they are the vaguest.
  [/folder|path|directory/i, () => <IconFolder />],
  [/settings|config|setup|wizard|options/i, () => <IconGear />],
  [/refresh|reload/i, () => <IconRefresh />],
];

/**
 * The glyph for a translation key, or undefined when nothing sensible matches.
 *
 * Undefined is a real answer, not a gap to be filled with a placeholder: a
 * button with no glyph keeps showing its text even in glyph mode, which is far
 * better than a symbol that means nothing.
 */
export function glyphFor(key: string): ReactNode | undefined {
  for (const [pattern, make] of RULES) {
    if (pattern.test(key)) return make();
  }
  return undefined;
}
