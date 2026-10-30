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
  "import-schedule": "settingsIO.problem.schedule",
  "import-path": "settingsIO.problem.path",
  "import-setting": "settingsIO.problem.setting",
  "import-integrations": "settingsIO.problem.integrations",
  "import-check-failed": "settingsIO.problem.checkFailed",
};

/** The sentences of the problems about one row of the file, by the part of the
 *  file the row is in, and for a place taken by what takes it. Each language
 *  writes every sentence whole, since a row named on its own would need another
 *  case or article in each sentence it stands in. */
const ROW_KEYS: Record<string, Record<string, TranslationKey>> = {
  "import-row-invalid": {
    target: "settingsIO.problem.targetInvalid",
    "zfs-server": "settingsIO.problem.zfsServerInvalid",
    "zfs-replica": "settingsIO.problem.zfsReplicaInvalid",
  },
  "import-row-name": {
    repository: "settingsIO.problem.repositoryName",
    destination: "settingsIO.problem.destinationName",
  },
  "import-row-location": {
    repository: "settingsIO.problem.repositoryLocation",
    destination: "settingsIO.problem.destinationLocation",
  },
  "import-row-compression": {
    target: "settingsIO.problem.targetCompression",
    repository: "settingsIO.problem.repositoryCompression",
    destination: "settingsIO.problem.destinationCompression",
  },
  "import-place-taken": {
    "repository:path": "settingsIO.problem.repositoryOnPath",
    "repository:offsite": "settingsIO.problem.repositoryOnOffsite",
    "repository:target": "settingsIO.problem.repositoryOnTarget",
    "stored-repository:path": "settingsIO.problem.storedOnPath",
    "stored-repository:offsite": "settingsIO.problem.storedOnOffsite",
    "stored-repository:target": "settingsIO.problem.storedOnTarget",
  },
  "import-place-twice": {
    repository: "settingsIO.problem.repositoryTwice",
    "stored-repository": "settingsIO.problem.storedTwice",
  },
};

/** The key of the sentence a problem is worded with, if it has one. */
function problemKey(p: ImportProblem): TranslationKey | undefined {
  if (!p.code) return undefined;
  const rows = ROW_KEYS[p.code];
  if (!rows) return PROBLEM_KEYS[p.code];
  const kind = p.code === "import-place-taken" ? `:${(p.place ?? "").split(":")[0]}` : "";
  return rows[`${p.part ?? ""}${kind}`];
}

/** importProblemText is the sentence a refused settings file shows, or null
 *  when the refusal is not one of its problems. A problem with a cause says
 *  that cause as well. */
export function importProblemText(t: T, lang: string, p: ImportProblem): string | null {
  const key = problemKey(p);
  if (!key) return null;
  const domain = (p.place ?? "").split(":")[1];
  const section = domain && domain in DOMAIN_LABEL ? t(DOMAIN_LABEL[domain as OffsiteDomain]) : "";
  const sentence = t(key)
    .replace("{n}", String(p.row ?? ""))
    .replace("{name}", () => isolateLtr(p.name ?? ""))
    .replace("{section}", () => section);
  if (!p.cause) return sentence;
  const because = placementErrorText(t, lang, { ok: false, code: p.cause, suggest: p.suggest }, "settingsIO.importFailed");
  return `${sentence} ${because}`;
}
