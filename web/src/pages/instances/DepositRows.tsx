// What a key keeper's person decides: whether a deposit this server keeps is
// handed out, and whether it is kept at all. The rows have the form of the
// other role requests, so they read as one list with them.
import { useState } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconCheck, IconTrash } from "../../components/navGlyphs";
import { unixOf } from "../../components/zfs/replica/replicaModel";
import { decideRelease, dropKeptDeposit } from "../../lib/api";
import type { DepositRelease, Deposits } from "../../lib/api";
import { depositSentence } from "../../lib/depositCodes";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { ROLES } from "./RoleTile";
import type { InstancesState } from "./useInstances";

type Kept = Deposits["kept"][number];

const ROW = "flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-background p-3";
const MARK = "grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface3 text-accentText";

function ago(t: ReturnType<typeof useT>["t"], stamp: string): string {
  return relativeTime(t, unixOf(stamp));
}

/** ReleaseRequests lists the requests to hand a deposit out that wait for an
 *  answer. Anyone with the twelve words can send one, so each row says who
 *  asks, whether this instance has seen that instance before and how often it
 *  asked. */
export function ReleaseRequests({ releases, state }: { releases: DepositRelease[]; state: InstancesState }) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState("");
  const ownLogin = useOwnLogin();
  const waiting = releases.filter((r) => r.state === "asked");

  async function decide(release: DepositRelease, decision: "allow" | "refuse") {
    setBusy(`${release.id}:${decision}`);
    try {
      const res = await decideRelease(release.id, decision);
      if (!res.ok) {
        if (!ownLogin.handled(res)) push(depositSentence(t, res), "fail");
      } else {
        const done = decision === "allow" ? "keeper.release.allowed" : "keeper.release.refused";
        push(t(done).replaceAll("{requester}", () => release.requesterName || release.requesterId), "success");
      }
      await state.reloadDeposits();
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      {waiting.map((r) => {
        const requester = r.requesterName || r.requesterId;
        return (
          <div key={r.id} data-release={r.id} className={ROW}>
            <span className={MARK}>{ROLES.keeper.glyph}</span>
            <div className="flex min-w-0 flex-[1_1_14rem] flex-col gap-1">
              <span className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
                {t(ROLES.keeper.label)}
                <Badge tone="warn">{t("instances.request")}</Badge>
              </span>
              <span className="text-xs text-carbon-text wrap-anywhere">
                {t("keeper.release.asks")
                  .replace("{requester}", () => requester)
                  .replace("{owner}", () => r.ownerName || r.ownerId)}
              </span>
              <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-carbon-textMuted">
                {r.knownSince ? (
                  <span>{t("keeper.release.known").replace("{date}", () => new Date(unixOf(r.knownSince) * 1000).toLocaleDateString())}</span>
                ) : (
                  <Badge tone="warn">{t("keeper.release.unknown")}</Badge>
                )}
                <span>
                  {t("keeper.release.count", r.requests)}
                  {", "}
                  {t("keeper.release.lastHour").replace("{n}", String(r.requestsLastHour))}
                </span>
              </span>
              <span className="text-xs text-carbon-textMuted">{t("keeper.release.allowHint")}</span>
            </div>
            <div className="ms-auto flex flex-wrap items-center justify-end gap-2">
              <Button
                label={t("zfs.receive.decline")}
                labelKey="zfs.receive.decline"
                tone="neutral"
                onClick={() => void decide(r, "refuse")}
                disabled={busy !== ""}
                busy={busy === `${r.id}:refuse`}
              />
              <Button
                label={t("zfs.receive.allow")}
                labelKey="zfs.receive.allow"
                glyph={<IconCheck />}
                tone="accent"
                onClick={() => void decide(r, "allow")}
                disabled={busy !== ""}
                busy={busy === `${r.id}:allow`}
              />
            </div>
          </div>
        );
      })}
      {ownLogin.dialog}
    </>
  );
}

/** KeptDeposits lists what this server keeps for others, each with the way
 *  to delete it. A deposit deleted here stays in the list, since the owner
 *  has to ask again before this server keeps a new one. */
export function KeptDeposits({ kept, state }: { kept: Kept[]; state: InstancesState }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState("");

  async function drop(deposit: Kept) {
    const owner = deposit.ownerName || deposit.ownerId;
    const question = t("keeper.kept.deleteAsk").replaceAll("{owner}", () => owner);
    if (!(await confirm(question, { confirmKey: "common.delete", confirmTone: "neutral" }))) return;
    setBusy(deposit.ownerId);
    try {
      const res = await dropKeptDeposit(deposit.ownerId);
      if (res.ok) push(t("keeper.kept.deleted"), "success");
      else push(depositSentence(t, res), "fail");
      await state.reloadDeposits();
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      {kept.map((d) => {
        const owner = d.ownerName || d.ownerId;
        const held = d.state !== "declined";
        return (
          <div key={d.ownerId} data-kept={d.ownerId} className={ROW}>
            <span className={MARK}>{ROLES.keeper.glyph}</span>
            <div className="flex min-w-0 flex-[1_1_14rem] flex-col gap-0.5">
              <span className="text-sm font-semibold text-carbon-text wrap-anywhere">
                {t("keeper.kept.of").replace("{owner}", () => owner)}
              </span>
              <span className="text-xs text-carbon-textMuted wrap-anywhere">
                {held
                  ? `${t("keeper.kept.since").replace("{when}", () => ago(t, d.createdAt))} · ${t("keeper.kept.renewed").replace("{when}", () => ago(t, d.updatedAt))}`
                  : t("keeper.kept.deletedHere").replaceAll("{owner}", () => owner)}
              </span>
            </div>
            {held && (
              <Button
                label={t("common.delete")}
                labelKey="common.delete"
                glyph={<IconTrash />}
                tone="neutral"
                onClick={() => void drop(d)}
                disabled={busy !== ""}
                busy={busy === d.ownerId}
                className="ms-auto"
              />
            )}
          </div>
        );
      })}
      {confirmDialog}
    </>
  );
}
