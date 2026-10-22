import { useState } from "react";
import { DEFINITION_ONLY, importDbDump, listDbDumps, restore } from "../../lib/api";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { importRefusedKey } from "../../lib/dbdump";
import type { useT } from "../../lib/i18n";
import { isRuntimeRefusal } from "../../lib/runReason";
import { Button } from "../Button";
import { IconDatabase } from "../navGlyphs";

/**
 * DumpOnlyRow restores a container the repositories hold as dumps alone and
 * imports its newest dump into it. The container comes back running, because
 * the import writes through the started server. The two runs are watched one
 * after the other, so the row can say which of them failed.
 */
export function DumpOnlyRow({
  name,
  t,
  otherActive,
}: {
  name: string;
  t: ReturnType<typeof useT>["t"];
  otherActive: boolean;
}) {
  const [busy, setBusy] = useState(false);
  // noRuntime marks a restore Docker refused for a GPU or runtime this host
  // lacks, which the row then offers without them.
  const [outcome, setOutcome] = useState<{ ok: boolean; message: string; noRuntime?: boolean } | null>(null);

  async function run(withoutRuntime = false) {
    setBusy(true);
    setOutcome(null);
    try {
      const restored = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: () => restore(name, DEFINITION_ONLY, true, undefined, undefined, withoutRuntime),
        t,
      });
      if (!restored.ok) {
        setOutcome({
          ok: false,
          message: restored.error ?? t("common.restoreFailed"),
          noRuntime: isRuntimeRefusal(restored.error),
        });
        return;
      }
      const list = await listDbDumps(name);
      const newest = (list.dumps ?? []).find((d) => !d.damaged);
      if (!newest) {
        setOutcome({ ok: false, message: t("dbdump.none") });
        return;
      }
      // The watch reports a refused start as its plain message, so the reason
      // is read where the start answers.
      const refused = { message: "" };
      const imported = await fireAndWaitRun({
        kind: "dbimport",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: async () => {
          const res = await importDbDump(name, newest.id);
          const key = res.ok ? null : importRefusedKey(res.code, newest.engine);
          if (key) {
            refused.message = t(key).replace("{server}", res.server ?? "").replace("{dump}", res.dump ?? "");
          }
          return res;
        },
        t,
      });
      setOutcome(
        imported.ok
          ? { ok: true, message: t("dbdump.importDone") }
          : { ok: false, message: refused.message || imported.error || t("common.actionFailed") }
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center gap-3">
        <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
          <IconDatabase />
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{name}</span>
        <Button
          label={t("recovery.restoreAndImport")}
          labelKey="recovery.restoreAndImport"
          glyph={<IconDatabase />}
          onClick={() => void run()}
          disabled={busy || otherActive}
          busy={busy}
          title={busy ? t("dbdump.busyImporting") : undefined}
        />
      </div>
      {outcome?.noRuntime && (
        <div className="flex flex-col items-start gap-2">
          <p className="text-xs text-statusFail wrap-break-word">{t("restore.noRuntime").replace("{name}", name)}</p>
          <Button
            label={t("restore.withoutRuntime")}
            labelKey="restore.withoutRuntime"
            onClick={() => void run(true)}
            disabled={busy || otherActive}
            busy={busy}
            hint={t("restore.withoutRuntimeHint")}
          />
        </div>
      )}
      {outcome && !outcome.noRuntime && (
        <p className={`text-xs wrap-break-word ${outcome.ok ? "text-statusOk" : "text-statusFail"}`}>{outcome.message}</p>
      )}
    </div>
  );
}
