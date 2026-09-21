import { useEffect, useState, type ReactNode } from "react";
import { setDbDumpEngine, setDbDumpOff, type Container, type DbEngine } from "../lib/api";
import { useAdvanced } from "../lib/advanced";
import { coverageKey, dumpLeftRunning, dumpWasCancelled, ENGINE_NAMES, remedyKey } from "../lib/dbdump";
import { humanBytes } from "../lib/forecast";
import type { TranslationKey, useT } from "../lib/i18n";
import { relativeTime } from "../lib/reltime";
import { RunReasonText } from "../lib/runReason";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { ToggleRow } from "../pages/settings/shared";
import { InfoBubble } from "./InfoBubble";
import { SelectField } from "./SelectField";

type T = ReturnType<typeof useT>["t"];

// Any coverage but "stopped" means the files backup is not a copy anyone can
// start from, so the dump is what carries this container.
const NEEDS_THE_DUMP = ["live", "none", "unknown"];

const OFF_CONFIRM_KEYS: Record<string, TranslationKey> = {
  live: "dbdump.offConfirmLive",
  none: "dbdump.offConfirmNone",
  unknown: "dbdump.offConfirmUnknown",
};

/**
 * DatabaseDumpRow is the switch for a container's database dump, with what the
 * backup of its data folder is worth and how the last dump went. The backend
 * sends states and numbers; every sentence is built here.
 */
export function DatabaseDumpRow({ container, t }: { container: Container; t: T }) {
  const { advanced } = useAdvanced();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const lookalike = container.dbTier === "lookalike";
  // A lookalike keeps the opt-out of a time it was recognised by its image or
  // label, and that opt-out still stops the dump.
  const stored = lookalike ? container.dbDumpEngine !== "" && !container.dbDumpOff : !container.dbDumpOff;
  const storedEngine = container.dbDumpEngine || container.dbSuggestedEngine;
  const [on, setOn] = useState(stored);
  const [chosenEngine, setChosenEngine] = useState(storedEngine);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  // Rows are keyed by name and do not remount, so a value reloaded from the
  // server has to be copied in.
  useEffect(() => setOn(stored), [stored]);
  useEffect(() => setChosenEngine(storedEngine), [storedEngine]);

  if (container.dbTier === "") return null;

  const engine = lookalike ? container.dbSuggestedEngine : container.dbEngine;
  const engineName = ENGINE_NAMES[engine as keyof typeof ENGINE_NAMES] ?? "";
  const coverage = container.dbDataCoverage;
  const needsTheDump = NEEDS_THE_DUMP.includes(coverage);
  // The label and the global switch both win over the stored value, so the row
  // shows what actually happens rather than what was stored.
  const forcedOff = container.dbDumpLabelOff || container.dbDumpsGlobalOff;
  const forcedOn = !forcedOff && container.dbTier === "label";
  const shownOn = forcedOn || (!forcedOff && on);

  function hintKey(): TranslationKey {
    if (container.dbDumpLabelOff) return "dbdump.labelOff";
    if (container.dbDumpsGlobalOff) return "dbdump.globalOff";
    if (lookalike) return "dbdump.lookalikeHint";
    if (forcedOn) return "dbdump.toggleHintLabel";
    if (coverage === "live" || coverage === "none") return "dbdump.toggleHintOnlyCopy";
    if (coverage === "unknown") return "dbdump.toggleHintUnknown";
    return "dbdump.toggleHint";
  }

  async function save(next: boolean, chosen?: DbEngine) {
    setBusy(true);
    const picked = chosen ?? chosenEngine;
    try {
      const res = lookalike
        ? await setDbDumpEngine(container.name, next ? picked : "")
        : await setDbDumpOff(container.name, !next);
      if (res.ok) {
        setOn(next);
        if (next) setChosenEngine(picked);
      } else {
        push(res.error || t("dbdump.settingFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("dbdump.settingFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  async function handleChange(next: boolean) {
    if (!next && needsTheDump) {
      const question = OFF_CONFIRM_KEYS[coverage];
      if (!(await confirm(t(question), { confirmKey: "common.confirm" }))) return;
    }
    await save(next);
  }

  const last = container.lastDbDump;

  function lastResult(): { node: ReactNode; remedy: TranslationKey | null } {
    if (!last) return { node: null, remedy: null };
    if (dumpWasCancelled(last.error)) {
      return {
        node: (
          <>
            <span className="text-carbon-textMuted">{t("dbdump.resultCancelled")}</span>
            {dumpLeftRunning(last.error) && (
              <>
                {" · "}
                <span className="text-statusWarn">{t("runReason.dbdumpOrphan")}</span>
              </>
            )}
          </>
        ),
        remedy: null,
      };
    }
    if (last.status === "failed") {
      return {
        node: (
          <span className="text-statusFail">
            <RunReasonText reason={last.error} t={t} />
          </span>
        ),
        remedy: remedyKey(last.error),
      };
    }
    return {
      node: (
        <>
          {humanBytes(last.bytes)}
          {last.error && (
            <>
              {" · "}
              <span className="text-statusWarn">
                <RunReasonText reason={last.error} t={t} />
              </span>
            </>
          )}
        </>
      ),
      remedy: null,
    };
  }

  const result = lastResult();
  const [beforeResult, afterResult] = last
    ? t("dbdump.lastLine").replace("{when}", relativeTime(t, last.at)).split("{result}")
    : ["", ""];
  const coverageLine = coverageKey(coverage);

  return (
    <div className="flex flex-col items-end gap-1">
      {confirmDialog}
      <ToggleRow
        label={t("dbdump.toggle")}
        hint={t(hintKey()).replace("{engine}", engineName)}
        checked={shownOn}
        onChange={(next) => void handleChange(next)}
        disabled={busy || forcedOff || forcedOn}
        shakeNonce={shake}
      />
      {lookalike && on && advanced && (
        <SelectField
          label={t("dbdump.engineLabel")}
          value={chosenEngine}
          onChange={(next) => void save(true, next as DbEngine)}
          options={Object.entries(ENGINE_NAMES).map(([value, label]) => ({ value, label }))}
          disabled={busy}
          className="text-xs"
        />
      )}
      {coverageLine && (
        <p className="flex items-center gap-1.5 text-xs text-end">
          <span className={needsTheDump ? "text-statusWarn" : "text-carbon-textMuted"}>{t(coverageLine)}</span>
          <InfoBubble tip={t("dbdump.coverageHint")} />
        </p>
      )}
      {container.dbDumpHookOverlap && (
        <p className="text-xs text-statusWarn text-end">{t("dbdump.hookOverlap")}</p>
      )}
      {last && (
        <p className="flex items-center gap-1.5 text-xs text-carbon-textMuted text-end">
          <span>
            {beforeResult}
            {result.node}
            {afterResult}
          </span>
          {result.remedy && <InfoBubble tip={t(result.remedy)} />}
        </p>
      )}
      {!last && shownOn && (
        <p className="text-xs text-carbon-textMuted text-end">{t("dbdump.noDumpYet")}</p>
      )}
    </div>
  );
}
