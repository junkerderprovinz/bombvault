// KeyDepositWindow is this server's side of the key deposits: where its own
// recovery code stands and which instances keep its deposit, and what it
// keeps for others, with the requests to hand that out.
import { useState } from "react";

import { Badge, type BadgeTone } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconKey, IconRefresh } from "../../components/glyphs";
import { unixOf } from "../../components/zfs/replica/replicaModel";
import { askKeeper, newDepositCode, renewDeposits } from "../../lib/api";
import type { DepositState, Deposits } from "../../lib/api";
import { depositSentence } from "../../lib/depositCodes";
import { useT, type TranslationKey } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { hueCounter } from "../settings/shared";
import { KeptDeposits, ReleaseRequests } from "./DepositRows";
import { InstanceWindow, WindowCard } from "./InstanceWindow";
import { RecoveryCodeDialog } from "./KeeperDialogs";
import { depositStateLabel } from "./RoleTile";
import type { InstancesState } from "./useInstances";

type Sent = Deposits["sent"][number];

const STATE_TONE: Partial<Record<string, BadgeTone>> = { held: "ok", declined: "neutral" };

const DEPOSIT_HELP: Partial<Record<DepositState, TranslationKey>> = {
  stale: "keeper.staleText",
  declined: "keeper.declinedText",
};

/** depositHelp is what a deposit that is not current needs, in a sentence.
 *  A current deposit has none, and neither has one in a state this build
 *  does not know. */
export function depositHelp(state: string): TranslationKey | undefined {
  return state in DEPOSIT_HELP ? DEPOSIT_HELP[state as DepositState] : undefined;
}

export function KeyDepositWindow({ state, onClose }: { state: InstancesState; onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState("");
  const [code, setCode] = useState<string | null>(null);
  const { codeSince, sent, kept, releases } = state.deposits;
  const nextHue = hueCounter();
  const waiting = releases.some((r) => r.state === "asked");

  /** Sends one call and shows the recovery code if it made one. An answer
   *  that is ok carries the code in the field a refusal names its reason in. */
  async function run(key: string, call: () => Promise<{ ok: boolean; code?: string; error?: string }>) {
    setBusy(key);
    try {
      const res = await call();
      if (!res.ok) push(depositSentence(t, res), "fail");
      else if (res.code) setCode(res.code);
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    } finally {
      setBusy("");
      await state.reloadDeposits();
    }
  }

  async function makeCode() {
    if (await confirm(t("keeper.own.newCodeAsk"), { confirmKey: "keeper.own.newCode" })) await run("code", newDepositCode);
  }

  function fix(deposit: Sent) {
    if (deposit.state === "held") return null;
    if (deposit.state === "declined") {
      return (
        <Button
          label={t("keeper.askAgain")}
          labelKey="keeper.askAgain"
          glyph={<IconKey />}
          tone="neutral"
          onClick={() => void run(deposit.memberId, () => askKeeper(deposit.memberId))}
          disabled={busy !== ""}
          busy={busy === deposit.memberId}
        />
      );
    }
    // Sending again is right for every other deposit that is not current, one
    // in a state this build does not know included.
    return (
      <Button
        label={t("keeper.sendNow")}
        labelKey="keeper.sendNow"
        glyph={<IconRefresh />}
        tone="neutral"
        onClick={() => void run(deposit.memberId, renewDeposits)}
        disabled={busy !== ""}
        busy={busy === deposit.memberId}
      />
    );
  }

  return (
    <InstanceWindow
      title={t("keeper.window.title")}
      hint={t("keeper.window.tip")}
      onClose={onClose}
      footer={<Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={onClose} />}
    >
      <WindowCard title={t("keeper.own.title")} hueIndex={nextHue()}>
        {codeSince === "" ? (
          <p className="text-sm text-carbon-textSub">{t("keeper.own.none")}</p>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
            <span data-testid="code-since" className="text-sm text-carbon-text">
              {t("keeper.own.codeSince").replace("{date}", () => new Date(unixOf(codeSince) * 1000).toLocaleDateString())}
            </span>
            <Button
              label={t("keeper.own.newCode")}
              labelKey="keeper.own.newCode"
              glyph={<IconKey />}
              tone="neutral"
              onClick={() => void makeCode()}
              disabled={busy !== ""}
              busy={busy === "code"}
            />
          </div>
        )}
        {sent.length > 0 && (
          <ul className="flex flex-col gap-2">
            {sent.map((d) => {
              const name = d.name || d.memberId;
              const help = depositHelp(d.state);
              return (
                <li
                  key={d.memberId}
                  data-keeper={d.memberId}
                  className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-background p-3"
                >
                  <div className="flex min-w-0 flex-[1_1_14rem] flex-col gap-0.5">
                    <span className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
                      <span className="min-w-0 wrap-anywhere">{name}</span>
                      <Badge tone={STATE_TONE[d.state] ?? "warn"}>{depositStateLabel(t, d.state)}</Badge>
                    </span>
                    <span className="text-xs text-carbon-textMuted wrap-anywhere">
                      {t("keeper.sent").replace("{when}", () => relativeTime(t, unixOf(d.sentAt)))}
                      {help && ` · ${t(help).replaceAll("{peer}", () => name)}`}
                    </span>
                  </div>
                  <span className="ms-auto">{fix(d)}</span>
                </li>
              );
            })}
          </ul>
        )}
      </WindowCard>

      <WindowCard title={t("keeper.kept.title")} hueIndex={nextHue()}>
        {kept.length === 0 && !waiting && <p className="text-sm text-carbon-textSub">{t("keeper.kept.none")}</p>}
        <ReleaseRequests releases={releases} state={state} />
        <KeptDeposits kept={kept} state={state} />
      </WindowCard>

      {confirmDialog}
      {code && <RecoveryCodeDialog code={code} onDone={() => setCode(null)} />}
    </InstanceWindow>
  );
}
