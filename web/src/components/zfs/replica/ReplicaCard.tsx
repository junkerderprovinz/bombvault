import { useEffect, useRef, useState, type ReactNode } from "react";

import { patchZFSReplica, runZFSReplica } from "../../../lib/api";
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
import { useDebouncedSave } from "../../../lib/useDebouncedSave";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { CadenceBuilder, EXACT_CADENCE_MODES } from "../../CadenceBuilder";
import { InfoBubble } from "../../InfoBubble";
import { Selector } from "../../Selector";
import { IconFleet, IconZFS } from "../../navGlyphs";
import { AddReplicaServerDialog } from "./AddReplicaServerDialog";
import { ReplicaKeepField } from "./ReplicaKeepField";
import { ReplicaStatePill } from "./ReplicaStatePill";
import { isRunning, peerHolds, targetName, unixOf } from "./replicaModel";
import { useGroup, useReplica, useReplicaServers } from "./replicaStore";

type T = ReturnType<typeof useT>["t"];

const NONE = "none";

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

function Row({ label, hint, aside, children }: { label: string; hint?: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {label}
          {hint && <InfoBubble tip={hint} />}
        </span>
        {aside && <span className="ms-auto">{aside}</span>}
      </div>
      {children}
    </div>
  );
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
  const { replica, reload, progressActive, restoreActive } = useReplica(itemId);
  const { servers, reload: reloadServers } = useReplicaServers();
  const group = useGroup();
  const [draft, setDraft] = useState<ZFSReplicaPatch>({});
  const [adding, setAdding] = useState(false);
  const [starting, setStarting] = useState(false);
  const { debouncedSave } = useDebouncedSave();
  const pending = useRef<ZFSReplicaPatch>({});
  const watched = useRef<{ first: boolean; seen: boolean } | null>(null);

  // A fresh answer from the server replaces whatever was shown ahead of it,
  // apart from the edits still waiting to be sent.
  useEffect(() => setDraft({ ...pending.current }), [replica]);

  const running = replica ? isRunning(replica.state, progressActive) : false;
  const shownName = replica ? targetName(replica, servers, group?.members ?? []) : "";

  // A run started here ends in a sentence about it once the card reads the
  // replica again.
  useEffect(() => {
    const w = watched.current;
    if (!w || !replica) return;
    if (running) {
      w.seen = true;
      return;
    }
    if (!w.seen) return;
    watched.current = null;
    if (replica.state !== "ok") return;
    push(
      w.first
        ? t("zfs.replica.firstDone").replace("{name}", () => name).replace("{target}", () => shownName)
        : t("zfs.replica.upToDate").replace("{name}", () => name),
      "success",
    );
  }, [running, replica, name, shownName, push, t]);

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

  // After a revoke the same target goes out as a new request, which the
  // receiving instance asks about again.
  async function askAgain() {
    if (await save({ target: view.target })) {
      push(t("zfs.replica.peer.requested").replaceAll("{peer}", () => shownName), "success");
    }
  }

  async function replicateNow() {
    setStarting(true);
    try {
      const res = await runZFSReplica(itemId);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
        return;
      }
      watched.current = { first: replica?.state !== "ok", seen: false };
      reload();
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setStarting(false);
    }
  }

  const peers = (group?.members ?? []).filter((m) => m.kind !== "android");
  const targets = [
    { id: NONE, label: t("zfs.replica.targetNone") },
    ...servers
      .filter((s) => s.enabled || (view.target.kind === "server" && view.target.id === s.id))
      .map((s) => ({ id: `server:${s.id}`, label: s.name, icon: <IconZFS /> })),
    ...peers.map((m) => ({ id: `peer:${m.id}`, label: m.name || m.id, icon: <IconFleet /> })),
  ];
  const hasTarget = view.target.kind !== "none";
  const peer = view.target.kind === "peer";

  return (
    <div className="relative glim-notch-card flex flex-col gap-4 rounded-card bg-carbon-surface p-4">
      <h3 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("zfs.replica.title")}
          <InfoBubble tip={t("zfs.replica.hint")} onAccent />
        </Badge>
      </h3>

      <Row
        label={t("zfs.replica.target")}
        hint={t("zfs.replica.targetHint")}
        aside={hasTarget && <ReplicaStatePill replica={view} running={running} peerName={shownName} />}
      >
        <div className="flex items-center gap-2 flex-wrap">
          <div className="min-w-0 flex-1">
            <Selector
              label={t("zfs.replica.target")}
              items={targets}
              active={targetId(view.target)}
              onChange={(id) => void pickTarget(id)}
              activation="manual"
            />
          </div>
          <Button
            label={t("zfs.replica.addServer")}
            labelKey="zfs.replica.addServer"
            tone="neutral"
            variant="icon"
            onClick={() => setAdding(true)}
          />
        </div>
      </Row>

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

      {hasTarget && (
        <>
          <Row label={t("zfs.replica.when")} hint={t("zfs.replica.whenHint")}>
            <Selector
              label={t("zfs.replica.when")}
              items={[
                { id: "after", label: t("zfs.replica.afterBackup") },
                { id: "own", label: t("zfs.replica.ownPlan") },
              ]}
              active={view.afterBackup ? "after" : "own"}
              onChange={(id) => change({ afterBackup: id === "after" })}
              activation="manual"
            />
          </Row>
          {!view.afterBackup && (
            <Row label={t("zfs.replica.plan")}>
              <CadenceBuilder
                label={t("zfs.replica.plan")}
                value={view.cadence}
                modes={EXACT_CADENCE_MODES}
                onChange={(cadence) => changeSoon({ cadence })}
              />
            </Row>
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
        </>
      )}

      {hasTarget && (
        <div className="flex flex-col gap-1.5">
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            {t("zfs.replica.members")}
            <InfoBubble tip={t("zfs.replica.membersHint")} />
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

      {hasTarget && !peerHolds(view) && view.state !== "waiting" && (
        <div className="flex justify-end">
          <Button
            label={t("zfs.replica.replicateNow")}
            labelKey="zfs.replica.replicateNow"
            tone="accent"
            onClick={() => void replicateNow()}
            disabled={running || starting || restoreActive}
            busy={running || starting || restoreActive}
          />
        </div>
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
