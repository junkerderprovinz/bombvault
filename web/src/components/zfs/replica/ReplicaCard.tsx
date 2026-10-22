import { useEffect, useRef, useState } from "react";

import { patchZFSReplica, setZFSReplicaOnly, takeOverZFSReplicaFolder } from "../../../lib/api";
import type {
  ZFSPeerState,
  ZFSReplica,
  ZFSReplicaMember,
  ZFSReplicaPatch,
  ZFSReplicaTarget,
} from "../../../lib/api";
import { humanBytes } from "../../../lib/forecast";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { formatDuration, relativeTime } from "../../../lib/reltime";
import { useConfirm } from "../../../lib/useConfirm";
import { useDebouncedSave } from "../../../lib/useDebouncedSave";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { CadenceBuilder, EXACT_CADENCE_MODES } from "../../CadenceBuilder";
import { InfoBubble } from "../../InfoBubble";
import { SelectField } from "../../SelectField";
import { Selector } from "../../Selector";
import { Toggle } from "../../Toggle";
import { Rows, SettingRow } from "../../storage/rows";
import { IconFleet, IconZFS } from "../../navGlyphs";
import { AddReplicaServerDialog } from "./AddReplicaServerDialog";
import { ReplicaKeepField } from "./ReplicaKeepField";
import { ReplicaStatePill } from "./ReplicaStatePill";
import { unixOf } from "./replicaModel";
import { useGroup, useReplicaServers } from "./replicaStore";
import { useReplicateNow } from "./useReplicateNow";

type T = ReturnType<typeof useT>["t"];

const NONE = "none";

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

const PEER_SENTENCE = {
  asked: "zfs.replica.peer.askedText",
  allowed: "zfs.replica.peer.allowedText",
  refused: "zfs.replica.peer.refusedText",
  revoked: "zfs.replica.peer.revokedText",
  off: "zfs.replica.peer.offText",
} as const satisfies Record<ZFSPeerState, TranslationKey>;

function targetId(target: ZFSReplicaTarget): string {
  return target.kind === "none" ? NONE : `${target.kind}:${target.id}`;
}

function parseTarget(id: string): ZFSReplicaTarget {
  if (id === NONE) return { kind: "none", id: "" };
  const at = id.indexOf(":");
  return { kind: id.slice(0, at) as ZFSReplicaTarget["kind"], id: id.slice(at + 1) };
}

function MemberState({ member, replica, running, t }: { member: ZFSReplicaMember; replica: ZFSReplica; running: boolean; t: T }) {
  if (running) return <Badge tone="active">{t("zfs.replica.state.running")}</Badge>;
  switch (member.state) {
    case "ok":
      return <Badge tone="ok">{t("zfs.replica.member.ok")}</Badge>;
    case "failed":
      return (
        <Badge tone="fail" wrap>
          {zfsCodeSentence(t, member.code || replica.code)}
        </Badge>
      );
    case "waiting":
      return <Badge tone="neutral">{t("zfs.replica.member.waiting")}</Badge>;
    default:
      return <Badge tone="neutral">{t("zfs.replica.member.never")}</Badge>;
  }
}

/** ReplicaCard is where a ZFS item chooses its replica: the target, when it
 *  runs, what the target keeps, and how each dataset of the tree stands. */
