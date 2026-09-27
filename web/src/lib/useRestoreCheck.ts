import { useCallback, useEffect, useState } from "react";
import { checkRestore, type CheckLine, type RestoreCheckRequest, type RestoreCheckResponse } from "./api";
import type { TranslationKey } from "./i18n";

export type RestoreCheckState =
  | { phase: "idle" }
  | { phase: "running" }
  | { phase: "done"; result: RestoreCheckResponse }
  | { phase: "error"; error: string };

export interface RestoreCheck {
  state: RestoreCheckState;
  /** True only for a finished check of exactly this request with no red line. */
  ready: boolean;
  recheck: () => void;
}

/**
 * useRestoreCheck checks the restore that req describes, and again whenever it
 * changes. null means the dialog does not describe a whole restore yet. An
 * answer counts only for the request it was asked for, so a changed target or
 * snapshot locks the restore again until its own check is back.
 */
export function useRestoreCheck(req: RestoreCheckRequest | null, delayMs = 400): RestoreCheck {
  const key = req ? JSON.stringify(req) : "";
  const [round, setRound] = useState(0);
  const [answer, setAnswer] = useState<{
    key: string;
    round: number;
    state: RestoreCheckState;
  } | null>(null);

  useEffect(() => {
    if (!key) return;
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      checkRestore(JSON.parse(key) as RestoreCheckRequest, ctrl.signal)
        .then((res) => {
          const state: RestoreCheckState = res.ok
            ? { phase: "done", result: res }
            : { phase: "error", error: res.error ?? "" };
          setAnswer({ key, round, state });
        })
        .catch((e: unknown) => {
          if (ctrl.signal.aborted) return;
          setAnswer({
            key,
            round,
            state: {
              phase: "error",
              error: e instanceof Error ? e.message : String(e),
            },
          });
        });
    }, delayMs);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [key, round, delayMs]);

  const recheck = useCallback(() => setRound((r) => r + 1), []);
  let state: RestoreCheckState = { phase: "idle" };
  if (key) state = answer && answer.key === key && answer.round === round ? answer.state : { phase: "running" };
  const ready = state.phase === "done" && state.result.ready === true;
  return { state, ready, recheck };
}

export const CHECK_LINE_LABEL: Record<CheckLine["id"], TranslationKey> = {
  repository: "restoreCheck.line.repository",
  key: "restoreCheck.line.key",
  snapshot: "restoreCheck.line.snapshot",
  space: "restoreCheck.line.space",
};

/**
 * restoreBlockReason says why Start is locked, for the (i) in the button, or
 * undefined when the check lets the restore start.
 */
export function restoreBlockReason(check: Pick<RestoreCheck, "state">, t: (key: TranslationKey) => string): string | undefined {
  const { state } = check;
  switch (state.phase) {
    case "idle":
      return undefined;
    case "running":
      return t("restoreCheck.waiting");
    case "error":
      return t("restoreCheck.blockedError");
    case "done": {
      const failed = state.result.checks?.find((c) => c.status === "fail");
      if (!failed) return undefined;
      return t("restoreCheck.blockedBy").replace("{line}", t(CHECK_LINE_LABEL[failed.id]));
    }
  }
}

/**
 * checkRestoreOnce asks for one check, for a row action that has no room to
 * show a running one. A failed request becomes the error state.
 */
export async function checkRestoreOnce(req: RestoreCheckRequest): Promise<Pick<RestoreCheck, "state" | "ready">> {
  let state: RestoreCheckState;
  try {
    const res = await checkRestore(req);
    state = res.ok ? { phase: "done", result: res } : { phase: "error", error: res.error ?? "" };
  } catch (e) {
    state = { phase: "error", error: e instanceof Error ? e.message : String(e) };
  }
  return { state, ready: state.phase === "done" && state.result.ready === true };
}
