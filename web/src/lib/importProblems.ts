import type { OffsiteDomain } from "./api";
import type { TranslationKey, useT } from "./i18n";
import { isolateLtr } from "./ltrFragments";
import { placementErrorText, type PlacementRefusal } from "./placementCodes";
import { DOMAIN_LABEL } from "./storageLocations";

type T = ReturnType<typeof useT>["t"];

/** A refused settings file: the kind of problem and what it is about. */
export type ImportProblem = PlacementRefusal & {
  /** The part of the file the row belongs to. */
  part?: string;
  /** The row of that part, counted from 1, or the file's format version. */
  row?: number;
  /** A setting's key, or the name of a repository set up here. */
  name?: string;
  /** Where a repository of the file collides: path:<section>, offsite:<section> or target. */
  place?: string;
  /** The code of the refusal behind the problem. */
  cause?: string;
};

const PROBLEM_KEYS: Record<string, TranslationKey> = {
  "import-version": "settingsIO.problem.version",
  "import-unreadable": "settingsIO.problem.unreadable",
  "import-row-invalid": "settingsIO.problem.rowInvalid",
  "import-row-name": "settingsIO.problem.rowName",
  "import-row-location": "settingsIO.problem.rowLocation",
  "import-row-compression": "settingsIO.problem.rowCompression",
  "import-schedule": "settingsIO.problem.schedule",
  "import-path": "settingsIO.problem.path",
  "import-setting": "settingsIO.problem.setting",
  "import-integrations": "settingsIO.problem.integrations",
  "import-place-taken": "settingsIO.problem.placeTaken",
  "import-place-twice": "settingsIO.problem.placeTwice",
  "import-check-failed": "settingsIO.problem.checkFailed",
};

const ROW_KEYS: Record<string, TranslationKey> = {
  target: "settingsIO.problem.rowTarget",
  repository: "settingsIO.problem.rowRepository",
  destination: "settingsIO.problem.rowDestination",
  "zfs-server": "settingsIO.problem.rowZfsServer",
  "zfs-replica": "settingsIO.problem.rowZfsReplica",
  "stored-repository": "settingsIO.problem.storedRepository",
};

/** The row a problem is about, as the sentence names it. */
function rowLabel(t: T, p: ImportProblem): string {
  const key = p.part ? ROW_KEYS[p.part] : undefined;
  if (!key) return "";
  return t(key)
    .replace("{n}", String(p.row ?? ""))
    .replace("{name}", () => isolateLtr(p.name ?? ""));
}

/** The place a repository of the file collides with. */
function placeLabel(t: T, place: string): string {
  const [kind, domain] = place.split(":");
  const section = domain && domain in DOMAIN_LABEL ? t(DOMAIN_LABEL[domain as OffsiteDomain]) : "";
  if (kind === "path") return t("settingsIO.problem.placePath").replace("{section}", () => section);
  if (kind === "offsite") return t("settingsIO.problem.placeOffsite").replace("{section}", () => section);
  return t("settingsIO.problem.placeTarget");
}

/** importProblemText is the sentence a refused settings file shows, or null
 *  when the refusal is not one of its problems. A problem with a cause says
 *  that cause as well. */
export function importProblemText(t: T, lang: string, p: ImportProblem): string | null {
  const key = p.code ? PROBLEM_KEYS[p.code] : undefined;
  if (!key) return null;
  const sentence = t(key)
    .replace("{row}", () => rowLabel(t, p))
    .replace("{n}", String(p.row ?? ""))
    .replace("{name}", () => isolateLtr(p.name ?? ""))
    .replace("{place}", () => placeLabel(t, p.place ?? ""));
  if (!p.cause) return sentence;
  const because = placementErrorText(t, lang, { ok: false, code: p.cause, suggest: p.suggest }, "settingsIO.importFailed");
  return `${sentence} ${because}`;
}
