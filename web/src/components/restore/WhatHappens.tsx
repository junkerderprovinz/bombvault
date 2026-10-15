// What the restore will do, in sentences, before the button names the whole
// action once more. A copy next to the original also says where it lands and
// how it differs from the original, as the check worked it out.
import type { RestorePlan } from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import { useT, type TranslationKey } from "../../lib/i18n";
import type { RestoreWhat, RestoreWhere } from "../../lib/restorePlan";

function sentences(
  kind: EntryKind,
  what: RestoreWhat,
  where: RestoreWhere,
  o: { stopped: boolean; backupFirst: boolean; exists: boolean }
): TranslationKey[] {
  if (where === "zip") return ["restoreWindow.says.zip", "restoreWindow.says.zipStick", "restoreWindow.says.zipFlash"];
  if (where === "folder") return ["restoreWindow.says.folder", "restoreWindow.untouched"];
  if (where === "newDataset") return ["restoreWindow.says.newDataset", "restoreWindow.untouched"];
  if (where === "beside") {
    return kind === "container"
      ? ["restoreWindow.says.copy", "restoreWindow.says.copyOriginal", "restoreWindow.says.copyUnscheduled"]
      : ["restoreWindow.says.besideFolder", "restoreWindow.untouched"];
  }
  const first: TranslationKey[] = o.backupFirst ? ["restoreWindow.says.backupFirst"] : [];
  if (what === "files") return ["restoreWindow.says.replaceFiles"];
  if (kind === "container" && !o.exists) {
    return [
      "restoreWindow.says.createContainer",
      o.stopped ? "restoreWindow.says.containerStopped" : "restoreWindow.says.createdContainerStarts",
    ];
  }
  if (kind === "vm" && !o.exists) {
    return ["restoreWindow.says.createVM", o.stopped ? "restoreWindow.says.vmOff" : "restoreWindow.says.createdVMStarts"];
  }
  if (kind === "container") {
    return [
      "restoreWindow.says.replaceContainer",
      o.stopped ? "restoreWindow.says.containerStopped" : "restoreWindow.says.containerStarts",
      ...first,
    ];
  }
  if (kind === "vm") {
    return ["restoreWindow.says.replaceVM", o.stopped ? "restoreWindow.says.vmOff" : "restoreWindow.says.vmStarts", ...first];
  }
  if (kind === "zfs") return ["restoreWindow.says.replaceDataset"];
  return ["restoreWindow.says.replaceFiles", ...first];
}

export function WhatHappens({
  kind,
  what,
  where,
  stopped,
  backupFirst,
  exists,
  plan,
}: {
  kind: EntryKind;
  what: RestoreWhat;
  where: RestoreWhere;
  stopped: boolean;
  backupFirst: boolean;
  /** Whether the container or VM is there now, or the restore creates it. */
  exists: boolean;
  /** The plan of the last check, once it is back. */
  plan: RestorePlan | null | undefined;
}) {
  const { t } = useT();
  const beside = where === "beside" ? plan?.beside : undefined;
  const copy = beside?.container;
  const refused = backupFirst && plan?.backupFirst?.status === "fail" ? plan.backupFirst : undefined;

  return (
    <section className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3 text-sm text-carbon-text">
      <h4 className="font-semibold">{t("restoreWindow.happens")}</h4>
      <p className="text-carbon-textSub">
        {sentences(kind, what, where, { stopped, backupFirst, exists })
          .map((key) => t(key))
          .join(" ")}
      </p>
      {refused && (
        <p className="text-statusWarn wrap-break-word">
          {t("restoreWindow.backupFirstRefused").replace("{reason}", () => refused.detail ?? "")}
        </p>
      )}
      {beside && beside.folders.length > 0 && (
        <ul className="flex flex-col gap-0.5 text-caption">
          {beside.folders.map((f) => (
            <li key={f.copy} dir="ltr" className="break-all font-mono text-start text-carbon-textSub">
              {f.original} → {f.copy}
            </li>
          ))}
        </ul>
      )}
      {copy && (
        <ul className="flex flex-col gap-1 text-caption text-carbon-textSub">
          {copy.ports.map((p) => (
            <li key={p.container}>
              <span className="font-semibold text-carbon-text">{t("restoreCheck.field.port")}</span>{" "}
              {p.copy
                ? t("restoreWindow.copyPort").replace("{copy}", p.copy).replace("{original}", p.original)
                : t("restoreWindow.copyPortOpen").replace("{original}", p.original)}
            </li>
          ))}
          {copy.fixedIp && <li>{t("restoreWindow.copyAddress").replace("{ip}", copy.fixedIp)}</li>}
          {copy.shared.length > 0 && (
            <li className="text-statusWarn wrap-break-word">
              {t("restoreWindow.copyShared").replace("{folders}", () => copy.shared.join(", "))}
            </li>
          )}
        </ul>
      )}
    </section>
  );
}
