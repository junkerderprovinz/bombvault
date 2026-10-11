import type { ReactNode } from "react";

import { Button } from "../../components/Button";
import { IconRefresh } from "../../components/glyphs";
import { IconCheck, IconClose } from "../../components/navGlyphs";
import { ProgressBar } from "../../components/ProgressBar";
import type { Run } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { runTargetText } from "../../lib/runDisplay";
import { InstanceWindow } from "../instances/InstanceWindow";
import { passShare, type LiveRun, type Pass } from "./headState";
import { liveRunName } from "./StatusHead";

type T = ReturnType<typeof useT>["t"];

function Line({ mark, tone, children }: { mark: ReactNode; tone: string; children: ReactNode }) {
  return (
    <li className="flex items-center gap-2.5 py-0.5 text-xs">
      <span aria-hidden="true" className={`flex w-4 flex-none justify-center [&_svg]:h-3.5 [&_svg]:w-3.5 ${tone}`}>{mark}</span>
      <span className="flex min-w-0 flex-1 items-baseline gap-2">{children}</span>
    </li>
  );
}

/**
 * PassWindow follows a pass over everything: how far it is, the entries it
 * has finished and how each ended, the one it is on, and how many still wait.
 */
export function PassWindow({
  t,
  pass,
  run,
  done,
  onClose,
}: {
  t: T;
  pass: Pass;
  /** The entry being backed up now. */
  run: LiveRun | null;
  /** The finished entries of the pass, oldest first. */
  done: Run[];
  onClose: () => void;
}) {
  const waiting = Math.max(0, pass.total - pass.done - (run?.kind ? 1 : 0));
  return (
    <InstanceWindow
      title={t("overview.pass.title")}
      facts={t("overview.pass.count").replace("{done}", String(pass.done)).replace("{total}", String(pass.total))}
      footer={<Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="subtle" onClick={onClose} />}
      onClose={onClose}
    >
      <div className="flex flex-col gap-4">
        <ProgressBar percent={passShare(pass, run)} active inline />
        <ul data-testid="pass-entries" className="flex flex-col rounded-card bg-carbon-background px-3 py-2 font-mono">
          {done.map((entry) => (
            <Line
              key={entry.id}
              mark={entry.status === "failed" ? <IconClose /> : <IconCheck />}
              tone={entry.status === "failed" ? "text-statusFail" : "text-statusOk"}
            >
              <span className="truncate font-semibold text-carbon-text">{runTargetText(t, entry)}</span>
            </Line>
          ))}
          {run?.kind && (
            <Line mark={<IconRefresh />} tone="text-accentText">
              <span className="truncate font-semibold text-carbon-text">{liveRunName(run, t)}</span>
              <span className="tabular-nums text-carbon-textMuted">{Math.round(run.percent)} %</span>
            </Line>
          )}
          {waiting > 0 && (
            <Line mark="·" tone="text-carbon-textMuted">
              <span className="text-carbon-textMuted">{t("overview.pass.waiting").replace("{n}", String(waiting))}</span>
            </Line>
          )}
        </ul>
      </div>
    </InstanceWindow>
  );
}
