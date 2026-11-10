import { useState } from "react";
import { useNavigate } from "react-router-dom";

import { leaveSharedRepo, type SharedMembership, type SharedMembershipState } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { Badge, type BadgeTone } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconSignOut } from "../../components/glyphs";
import { IconTabStorage } from "../../components/navGlyphs";
import { Rows, SettingRow } from "../../components/storage/rows";
import { Card } from "../settings/shared";
import { SpaceLine } from "./SharedUpkeepSection";
import type { SharedRepoState } from "./useSharedRepo";

const STATE: Record<SharedMembershipState, { label: TranslationKey; tone: BadgeTone; note?: TranslationKey }> = {
  asked: { label: "shared.member.asked", tone: "warn", note: "shared.member.askedNote" },
  allowed: { label: "shared.member.allowed", tone: "neutral", note: "shared.member.allowedNote" },
  refused: { label: "shared.member.refused", tone: "neutral", note: "shared.member.refusedNote" },
  expired: { label: "shared.member.expired", tone: "neutral", note: "shared.member.expiredNote" },
  joined: { label: "shared.member.joined", tone: "ok" },
  removed: { label: "shared.member.removed", tone: "neutral", note: "shared.member.removedNote" },
};

/** SharedMembershipsCard lists the shared repositories of the group this
 *  instance copies into or asked to join. Their owner prunes them by its own
 *  rules, so this side only reads who does and who else is in. */
export function SharedMembershipsCard({ shared, hueIndex }: { shared: SharedRepoState; hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const navigate = useNavigate();
  const [busy, setBusy] = useState("");
  const named = (key: TranslationKey, name: string) => t(key).replaceAll("{name}", () => name);

  async function leave(m: SharedMembership) {
    if (!(await confirm(named("shared.member.leaveAsk", m.ownerName), { confirmKey: "shared.member.leave" }))) return;
    setBusy(m.ownerId);
    try {
      const r = await leaveSharedRepo(m.ownerId);
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return;
      }
      push(named("shared.member.left", m.ownerName), "success");
      await shared.reload();
    } catch (e) {
      push(e instanceof Error ? e.message : t("settings.error"), "fail");
    } finally {
      setBusy("");
    }
  }

  return (
    <Card title={t("shared.member.title")} hint={t("shared.member.hint")} hueIndex={hueIndex} newId="shared-repo-member">
      {shared.memberships.map((m) => {
        const state = STATE[m.state];
        const inside = m.state === "asked" || m.state === "allowed" || m.state === "joined";
        return (
          <section
            key={m.ownerId}
            data-membership={m.ownerId}
            aria-label={m.ownerName}
            className="flex flex-col gap-3 rounded-card bg-carbon-background p-3 md:p-4"
          >
            <div className="flex flex-wrap items-center gap-2">
              <span className="min-w-0 text-sm font-semibold text-carbon-text wrap-anywhere">
                {named("shared.offer.tile", m.ownerName)}
              </span>
              <Badge tone={state.tone}>{t(state.label)}</Badge>
            </div>
            {state.note && <p className="text-xs text-carbon-textSub">{named(state.note, m.ownerName)}</p>}
            {m.move && (
              <div className="flex flex-col gap-1">
                <p className="text-sm text-statusWarn">{named("shared.member.moveHeld", m.move.holder)}</p>
                <p className="text-xs text-carbon-textSub wrap-break-word">{m.move.reason}</p>
              </div>
            )}
            {m.state === "joined" && (
              <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                <p className="min-w-0 flex-[1_1_16rem] text-sm text-carbon-text">{t("shared.member.switchOn")}</p>
                <Button
                  label={t("shared.member.openStorage")}
                  labelKey="shared.member.openStorage"
                  glyph={<IconTabStorage />}
                  tone="accent"
                  onClick={() => navigate("/storage")}
                />
              </div>
            )}
            {m.state === "joined" && (
              <div data-new="shared-member-space">
                <SpaceLine t={t} space={m.space} />
              </div>
            )}
            <Rows>
              {m.repo && (
                <SettingRow label={t("receiver.server.address")}>
                  <bdi dir="ltr" className="font-mono text-xs text-carbon-text break-all text-start">
                    {m.repo}
                  </bdi>
                </SettingRow>
              )}
              {m.move?.repo && (
                <SettingRow label={t("shared.member.moveAddress")}>
                  <bdi dir="ltr" className="font-mono text-xs text-carbon-text break-all text-start">
                    {m.move.repo}
                  </bdi>
                </SettingRow>
              )}
              <SettingRow label={t("shared.member.prunes")}>
                <span className="text-sm text-carbon-text text-end">{named("shared.member.prunesValue", m.ownerName)}</span>
              </SettingRow>
              {m.state === "joined" && (
                <SettingRow label={t("shared.member.window")} hint={named("shared.member.windowTip", m.ownerName)}>
                  <span className="text-sm text-carbon-text text-end">
                    {m.nextWindow ? formatTs(m.nextWindow) : t("shared.member.windowUnknown")}
                  </span>
                </SettingRow>
              )}
              {m.state === "joined" && (
                <SettingRow
                  label={t("shared.member.lastCopy")}
                  note={m.deferredAt ? <span className="text-statusWarn">{named("shared.member.copyWaited", m.ownerName)}</span> : undefined}
                >
                  <span className="text-sm text-carbon-text text-end">
                    {m.lastCopyAt ? relativeTime(t, m.lastCopyAt) : t("shared.member.noCopy")}
                  </span>
                </SettingRow>
              )}
              <SettingRow label={t("shared.member.together")}>
                <span className="text-sm text-carbon-text text-end wrap-anywhere">
                  {m.together.length > 0 ? m.together.map((o) => o.name).join(", ") : t("shared.member.alone")}
                </span>
              </SettingRow>
            </Rows>
            {inside && (
              <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
                <p className="me-auto min-w-0 flex-[1_1_16rem] text-xs text-carbon-textMuted">{t("shared.member.readable")}</p>
                <Button
                  label={t("shared.member.leave")}
                  labelKey="shared.member.leave"
                  glyph={<IconSignOut />}
                  tone="neutral"
                  onClick={() => void leave(m)}
                  disabled={busy !== ""}
                  busy={busy === m.ownerId}
                />
              </div>
            )}
          </section>
        );
      })}
      {confirmDialog}
    </Card>
  );
}
