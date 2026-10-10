import { useState } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconCheck } from "../../components/navGlyphs";
import { unixOf } from "../../components/zfs/replica/replicaModel";
import { decideRole } from "../../lib/api";
import type { RequestedRole, RoleDecision, RoleRequest } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { domainLabelKey } from "./PeerScorecard";
import { ReceiverServerWindow } from "./ReceiverServerWindow";
import { ROLES } from "./RoleTile";
import type { Instance } from "./instancesModel";
import type { InstancesState } from "./useInstances";

const REQUESTED: readonly RequestedRole[] = ["receiver", "fetcher"];

const ASK_TEXT: Record<RequestedRole, TranslationKey> = {
  receiver: "instances.ask.receiver",
  fetcher: "instances.ask.fetcher",
};

const REVOKE_TEXT: Record<RequestedRole, TranslationKey> = {
  receiver: "instances.grant.revokeReceiver",
  fetcher: "instances.grant.revokeFetcher",
};

/** RoleRequests is what an instance asked this server to be for it: the
 *  Receiver and Fetcher requests that wait for an answer, or the ones that
 *  were allowed and can be taken back. It draws one of the two parts, so the
 *  ZFS requests can stand between them in the same list. */
export function RoleRequests({
  instance,
  part,
  state,
  onFetcherAllowed,
}: {
  instance: Instance;
  part: "asked" | "allowed";
  state: InstancesState;
  /** Allowing a Fetcher leaves this server with nothing to fetch yet. */
  onFetcherAllowed: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState("");
  const [needsServer, setNeedsServer] = useState<RoleRequest | null>(null);
  const named = (key: TranslationKey) => t(key).replaceAll("{peer}", () => instance.name);
  const side = instance.roles?.forMember;
  const shown = REQUESTED.flatMap((role) => {
    const request = side?.[role];
    return request?.state === part ? [{ role, request }] : [];
  });

  async function decide(role: RequestedRole, request: RoleRequest, decision: RoleDecision, done: string) {
    setBusy(`${request.id}:${decision.decision}`);
    try {
      const res = await decideRole(request.id, decision);
      if (res.code === "no-receiver") {
        setNeedsServer(request);
        return;
      }
      if (res.code === "request-changed") push(named("instances.ask.changed"), "warn");
      else if (!res.ok) push(res.error ?? t("instances.role.saveError"), "fail");
      else push(done, "success");
      await state.reloadRoles();
      if (res.ok && decision.decision === "allow" && role === "fetcher") onFetcherAllowed();
    } catch (err) {
      push(err instanceof Error ? err.message : t("instances.role.saveError"), "fail");
    } finally {
      setBusy("");
    }
  }

  const allow = (role: RequestedRole, request: RoleRequest) =>
    decide(role, request, { decision: "allow", sections: request.sections }, named("instances.ask.allowed"));

  async function revoke(role: RequestedRole, request: RoleRequest) {
    const question = named("zfs.receive.revokeQuestion") + " " + named(REVOKE_TEXT[role]);
    if (!(await confirm(question, { confirmKey: "zfs.receive.revokeConfirm" }))) return;
    await decide(role, request, { decision: "revoke" }, t("zfs.receive.revoked"));
  }

  const sections = (request: RoleRequest) => request.sections.map((s) => t(domainLabelKey(s))).join(", ");

  return (
    <>
      {shown.map(({ role, request }) => (
        <div key={role} data-request={role} className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-background p-3">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-control bg-carbon-surface3 text-accentText">
            {ROLES[role].glyph}
          </span>
          <div className="flex min-w-0 flex-[1_1_14rem] flex-col gap-0.5">
            <span className="flex flex-wrap items-center gap-2 text-sm font-semibold text-carbon-text">
              {t(ROLES[role].label)}
              {part === "asked" && <Badge tone="warn">{t("instances.request")}</Badge>}
            </span>
            <span className="text-xs text-carbon-textMuted wrap-anywhere">
              {part === "asked"
                ? named(ASK_TEXT[role]).replace("{types}", () => sections(request))
                : `${t("instances.grant.allowed").replace("{when}", () => relativeTime(t, unixOf(request.decidedAt)))} · ${sections(request)}`}
            </span>
          </div>
          <div className="ms-auto flex flex-wrap items-center justify-end gap-2">
            {part === "asked" ? (
              <>
                <Button
                  label={t("zfs.receive.decline")}
                  labelKey="zfs.receive.decline"
                  tone="neutral"
                  onClick={() => void decide(role, request, { decision: "refuse" }, named("zfs.receive.declined"))}
                  disabled={busy !== ""}
                  busy={busy === `${request.id}:refuse`}
                />
                <Button
                  label={t("zfs.receive.allow")}
                  labelKey="zfs.receive.allow"
                  glyph={<IconCheck />}
                  tone="accent"
                  onClick={() => void allow(role, request)}
                  disabled={busy !== ""}
                  busy={busy === `${request.id}:allow`}
                />
              </>
            ) : (
              <Button
                label={t("zfs.receive.revoke")}
                labelKey="zfs.receive.revoke"
                tone="neutral"
                onClick={() => void revoke(role, request)}
                disabled={busy !== ""}
                busy={busy === `${request.id}:revoke`}
              />
            )}
          </div>
        </div>
      ))}
      {confirmDialog}
      {needsServer && (
        <ReceiverServerWindow
          t={t}
          lead={t("receiver.server.neededLead")}
          onClose={(running) => {
            const request = needsServer;
            setNeedsServer(null);
            void state.reloadServer();
            if (running) void allow("receiver", request);
          }}
        />
      )}
    </>
  );
}