export function ReplicaCard({ itemId, name }: { itemId: string; name: string }) {
  const { t } = useT();
  const { push } = useToast();
  const { replica, reload, running, shownName } = useReplicateNow(itemId, name);
  const { servers, reload: reloadServers } = useReplicaServers();
  const group = useGroup();
  const { confirm, confirmDialog } = useConfirm();
  const [draft, setDraft] = useState<ZFSReplicaPatch>({});
  const [adding, setAdding] = useState(false);
  const { debouncedSave } = useDebouncedSave();
  const pending = useRef<ZFSReplicaPatch>({});

  // A fresh answer from the server replaces whatever was shown ahead of it,
  // apart from the edits still waiting to be sent.
  useEffect(() => setDraft({ ...pending.current }), [replica]);

  if (!replica) return null;
  const view: ZFSReplica = { ...replica, ...draft };

  async function save(patch: ZFSReplicaPatch): Promise<boolean> {
    try {
      const res = await patchZFSReplica(itemId, patch);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
        setDraft({});
      }
      return res.ok;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setDraft({});
      return false;
    } finally {
      // A refusal can come after part of the change is stored, such as a
      // new target whose paired instance did not answer.
      reload();
    }
  }

  function change(patch: ZFSReplicaPatch) {
    setDraft((d) => ({ ...d, ...patch }));
    void save(patch);
  }

  // Edits made while typing go out together once it stops, so one field
  // never drops the other's pending save.
  function changeSoon(patch: ZFSReplicaPatch) {
    setDraft((d) => ({ ...d, ...patch }));
    pending.current = { ...pending.current, ...patch };
    debouncedSave(() => {
      const queued = pending.current;
      pending.current = {};
      void save(queued);
    });
  }

  async function pickTarget(id: string) {
    const target = parseTarget(id);
    if (targetId(target) === targetId(view.target)) return;
    setDraft((d) => ({ ...d, target }));
    if (!(await save({ target }))) return;
    if (target.kind === "peer") {
      const peer = group?.members.find((m) => m.id === target.id)?.name || target.id;
      push(t("zfs.replica.peer.requested").replaceAll("{peer}", () => peer), "success");
    } else if (target.kind === "server") {
      const server = servers.find((s) => s.id === target.id)?.name ?? target.id;
      push(
        t("zfs.replica.serverPicked").replace("{name}", () => name).replace("{target}", () => server),
        "success",
      );
    }
  }

  // While the ZFS section takes no backups a run only sends the replica.
  async function setReplicaOnly(on: boolean) {
    try {
      const res = await setZFSReplicaOnly(on);
      if (!res.ok) throw new Error(res.error ?? t("settings.error"));
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      reload();
    }
  }

  // The folder carries the instance id of an earlier install of this
  // BombVault, which a fresh database does not have any more.
  async function takeOver() {
    if (!(await confirm(t("zfs.replica.takeOverConfirm").replace("{server}", () => shownName), { confirmKey: "zfs.replica.takeOver" }))) {
      return;
    }
    try {
      const res = await takeOverZFSReplicaFolder(itemId);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
        return;
      }
      push(t("zfs.replica.takenOver").replace("{server}", () => shownName), "success");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      reload();
    }
  }

  // After a revoke the same target goes out as a new request, which the
  // receiving instance asks about again.
  async function askAgain() {
    if (await save({ target: view.target })) {
      push(t("zfs.replica.peer.requested").replaceAll("{peer}", () => shownName), "success");
    }
  }

  const peers = (group?.members ?? []).filter((m) => m.kind !== "android");
  const targets = [
    { value: NONE, label: t("zfs.replica.targetNone") },
    ...servers
      .filter((s) => s.enabled || (view.target.kind === "server" && view.target.id === s.id))
      .map((s) => ({ value: `server:${s.id}`, label: t("zfs.replica.targetOption.server").replace("{name}", () => s.name), glyph: <IconZFS /> })),
    ...peers.map((m) => ({
      value: `peer:${m.id}`,
      label: t("zfs.replica.targetOption.peer").replace("{name}", () => m.name || m.id),
      glyph: <IconFleet />,
    })),
  ];
  const hasTarget = view.target.kind !== "none";
  const peer = view.target.kind === "peer";

  return (
    <div className="relative glim-notch-card flex flex-col gap-4 rounded-card bg-carbon-surface p-4">
      {confirmDialog}
      <h3 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("zfs.replica.title")}
          <InfoBubble tip={t("zfs.replica.hint")} onAccent />
        </Badge>
      </h3>

      <Rows>
        <SettingRow
          label={t("zfs.replica.target")}
          hint={`${t("zfs.replica.targetHint")} ${t("zfs.replica.targetKindHint")}`}
        >
          {hasTarget && <ReplicaStatePill replica={view} running={running} peerName={shownName} />}
          <SelectField
            label={t("zfs.replica.target")}
            value={targetId(view.target)}
            options={targets}
            onChange={(id) => void pickTarget(id)}
            className={FIELD}
          />
          <Button
            label={t("zfs.replica.addServer")}
            labelKey="zfs.replica.addServer"
            tone="neutral"
            variant="icon"
            className="glim-btn-wrap"
            onClick={() => setAdding(true)}
          />
        </SettingRow>
      </Rows>

      {peer && view.peerState !== "" && (
        <div className="flex items-center gap-3 flex-wrap">
          <p className="flex-1 min-w-0 text-xs text-carbon-textSub">
            {t(PEER_SENTENCE[view.peerState]).replaceAll("{peer}", () => shownName)}
          </p>
          {view.peerState === "revoked" && (
            <Button
              label={t("zfs.replica.peer.askAgain")}
              labelKey="zfs.replica.peer.askAgain"
              tone="neutral"
              onClick={() => void askAgain()}
            />
          )}
        </div>
      )}

      {view.target.kind === "server" && view.state === "failed" && view.code === "target-owned" && (
        <div className="flex items-center gap-3 flex-wrap">
          <p className="flex-1 min-w-0 text-xs text-carbon-textSub">
            {`${t("zfs.code.target-owned")} ${t("zfs.replica.takeOverText")}`}
          </p>
          <Button label={t("zfs.replica.takeOver")} labelKey="zfs.replica.takeOver" tone="neutral" onClick={() => void takeOver()} />
        </div>
      )}

      {hasTarget && (
        <Rows>
          <SettingRow label={t("zfs.replica.onlyToggle")} hint={t("zfs.replica.onlyToggleHint")}>
            <span data-new="zfs-replica-only-switch" className="flex">
              <Toggle
                hideLabel
                label={t("zfs.replica.onlyToggle")}
                checked={Boolean(view.only)}
                onChange={(on) => void setReplicaOnly(on)}
              />
            </span>
          </SettingRow>
          <SettingRow
            label={t("zfs.replica.when")}
            hint={view.only ? `${t("zfs.replica.whenHint")} ${t("zfs.replica.onlyHint")}` : t("zfs.replica.whenHint")}
          >
            <Selector
              label={t("zfs.replica.when")}
              items={[
                { id: "after", label: t("zfs.replica.afterBackup") },
                { id: "own", label: t("zfs.replica.ownPlan") },
              ]}
              active={view.afterBackup ? "after" : "own"}
              onChange={(id) => change({ afterBackup: id === "after" })}
              activation="manual"
              inline
            />
          </SettingRow>
          {!view.afterBackup && (
            <div className="flex flex-col gap-1.5 py-3 first:pt-0 last:pb-0">
              <span className="text-sm text-carbon-text">{t("zfs.replica.plan")}</span>
              <CadenceBuilder
                label={t("zfs.replica.plan")}
                value={view.cadence}
                modes={EXACT_CADENCE_MODES}
                onChange={(cadence) => changeSoon({ cadence })}
              />
            </div>
          )}
          {/* A paired instance runs its own retention; until it has
              allowed the replica, the rule here is what it is offered. */}
          {(!peer || view.peerState !== "allowed") && (
            <ReplicaKeepField
              label={peer ? t("zfs.replica.peer.keep").replaceAll("{peer}", () => shownName) : t("zfs.replica.keepTarget")}
              hint={peer ? t("zfs.replica.peer.keepHint").replaceAll("{peer}", () => shownName) : t("zfs.replica.keepTargetHint")}
              keep={view.keep}
              onChange={(keep) => changeSoon({ keep })}
            />
          )}
        </Rows>
      )}

      {hasTarget && (
        <div className="flex flex-col gap-1.5">
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            {t("zfs.replica.members")}
            <InfoBubble
              tip={t(
                servers.find((s) => s.id === view.target.id)?.mountReadOnly
                  ? "zfs.replica.membersMountedHint"
                  : "zfs.replica.membersHint",
              )}
            />
          </span>
          <ul aria-label={t("zfs.replica.members")} className="flex flex-col gap-2">
            {view.members.map((m) => (
              <li key={m.dataset} className="flex items-start gap-2 flex-wrap">
                <div className="flex-1 min-w-0 flex flex-col gap-0.5">
                  <span className="flex items-center gap-1.5">
                    <span dir="ltr" className="font-mono text-xs text-carbon-text text-start wrap-anywhere">
                      {m.dataset}
                    </span>
                    {m.volume && (
                      <Badge tone="neutral" size="small">
                        {t("zfs.replica.volume")}
                      </Badge>
                    )}
                  </span>
                  <span className="text-caption text-carbon-textMuted">
                    {/* A paired instance picks its own pool and root, so
                        the path this side knows is not where the copy is. */}
                    {!peer && <bdi dir="ltr">{t("zfs.replica.memberPath").replace("{path}", () => m.targetPath)}</bdi>}
                    {m.state === "ok" && m.lastBytes > 0 && (
                      <>
                        {!peer && " · "}
                        <span className="glim-num">
                          {t("zfs.replica.memberAdded").replace("{size}", () => humanBytes(m.lastBytes))}
                        </span>
                      </>
                    )}
                    {m.mount && (
                      <>
                        {(!peer || (m.state === "ok" && m.lastBytes > 0)) && " · "}
                        {t(`zfs.replica.mount.${m.mount}`)}
                      </>
                    )}
                  </span>
                </div>
                <MemberState member={m} replica={view} running={running} t={t} />
              </li>
            ))}
          </ul>
        </div>
      )}

      {hasTarget && view.state === "ok" && (
        <p className="text-xs text-carbon-textMuted">
          {t("zfs.replica.lastRun")
            .replace("{when}", () => relativeTime(t, unixOf(view.lastRun)))
            .replace("{sent}", () => humanBytes(view.lastBytes))
            .replace("{took}", () => formatDuration(view.lastSeconds))}
        </p>
      )}

      {adding && (
        <AddReplicaServerDialog
          onClose={() => setAdding(false)}
          onSaved={() => {
            setAdding(false);
            reloadServers();
          }}
        />
      )}
    </div>
  );
}
