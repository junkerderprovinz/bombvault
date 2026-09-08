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

  // Local storage, as opposed to off-site ([321]).
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

  // The About card's two non-brand offers. Both are nouns rather than verbs,
  // which is why nothing above them matched: this table is built around what a
  // button DOES, and "coffee" and "mail" are what it is ABOUT. They earn their
  // own rules rather than an explicit glyph at the call site because the
  // meanings are general — any future "write to us" wears the same envelope.
  //
  // The third button on that card, the repository one, is deliberately NOT
  // here: it wears GitHub's own mark, and a brand mark must never be reachable
  // by pattern (a rule keyed on "repo" would put GitHub's logo on repository
  // settings that have nothing to do with GitHub). It is passed explicitly at
  // its one call site. See gen_glyphs.py's IconGithub entry for the full rule.
  [/coffee|donate|sponsor/i, () => <IconCoffee />],
  [/\.mail|contact|writeToUs/i, () => <IconMail />],

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
