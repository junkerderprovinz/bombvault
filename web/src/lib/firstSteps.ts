// The way from a new install to its first backup: three steps, what the
// server holds that could be backed up, and where the person stands.

import type { BackupItem, StorageLocation } from "./api";
import { isGone } from "./backupList";
import type { TranslationKey } from "./i18n";
import { formatList } from "./placement";

type T = (key: TranslationKey, n?: number) => string;

export const FIRST_STEPS = ["sections", "location", "backup"] as const;

export type FirstStep = (typeof FIRST_STEPS)[number];

export type StepState = "done" | "next" | "open";

/**
 * stepStates marks the first step still to do as next and every later one as
 * open. The first backup is never done here: the steps go when it is.
 */
export function stepStates(done: { sections: boolean; location: boolean }): Record<FirstStep, StepState> {
  const finished: Record<FirstStep, boolean> = { ...done, backup: false };
  const next = FIRST_STEPS.find((step) => !finished[step]);
  const state = (step: FirstStep): StepState => (finished[step] ? "done" : step === next ? "next" : "open");
  return { sections: state("sections"), location: state("location"), backup: state("backup") };
}

/**
 * addedLocations are the storage locations somebody set up. The folder every
 * section starts with exists on a new install and says nothing about the
 * second step.
 */
export function addedLocations(locations: StorageLocation[]): StorageLocation[] {
  return locations.filter((location) => location.enabled && location.object !== "path");
}

export interface Found {
  container: number;
  vm: number;
  files: number;
  zfs: number;
  flash: boolean;
}

/**
 * foundOnServer counts what the server reports that could be backed up,
 * whether its kind is switched on or not. Only Unraid boots from a flash
 * drive.
 */
export function foundOnServer(reported: BackupItem[], platform: string): Found {
  const count = (kind: BackupItem["kind"]) =>
    reported.filter((item) => item.kind === kind && !item.self && !isGone(item)).length;
  return {
    container: count("container"),
    vm: count("vm"),
    files: count("files"),
    zfs: count("zfs"),
    flash: platform === "unraid",
  };
}

/** foundSentence says what was found, or nothing where the server reports
 *  nothing that could be backed up. */
export function foundSentence(found: Found, t: T, lang: string): string | null {
  const parts = [
    found.container > 0 && t("firstSteps.found.containers", found.container),
    found.vm > 0 && t("firstSteps.found.vms", found.vm),
    found.files > 0 && t("firstSteps.found.folders", found.files),
    found.zfs > 0 && t("firstSteps.found.datasets", found.zfs),
    found.flash && t("firstSteps.found.flash"),
  ].filter((part) => part !== false);
  if (parts.length === 0) return null;
  return t("firstSteps.found").replace("{list}", () => formatList(lang, parts));
}

const SECTIONS_CHOSEN = "bombvault.firstSteps.sectionsChosen";

export function sectionsChosen(): boolean {
  try {
    return localStorage.getItem(SECTIONS_CHOSEN) === "1";
  } catch {
    return false;
  }
}

export function rememberSectionsChosen(): void {
  try {
    localStorage.setItem(SECTIONS_CHOSEN, "1");
  } catch {
    // Without storage the first step is asked again on the next visit.
  }
}

/** forgetFirstSteps keeps a finished first start from answering for a new
 *  install that is later reached at the same address. */
export function forgetFirstSteps(): void {
  try {
    localStorage.removeItem(SECTIONS_CHOSEN);
  } catch {
    // Nothing was stored where storage is unavailable.
  }
}
