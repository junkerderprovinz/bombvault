// RuntimeRetry follows a restore of several containers, a bulk or a stack
// restore. It names the ones Docker could not recreate because this host lacks
// their GPU or runtime, and restores just those again without them, one after
// the other, as the bulk restores do.

import { useState } from "react";
import { restore } from "../../lib/api";
import { fireAndWaitRun } from "../../lib/backupWatch";
import type { useT } from "../../lib/i18n";
import { isRuntimeRefusal } from "../../lib/runReason";
import { Button } from "../Button";

type T = ReturnType<typeof useT>["t"];

export function RuntimeRetry({
  names,
  source,
  leaveStopped,
  onDone,
  t,
}: {
  /** The containers whose restore Docker refused for their GPU or runtime. */
  names: string[];
  source?: string;
  leaveStopped?: boolean;
  /** Runs once the retry has finished, so the caller can reload its list. */
  onDone?: () => void;
  t: T;
}) {
  const [left, setLeft] = useState(names);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ ok: number; fail: number } | null>(null);

  async function retry() {
    setBusy(true);
    let ok = 0;
    let fail = 0;
    const refused: string[] = [];
    for (const name of left) {
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === "container" && r.target === name,
        start: () => restore(name, "latest", true, source, leaveStopped, true),
        t,
      });
      if (res.ok) ok++;
      else {
        fail++;
        if (isRuntimeRefusal(res.error)) refused.push(name);
      }
    }
    setLeft(refused);
    setResult({ ok, fail });
    setBusy(false);
    onDone?.();
  }

  if (left.length === 0 && !result) return null;
  return (
    <div className="flex flex-col items-start gap-2">
      {left.length > 0 && (
        <>
          <p className="text-xs text-statusFail wrap-break-word">
            {t("restore.noRuntimeList").replace("{names}", left.join(", "))}
          </p>
          <Button
            label={t("restore.withoutRuntime")}
            labelKey="restore.withoutRuntime"
            tone="accent"
            onClick={() => void retry()}
            disabled={busy}
            busy={busy}
            hint={t("restore.withoutRuntimeHint")}
          />
        </>
      )}
      {result && (
        <p className={`text-xs ${result.fail > 0 ? "text-statusWarn" : "text-statusOk"}`}>
          {t("containers.bulkResult").replace("{ok}", String(result.ok)).replace("{fail}", String(result.fail))}
        </p>
      )}
    </div>
  );
}
