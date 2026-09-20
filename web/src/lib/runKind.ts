// The name a run's kind carries wherever runs are listed: the run history, the
// error panel and the activity log's type filter.

import type { useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

/** The kind of work a run did, in the reader's language. An unknown kind from a
 *  newer backend shows its raw literal rather than a wrong label. */
export function runKindLabel(t: T, kind: string): string {
  switch (kind) {
    case "backup":
      return t("run.kindBackup");
    case "restore":
      return t("run.kindRestore");
    case "update":
      return t("run.kindUpdate");
    case "prune":
      return t("activityLog.typePrune");
    case "verify":
      return t("activityLog.typeVerify");
    case "offsite":
      return t("activityLog.typeOffsite");
    case "drill":
      return t("activityLog.jobDrill");
    case "drdrill":
      return t("run.kindDRDrill");
    case "tamper":
      return t("activityLog.jobTamper");
    case "export":
      return t("run.kindExport");
    case "dbdump":
      return t("run.kindDbDump");
    case "dbdumpsave":
      return t("run.kindDbDumpSave");
    case "dbimport":
      return t("run.kindDbImport");
    default:
      return kind;
  }
}
